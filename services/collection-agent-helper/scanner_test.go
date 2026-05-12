package main

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
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
	got := scanDictEntries(data)
	assert.Equal(t, map[int]int{96804: 3}, got)
}

func TestScanDictEntries_EmptyInput(t *testing.T) {
	assert.Empty(t, scanDictEntries(nil))
	assert.Empty(t, scanDictEntries([]byte{}))
}

func TestScanDictEntries_PartialRecord(t *testing.T) {
	// 15 bytes — not a full 16-byte record, should produce no results.
	assert.Empty(t, scanDictEntries(make([]byte, 15)))
}

func TestScanDictEntries_HashCodeMismatch(t *testing.T) {
	data := makeEntry(99999, -1, 96804, 3)
	assert.Empty(t, scanDictEntries(data))
}

func TestScanDictEntries_KeyAtMinBoundary(t *testing.T) {
	data := makeEntry(minGRPID, -1, minGRPID, 1)
	got := scanDictEntries(data)
	assert.Equal(t, map[int]int{minGRPID: 1}, got)
}

func TestScanDictEntries_KeyAtMaxBoundary(t *testing.T) {
	data := makeEntry(maxGRPID, -1, maxGRPID, 1)
	got := scanDictEntries(data)
	assert.Equal(t, map[int]int{maxGRPID: 1}, got)
}

func TestScanDictEntries_KeyBelowMin(t *testing.T) {
	key := int32(minGRPID - 1)
	data := makeEntry(key, -1, key, 1)
	assert.Empty(t, scanDictEntries(data))
}

func TestScanDictEntries_KeyAboveMax(t *testing.T) {
	key := int32(maxGRPID + 1)
	data := makeEntry(key, -1, key, 1)
	assert.Empty(t, scanDictEntries(data))
}

func TestScanDictEntries_QuantityTooLow(t *testing.T) {
	data := makeEntry(96804, -1, 96804, int32(minQty-1))
	assert.Empty(t, scanDictEntries(data))
}

func TestScanDictEntries_QuantityTooHigh(t *testing.T) {
	data := makeEntry(96804, -1, 96804, int32(maxQty+1))
	assert.Empty(t, scanDictEntries(data))
}

func TestScanDictEntries_BadNextIdx(t *testing.T) {
	data := makeEntry(96804, int32(maxNext+1), 96804, 3)
	assert.Empty(t, scanDictEntries(data))
}

func TestScanDictEntries_DuplicateKeyKeepsHigherQty(t *testing.T) {
	var data []byte
	data = append(data, makeEntry(96804, -1, 96804, 2)...)
	data = append(data, makeEntry(96804, -1, 96804, 4)...)
	got := scanDictEntries(data)
	assert.Equal(t, map[int]int{96804: 4}, got)
}

func TestScanDictEntries_MultipleValidEntries(t *testing.T) {
	var data []byte
	data = append(data, makeEntry(96804, -1, 96804, 3)...)
	data = append(data, makeEntry(100500, -1, 100500, 1)...)
	got := scanDictEntries(data)
	assert.Equal(t, map[int]int{96804: 3, 100500: 1}, got)
}
