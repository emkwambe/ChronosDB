// Package core owns the physical layout of ChronosDB's storage. This
// file defines the exact on-disk key encoding for every logical entity:
// nodes, edges, property history, shortcuts, secondary indexes, and
// metadata.
//
// The layout here is load-bearing. Queries, compaction, and the
// Predictive Graph Index all depend on the invariants documented in
// INVARIANTS.md in this package. Changing an encoder without updating
// INVARIANTS.md and the bitemporal test suite is a bug.
//
// Key anatomy:
//
//	<cf-prefix><component>[:<component>...]
//
// The CF prefix is one of the CF* constants in engine.go and identifies
// the logical column family. Within a CF, components are separated by
// ':' (0x3A). IDs must not contain ':'; the caller is responsible for
// that at ingest time — this is validated in the bitemporal test suite.
//
// All timestamps are microseconds since Unix epoch (int64). All txn IDs
// are monotonic int64 allocated by the transaction manager (Sprint 1.2);
// for tests and early-phase code, callers may supply txn IDs directly.
package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// KeySep is the byte separator between components within a key.
// Declared as a byte (not rune) so comparisons are unambiguous.
const KeySep byte = ':'

// Additional CF prefixes beyond the four originally defined in engine.go.
// Added here so downstream sprints (shortcuts in Phase 2, indexes in
// Phase 1, meta in Phase 1) have a stable namespace to target.
const (
	CFProperties = "ph:" // node property history: ph:<entity_id>:<prop>:<valid_from>:<txn_id>
	CFIndexes    = "ix:" // secondary indexes:     ix:<prop>:<value>:<entity_id>
	CFShortcuts  = "sc:" // PGI shortcuts:         sc:<pattern>:<src>:<dst>:<valid_from>:<txn_id>
	CFMeta       = "mt:" // schema, counters:      mt:<key>
)

// ErrInvalidID is returned when a supplied identifier contains the
// reserved separator byte. Validated once at encode time rather than
// propagating malformed keys into storage.
var ErrInvalidID = errors.New("identifier contains reserved separator ':'")

// validateID rejects IDs containing the reserved separator. Kept
// deliberately strict: the alternative (escape sequences) would
// introduce ambiguity in range scans.
func validateID(id string) error {
	if strings.IndexByte(id, KeySep) >= 0 {
		return fmt.Errorf("%w: %q", ErrInvalidID, id)
	}
	return nil
}

// NodeCurrentKey encodes the current-state key for a node.
//
//	nc:<id>
func NodeCurrentKey(id string) ([]byte, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	return []byte(CFNodesCurrent + id), nil
}

// NodeHistoryKey encodes one versioned row for a node. Keys within a
// given node sort first by valid_from, then by txn_id — this is
// required so that iteration under a common prefix returns versions in
// bitemporal order.
//
//	nh:<id>:<valid_from_be>:<txn_id_be>
//
// valid_from and txn_id are encoded as big-endian int64 so byte-order
// comparison matches numeric order.
func NodeHistoryKey(id string, validFrom, txnID int64) ([]byte, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	return buildHistoryKey(CFNodesHistory, id, validFrom, txnID), nil
}

// NodeHistoryPrefix returns the byte prefix under which all versions of
// a given node live. Suitable for Badger iterator seeks.
func NodeHistoryPrefix(id string) ([]byte, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	return []byte(CFNodesHistory + id + string(KeySep)), nil
}

// EdgeCurrentKey and EdgeHistoryKey mirror the node forms.
func EdgeCurrentKey(id string) ([]byte, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	return []byte(CFEdgesCurrent + id), nil
}

func EdgeHistoryKey(id string, validFrom, txnID int64) ([]byte, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	return buildHistoryKey(CFEdgesHistory, id, validFrom, txnID), nil
}

func EdgeHistoryPrefix(id string) ([]byte, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	return []byte(CFEdgesHistory + id + string(KeySep)), nil
}

// PropertyHistoryKey encodes one version of a single property on an
// entity. Separate column family so per-property history iteration is
// cheap without skipping unrelated properties.
//
//	ph:<entity_id>:<prop>:<valid_from_be>:<txn_id_be>
func PropertyHistoryKey(entityID, prop string, validFrom, txnID int64) ([]byte, error) {
	if err := validateID(entityID); err != nil {
		return nil, err
	}
	if err := validateID(prop); err != nil {
		return nil, err
	}
	buf := make([]byte, 0, len(CFProperties)+len(entityID)+len(prop)+2+16)
	buf = append(buf, CFProperties...)
	buf = append(buf, entityID...)
	buf = append(buf, KeySep)
	buf = append(buf, prop...)
	buf = append(buf, KeySep)
	buf = appendBigEndianInt64(buf, validFrom)
	buf = append(buf, KeySep)
	buf = appendBigEndianInt64(buf, txnID)
	return buf, nil
}

// ShortcutKey is the encoding for PGI-materialized shortcut edges.
// Kept here (rather than in a pgi package) so the storage invariants
// remain in one file.
//
//	sc:<pattern>:<src>:<dst>:<valid_from_be>:<txn_id_be>
func ShortcutKey(pattern, src, dst string, validFrom, txnID int64) ([]byte, error) {
	for _, s := range []string{pattern, src, dst} {
		if err := validateID(s); err != nil {
			return nil, err
		}
	}
	buf := make([]byte, 0, len(CFShortcuts)+len(pattern)+len(src)+len(dst)+3+16)
	buf = append(buf, CFShortcuts...)
	buf = append(buf, pattern...)
	buf = append(buf, KeySep)
	buf = append(buf, src...)
	buf = append(buf, KeySep)
	buf = append(buf, dst...)
	buf = append(buf, KeySep)
	buf = appendBigEndianInt64(buf, validFrom)
	buf = append(buf, KeySep)
	buf = appendBigEndianInt64(buf, txnID)
	return buf, nil
}

// MetaKey encodes a metadata lookup — used for schema, sequence counters,
// per-edge decay rates (Phase 5 candidate), and shortcut manifests.
func MetaKey(key string) []byte {
	return []byte(CFMeta + key)
}

// DecodeHistoryKey extracts the components of a node or edge history
// key. Returns the column-family prefix so callers can check they are
// reading from the expected CF.
func DecodeHistoryKey(key []byte) (cf string, id string, validFrom, txnID int64, err error) {
	switch {
	case bytes.HasPrefix(key, []byte(CFNodesHistory)):
		cf = CFNodesHistory
	case bytes.HasPrefix(key, []byte(CFEdgesHistory)):
		cf = CFEdgesHistory
	default:
		return "", "", 0, 0, fmt.Errorf("not a history key: %q", key)
	}
	body := key[len(cf):]

	// Body format: <id>:<valid_from:8B BE>:<txn_id:8B BE>
	if len(body) < 1+1+8+1+8 {
		return "", "", 0, 0, fmt.Errorf("history key too short: %d bytes", len(body))
	}
	sepOne := bytes.IndexByte(body, KeySep)
	if sepOne < 0 || len(body)-sepOne != 1+8+1+8 {
		return "", "", 0, 0, fmt.Errorf("malformed history key: %q", key)
	}
	id = string(body[:sepOne])
	vf := body[sepOne+1 : sepOne+1+8]
	if body[sepOne+1+8] != KeySep {
		return "", "", 0, 0, fmt.Errorf("malformed history key: missing sep before txn_id")
	}
	tx := body[sepOne+1+8+1:]

	validFrom = int64(binary.BigEndian.Uint64(vf))
	txnID = int64(binary.BigEndian.Uint64(tx))
	return cf, id, validFrom, txnID, nil
}

// buildHistoryKey is the shared encoder for node and edge history keys.
// Centralized so any layout change flips both at once.
func buildHistoryKey(cf, id string, validFrom, txnID int64) []byte {
	buf := make([]byte, 0, len(cf)+len(id)+2+16)
	buf = append(buf, cf...)
	buf = append(buf, id...)
	buf = append(buf, KeySep)
	buf = appendBigEndianInt64(buf, validFrom)
	buf = append(buf, KeySep)
	buf = appendBigEndianInt64(buf, txnID)
	return buf
}

// appendBigEndianInt64 appends val as 8 big-endian bytes. Big-endian
// ensures byte-order comparison matches numeric order for non-negative
// values, which is what both valid_from (microseconds since epoch) and
// txn_id (monotonic counter) always are.
func appendBigEndianInt64(buf []byte, val int64) []byte {
	var tmp [8]byte
	binary.BigEndian.PutUint64(tmp[:], uint64(val))
	return append(buf, tmp[:]...)
}
