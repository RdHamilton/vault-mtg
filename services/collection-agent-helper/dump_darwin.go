//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// RegionManifestEntry is one record in the manifest.json written by --dump-regions.
type RegionManifestEntry struct {
	RegionN int    `json:"region_n"`
	AddrHex string `json:"addr_hex"`
	Size    uint64 `json:"size_bytes"`
	File    string `json:"file"`
}

// runDumpRegions implements the --dump-regions <PID> <outdir> mode.
// It uses the same non-intrusive listReadableRegions + readMemory path as the
// production scanner. No process suspension, no debugger attachment.
func runDumpRegions(pid int, outdir string) error {
	outdir = filepath.Clean(outdir)
	if err := os.MkdirAll(outdir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", outdir, err)
	}

	task, err := openTask(pid)
	if err != nil {
		return fmt.Errorf("openTask pid=%d: %w", pid, err)
	}

	regions := listReadableRegions(task, minRegionSize)
	log.Printf("dump: found %d readable regions >= %d bytes in PID %d", len(regions), minRegionSize, pid)

	var manifest []RegionManifestEntry

	for n, r := range regions {
		filename := fmt.Sprintf("region_%04d_0x%x.bin", n, r.addr)
		fpath := filepath.Join(outdir, filename)

		var regionData []byte
		var scanned uint64
		for scanned < r.size {
			chunk := uint64(chunkSize)
			if rem := r.size - scanned; chunk > rem {
				chunk = rem
			}
			data, readErr := readMemory(task, r.addr+scanned, chunk)
			if readErr != nil {
				log.Printf("dump: region %d 0x%x+0x%x read error: %v (skipping chunk)", n, r.addr, scanned, readErr)
				scanned += chunk
				continue
			}
			regionData = append(regionData, data...)
			scanned += chunk
		}

		if len(regionData) == 0 {
			log.Printf("dump: region %d 0x%x: no bytes read, skipping", n, r.addr)
			continue
		}

		if err := os.WriteFile(fpath, regionData, 0o640); err != nil {
			return fmt.Errorf("write region %d: %w", n, err)
		}
		log.Printf("dump: wrote region %04d 0x%x size=%d -> %s", n, r.addr, len(regionData), filename)

		manifest = append(manifest, RegionManifestEntry{
			RegionN: n,
			AddrHex: fmt.Sprintf("0x%x", r.addr),
			Size:    uint64(len(regionData)),
			File:    filename,
		})
	}

	manifestPath := filepath.Join(outdir, "manifest.json")
	mf, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return fmt.Errorf("create manifest: %w", err)
	}
	enc := json.NewEncoder(mf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		_ = mf.Close()
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := mf.Close(); err != nil {
		return fmt.Errorf("close manifest: %w", err)
	}

	log.Printf("dump: complete — %d regions written to %s, manifest at %s", len(manifest), outdir, manifestPath)
	log.Printf("IMPORTANT: delete %s after analysis — contains raw process memory (PII)", outdir)
	return nil
}
