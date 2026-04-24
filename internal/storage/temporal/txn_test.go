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
