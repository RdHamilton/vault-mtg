//go:build darwin

package main

/*
#include <mach/mach.h>
#include <mach/mach_vm.h>
#include <stdlib.h>

// mach_task_self() is a macro — wrap it so CGo can call it.
static mach_port_t selfTask() { return mach_task_self(); }

static kern_return_t vmRead(mach_port_t task, uint64_t addr, uint64_t size,
                             uintptr_t *outPtr, uint32_t *outSize) {
	vm_offset_t data = 0;
	mach_msg_type_number_t cnt = 0;
	kern_return_t kr = mach_vm_read(task,
		(mach_vm_address_t)addr,
		(mach_vm_size_t)size,
		&data, &cnt);
	if (kr == KERN_SUCCESS) {
		*outPtr = (uintptr_t)data;
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

// openTask returns a Mach task port for pid. Requires root / task_for_pid entitlement.
func openTask(pid int) (C.mach_port_t, error) {
	self := C.selfTask()
	var task C.mach_port_t
	if kr := C.task_for_pid(self, C.int(pid), &task); kr != C.KERN_SUCCESS {
		return 0, fmt.Errorf("task_for_pid failed: kern_return_t=%d (run as root?)", kr)
	}
	return task, nil
}

type vmRegion struct {
	addr uint64
	size uint64
	prot uint32
}

// listReadableRegions enumerates all VM regions in task that are readable
// and at least minSize bytes.
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
			(C.vm_region_info_t)(unsafe.Pointer(&info)),
			&count,
			&objName,
		)
		if kr != C.KERN_SUCCESS {
			break
		}
		if uint64(size) >= minSize && (info.protection&C.VM_PROT_READ) != 0 {
			regions = append(regions, vmRegion{
				addr: uint64(addr),
				size: uint64(size),
				prot: uint32(info.protection),
			})
		}
		addr += size
	}
	return regions
}

// readMemory copies size bytes from addr in task into a Go slice.
func readMemory(task C.mach_port_t, addr, size uint64) ([]byte, error) {
	var ptr C.uintptr_t
	var sz C.uint32_t
	if kr := C.vmRead(task, C.uint64_t(addr), C.uint64_t(size), &ptr, &sz); kr != C.KERN_SUCCESS {
		return nil, fmt.Errorf("mach_vm_read addr=0x%x failed: kern_return_t=%d", addr, kr)
	}
	data := C.GoBytes(unsafe.Pointer(uintptr(ptr)), C.int(sz))
	C.vm_deallocate(C.selfTask(), C.vm_address_t(ptr), C.vm_size_t(sz))
	return data, nil
}
