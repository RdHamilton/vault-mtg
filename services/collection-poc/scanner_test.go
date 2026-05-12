package main

import (
	"encoding/binary"
	"testing"
)

func makeEntry(hashCode, next, key, value int32) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint32(b[0:4], uint32(hashCode))
	binary.LittleEndian.PutUint32(b[4:8], uint32(next))
	binary.LittleEndian.PutUint32(b[8:12], uint32(key))
	binary.LittleEndian.PutUint32(b[12:16], uint32(value))
	return b
}

func TestScanDictEntries_ValidEntry(t *testing.T) {
	data := makeEntry(96804, -1, 96804, 3)
	got := ScanDictEntries(data)
	if got[96804] != 3 {
		t.Fatalf("expected 96804->3, got %v", got)
	}
}

func TestScanDictEntries_HashCodeMismatch(t *testing.T) {
	data := makeEntry(99999, -1, 96804, 3)
	got := ScanDictEntries(data)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestScanDictEntries_InvalidQuantity(t *testing.T) {
	data := append(makeEntry(96804, -1, 96804, 0), makeEntry(96805, -1, 96805, 9)...)
	got := ScanDictEntries(data)
	if len(got) != 0 {
		t.Fatalf("expected empty for qty 0 and 9, got %v", got)
	}
}

func TestScanDictEntries_KeyOutOfRange(t *testing.T) {
	data := append(makeEntry(500, -1, 500, 1), makeEntry(3000000, -1, 3000000, 1)...)
	got := ScanDictEntries(data)
	if len(got) != 0 {
		t.Fatalf("expected empty for out-of-range keys, got %v", got)
	}
}

func TestScanDictEntries_TakesMaxQuantity(t *testing.T) {
	// Same GRP ID seen with qty 2 and then qty 4 — should keep 4.
	data := append(makeEntry(96804, -1, 96804, 2), makeEntry(96804, -1, 96804, 4)...)
	got := ScanDictEntries(data)
	if got[96804] != 4 {
		t.Fatalf("expected 96804->4, got %v", got)
	}
}

func TestScanDictEntries_MultipleValidEntries(t *testing.T) {
	var data []byte
	data = append(data, makeEntry(96819, -1, 96819, 4)...)
	data = append(data, makeEntry(96804, -1, 96804, 3)...)
	data = append(data, makeEntry(96580, 2616, 96580, 1)...) // non-(-1) next is valid
	got := ScanDictEntries(data)
	if got[96819] != 4 || got[96804] != 3 || got[96580] != 1 {
		t.Fatalf("unexpected result: %v", got)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
}

func TestScanDictEntries_EmptyInput(t *testing.T) {
	got := ScanDictEntries(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestScanDictEntries_PartialEntry(t *testing.T) {
	// 12 bytes — not a complete 16-byte entry, should be ignored.
	data := makeEntry(96804, -1, 96804, 3)[:12]
	got := ScanDictEntries(data)
	if len(got) != 0 {
		t.Fatalf("expected empty for partial entry, got %v", got)
	}
}
