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

const (
	minRegionSize = 4 * 1024 * 1024
	chunkSize     = 4 * 1024 * 1024
	minEntries    = 500
	maxFillPct    = 3.0
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
