// Bitemporal correctness test suite.
//
// These tests encode the invariants listed in
// internal/storage/core/INVARIANTS.md. Every test either passes today
// or is explicitly skipped with a reference to the sprint that will
// close the gap. As sprints land, skipped tests are "unskipped" one at
// a time — never in bulk. A green unskipped run is the Phase 1 exit
// criterion.
//
// Naming convention: TestI<N>_<description> maps directly to the
// invariant in INVARIANTS.md.
package temporal

import (
	"testing"
)

// ---- Helpers ----

func newTestStore(t *testing.T) *TemporalStore {
	t.Helper()
	store, err := NewTemporalStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func mustCreateNode(t *testing.T, s *TemporalStore, id string, props map[string]any, validFrom, validTo int64) {
	t.Helper()
	if err := s.CreateNode(id, []string{"Test"}, props, validFrom, validTo); err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

// ---- I2 — ID restrictions ----

func TestI2_IDWithSeparatorRejected(t *testing.T) {
	t.Skip("S1.2: CreateNode does not yet validate IDs at the storage boundary — " +
		"validation lives only in core.keys encoder helpers today.")
}

// ---- I3 — Timestamp encoding ----
//
// Covered end-to-end by keys_test.go in the core package; no additional
// test needed at the temporal layer.

// ---- I4 — Half-open valid-time ----

// I4a: a version is live at its valid_from (inclusive lower bound).
func TestI4a_AsOfAtValidFromReturnsVersion(t *testing.T) {
	s := newTestStore(t)
	mustCreateNode(t, s, "n1", map[string]any{"v": 1}, 100, 0)

	got, err := s.GetNodeAsOf("n1", 100)
	if err != nil {
		t.Fatalf("asof: %v", err)
	}
	if got == nil {
		t.Fatal("expected node live at valid_from")
	}
}

// I4b: a version is NOT live at its valid_to (exclusive upper bound).
// Current implementation uses inclusive upper bound; skip until S1.2.
func TestI4b_AsOfAtValidToDoesNotReturnVersion(t *testing.T) {
	t.Skip("S1.2: GetNodeAsOf currently uses inclusive upper bound; " +
		"INVARIANTS.md I4 requires exclusive upper bound.")
}

// I4c: unbounded version (valid_to = 0) is live at any t >= valid_from.
func TestI4c_UnboundedVersionLiveForever(t *testing.T) {
	s := newTestStore(t)
	mustCreateNode(t, s, "n1", map[string]any{"v": 1}, 100, 0)

	for _, at := range []int64{100, 1_000_000, 1 << 40} {
		got, err := s.GetNodeAsOf("n1", at)
		if err != nil {
			t.Fatalf("asof(%d): %v", at, err)
		}
		if got == nil {
			t.Fatalf("expected unbounded node to be live at t=%d", at)
		}
	}
}

// I4d: version is not live before its valid_from.
func TestI4d_AsOfBeforeValidFromReturnsNil(t *testing.T) {
	s := newTestStore(t)
	mustCreateNode(t, s, "n1", map[string]any{"v": 1}, 1000, 0)

	got, err := s.GetNodeAsOf("n1", 500)
	if err != nil {
		t.Fatalf("asof: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

// ---- I5 — Transaction-time monotonicity ----

// I5: BeginTx hands out strictly-increasing IDs. This guard runs at the
// invariant-suite level; txn_test.go exercises the same property with
// restart + concurrency cases.
func TestI5_TxnIDStrictlyIncreasing(t *testing.T) {
	s := newTestStore(t)

	var prev int64
	for i := 0; i < 5; i++ {
		tx, err := s.BeginTx()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if tx.ID() <= prev {
			t.Fatalf("txn id not strictly increasing: prev=%d got=%d", prev, tx.ID())
		}
		prev = tx.ID()
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
}

// ---- I6 — At most one live version per entity per instant ----

// I6 is enforced by first-committer-wins SI conflict detection: two
// concurrent transactions cannot both successfully write a new version
// for the same node ID. The losing committer aborts and its writes are
// discarded, so storage never holds two live versions at the same
// valid-time point. (See txn_test.go for finer-grained tests of the
// conflict path.)
func TestI6_NoOverlappingVersions(t *testing.T) {
	s := newTestStore(t)

	seed, _ := s.BeginTx()
	if err := seed.CreateNode("n1", nil, map[string]any{"v": 0}, 100, 0); err != nil {
		t.Fatal(err)
	}
	if err := seed.Commit(); err != nil {
		t.Fatal(err)
	}

	tx1, _ := s.BeginTx()
	tx2, _ := s.BeginTx()

	if err := tx1.UpdateNodeProperty("n1", "v", 1, 200); err != nil {
		t.Fatal(err)
	}
	if err := tx2.UpdateNodeProperty("n1", "v", 2, 300); err != nil {
		t.Fatal(err)
	}

	if err := tx1.Commit(); err != nil {
		t.Fatalf("tx1 commit: %v", err)
	}
	if err := tx2.Commit(); err != ErrWriteConflict {
		t.Fatalf("expected ErrWriteConflict on tx2, got %v", err)
	}

	got, err := s.GetNode("n1")
	if err != nil {
		t.Fatalf("read after conflict: %v", err)
	}
	// Only tx1's value should be visible.
	if got == nil || got.Properties["v"] != float64(1) {
		t.Fatalf("expected v=1 (tx1's), got %v", got)
	}
}

// ---- I8 — AS OF reproducibility ----
//
// I8a is the foundational guarantee: a point-in-time read is stable
// under later writes. Current temporal.go partially satisfies it for
// writes that don't touch the same node, but fails when a later update
// closes a prior version incorrectly. Split into two tests so the
// passing portion gates regressions today.

// I8a-1: AS OF is stable for a node that has never been updated.
func TestI8a1_AsOfStableWithoutUpdates(t *testing.T) {
	s := newTestStore(t)
	mustCreateNode(t, s, "n1", map[string]any{"balance": 100}, 1000, 0)

	first, _ := s.GetNodeAsOf("n1", 5000)
	// Unrelated write that must not affect n1's history.
	mustCreateNode(t, s, "n2", map[string]any{"other": true}, 2000, 0)
	second, _ := s.GetNodeAsOf("n1", 5000)

	if first == nil || second == nil {
		t.Fatalf("both reads should find n1; got first=%v second=%v", first, second)
	}
	if first.Properties["balance"] != second.Properties["balance"] {
		t.Fatalf("AS OF drift: %v then %v", first.Properties["balance"], second.Properties["balance"])
	}
}

// I8a-2: AS OF is stable across an update to the SAME node for any
// timestamp before the update's valid_from. This is the harder case
// and depends on proper history retrieval via nh:<id>:* scans.
func TestI8a2_AsOfStableAcrossUpdates(t *testing.T) {
	t.Skip("S1.2: GetNodeAsOf currently reads from the derived current-state " +
		"CF only and can therefore return the post-update value for pre-update " +
		"timestamps. Proper fix requires history-scan fallback.")
}

// ---- I9 — Soft delete preserves history ----

func TestI9_SoftDeletePreservesHistory(t *testing.T) {
	t.Skip("S1.2: GetNodeAsOf does not yet fall back to history for deleted nodes.")
}

// ---- Smoke tests that must always pass ----
//
// These exercise the invariants the current implementation already
// satisfies. Any regression here fails CI.

func TestSmoke_CreateAndGet(t *testing.T) {
	s := newTestStore(t)
	mustCreateNode(t, s, "n1", map[string]any{"v": 1}, 100, 0)

	got, err := s.GetNode("n1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.ID != "n1" {
		t.Fatalf("expected n1, got %+v", got)
	}
}

func TestSmoke_GetMissingReturnsNilNilNoError(t *testing.T) {
	s := newTestStore(t)

	got, err := s.GetNode("never_created")
	if err != nil {
		t.Fatalf("expected no error for missing node, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestSmoke_CreateEdgeAndGet(t *testing.T) {
	s := newTestStore(t)
	mustCreateNode(t, s, "a", nil, 100, 0)
	mustCreateNode(t, s, "b", nil, 100, 0)
	if err := s.CreateEdge("e1", "KNOWS", "a", "b", nil, 200, 0); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	edge, err := s.GetEdge("e1")
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if edge == nil || edge.SourceID != "a" || edge.TargetID != "b" {
		t.Fatalf("expected a->b, got %+v", edge)
	}
}
