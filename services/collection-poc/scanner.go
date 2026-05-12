package main

import "encoding/binary"

const (
	minGRPID = 1_000
	maxGRPID = 2_000_000
	minQty   = 1
	maxQty   = 8
	maxNext  = 100_000
)

// ScanDictEntries scans a byte slice for C# Dictionary<int,int> _entries.
//
// Each entry is 16 bytes: [int32 hashCode][int32 next][int32 key][int32 value]
// For int keys: hashCode == key. next == -1 means end of hash chain.
// key is the MTGA GRP card ID; value is the owned quantity.
func ScanDictEntries(data []byte) map[int]int {
	collection := make(map[int]int)
	n := len(data) / 16
	for i := 0; i < n; i++ {
		b := data[i*16 : i*16+16]
		hashCode := int32(binary.LittleEndian.Uint32(b[0:4]))
		nextIdx := int32(binary.LittleEndian.Uint32(b[4:8]))
		key := int32(binary.LittleEndian.Uint32(b[8:12]))
		value := int32(binary.LittleEndian.Uint32(b[12:16]))

		if hashCode != key {
			continue
		}
		if key <= minGRPID || key >= maxGRPID {
			continue
		}
		if value < minQty || value > maxQty {
			continue
		}
		if nextIdx != -1 && (nextIdx < 0 || nextIdx >= maxNext) {
			continue
		}

		id, qty := int(key), int(value)
		if existing, ok := collection[id]; !ok || qty > existing {
			collection[id] = qty
		}
	}
	return collection
}
