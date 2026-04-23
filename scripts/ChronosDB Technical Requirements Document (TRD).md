## **Technical Requirements Document (TRD)**

| Document Title | ChronosDB: Predictive Temporal Graph Database – Technical Requirements |
| :---- | :---- |
| Version | 1.0 |
| Date | 2025-03-11 |
| Status | Draft |
| Authors | \[Engineering Team\] |

---

### **1\. Introduction**

This Technical Requirements Document (TRD) defines the architecture, components, interfaces, and technical specifications for ChronosDB – a distributed graph database with native temporal support and a self‑optimizing predictive index layer. It is intended for software architects, engineers, and DevOps teams who will design, implement, and operate the system.

The TRD translates the functional and non‑functional requirements from the Product Requirements Document (PRD) into concrete technical designs, data structures, algorithms, and performance targets.

---

### **2\. System Overview**

ChronosDB is composed of three primary subsystems:

1. Temporal Graph Core – Stores nodes, relationships, and properties with time semantics (valid and transaction time). Provides ACID transactions, snapshot isolation, and efficient time‑travel queries.  
2. Predictive Graph Index (PGI) Module – Monitors query workload, builds lightweight models to forecast access patterns, and creates/deletes shortcut edges or materialized views to accelerate recurring temporal queries.  
3. Query & API Layer – Exposes ChronosQL (Cypher extended with temporal operators) via HTTP/gRPC, along with management and ingestion APIs.

All components are designed for horizontal scalability, high availability, and integration with cloud storage tiers.

---

### **3\. Architecture Diagram**

`text`

`┌─────────────────────────────────────────────────────────────┐`  
`│                      Client Applications                      │`  
`└───────────────────────────────┬─────────────────────────────┘`  
                                `│`  
                `┌───────────────┴───────────────┐`  
                `│        Load Balancer            │`  
                `└───────────────┬───────────────┘`  
                                `│`  
`┌───────────────────────────────▼─────────────────────────────┐`  
`│                     Query Router / Coordinator               │`  
`│  (Parsing, planning, distribution, result merging)          │`  
`└───────────────────────────────┬─────────────────────────────┘`  
                                `│`  
        `┌───────────────────────┼───────────────────────┐`  
        `│                       │                       │`  
`┌───────▼───────┐       ┌───────▼───────┐       ┌───────▼───────┐`  
`│  Storage Node  │       │  Storage Node  │       │  Storage Node  │`  
`│  (Shard 1)     │       │  (Shard 2)     │       │  (Shard N)     │`  
`│ ┌───────────┐  │       │ ┌───────────┐  │       │ ┌───────────┐  │`  
`│ │   Core    │  │       │ │   Core    │  │       │ │   Core    │  │`  
`│ │  Engine   │  │       │ │  Engine   │  │       │ │  Engine   │  │`  
`│ └───────────┘  │       │ └───────────┘  │       │ └───────────┘  │`  
`│ ┌───────────┐  │       │ ┌───────────┐  │       │ ┌───────────┐  │`  
`│ │PGI Module │  │       │ │PGI Module │  │       │ │PGI Module │  │`  
`│ └───────────┘  │       │ └───────────┘  │       │ └───────────┘  │`  
`└────────────────┘       └────────────────┘       └────────────────┘`  
        `│                       │                       │`  
        `└───────────────────────┼───────────────────────┘`  
                                `│`  
                `┌───────────────▼───────────────┐`  
                `│    Distributed Coordination    │`  
                `│   (Metadata, membership,      │`  
                `│    cluster state)              │`

                `└─────────────────────────────────┘`

---

### **4\. Detailed Component Specifications**

#### **4.1 Storage Engine (Core)**

Objective: Persist graph entities with temporal versioning and support efficient time‑travel queries.

* Storage Backend: Log‑Structured Merge‑Tree (LSM) – RocksDB (or equivalent) for high write throughput and ordered key access.  
* Key Encoding:  
  * Node Key: `{partition_id}:node:{node_id}:{timestamp}`  
  * Edge Key: `{partition_id}:edge:{edge_id}:{timestamp}`  
  * Property Key: `{partition_id}:prop:{entity_type}:{entity_id}:{prop_name}:{timestamp}`  
  * Index Entry: Depends on index type.  
* Column Families:  
  * `nodes_current`: Latest snapshot of node labels and properties.  
  * `edges_current`: Latest snapshot of edge type, source/target, properties.  
  * `nodes_history`: Temporal deltas for nodes.  
  * `edges_history`: Temporal deltas for edges.  
  * `indexes`: Secondary indexes (B‑Tree / skip list).  
  * `shortcuts`: Materialized paths/edges created by PGI.  
* Temporal Storage Strategy:  
  * Delta \+ Periodic Snapshots: Base snapshot \+ change logs. Compaction merges deltas into new snapshots (configurable interval, e.g., daily).  
  * Time‑Partitioned Buckets: Data older than a threshold moved to read‑only partitions (e.g., monthly buckets). Buckets can be stored on cheaper storage (S3).  
* Transaction Support:  
  * Snapshot isolation using MVCC.  
  * Write‑ahead log (WAL) for durability.  
  * Distributed transactions via two‑phase commit (2PC) or use of a consensus protocol (Raft) for metadata operations.  
* Caching:  
  * Block cache (RocksDB) for hot data.  
  * Separate cache for frequently accessed time slices.  
  * Shortcut cache.

#### **4.2 Query Engine**

Objective: Parse, optimize, and execute ChronosQL queries across distributed shards.

* Parser: Based on openCypher grammar, extended with temporal clauses:  
  * `AS OF <timestamp>`  
  * `BETWEEN <start> AND <end>`  
  * `HISTORY [OF] <property>`  
  * `AT <timepoint>`  
  * Temporal functions: `duration.between()`, `timepoint()`, `overlaps()`, etc.  
* Logical Planner:  
  * Convert AST to a logical tree with temporal operators (e.g., `TimeSlice`, `TemporalJoin`, `TemporalAggregate`).  
  * Use relational algebra extended with time.  
* Physical Planner:  
  * Cost‑based optimization using statistics (histograms, cardinalities).  
  * Choose access paths: primary key lookup, secondary index scan, full shard scan, or shortcut scan.  
  * For time‑range queries, prune partitions using time bounds.  
  * Generate a distributed plan: operations pushed down to shards; final merge at coordinator.  
* Execution Engine:  
  * Pull‑based (Volcano‑style) or push‑based (dataflow).  
  * Parallel execution across shards.  
  * Support for streaming results.

#### **4.3 Predictive Graph Index (PGI) Module**

Objective: Learn query patterns and create/remove shortcuts to accelerate future queries.

* Workload Monitor:  
  * Taps into query log (sampled or full) to extract:  
    * Query signatures (normalized form).  
    * Frequency, execution time, entities accessed.  
    * Temporal filters (time ranges, AS OF timestamps).  
  * Stores patterns in a time‑series store (e.g., Prometheus TSDB or internal ring buffer).  
* Predictive Model:  
  * Query Prediction: Lightweight Markov chain or RNN to predict next query types based on recent sequence.  
  * Path Popularity: Count occurrences of paths (e.g., `(A)-[r]->(B)`) over time; detect seasonal patterns.  
  * Temporal Pattern Detection: Identify recurring time‑bound access (e.g., every Monday morning).  
  * Models retrained periodically (e.g., hourly) using recent logs.  
* Shortcut Manager:  
  * Evaluates candidate shortcuts:  
    * Frequent paths (e.g., `(Customer)-[:BOUGHT]->(Product)`).  
    * Frequent multi‑hop patterns.  
    * Paths with time constraints that can be pre‑joined.  
  * Cost‑benefit analysis:  
    * Benefit \= estimated time saved × frequency.  
    * Cost \= storage overhead \+ maintenance overhead.  
    * If benefit \> threshold, create shortcut.  
  * Shortcuts are stored as special edge types in the `shortcuts` column family, with optional time validity.  
  * When base data changes, shortcuts are updated asynchronously (via background jobs).  
  * If a shortcut is not used for a while, it is dropped.  
* Integration with Query Optimizer:  
  * Optimizer considers shortcuts as alternative access paths.  
  * Shortcut metadata includes its time validity, source/target types, and estimated selectivity.

#### **4.4 APIs and Interfaces**

* gRPC API (primary, high‑performance):  
  * `ExecuteQuery(QueryRequest) returns (stream QueryResponse)`  
  * `BulkImport(stream ImportRecord) returns (ImportSummary)`  
  * `Admin` calls for cluster management.  
* RESTful HTTP API (secondary, for convenience):  
  * `POST /db/{db}/query` with JSON body.  
  * `POST /db/{db}/import` for batch.  
  * OpenAPI specification.  
* Language Drivers:  
  * Python, Java, Go, Node.js – wrapping gRPC.  
* Streaming Ingestion:  
  * Kafka Connect sink plugin to consume events and write to ChronosDB via gRPC.  
* Management Console:  
  * Web UI (React) interacting with REST APIs.  
  * Dashboards for metrics, query profiling, shortcut visualization.

#### **4.5 Cluster Coordination and Metadata**

* Metadata Store:  
  * Cluster topology, shard assignments, schema info (labels, relationship types), indexes, shortcuts.  
  * Stored in a highly available consistent key‑value store (etcd or similar) or using Raft within the cluster.  
* Sharding:  
  * Consistent hashing on node ID (hash ring) for even distribution.  
  * Optional time‑based partitioning for historical data.  
* Replication:  
  * Synchronous or asynchronous replication per shard (configurable).  
  * Leader‑follower model with automatic failover.  
* Query Routing:  
  * Coordinator node parses query, determines involved shards (using metadata), sends sub‑queries, merges results.

---

### **5\. Data Models and Schemas**

#### **5.1 Internal Temporal Graph Representation**

Node Record (binary serialized):

`text`

`message NodeRecord {`  
  `string id;                     // Unique node identifier`  
  `repeated string labels;         // Labels (can change over time)`  
  `map<string, TemporalValue> properties; // Property name -> temporal values`  
  `int64 valid_from;               // Microseconds since epoch`  
  `int64 valid_to;                  // 0 = unbounded future`  
  `int64 txn_id;                    // Transaction ID that created this version`

`}`

Edge Record:

`text`

`message EdgeRecord {`  
  `string id;`  
  `string type;`  
  `string source_id;`  
  `string target_id;`  
  `map<string, TemporalValue> properties;`  
  `int64 valid_from;`  
  `int64 valid_to;`  
  `int64 txn_id;`

`}`

TemporalValue (stores a time‑series of values):

`text`

`message TemporalValue {`  
  `repeated ValueChange changes;`  
`}`  
`message ValueChange {`  
  `oneof value { int64 int_val; double double_val; string str_val; bool bool_val; ... }`  
  `int64 valid_from;`  
  `int64 valid_to;`

`}`

Shortcut Record (special edge):

`text`

`message ShortcutRecord {`  
  `string id;`  
  `string type;                    // e.g., "SHORTCUT::Customer_to_Product"`  
  `string source_id;`  
  `string target_id;`  
  `map<string, Value> properties;   // Pre‑aggregated or derived values`  
  `int64 valid_from;                 // Time range this shortcut is applicable (if any)`  
  `int64 valid_to;`  
  `double estimated_benefit;         // For manager decisions`

`}`

#### **5.2 Indexes**

* Secondary indexes: B‑Tree on property values. Keys: `{prop_name}:{prop_value}:{entity_id}`.  
* Temporal indexes: For range queries, e.g., `valid_from` – can be implemented using time‑partitioned buckets.  
* Full‑text search: Optional integration with inverted index (e.g., Lucene) for string properties.

---

### **6\. Interfaces and Protocols**

#### **6.1 ChronosQL Grammar Extensions (ABNF snippets)**

`abnf`

`query           = regular-cypher [ temporal-clause ]`  
`temporal-clause = "AS OF" timestamp`  
                `| "BETWEEN" timestamp "AND" timestamp`  
                `| "HISTORY" "OF" property-expression`  
                `| "AT" timestamp`

`timestamp       = ISO-8601-datetime / integer-microseconds / parameter`

Examples already covered in PRD.

#### **6.2 gRPC Service Definition (proto)**

`protobuf`

`service ChronosDB {`  
  `rpc Execute(QueryRequest) returns (stream QueryResponse);`  
  `rpc Import(stream ImportRecord) returns (ImportSummary);`  
  `rpc GetMetadata(MetadataRequest) returns (MetadataResponse);`  
  `rpc Admin(AdminRequest) returns (AdminResponse);`  
`}`

`message QueryRequest {`  
  `string database = 1;`  
  `string query_text = 2;`  
  `map<string, Value> parameters = 3;`  
  `QueryOptions options = 4;`  
`}`

`message QueryResponse {`  
  `oneof result {`  
    `Row row = 1;`  
    `QueryStats stats = 2;`  
    `Error error = 3;`  
  `}`  
`}`

*`// ... other messages`*

#### **6.3 REST Endpoints**

* `POST /v1/databases/{db}/query` – JSON: `{"query": "...", "params": {...}}`  
* `POST /v1/databases/{db}/import` – multipart or JSON stream.  
* `GET /v1/databases/{db}/stats` – metrics.  
* `POST /v1/admin/cluster/rebalance` – etc.

---

### **7\. Scalability and Performance Requirements**

* Write Throughput: 100k updates/sec/node (with SSD). Each update may include multiple property changes. Achieved by batching writes in LSM and using async replication.  
* Query Latency:  
  * Point lookup (node by ID): P99 \< 10ms.  
  * Single‑hop traversal with temporal filter: P99 \< 50ms.  
  * Complex multi‑hop with time joins: P99 \< 2s without shortcuts; \< 200ms with shortcuts.  
* Shortcut Creation Latency: New shortcut available within 5 minutes of pattern detection (background job).  
* Ingestion: Bulk import at 500 MB/s per node.  
* Cluster Size: Linear scale‑out up to 100 nodes.  
* Availability: 99.99% with replication factor 3; automatic failover \< 30s.

---

### **8\. Security Requirements**

* Authentication: Mutual TLS (mTLS) for service‑to‑service; username/password or OAuth2 for clients.  
* Authorization: Role‑Based Access Control (RBAC) at database/collection level. Fine‑grained per node/edge type optional.  
* Encryption: TLS 1.3 for all network communication. Encryption at rest using LSM storage encryption (RocksDB encryption at rest) or cloud KMS integration.  
* Audit Logging: All data access (queries, updates) logged to secure, immutable store.

---

### **9\. Deployment and Operations**

* Packaging: Docker images, Helm charts for Kubernetes, bare‑metal install scripts.  
* Configuration: YAML files with hot‑reload support for some parameters.  
* Monitoring:  
  * Expose Prometheus metrics: QPS, latency histograms, cache hit ratios, shortcut usage, compaction status.  
  * Grafana dashboards.  
* Backup/Restore:  
  * Snapshot‑based (file copy) with consistency via quiesce.  
  * Incremental backup using WAL archiving.  
* Upgrades: Rolling upgrades with version compatibility; blue‑green for major changes.

---

### **10\. Testing Requirements**

* Unit Testing: Core algorithms (temporal merging, cost models).  
* Integration Testing: API correctness with temporal queries.  
* Performance Testing: Simulate workloads (YCSB \+ temporal extensions) to verify latency/throughput targets.  
* Chaos Testing: Kill nodes, network partitions to verify resilience.  
* Predictive Model Validation: Compare shortcut‑based vs. original query latency on real workloads.

---

### **11\. Future Technical Extensions (Roadmap)**

* Graph Neural Network for Prediction: Replace Markov chains with GNN to better capture graph structure.  
* Temporal Graph Algorithms Library: Built‑in PageRank over time, community detection in temporal graphs.  
* Multi‑Model Access: SQL via Apache Calcite adapter; document API.  
* Federated Query: Query across multiple clusters or external data sources (e.g., Neo4j, PostgreSQL) via connectors.

---

### **12\. Glossary**

* TKG: Temporal Knowledge Graph – graph with time‑aware nodes/edges.  
* PGI: Predictive Graph Index – subsystem for learning and shortcuts.  
* Shortcut: Pre‑computed path/edge stored to accelerate queries.  
* Valid Time: Time when a fact is true in reality.  
* Transaction Time: Time when a fact was recorded in the database.  
* MVCC: Multi‑Version Concurrency Control.  
* LSM: Log‑Structured Merge‑tree.

---

### **13\. References**

* \[1\] Neo4j Cypher temporal proposals.  
* \[2\] RocksDB Wiki.  
* \[3\] “Temporal Graph Databases” survey (Debrouvier et al.).  
* \[4\] “Learned Index Structures” (Kraska et al.) for predictive indexing concepts.  
* \[5\] Google Spanner (for distributed transaction ideas).

---

End of Technical Requirements Documen  
