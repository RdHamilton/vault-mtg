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
	minRegionSize = 4 * 1024 * 1024 // skip regions smaller than 4MB
	chunkSize     = 4 * 1024 * 1024 // read in 4MB chunks to bound per-call memory
	minEntries    = 500             // minimum entries to treat a region as a candidate
	// maxFillPct caps fill rate at 3%. The C# collection Dictionary is sparsely filled
	// (~1%) because .NET over-allocates buckets. Dense dictionaries (card pool DB, etc.)
	// exceed this and must be excluded.
	maxFillPct = 3.0
)

type candidate struct {
	addr    uint64
	size    uint64
	entries map[int]int
	fillPct float64
}

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

	var candidates []candidate

	for _, r := range regions {
		entries := make(map[int]int)
		var scanned uint64
		for scanned < r.size {
			chunk := uint64(chunkSize)
			if rem := r.size - scanned; chunk > rem {
				chunk = rem
			}
			data, readErr := readMemory(task, r.addr+scanned, chunk)
			if readErr != nil {
				scanned += chunk
				continue
			}
			for id, qty := range ScanDictEntries(data) {
				if existing, ok := entries[id]; !ok || qty > existing {
					entries[id] = qty
				}
			}
			scanned += chunk
		}

		fillPct := 100 * float64(len(entries)) / float64(r.size/16)

		switch {
		case len(entries) < minEntries:
			// too sparse to be the collection
		case fillPct > maxFillPct:
			fmt.Printf("  0x%010x  size=%-6dMB  entries=%-6d  fill=%.2f%%  SKIP (card pool DB?)\n",
				r.addr, r.size/1024/1024, len(entries), fillPct)
		default:
			fmt.Printf("  0x%010x  size=%-6dMB  entries=%-6d  fill=%.2f%%  candidate\n",
				r.addr, r.size/1024/1024, len(entries), fillPct)
			candidates = append(candidates, candidate{r.addr, r.size, entries, fillPct})
		}
	}

	if len(candidates) == 0 {
		fmt.Fprintln(os.Stderr, "no collection region found")
		os.Exit(1)
	}

	// The live Dictionary always has more entries than any stale resize-copy.
	// Pick the candidate with the highest entry count.
	best := candidates[0]
	for _, c := range candidates[1:] {
		if len(c.entries) > len(best.entries) {
			best = c
		}
	}
	fmt.Printf("\nSelected 0x%010x  (%d entries, %.2f%% fill)\n",
		best.addr, len(best.entries), best.fillPct)

	ids := make([]int, 0, len(best.entries))
	for id := range best.entries {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	total := 0
	for _, qty := range best.entries {
		total += qty
	}

	fmt.Printf("\n=== COLLECTION ===\n")
	fmt.Printf("Unique GRP IDs : %d\n", len(best.entries))
	fmt.Printf("Total copies   : %d\n", total)
	fmt.Printf("\nFirst 50 entries:\n")
	lim := 50
	if len(ids) < lim {
		lim = len(ids)
	}
	for _, id := range ids[:lim] {
		fmt.Printf("  %d -> %d\n", id, best.entries[id])
	}

	outPath := "/tmp/collection_go.json"
	f, createErr := os.Create(outPath)
	if createErr == nil {
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		encErr := enc.Encode(best.entries)
		closeErr := f.Close()
		switch {
		case encErr != nil:
			fmt.Fprintf(os.Stderr, "error: encode %s: %v\n", outPath, encErr)
			_ = os.Remove(outPath)
		case closeErr != nil:
			fmt.Fprintf(os.Stderr, "error: close %s: %v\n", outPath, closeErr)
		default:
			fmt.Printf("\nSaved to %s\n", outPath)
		}
	}
}
