// Package synthetic produces deterministic, reproducible bitemporal graph
// datasets that all ChronosDB correctness tests and benchmarks are
// evaluated against.
//
// Determinism: every call to NewGenerator(seed, scale) followed by
// Generate() returns byte-identical records. This is load-bearing for
// benchmark comparability across commits.
//
// Scale: dataset size scales linearly with Scale.EdgeCount. The default
// Small profile targets CI in <1s; Large targets the 10M-edge Phase 4
// benchmark workload.
package synthetic

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// Scale parameterizes dataset size. Counts are approximate; exact numbers
// fall out of the generator's deterministic iteration.
type Scale struct {
	NodeCount int
	EdgeCount int
	// RevisionsPerNode is the average number of bitemporal updates per node.
	// Higher values stress history storage and AS OF queries.
	RevisionsPerNode int
}

// Preset scales. Benchmarks must use Small by default; 10M-edge Large is
// reserved for manual reproduction of published results.
var (
	Small  = Scale{NodeCount: 1_000, EdgeCount: 5_000, RevisionsPerNode: 3}
	Medium = Scale{NodeCount: 100_000, EdgeCount: 500_000, RevisionsPerNode: 5}
	Large  = Scale{NodeCount: 2_000_000, EdgeCount: 10_000_000, RevisionsPerNode: 5}
)

// Node is a synthetic bitemporal node. Labels encode the toy domain
// (Customer / Product / Order) used by the ground-truth query set.
type Node struct {
	ID         string
	Labels     []string
	Properties map[string]any
	ValidFrom  int64
	ValidTo    int64 // 0 = unbounded
	TxnID      int64
}

// Edge is a synthetic bitemporal edge.
type Edge struct {
	ID         string
	Type       string
	SourceID   string
	TargetID   string
	Properties map[string]any
	ValidFrom  int64
	ValidTo    int64
	TxnID      int64
}

// Revision captures a property change applied to a node at a specific
// time. Revisions drive the bitemporal history that AS OF queries probe.
type Revision struct {
	NodeID    string
	Property  string
	OldValue  any
	NewValue  any
	ValidFrom int64
	TxnID     int64
}

// Dataset is the complete output of one generator run.
type Dataset struct {
	Seed      uint64
	Scale     Scale
	Epoch     int64
	Nodes     []Node
	Edges     []Edge
	Revisions []Revision
	// GroundTruth is a small set of (query, expected-result) pairs used
	// by correctness tests. Keys are human-readable query IDs; values
	// are the expected results in a schema-stable form.
	GroundTruth map[string]GroundTruthQuery
}

// GroundTruthQuery couples an evaluable specification with its expected
// result. The spec is intentionally language-agnostic (not ChronosQL)
// so the test harness can evaluate it directly against the dataset
// before any parser or executor exists.
type GroundTruthQuery struct {
	Kind        string // "node_as_of" | "edge_count_between" | "neighbors_as_of"
	NodeID      string
	EdgeType    string
	AtTime      int64
	FromTime    int64
	ToTime      int64
	Description string
	// Expected holds the reference answer. Type depends on Kind:
	//   node_as_of         -> *Node (nil if not present at AtTime)
	//   edge_count_between -> int
	//   neighbors_as_of    -> []string (sorted target IDs)
	Expected any
}

// Generator produces deterministic bitemporal datasets.
type Generator struct {
	rng   *rand.Rand
	seed  uint64
	scale Scale
	// Epoch is the valid-time origin for all generated records. Kept
	// fixed across generator runs (not derived from wall clock) so
	// datasets are reproducible byte-for-byte.
	epoch int64
}

// NewGenerator constructs a generator that, given the same (seed, scale),
// produces byte-identical output on every invocation.
func NewGenerator(seed uint64, scale Scale) *Generator {
	return &Generator{
		rng:   rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15)),
		seed:  seed,
		scale: scale,
		epoch: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMicro(),
	}
}

// Generate materializes the full dataset in memory. For very large
// scales, prefer GenerateStream (not yet implemented; Phase 1).
func (g *Generator) Generate() *Dataset {
	ds := &Dataset{
		Seed:        g.seed,
		Scale:       g.scale,
		Epoch:       g.epoch,
		Nodes:       make([]Node, 0, g.scale.NodeCount),
		Edges:       make([]Edge, 0, g.scale.EdgeCount),
		Revisions:   make([]Revision, 0, g.scale.NodeCount*g.scale.RevisionsPerNode),
		GroundTruth: make(map[string]GroundTruthQuery),
	}

	// Two-thirds Customers, one-third Products. Keeps the motif
	// distribution skewed so PGI has something to learn in Phase 2.
	customerCount := (g.scale.NodeCount * 2) / 3
	productCount := g.scale.NodeCount - customerCount

	for i := 0; i < customerCount; i++ {
		ds.Nodes = append(ds.Nodes, g.makeCustomer(i))
	}
	for i := 0; i < productCount; i++ {
		ds.Nodes = append(ds.Nodes, g.makeProduct(i))
	}

	for i := 0; i < g.scale.EdgeCount; i++ {
		src := ds.Nodes[g.rng.IntN(customerCount)]
		dst := ds.Nodes[customerCount+g.rng.IntN(productCount)]
		ds.Edges = append(ds.Edges, g.makePurchase(i, src.ID, dst.ID))
	}

	for _, n := range ds.Nodes {
		for r := 0; r < g.scale.RevisionsPerNode; r++ {
			ds.Revisions = append(ds.Revisions, g.makeRevision(n, r))
		}
	}

	g.populateGroundTruth(ds)
	return ds
}

func (g *Generator) makeCustomer(i int) Node {
	cities := []string{"NYC", "LAX", "CHI", "HOU", "PHX"}
	validFrom := g.epoch + int64(g.rng.IntN(60*86_400*1_000_000)) // first 60 days
	return Node{
		ID:     fmt.Sprintf("cust_%06d", i),
		Labels: []string{"Customer"},
		Properties: map[string]any{
			"name":    fmt.Sprintf("Customer_%d", i),
			"age":     18 + g.rng.IntN(60),
			"city":    cities[g.rng.IntN(len(cities))],
			"balance": float64(g.rng.IntN(10_000)),
		},
		ValidFrom: validFrom,
		ValidTo:   0,
		TxnID:     int64(i + 1),
	}
}

func (g *Generator) makeProduct(i int) Node {
	cats := []string{"Electronics", "Books", "Home", "Sports", "Food"}
	validFrom := g.epoch + int64(g.rng.IntN(30*86_400*1_000_000))
	return Node{
		ID:     fmt.Sprintf("prod_%06d", i),
		Labels: []string{"Product"},
		Properties: map[string]any{
			"name":     fmt.Sprintf("Product_%d", i),
			"category": cats[g.rng.IntN(len(cats))],
			"price":    float64(10 + g.rng.IntN(990)),
			"stock":    g.rng.IntN(200),
		},
		ValidFrom: validFrom,
		ValidTo:   0,
		TxnID:     int64(i + 1_000_000),
	}
}

func (g *Generator) makePurchase(i int, src, dst string) Edge {
	validFrom := g.epoch + int64(g.rng.IntN(180*86_400*1_000_000))
	return Edge{
		ID:       fmt.Sprintf("purch_%08d", i),
		Type:     "PURCHASED",
		SourceID: src,
		TargetID: dst,
		Properties: map[string]any{
			"quantity": 1 + g.rng.IntN(5),
			"price":    float64(10 + g.rng.IntN(990)),
		},
		ValidFrom: validFrom,
		ValidTo:   0,
		TxnID:     int64(i + 2_000_000),
	}
}

func (g *Generator) makeRevision(n Node, r int) Revision {
	validFrom := n.ValidFrom + int64((r+1)*7*86_400*1_000_000)
	return Revision{
		NodeID:    n.ID,
		Property:  "balance",
		OldValue:  n.Properties["balance"],
		NewValue:  float64(g.rng.IntN(10_000)),
		ValidFrom: validFrom,
		TxnID:     int64(3_000_000 + r),
	}
}

// populateGroundTruth builds a small set of queries with known answers.
// These are the contract that correctness tests evaluate against: if the
// engine ever disagrees with a ground-truth result, either the engine is
// broken or the generator drifted — both are bugs.
func (g *Generator) populateGroundTruth(ds *Dataset) {
	if len(ds.Nodes) == 0 {
		return
	}

	// Query 1: node_as_of for the first customer at epoch+1h (before any
	// revisions). Expected = node with original balance.
	firstCust := ds.Nodes[0]
	ds.GroundTruth["q1_cust_at_epoch"] = GroundTruthQuery{
		Kind:        "node_as_of",
		NodeID:      firstCust.ID,
		AtTime:      g.epoch + 3600*1_000_000,
		Description: "first customer as of 1h after epoch",
		Expected:    asOfExpectedNode(ds, firstCust.ID, g.epoch+3600*1_000_000),
	}

	// Query 2: edge_count_between for PURCHASED across the full
	// valid-time envelope.
	ds.GroundTruth["q2_purchase_count_full"] = GroundTruthQuery{
		Kind:        "edge_count_between",
		EdgeType:    "PURCHASED",
		FromTime:    g.epoch,
		ToTime:      g.epoch + 365*86_400*1_000_000,
		Description: "count of PURCHASED edges in 365 days from epoch",
		Expected:    countEdgesBetween(ds, "PURCHASED", g.epoch, g.epoch+365*86_400*1_000_000),
	}

	// Query 3: neighbors_as_of for the first customer at a mid-timeline
	// point. Expected = sorted target IDs of edges live at that time.
	at := g.epoch + 90*86_400*1_000_000
	ds.GroundTruth["q3_first_cust_neighbors_at_90d"] = GroundTruthQuery{
		Kind:        "neighbors_as_of",
		NodeID:      firstCust.ID,
		EdgeType:    "PURCHASED",
		AtTime:      at,
		Description: "first customer's PURCHASED neighbors at t=epoch+90d",
		Expected:    neighborsAsOf(ds, firstCust.ID, "PURCHASED", at),
	}
}

func asOfExpectedNode(ds *Dataset, id string, at int64) *Node {
	for i := range ds.Nodes {
		n := &ds.Nodes[i]
		if n.ID != id {
			continue
		}
		if n.ValidFrom <= at && (n.ValidTo == 0 || at <= n.ValidTo) {
			copy := *n
			return &copy
		}
	}
	return nil
}

func countEdgesBetween(ds *Dataset, edgeType string, from, to int64) int {
	count := 0
	for _, e := range ds.Edges {
		if e.Type != edgeType {
			continue
		}
		if e.ValidFrom <= to && (e.ValidTo == 0 || e.ValidTo >= from) {
			count++
		}
	}
	return count
}

func neighborsAsOf(ds *Dataset, nodeID, edgeType string, at int64) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, e := range ds.Edges {
		if e.SourceID != nodeID || e.Type != edgeType {
			continue
		}
		if e.ValidFrom > at || (e.ValidTo != 0 && e.ValidTo < at) {
			continue
		}
		if _, ok := seen[e.TargetID]; ok {
			continue
		}
		seen[e.TargetID] = struct{}{}
		out = append(out, e.TargetID)
	}
	sortStrings(out)
	return out
}

// sortStrings is a tiny inline sort to avoid importing sort just for this.
// Bubble sort is fine at the scales where this runs (ground-truth only).
func sortStrings(s []string) {
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
