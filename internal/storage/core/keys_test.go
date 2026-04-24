package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestNodeCurrentKey(t *testing.T) {
	k, err := NodeCurrentKey("cust_000042")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []byte("nc:cust_000042")
	if !bytes.Equal(k, want) {
		t.Fatalf("got %q, want %q", k, want)
	}
}

func TestIDWithSeparatorRejected(t *testing.T) {
	if _, err := NodeCurrentKey("has:colon"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
	if _, err := NodeHistoryKey("has:colon", 1, 1); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("expected ErrInvalidID, got %v", err)
	}
}

// TestHistoryKeyRoundTrip asserts invariant I3 (timestamp encoding)
// and I7 (key ordering). Encode -> decode -> values preserved.
func TestHistoryKeyRoundTrip(t *testing.T) {
	cases := []struct {
		id        string
		validFrom int64
		txnID     int64
	}{
		{"a", 0, 0},
		{"node_123", 1_700_000_000_000_000, 42},
		{"x", 1<<62 - 1, 1 << 31},
	}
	for _, c := range cases {
		k, err := NodeHistoryKey(c.id, c.validFrom, c.txnID)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		cf, id, vf, tx, err := DecodeHistoryKey(k)
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if cf != CFNodesHistory || id != c.id || vf != c.validFrom || tx != c.txnID {
			t.Fatalf("round-trip drift: cf=%s id=%s vf=%d tx=%d (want %s %d %d)",
				cf, id, vf, tx, c.id, c.validFrom, c.txnID)
		}
	}
}

// TestHistoryKeyOrdering asserts invariant I7: iteration under a common
// prefix yields versions in (valid_from, txn_id) order because keys are
// encoded big-endian.
func TestHistoryKeyOrdering(t *testing.T) {
	earlier, _ := NodeHistoryKey("n1", 100, 1)
	laterSameVF, _ := NodeHistoryKey("n1", 100, 2)
	evenLater, _ := NodeHistoryKey("n1", 200, 1)

	if bytes.Compare(earlier, laterSameVF) >= 0 {
		t.Fatalf("expected earlier < laterSameVF")
	}
	if bytes.Compare(laterSameVF, evenLater) >= 0 {
		t.Fatalf("expected laterSameVF < evenLater")
	}
}

func TestHistoryKeyPrefixMatches(t *testing.T) {
	k, _ := NodeHistoryKey("n1", 100, 1)
	p, _ := NodeHistoryPrefix("n1")
	if !bytes.HasPrefix(k, p) {
		t.Fatalf("history key %q does not start with prefix %q", k, p)
	}
	// Prefix for a different id should not match.
	other, _ := NodeHistoryPrefix("n2")
	if bytes.HasPrefix(k, other) {
		t.Fatalf("prefix collision between n1 key and n2 prefix")
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	if _, _, _, _, err := DecodeHistoryKey([]byte("xy:garbage")); err == nil {
		t.Fatal("expected error on non-history CF prefix")
	}
	short := []byte("nh:id:")
	if _, _, _, _, err := DecodeHistoryKey(short); err == nil {
		t.Fatal("expected error on short body")
	}
}

func TestPropertyHistoryKey(t *testing.T) {
	k, err := PropertyHistoryKey("n1", "balance", 123, 7)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	// ph:n1:balance:<8B BE 123>:<8B BE 7>
	wantPrefix := []byte("ph:n1:balance:")
	if !bytes.HasPrefix(k, wantPrefix) {
		t.Fatalf("got %q, want prefix %q", k, wantPrefix)
	}
	body := k[len(wantPrefix):]
	if len(body) != 8+1+8 {
		t.Fatalf("unexpected body length %d", len(body))
	}
	if v := int64(binary.BigEndian.Uint64(body[:8])); v != 123 {
		t.Fatalf("got valid_from=%d, want 123", v)
	}
	if v := int64(binary.BigEndian.Uint64(body[9:])); v != 7 {
		t.Fatalf("got txn_id=%d, want 7", v)
	}
}

func TestShortcutKey(t *testing.T) {
	k, err := ShortcutKey("Customer_to_Product", "cust_1", "prod_5", 1_000, 2)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !bytes.HasPrefix(k, []byte("sc:Customer_to_Product:cust_1:prod_5:")) {
		t.Fatalf("unexpected shortcut key: %q", k)
	}
}

func TestMetaKey(t *testing.T) {
	if got, want := string(MetaKey("shortcut_manifest")), "mt:shortcut_manifest"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
