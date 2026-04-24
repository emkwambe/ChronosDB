// Package load contains in-process benchmarks that exercise the
// TemporalStore directly, without requiring a running chronosd server.
// These run in CI and gate on regression.
package load

import (
	"fmt"
	"testing"

	"github.com/emkwambe/chronosdb/internal/storage/temporal"
	"github.com/emkwambe/chronosdb/test/synthetic"
)

func newStore(b *testing.B) *temporal.TemporalStore {
	b.Helper()
	store, err := temporal.NewTemporalStore(b.TempDir())
	if err != nil {
		b.Fatalf("create store: %v", err)
	}
	b.Cleanup(func() { _ = store.Close() })
	return store
}

// BenchmarkCreateNode measures single-node write throughput against the
// bare TemporalStore — no network, no executor, no planner. Establishes
// the baseline ceiling for Phase 1's write-throughput exit criterion.
func BenchmarkCreateNode(b *testing.B) {
	store := newStore(b)
	ds := synthetic.NewGenerator(1, synthetic.Small).Generate()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := ds.Nodes[i%len(ds.Nodes)]
		id := fmt.Sprintf("%s_%d", n.ID, i)
		if err := store.CreateNode(id, n.Labels, n.Properties, n.ValidFrom, n.ValidTo); err != nil {
			b.Fatalf("create: %v", err)
		}
	}
}

// BenchmarkGetNode measures point-lookup latency on a pre-populated store.
func BenchmarkGetNode(b *testing.B) {
	store := newStore(b)
	ds := synthetic.NewGenerator(1, synthetic.Small).Generate()
	for _, n := range ds.Nodes {
		if err := store.CreateNode(n.ID, n.Labels, n.Properties, n.ValidFrom, n.ValidTo); err != nil {
			b.Fatalf("create: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := ds.Nodes[i%len(ds.Nodes)]
		if _, err := store.GetNode(n.ID); err != nil {
			b.Fatalf("get: %v", err)
		}
	}
}

// BenchmarkGetNodeAsOf measures point-in-time lookup latency. This is the
// direct proxy for the Phase 1 target: AS OF p99 < 10ms.
func BenchmarkGetNodeAsOf(b *testing.B) {
	store := newStore(b)
	ds := synthetic.NewGenerator(1, synthetic.Small).Generate()
	for _, n := range ds.Nodes {
		if err := store.CreateNode(n.ID, n.Labels, n.Properties, n.ValidFrom, n.ValidTo); err != nil {
			b.Fatalf("create: %v", err)
		}
	}
	at := ds.Epoch + 30*86_400*1_000_000

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		n := ds.Nodes[i%len(ds.Nodes)]
		if _, err := store.GetNodeAsOf(n.ID, at); err != nil {
			b.Fatalf("asof: %v", err)
		}
	}
}

// BenchmarkCreateEdge measures edge-write throughput.
func BenchmarkCreateEdge(b *testing.B) {
	store := newStore(b)
	ds := synthetic.NewGenerator(1, synthetic.Small).Generate()
	for _, n := range ds.Nodes {
		_ = store.CreateNode(n.ID, n.Labels, n.Properties, n.ValidFrom, n.ValidTo)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e := ds.Edges[i%len(ds.Edges)]
		id := fmt.Sprintf("%s_%d", e.ID, i)
		if err := store.CreateEdge(id, e.Type, e.SourceID, e.TargetID, e.Properties, e.ValidFrom, e.ValidTo); err != nil {
			b.Fatalf("edge: %v", err)
		}
	}
}
