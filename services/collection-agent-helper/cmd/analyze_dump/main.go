// Command analyze_dump runs scanDictEntries against each region .bin captured by
// `collection-helper --dump-regions` and prints per-region entry counts, fillPct,
// and a sample of GRP IDs. Use this to resolve H1 vs H2 without live MTGA contact.
//
// Usage:
//
//	go run ./cmd/analyze_dump <outdir> <outdir>/manifest.json
//
// H1 (region filter too strict): at least one region shows >= 500 entries at <= 3% fill.
// H2 (Unity layout drift): no region shows any matches — inspect bytes around a known GRP ID.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/RdHamilton/vault-mtg/services/collection-agent-helper/internal/scanner"
)

type manifestEntry struct {
	RegionN int    `json:"region_n"`
	AddrHex string `json:"addr_hex"`
	Size    uint64 `json:"size_bytes"`
	File    string `json:"file"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: analyze_dump <outdir> <manifest.json>")
		os.Exit(1)
	}
	outdir := os.Args[1]
	manifestPath := os.Args[2]

	mf, err := os.Open(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open manifest: %v\n", err)
		os.Exit(1)
	}
	var entries []manifestEntry
	if err := json.NewDecoder(mf).Decode(&entries); err != nil {
		_ = mf.Close()
		fmt.Fprintf(os.Stderr, "decode manifest: %v\n", err)
		os.Exit(1)
	}
	_ = mf.Close()

	fmt.Printf("%-6s  %-18s  %-12s  %-8s  %-8s  %s\n",
		"REGION", "ADDR", "SIZE_MB", "ENTRIES", "FILL%", "SAMPLE_GRP_IDS")
	fmt.Println("------  ------------------  ------------  --------  --------  -------------------------")

	var totalEntries int
	var bestRegion *manifestEntry
	var bestEntries map[int]int

	for i := range entries {
		e := &entries[i]
		binPath := filepath.Join(outdir, e.File)
		data, err := os.ReadFile(binPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", binPath, err)
			continue
		}

		got := scanner.ScanDictEntries(data)
		sizeMB := float64(e.Size) / (1024 * 1024)

		var fillPct float64
		if e.Size >= 16 {
			fillPct = 100 * float64(len(got)) / float64(e.Size/16)
		}

		// Collect sorted sample GRP IDs (first 10).
		ids := make([]int, 0, len(got))
		for id := range got {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		sample := ids
		if len(sample) > 10 {
			sample = sample[:10]
		}

		fmt.Printf("%-6d  %-18s  %-12.2f  %-8d  %-8.4f  %v\n",
			e.RegionN, e.AddrHex, sizeMB, len(got), fillPct, sample)

		totalEntries += len(got)
		if bestRegion == nil || len(got) > len(bestEntries) {
			bestRegion = e
			bestEntries = got
		}
	}

	fmt.Printf("\nTotal entries across all regions: %d\n", totalEntries)
	if bestRegion != nil && len(bestEntries) > 0 {
		fmt.Printf("Best region: %s (region_%04d) — %d entries\n",
			bestRegion.AddrHex, bestRegion.RegionN, len(bestEntries))
	}

	// H1/H2 verdict
	fmt.Println()
	if bestRegion != nil && len(bestEntries) >= 500 {
		fmt.Println("VERDICT: H1 — region has >= 500 entries but was filtered out by minEntries/maxFillPct.")
		fmt.Println("  -> Check actual fillPct above. Adjust minEntries or maxFillPct in mem_darwin.go.")
	} else if totalEntries > 0 && (bestRegion == nil || len(bestEntries) < 500) {
		fmt.Println("VERDICT: H1 (partial) — entries found but below minEntries=500 threshold.")
		fmt.Printf("  -> Best region has %d entries. Lower minEntries or check maxFillPct cap.\n", len(bestEntries))
	} else {
		fmt.Println("VERDICT: H2 (likely) — no entries found in any region.")
		fmt.Println("  -> Unity dictionary layout may have changed.")
		fmt.Println("  -> Search a .bin for a known GRP ID in little-endian to inspect stride.")
		fmt.Println("     Example: card 96804 = 0x17A64 -> bytes: 64 7A 01 00")
	}
}
