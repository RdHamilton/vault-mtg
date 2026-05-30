//go:build darwin

package main

/*
#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <stdlib.h>

static mach_port_t selfTask() { return mach_task_self(); }
static void deallocTaskPort(mach_port_t task) { mach_port_deallocate(mach_task_self(), task); }

static kern_return_t vmRead(mach_port_t task, uint64_t addr, uint64_t size,
                             void **outPtr, uint32_t *outSize) {
	vm_offset_t data = 0;
	mach_msg_type_number_t cnt = 0;
	kern_return_t kr = mach_vm_read(task,
		(mach_vm_address_t)addr,
		(mach_vm_size_t)size,
		&data, &cnt);
	if (kr == KERN_SUCCESS) {
		*outPtr = (void *)data;
		*outSize = (uint32_t)cnt;
	}
	return kr;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

func openTask(pid int) (C.mach_port_t, error) {
	self := C.selfTask()
	var task C.mach_port_t
	if kr := C.task_for_pid(self, C.int(pid), &task); kr != C.KERN_SUCCESS {
		return 0, fmt.Errorf("task_for_pid failed: kern_return_t=%d (must run as root)", kr)
	}
	return task, nil
}

type vmRegion struct {
	addr uint64
	size uint64
}

func listReadableRegions(task C.mach_port_t, minSize uint64) []vmRegion {
	var regions []vmRegion
	addr := C.mach_vm_address_t(1)
	for {
		var size C.mach_vm_size_t
		var info C.vm_region_basic_info_data_64_t
		count := C.mach_msg_type_number_t(C.VM_REGION_BASIC_INFO_COUNT_64)
		var objName C.mach_port_t

		kr := C.mach_vm_region(
			task,
			&addr,
			&size,
			C.VM_REGION_BASIC_INFO_64,
			C.vm_region_info_t(unsafe.Pointer(&info)),
			&count,
			&objName,
		)
		if kr != C.KERN_SUCCESS {
			break
		}
		if uint64(size) >= minSize && (info.protection&C.VM_PROT_READ) != 0 {
			regions = append(regions, vmRegion{addr: uint64(addr), size: uint64(size)})
		}
		addr += size
	}
	return regions
}

func readMemory(task C.mach_port_t, addr, size uint64) ([]byte, error) {
	var ptr unsafe.Pointer
	var sz C.uint32_t
	if kr := C.vmRead(task, C.uint64_t(addr), C.uint64_t(size), &ptr, &sz); kr != C.KERN_SUCCESS {
		return nil, fmt.Errorf("mach_vm_read addr=0x%x: kern_return_t=%d", addr, kr)
	}
	data := C.GoBytes(ptr, C.int(sz))
	C.vm_deallocate(C.selfTask(), C.vm_address_t(uintptr(ptr)), C.vm_size_t(sz))
	return data, nil
}

// CollectionSignatureVersion identifies the currently active memory-scan signature.
// Bump this (and add a corresponding entry to knownSignatureVersions) whenever
// re-deriving after an MTGA patch shifts the Unity heap layout.
//
// Derivation record for 20260529-001:
//
//	Tool:           collection-helper --dump-regions <MTGA PID> (read-only; same task_for_pid +
//	                mach_vm_read path as the production scanner — no LLDB, no process suspension)
//	MTGA build:     2026-05-29 patch (mtga_build=unknown — v0.3.5 adds Info.plist detection)
//	Dump directory: /tmp/collection-dump — 308 regions, manifest.json
//	Offline analysis: go run ./cmd/analyze_dump /tmp/collection-dump /tmp/collection-dump/manifest.json
//	H1/H2 outcome: H1 CONFIRMED — region filter thresholds unchanged; no Unity layout drift.
//	  Best region: 0x389c30000 (region_0298), 16 MB, 19114 entries, fillPct=1.82%
//	  Constants confirmed: minEntries=500, maxFillPct=3.0, stride=16 bytes unchanged.
//	  Root cause of "no collection region found" in v0.3.3-rc2: the installed binary was
//	  the 2026-05-12 build; the 2026-05-29 MTGA update shifted heap layout slightly,
//	  moving the collection dictionary to a different region address range. Recompiling
//	  and releasing a new helper binary with this versioned signature resolves the issue.
//	Entries found:  19114 (region 0x389c30000); 10060 and 9088 in two mirror regions
//
// RE-DERIVE PROCEDURE (ADR-040 Option-1 stopgap, vault-mtg-tickets#202 — REVISED SAFE METHOD):
//
// IMPORTANT: Do NOT attach LLDB or any debugger to MTGA. Debugger attachment suspends the
// process and trips MTGA/Unity anti-debug protections, causing a crash. The procedure below
// uses only the same non-intrusive, read-only task_for_pid + mach_vm_read path that the
// production scanner uses on every collection sync. It does not suspend MTGA.
//
// Prerequisites: MTGA running, Collection screen open, helper binary built (CGO, darwin/arm64).
// The helper must run as root (same requirement as production). MTGA must NOT be touched
// (no windows moved, no UI interaction) while the dump is running.
//
//  1. Build the helper with the dump flag enabled (no source change required — the flag is
//     gated in main.go via os.Args):
//
//     sudo ./collection-helper --dump-regions $(pgrep -x MTGA) /tmp/collection-dump
//
//     This runs listReadableRegions (mach_vm_region enumeration, read-only) and then
//     readMemory (mach_vm_read, read-only) for every region >= minRegionSize that has
//     VM_PROT_READ. Writes:
//     /tmp/collection-dump/manifest.json         — region index (addr, size, prot flags)
//     /tmp/collection-dump/region_<N>_0x<addr>.bin — raw bytes for each region
//
//  2. Bob runs the offline analysis locally against the dump (no live MTGA required):
//
//     go run ./cmd/analyze_dump /tmp/collection-dump /tmp/collection-dump/manifest.json
//
//     The analyze_dump script calls scanDictEntries on each region .bin file and prints:
//     - region address, size, entry count, fillPct
//     - sample of GRP IDs found (first 10, sorted)
//     This resolves H1 vs H2 without any live process contact.
//
//     IMPORTANT: delete the dump directory after analysis — it contains raw process
//     memory (PII: card collection IDs, account data). Never commit or transmit
//     the dump. Example: rm -rf /tmp/collection-dump
//
//  3. If a region returns >= minEntries hits with the current heuristic → H1 (region filter
//     too strict; adjust minEntries or maxFillPct). Record the region address range and
//     adjusted constants in this comment.
//
//     If no region returns hits → H2 (Unity layout drift; inspect bytes around a known GRP ID
//     in the .bin to determine whether the 16-byte [hashCode][next][key][value] stride is
//     intact). Record the new layout and fix scanDictEntries in scanner.go.
//
//  4. Update CollectionSignatureVersion, knownSignatureVersions, and this comment block
//     with the confirmed outcome before opening the PR.
//
// Why this is safe: mach_vm_read is a kernel call that reads another process's address space
// without signaling, suspending, or instrumenting the target. It is the identical mechanism
// used in every production collection sync — MTGA has been running against this path since
// the helper shipped. LLDB's attach is categorically different: it injects a SIGTRAP / uses
// the Mach exception port, suspending all threads, which triggers anti-debug logic.
//
// TODO(v0.3.5): detect MTGA build string via task port Info.plist lookup (ADR-040 §G4).
const CollectionSignatureVersion = "20260529-001"

const (
	minRegionSize = 4 * 1024 * 1024
	chunkSize     = 4 * 1024 * 1024
	// minEntries and maxFillPct are the region-filter thresholds for the collection
	// dictionary scan. Re-confirmed for the 2026-05-29 MTGA build (H1 derivation,
	// vault-mtg-tickets#202): region 0x389c30000 returned 19114 entries at 1.82%
	// fill with these constants — no adjustment required.
	minEntries = 500
	maxFillPct = 3.0
)

// scanProcess reads the collection from pid's memory. Must run as root.
func scanProcess(pid int) (map[int]int, error) {
	task, err := openTask(pid)
	if err != nil {
		return nil, err
	}
	defer C.deallocTaskPort(task)

	regions := listReadableRegions(task, minRegionSize)

	type candidate struct {
		entries map[int]int
		fillPct float64
	}
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
			for id, qty := range scanDictEntries(data) {
				if existing, ok := entries[id]; !ok || qty > existing {
					entries[id] = qty
				}
			}
			scanned += chunk
		}

		fillPct := 100 * float64(len(entries)) / float64(r.size/16)
		if len(entries) >= minEntries && fillPct <= maxFillPct {
			candidates = append(candidates, candidate{entries, fillPct})
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no collection region found in PID %d", pid)
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if len(c.entries) > len(best.entries) {
			best = c
		}
	}
	return best.entries, nil
}
