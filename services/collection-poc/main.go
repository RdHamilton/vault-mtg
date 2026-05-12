//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
)

const (
	minRegionSize = 4 * 1024 * 1024 // 4MB — skip tiny regions
	chunkSize     = 4 * 1024 * 1024 // read in 4MB chunks to bound per-call memory
	minDensity    = 100             // a region with fewer entries than this isn't the collection
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: sudo %s <pid>\n", os.Args[0])
		os.Exit(1)
	}
	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid pid: %v\n", err)
		os.Exit(1)
	}

	task, err := openTask(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "attach: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Attached to PID %d\n", pid)

	regions := listReadableRegions(task, minRegionSize)
	fmt.Printf("Scanning %d readable regions >= 4MB...\n", len(regions))

	// Merge results across all regions; take max quantity when an ID appears in multiple.
	collection := make(map[int]int)

	for _, r := range regions {
		regionEntries := make(map[int]int)
		var scanned uint64
		for scanned < r.size {
			chunk := uint64(chunkSize)
			if remaining := r.size - scanned; chunk > remaining {
				chunk = remaining
			}
			data, err := readMemory(task, r.addr+scanned, chunk)
			if err != nil {
				scanned += chunk
				continue
			}
			for id, qty := range ScanDictEntries(data) {
				if existing, ok := regionEntries[id]; !ok || qty > existing {
					regionEntries[id] = qty
				}
			}
			scanned += chunk
		}

		if len(regionEntries) >= minDensity {
			fmt.Printf("  0x%010x  size=%-6dMB  entries=%d\n",
				r.addr, r.size/1024/1024, len(regionEntries))
			for id, qty := range regionEntries {
				if existing, ok := collection[id]; !ok || qty > existing {
					collection[id] = qty
				}
			}
		}
	}

	ids := make([]int, 0, len(collection))
	for id := range collection {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	total := 0
	for _, qty := range collection {
		total += qty
	}

	fmt.Printf("\n=== COLLECTION ===\n")
	fmt.Printf("Unique GRP IDs : %d\n", len(collection))
	fmt.Printf("Total copies   : %d\n", total)
	fmt.Printf("\nFirst 50 entries:\n")
	lim := 50
	if len(ids) < lim {
		lim = len(ids)
	}
	for _, id := range ids[:lim] {
		fmt.Printf("  %d -> %d\n", id, collection[id])
	}

	outPath := "/tmp/collection_go.json"
	f, err := os.Create(outPath)
	if err == nil {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		_ = enc.Encode(collection)
		f.Close()
		fmt.Printf("\nSaved to %s\n", outPath)
	}
}
