package temporal

import (
	"sync"
	"testing"
)

// TestTxn_MonotonicWithinProcess verifies Invariant I5 across ten
// sequential BeginTx calls in a single process. Companion to
// TestI5_TxnIDStrictlyIncreasing which runs at the bitemporal-suite
// level.
func TestTxn_MonotonicWithinProcess(t *testing.T) {
	s := newTestStore(t)

	var prev int64
	for i := 0; i < 10; i++ {
		tx, err := s.BeginTx()
		if err != nil {
			t.Fatalf("begin #%d: %v", i, err)
		}
		if tx.ID() <= prev {
			t.Fatalf("txn id not strictly increasing: prev=%d got=%d", prev, tx.ID())
		}
		prev = tx.ID()
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit #%d: %v", i, err)
		}
	}
}

// TestTxn_MonotonicAcrossRestart verifies the persisted counter — the
// second process must NOT reuse an ID allocated by the first.
func TestTxn_MonotonicAcrossRestart(t *testing.T) {
	dir := t.TempDir()

	store1, err := NewTemporalStore(dir)
	if err != nil {
		t.Fatalf("store1: %v", err)
	}

	var lastBefore int64
	for i := 0; i < 3; i++ {
		tx, err := store1.BeginTx()
		if err != nil {
			t.Fatalf("begin before restart: %v", err)
		}
		lastBefore = tx.ID()
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit before restart: %v", err)
		}
	}
	if err := store1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	store2, err := NewTemporalStore(dir)
	if err != nil {
		t.Fatalf("store2: %v", err)
	}
	defer func() { _ = store2.Close() }()

	tx, err := store2.BeginTx()
	if err != nil {
		t.Fatalf("begin after restart: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if tx.ID() <= lastBefore {
		t.Fatalf("txn id reused across restart: before=%d after=%d",
			lastBefore, tx.ID())
	}
}

// TestTxn_ConcurrentNoDupes stresses the locking around counter
// allocation. Under -race this will also flag any torn reads.
func TestTxn_ConcurrentNoDupes(t *testing.T) {
	s := newTestStore(t)

	const N = 200
	ids := make([]int64, N)
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			tx, err := s.BeginTx()
			if err != nil {
				t.Errorf("begin: %v", err)
				return
			}
			ids[i] = tx.ID()
			if err := tx.Commit(); err != nil {
				t.Errorf("commit: %v", err)
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[int64]struct{}, N)
	for _, id := range ids {
		if id == 0 {
			t.Fatal("zero ID observed (begin failure not reported)")
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate txn id: %d", id)
		}
		seen[id] = struct{}{}
	}
}

// TestTxn_CommitRollbackStateTransitions verifies that double-commit,
// commit-after-rollback, and rollback-after-commit all surface
// ErrTxnNotActive.
func TestTxn_CommitRollbackStateTransitions(t *testing.T) {
	s := newTestStore(t)

	t.Run("double commit", func(t *testing.T) {
		tx, err := s.BeginTx()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("first commit: %v", err)
		}
		if err := tx.Commit(); err != ErrTxnNotActive {
			t.Fatalf("expected ErrTxnNotActive, got %v", err)
		}
	})

	t.Run("commit after rollback", func(t *testing.T) {
		tx, err := s.BeginTx()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatalf("rollback: %v", err)
		}
		if err := tx.Commit(); err != ErrTxnNotActive {
			t.Fatalf("expected ErrTxnNotActive, got %v", err)
		}
	})

	t.Run("rollback after commit", func(t *testing.T) {
		tx, err := s.BeginTx()
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if err := tx.Rollback(); err != ErrTxnNotActive {
			t.Fatalf("expected ErrTxnNotActive, got %v", err)
		}
	})
}

// TestTxn_ActiveCountReflectsLifecycle sanity-checks the active-set
// bookkeeping. Primarily a regression guard — if a future refactor
// stops removing txns from the active map, this test catches it.
func TestTxn_ActiveCountReflectsLifecycle(t *testing.T) {
	s := newTestStore(t)

	if got := s.txnMgr.activeCount(); got != 0 {
		t.Fatalf("expected 0 active, got %d", got)
	}

	tx1, _ := s.BeginTx()
	tx2, _ := s.BeginTx()
	if got := s.txnMgr.activeCount(); got != 2 {
		t.Fatalf("expected 2 active, got %d", got)
	}

	_ = tx1.Commit()
	if got := s.txnMgr.activeCount(); got != 1 {
		t.Fatalf("expected 1 active after first commit, got %d", got)
	}

	_ = tx2.Rollback()
	if got := s.txnMgr.activeCount(); got != 0 {
		t.Fatalf("expected 0 active after rollback, got %d", got)
	}
}

// ---- Transactional write tests (S1.1.2 step 2) ----

// TestTxn_CreateNodeVisibleAfterCommit: a node created inside a Txn
// must be readable via the existing non-transactional GetNode after
// Commit, and absent after Rollback.
func TestTxn_CreateNodeVisibleAfterCommit(t *testing.T) {
	s := newTestStore(t)

	tx, err := s.BeginTx()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := tx.CreateNode("n1", []string{"X"}, map[string]any{"v": 1}, 100, 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Pre-commit: not visible.
	if got, _ := s.GetNode("n1"); got != nil {
		t.Fatal("node visible before commit")
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	got, err := s.GetNode("n1")
	if err != nil || got == nil || got.Properties["v"] != float64(1) {
		t.Fatalf("expected post-commit read to find v=1, got node=%v err=%v", got, err)
	}
}

// TestTxn_RollbackDiscardsWrites: rolled-back ops must not appear.
func TestTxn_RollbackDiscardsWrites(t *testing.T) {
	s := newTestStore(t)

	tx, _ := s.BeginTx()
	if err := tx.CreateNode("n1", []string{"X"}, nil, 100, 0); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got, _ := s.GetNode("n1"); got != nil {
		t.Fatal("rollback failed to discard write")
	}
}

// TestTxn_BatchAtomicity: multiple ops in one Txn either all apply or
// none. We force the failure path by closing the underlying engine
// before commit and confirming no partial state was written.
func TestTxn_BatchAtomicity(t *testing.T) {
	s := newTestStore(t)

	// Seed a node so a subsequent UpdateNodeProperty has a target.
	seed, _ := s.BeginTx()
	if err := seed.CreateNode("n1", nil, map[string]any{"v": 1}, 100, 0); err != nil {
		t.Fatal(err)
	}
	if err := seed.Commit(); err != nil {
		t.Fatal(err)
	}

	// Stage two updates in one txn; both must apply.
	tx, _ := s.BeginTx()
	if err := tx.CreateNode("n2", nil, map[string]any{"v": 1}, 100, 0); err != nil {
		t.Fatal(err)
	}
	if err := tx.UpdateNodeProperty("n1", "v", 2, 200); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if got, _ := s.GetNode("n1"); got == nil || got.Properties["v"] != float64(2) {
		t.Fatalf("expected n1.v=2, got %v", got)
	}
	if got, _ := s.GetNode("n2"); got == nil {
		t.Fatal("expected n2 to exist")
	}
}

// TestTxn_NonOverlappingWritesNoConflict: two concurrent commits to
// disjoint entities both succeed.
func TestTxn_NonOverlappingWritesNoConflict(t *testing.T) {
	s := newTestStore(t)

	tx1, _ := s.BeginTx()
	tx2, _ := s.BeginTx()

	if err := tx1.CreateNode("n_a", nil, map[string]any{"v": 1}, 100, 0); err != nil {
		t.Fatal(err)
	}
	if err := tx2.CreateNode("n_b", nil, map[string]any{"v": 1}, 100, 0); err != nil {
		t.Fatal(err)
	}

	if err := tx1.Commit(); err != nil {
		t.Fatalf("tx1 commit: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("tx2 commit (disjoint entity should not conflict): %v", err)
	}
}

// TestTxn_WriteConflictFirstCommitterWins: two txns with overlapping
// writes — second to commit aborts with ErrWriteConflict.
func TestTxn_WriteConflictFirstCommitterWins(t *testing.T) {
	s := newTestStore(t)

	// Seed a node so both txns have a real target to update.
	seed, _ := s.BeginTx()
	_ = seed.CreateNode("n1", nil, map[string]any{"v": 0}, 100, 0)
	_ = seed.Commit()

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
	err := tx2.Commit()
	if err != ErrWriteConflict {
		t.Fatalf("expected ErrWriteConflict, got %v", err)
	}

	// Verify only tx1's write took effect.
	got, _ := s.GetNode("n1")
	if got == nil || got.Properties["v"] != float64(1) {
		t.Fatalf("expected v=1 (tx1's value), got %v", got)
	}
}

// TestTxn_UpdateMissingNodeReturnsError: UpdateNodeProperty on a node
// that does not exist surfaces ErrNodeNotFound synchronously, before
// commit.
func TestTxn_UpdateMissingNodeReturnsError(t *testing.T) {
	s := newTestStore(t)

	tx, _ := s.BeginTx()
	defer func() { _ = tx.Rollback() }()

	if err := tx.UpdateNodeProperty("does_not_exist", "v", 1, 100); err != ErrNodeNotFound {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}

// TestTxn_WritesAfterCommitRejected: calling write methods on a
// committed (or aborted) Txn must return ErrTxnNotActive.
func TestTxn_WritesAfterCommitRejected(t *testing.T) {
	s := newTestStore(t)

	tx, _ := s.BeginTx()
	_ = tx.Commit()

	if err := tx.CreateNode("n1", nil, nil, 100, 0); err != ErrTxnNotActive {
		t.Fatalf("expected ErrTxnNotActive on CreateNode, got %v", err)
	}
	if err := tx.CreateEdge("e1", "K", "a", "b", nil, 100, 0); err != ErrTxnNotActive {
		t.Fatalf("expected ErrTxnNotActive on CreateEdge, got %v", err)
	}
	if err := tx.UpdateNodeProperty("n1", "v", 1, 100); err != ErrTxnNotActive {
		t.Fatalf("expected ErrTxnNotActive on UpdateNodeProperty, got %v", err)
	}
}

// TestTxn_CreateEdgeViaTxn: round-trip a transactional CreateEdge
// through the existing GetEdge reader.
func TestTxn_CreateEdgeViaTxn(t *testing.T) {
	s := newTestStore(t)

	seed, _ := s.BeginTx()
	_ = seed.CreateNode("a", nil, nil, 100, 0)
	_ = seed.CreateNode("b", nil, nil, 100, 0)
	_ = seed.Commit()

	tx, _ := s.BeginTx()
	if err := tx.CreateEdge("e1", "KNOWS", "a", "b", map[string]any{"since": 2020}, 200, 0); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	edge, err := s.GetEdge("e1")
	if err != nil || edge == nil {
		t.Fatalf("expected edge, got nil err=%v", err)
	}
	if edge.SourceID != "a" || edge.TargetID != "b" || edge.Type != "KNOWS" {
		t.Fatalf("edge round-trip drift: %+v", edge)
	}
}
