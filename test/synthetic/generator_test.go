package synthetic

import (
	"fmt"
	"testing"
)

// TestDeterminism is load-bearing: if the same seed + scale ever produces
// different output, every benchmark and regression baseline downstream is
// invalidated.
func TestDeterminism(t *testing.T) {
	scale := Scale{NodeCount: 100, EdgeCount: 300, RevisionsPerNode: 2}

	a := NewGenerator(42, scale).Generate()
	b := NewGenerator(42, scale).Generate()

	if len(a.Nodes) != len(b.Nodes) {
		t.Fatalf("node count drift: %d vs %d", len(a.Nodes), len(b.Nodes))
	}
	if len(a.Edges) != len(b.Edges) {
		t.Fatalf("edge count drift: %d vs %d", len(a.Edges), len(b.Edges))
	}
	if len(a.Revisions) != len(b.Revisions) {
		t.Fatalf("revision count drift: %d vs %d", len(a.Revisions), len(b.Revisions))
	}

	for i := range a.Nodes {
		if a.Nodes[i].ID != b.Nodes[i].ID ||
			a.Nodes[i].ValidFrom != b.Nodes[i].ValidFrom ||
			a.Nodes[i].TxnID != b.Nodes[i].TxnID {
			t.Fatalf("node %d drift: %+v vs %+v", i, a.Nodes[i], b.Nodes[i])
		}
	}
	for i := range a.Edges {
		if a.Edges[i].ID != b.Edges[i].ID ||
			a.Edges[i].SourceID != b.Edges[i].SourceID ||
			a.Edges[i].TargetID != b.Edges[i].TargetID ||
			a.Edges[i].ValidFrom != b.Edges[i].ValidFrom {
			t.Fatalf("edge %d drift: %+v vs %+v", i, a.Edges[i], b.Edges[i])
		}
	}
}

// TestGroundTruthConsistency re-evaluates every ground-truth query against
// the dataset and confirms the stored expected answer matches. This
// catches drift between the dataset and its own answer key.
func TestGroundTruthConsistency(t *testing.T) {
	ds := NewGenerator(1, Small).Generate()

	if len(ds.GroundTruth) == 0 {
		t.Fatal("expected at least one ground-truth query")
	}

	for id, q := range ds.GroundTruth {
		t.Run(id, func(t *testing.T) {
			var got any
			switch q.Kind {
			case "node_as_of":
				got = asOfExpectedNode(ds, q.NodeID, q.AtTime)
			case "edge_count_between":
				got = countEdgesBetween(ds, q.EdgeType, q.FromTime, q.ToTime)
			case "neighbors_as_of":
				got = neighborsAsOf(ds, q.NodeID, q.EdgeType, q.AtTime)
			default:
				t.Fatalf("unknown ground-truth kind: %s", q.Kind)
			}
			if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", q.Expected) {
				t.Fatalf("ground-truth %s drift:\n  got:      %v\n  expected: %v",
					id, got, q.Expected)
			}
		})
	}
}

func TestScaleProfiles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scale Scale
	}{
		{"small", Small},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := NewGenerator(0, tc.scale).Generate()
			if len(ds.Nodes) != tc.scale.NodeCount {
				t.Fatalf("expected %d nodes, got %d", tc.scale.NodeCount, len(ds.Nodes))
			}
			if len(ds.Edges) != tc.scale.EdgeCount {
				t.Fatalf("expected %d edges, got %d", tc.scale.EdgeCount, len(ds.Edges))
			}
		})
	}
}
