package temporal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/emkwambe/chronosdb/internal/storage/core"
)

// metaNextTxnID is the `mt:` key holding the next-to-allocate txn ID.
// Persisted as big-endian int64 (consistent with the rest of the
// storage layer per INVARIANTS.md I3).
const metaNextTxnID = "next_txn_id"

// txnState tracks where a transaction sits in its lifecycle. Declared
// internal because callers should only observe state through errors.
type txnState uint8

const (
	txnActive txnState = iota
	txnCommitted
	txnAborted
)

// ErrTxnNotActive is returned by Commit and Rollback on a transaction
// that has already been closed.
var ErrTxnNotActive = errors.New("transaction is not active")

// txnManager allocates monotonic transaction IDs, persists the counter
// so IDs never reuse across process restarts, and tracks in-flight
// transactions.
//
// Goroutine-safe. Owned by TemporalStore; not exposed directly to
// callers.
//
// Step 1 scope: ID allocation + active-set tracking only. Write
// staging, read snapshots, and conflict detection land in the next
// commit.
type txnManager struct {
	engine *core.StorageEngine

	mu     sync.Mutex
	next   int64
	active map[int64]*Txn
}

// newTxnManager loads the persisted next-id counter (or initializes to
// 1 if none is stored).
func newTxnManager(engine *core.StorageEngine) (*txnManager, error) {
	next, err := loadNextTxnID(engine)
	if err != nil {
		return nil, fmt.Errorf("txn manager init: %w", err)
	}
	return &txnManager{
		engine: engine,
		next:   next,
		active: make(map[int64]*Txn),
	}, nil
}

// begin allocates a new txn ID, persists the incremented counter, and
// registers the txn as active.
//
// The counter is persisted *before* the ID is handed out so that a
// crash after return can never cause a later process to allocate the
// same ID. On persist failure the in-memory counter is rolled back so
// the same ID may be retried safely.
func (tm *txnManager) begin() (*Txn, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	id := tm.next
	if err := persistNextTxnID(tm.engine, id+1); err != nil {
		return nil, fmt.Errorf("persist next_txn_id: %w", err)
	}
	tm.next = id + 1

	txn := &Txn{manager: tm, id: id, state: txnActive}
	tm.active[id] = txn
	return txn, nil
}

// closeTxn removes a txn from the active set and transitions its state.
// Returns ErrTxnNotActive if the txn has already been closed.
func (tm *txnManager) closeTxn(t *Txn, newState txnState) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if t.state != txnActive {
		return ErrTxnNotActive
	}
	delete(tm.active, t.id)
	t.state = newState
	return nil
}

// activeCount returns the number of in-flight transactions. Test-only.
func (tm *txnManager) activeCount() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.active)
}

// Txn is a handle to an in-progress transaction.
//
// Step 1 exposes only ID and lifecycle control. Transactional write
// methods (CreateNode, UpdateNodeProperty, etc.) land in the next
// commit. Once those land, non-transactional methods on TemporalStore
// will route through an implicit single-statement transaction so all
// writes share a consistent key format (Invariant I5).
type Txn struct {
	manager *txnManager
	id      int64
	state   txnState
}

// ID returns the monotonically-allocated transaction ID. Strictly
// increasing across begin() calls within a process and across restarts
// (Invariant I5).
func (t *Txn) ID() int64 { return t.id }

// Commit finalizes the transaction. In Step 1 no writes are staged, so
// commit is effectively a state transition plus removal from the
// active set.
func (t *Txn) Commit() error {
	return t.manager.closeTxn(t, txnCommitted)
}

// Rollback discards the transaction.
func (t *Txn) Rollback() error {
	return t.manager.closeTxn(t, txnAborted)
}

// ---- persistence ----

func persistNextTxnID(engine *core.StorageEngine, next int64) error {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(next))
	return engine.Put(core.CFMeta, []byte(metaNextTxnID), buf[:])
}

func loadNextTxnID(engine *core.StorageEngine) (int64, error) {
	raw, err := engine.Get(core.CFMeta, []byte(metaNextTxnID))
	if err != nil {
		return 0, err
	}
	if raw == nil {
		return 1, nil
	}
	if len(raw) != 8 {
		return 0, fmt.Errorf("malformed next_txn_id: %d bytes", len(raw))
	}
	return int64(binary.BigEndian.Uint64(raw)), nil
}
