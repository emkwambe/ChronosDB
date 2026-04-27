package temporal

import (
	"encoding/binary"
	"encoding/json"
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

// ErrWriteConflict is returned by Commit when another transaction
// committed a conflicting write to the same entity after this txn's
// snapshot was taken (first-committer-wins). The txn is left in
// txnAborted state and its writes are discarded.
var ErrWriteConflict = errors.New("write conflict")

// ErrNodeNotFound surfaces when a transactional update targets a node
// that has not been created. Returned synchronously from
// UpdateNodeProperty so the caller can react before Commit.
var ErrNodeNotFound = errors.New("node not found")

// txnManager allocates monotonic transaction IDs, persists the counter
// so IDs never reuse across process restarts, and tracks in-flight
// transactions for snapshot-isolation conflict detection.
//
// Goroutine-safe. Owned by TemporalStore; not exposed directly to
// callers.
type txnManager struct {
	engine *core.StorageEngine

	mu     sync.Mutex
	next   int64
	active map[int64]*Txn

	// lastCommittedTs is the highest txn ID that has reached committed
	// state. Used to set snapshotTs at BeginTx so that a starting txn
	// only sees the writes that committed before it began.
	lastCommittedTs int64

	// lastCommittedWrite tracks, per entity touched by any committed
	// txn, the highest committing txn's ID. At commit time, a txn T
	// aborts if any entity in its write set has lastCommittedWrite >
	// T.snapshotTs — the classic SI first-committer-wins rule.
	lastCommittedWrite map[string]int64
}

// newTxnManager loads the persisted next-id counter (or initializes to
// 1 if none is stored).
func newTxnManager(engine *core.StorageEngine) (*txnManager, error) {
	next, err := loadNextTxnID(engine)
	if err != nil {
		return nil, fmt.Errorf("txn manager init: %w", err)
	}
	return &txnManager{
		engine:             engine,
		next:               next,
		active:             make(map[int64]*Txn),
		lastCommittedWrite: make(map[string]int64),
		// lastCommittedTs is left at zero on a cold start. Until any
		// txn commits, every begin() sees snapshotTs = 0, which is
		// correct: there are no committed writes for them to conflict
		// with. Across restart, in-memory conflict bookkeeping is
		// reset; that is acceptable today because no other process
		// holds an active txn at the moment of restart. Crash-recovery
		// of in-flight conflicts arrives with the WAL in S1.1.3.
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

	txn := &Txn{
		manager:    tm,
		id:         id,
		snapshotTs: tm.lastCommittedTs,
		state:      txnActive,
		entities:   make(map[string]struct{}),
	}
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

// Txn is a handle to an in-progress transaction. Write methods stage
// operations into ops; nothing reaches storage until Commit. Reads
// (other than the ones implicit in UpdateNodeProperty) are not yet
// snapshot-isolated — that lands together with the history-scan
// fallback in Sprint 1.2.
type Txn struct {
	manager    *txnManager
	id         int64
	snapshotTs int64
	state      txnState

	// entities is the set of entity keys (e.g. "n:cust_001",
	// "e:edge_5") this txn has written to. Drives conflict detection.
	entities map[string]struct{}

	// ops is the ordered list of KV writes to apply atomically at
	// commit. Order does not affect correctness — Badger commits the
	// whole batch — but is preserved for predictable diagnostics.
	ops []core.BatchOp
}

// ID returns the monotonically-allocated transaction ID. Strictly
// increasing across begin() calls within a process and across restarts
// (Invariant I5).
func (t *Txn) ID() int64 { return t.id }

// CreateNode stages a node creation. The full Node JSON is written to
// both the current-state CF (for fast lookup) and the history CF (the
// authoritative version) at commit time.
//
// validTo == 0 means "unbounded future".
func (t *Txn) CreateNode(id string, labels []string, props map[string]any, validFrom, validTo int64) error {
	if t.state != txnActive {
		return ErrTxnNotActive
	}
	if props == nil {
		props = map[string]any{}
	}

	node := Node{
		ID:         id,
		Labels:     labels,
		Properties: props,
		TimeRange:  TimeRange{ValidFrom: validFrom, ValidTo: validTo},
	}
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node: %w", err)
	}

	curKey, err := core.NodeCurrentKey(id)
	if err != nil {
		return err
	}
	histKey, err := core.NodeHistoryKey(id, validFrom, t.id)
	if err != nil {
		return err
	}

	t.stage("n:"+id,
		core.BatchOp{CF: "", Key: curKey, Value: data},
		core.BatchOp{CF: "", Key: histKey, Value: data},
	)
	return nil
}

// UpdateNodeProperty stages a property update on an existing node at
// validFrom. At commit, two history rows are written:
//
//   - a closing row for the prior version with valid_to = validFrom
//   - an opening row for the new version with valid_from = validFrom,
//     valid_to = 0
//
// The current-state CF row is overwritten with the new version. Both
// closing and opening history rows carry this txn's ID, so a later
// reader iterating nh:<id>: in (valid_from, txn_id) order replays the
// timeline correctly.
//
// Reads-during-write currently consult only the current-state CF (not
// snapshot-isolated). If a concurrent txn updated the same node after
// our snapshot, this txn's commit will detect the conflict and abort.
func (t *Txn) UpdateNodeProperty(id string, prop string, value any, validFrom int64) error {
	if t.state != txnActive {
		return ErrTxnNotActive
	}

	curKey, err := core.NodeCurrentKey(id)
	if err != nil {
		return err
	}
	raw, err := t.manager.engine.Get(core.CFNodesCurrent, []byte(id))
	if err != nil {
		return fmt.Errorf("read current node: %w", err)
	}
	if raw == nil {
		return ErrNodeNotFound
	}

	var prior Node
	if err := json.Unmarshal(raw, &prior); err != nil {
		return fmt.Errorf("unmarshal current node: %w", err)
	}

	// Closing row: same labels/props as prior, with valid_to set to
	// validFrom (the half-open upper bound is the new version's
	// valid_from per INVARIANTS.md I4).
	closing := prior
	closing.TimeRange.ValidTo = validFrom
	closingData, err := json.Marshal(closing)
	if err != nil {
		return fmt.Errorf("marshal closing version: %w", err)
	}
	closingKey, err := core.NodeHistoryKey(id, prior.TimeRange.ValidFrom, t.id)
	if err != nil {
		return err
	}

	// Opening row: copy of prior with the property updated.
	opening := Node{
		ID:         prior.ID,
		Labels:     prior.Labels,
		Properties: copyProps(prior.Properties),
		TimeRange:  TimeRange{ValidFrom: validFrom, ValidTo: 0},
	}
	opening.Properties[prop] = value
	openingData, err := json.Marshal(opening)
	if err != nil {
		return fmt.Errorf("marshal opening version: %w", err)
	}
	openingKey, err := core.NodeHistoryKey(id, validFrom, t.id)
	if err != nil {
		return err
	}

	t.stage("n:"+id,
		core.BatchOp{Key: closingKey, Value: closingData},
		core.BatchOp{Key: openingKey, Value: openingData},
		core.BatchOp{Key: curKey, Value: openingData},
	)
	return nil
}

// CreateEdge stages an edge creation. Symmetric to CreateNode.
func (t *Txn) CreateEdge(id, edgeType, sourceID, targetID string, props map[string]any, validFrom, validTo int64) error {
	if t.state != txnActive {
		return ErrTxnNotActive
	}
	if props == nil {
		props = map[string]any{}
	}

	edge := Edge{
		ID:         id,
		Type:       edgeType,
		SourceID:   sourceID,
		TargetID:   targetID,
		Properties: props,
		TimeRange:  TimeRange{ValidFrom: validFrom, ValidTo: validTo},
	}
	data, err := json.Marshal(edge)
	if err != nil {
		return fmt.Errorf("marshal edge: %w", err)
	}

	curKey, err := core.EdgeCurrentKey(id)
	if err != nil {
		return err
	}
	histKey, err := core.EdgeHistoryKey(id, validFrom, t.id)
	if err != nil {
		return err
	}

	t.stage("e:"+id,
		core.BatchOp{Key: curKey, Value: data},
		core.BatchOp{Key: histKey, Value: data},
	)
	return nil
}

// stage records ops with their owning entity in one place. Keeping the
// helper centralized makes it easy to reason about which paths produce
// pending writes.
func (t *Txn) stage(entityKey string, ops ...core.BatchOp) {
	t.entities[entityKey] = struct{}{}
	// Some callers leave op.CF empty when the key already includes the
	// CF prefix (CreateNode does this for both current and history
	// rows so we can build the keys once via core.keys helpers). Strip
	// the prefix into op.CF so ApplyBatch produces the right full key.
	for i, op := range ops {
		if op.CF == "" {
			op.CF, op.Key = splitFirstCF(op.Key)
			ops[i] = op
		}
	}
	t.ops = append(t.ops, ops...)
}

// Commit applies all staged writes atomically after checking for
// conflicts. On conflict the txn is aborted and ErrWriteConflict is
// returned; staged writes are discarded.
func (t *Txn) Commit() error {
	tm := t.manager
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t.state != txnActive {
		return ErrTxnNotActive
	}

	// First-committer-wins: any entity we wrote that another txn
	// committed after our snapshot was taken? If so, we lose.
	for entity := range t.entities {
		if last, ok := tm.lastCommittedWrite[entity]; ok && last > t.snapshotTs {
			delete(tm.active, t.id)
			t.state = txnAborted
			t.ops = nil
			return ErrWriteConflict
		}
	}

	if err := tm.engine.ApplyBatch(t.ops); err != nil {
		delete(tm.active, t.id)
		t.state = txnAborted
		t.ops = nil
		return fmt.Errorf("apply batch: %w", err)
	}

	for entity := range t.entities {
		tm.lastCommittedWrite[entity] = t.id
	}
	if t.id > tm.lastCommittedTs {
		tm.lastCommittedTs = t.id
	}

	delete(tm.active, t.id)
	t.state = txnCommitted
	t.ops = nil
	return nil
}

// Rollback discards the transaction and any staged writes.
func (t *Txn) Rollback() error {
	t.ops = nil
	return t.manager.closeTxn(t, txnAborted)
}

// ---- helpers ----

// splitFirstCF separates a "cf:body" key produced by core/keys.go into
// its CF prefix and body. Every CF defined in core is exactly two
// alphanumeric characters followed by the ':' separator (see
// INVARIANTS.md §I7).
func splitFirstCF(fullKey []byte) (cf string, body []byte) {
	if len(fullKey) >= 3 && fullKey[2] == core.KeySep {
		return string(fullKey[:3]), fullKey[3:]
	}
	return "", fullKey
}

func copyProps(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
