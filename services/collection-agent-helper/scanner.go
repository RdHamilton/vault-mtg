package main

import "github.com/RdHamilton/vault-mtg/services/collection-agent-helper/internal/scanner"

const (
	minGRPID = scanner.MinGRPID
	maxGRPID = scanner.MaxGRPID
	minQty   = scanner.MinQty
	maxQty   = scanner.MaxQty
	maxNext  = scanner.MaxNext
)

// scanDictEntries scans a byte slice for C# Dictionary<int,int> _entries.
// Each entry is 16 bytes: [int32 hashCode][int32 next][int32 key][int32 value].
// For int keys: hashCode == key. key is the MTGA GRP card ID; value is owned quantity.
func scanDictEntries(data []byte) map[int]int {
	return scanner.ScanDictEntries(data)
}
