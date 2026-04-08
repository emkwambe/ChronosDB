# ChronosDB Architecture Guardrails

## 1. Technology Stack
- **Language**: Go (1.21+) for performance and concurrency.
- **Storage Engine**: RocksDB (via Go bindings) for LSM tree.
- **API**: gRPC (primary) and REST (secondary) via grpc-gateway.
- **Serialization**: Protocol Buffers for internal and external messages.
- **Coordination**: etcd for cluster metadata (or embedded Raft).
- **Monitoring**: Prometheus metrics exposed via HTTP.
- **Logging**: Structured logging with slog.
- **Configuration**: YAML files with environment variable override.

## 2. Repository Structure (Monorepo)

```
/chronosdb/
├── cmd/
│   └── chronosd/          # Main server binary
├── internal/
│   ├── storage/           # Temporal graph storage engine
│   │   ├── core/          # LSM wrappers, column families
│   │   ├── temporal/      # Temporal versioning logic
│   │   └── compaction/    # Background compaction
│   ├── query/             # Query parsing, planning, execution
│   │   ├── parser/        # ChronosQL parser (extended Cypher)
│   │   ├── planner/       # Logical/physical planner
│   │   └── executor/      # Execution engine
│   ├── pgi/               # Predictive Graph Index
│   │   ├── monitor/       # Query log monitor
│   │   ├── model/         # Lightweight prediction models
│   │   └── shortcut/      # Shortcut creation/maintenance
│   ├── pal/               # Predictive Analytics Layer (Phase 4)
│   │   ├── modelrepo/     # Model storage
│   │   ├── trainer/       # Training pipelines
│   │   └── inference/     # Inference engine
│   ├── cluster/           # Cluster management
│   │   ├── membership/    # Node discovery, heartbeats
│   │   ├── sharding/      # Consistent hashing
│   │   └── replication/   # Replication logic
│   ├── api/               # API handlers
│   │   ├── grpc/          # gRPC service implementation
│   │   ├── rest/          # REST gateway
│   │   └── management/    # Admin console backend
│   └── security/          # Auth, encryption, audit
├── pkg/
│   ├── chronosql/         # Public ChronosQL types (if any)
│   └── client/            # Official Go client
├── proto/                 # Protocol Buffer definitions
├── docs/                  # Documentation
├── deployments/           # Kubernetes, Docker, etc.
└── scripts/               # Build, test, dev scripts
```

## 3. Coding Standards
- Follow standard Go project layout.
- Use `context.Context` for cancellation and timeouts.
- All errors must be wrapped with context.
- Unit tests for all packages; integration tests for end-to-end scenarios.
- gRPC services defined in `.proto` files; generate Go code.
- Commit messages follow conventional commits.

## 4. Development Workflow
- Each phase is implemented in a feature branch.
- After completing a phase, run the verification checklist.
- Merge only after all checklist items pass.
- Maintain backward compatibility within the same major version.
