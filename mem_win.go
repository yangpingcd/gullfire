// +build windows

package main

/*
#include "windows.h"

static DWORDLONG get_mem_available() {
	MEMORYSTATUSEX status;
	memset(&status, 0, sizeof(status));
	status.dwLength = sizeof(status);

	GlobalMemoryStatusEx(&status);

	return status.ullAvailPhys;
}


static DWORDLONG get_mem_total() {
	MEMORYSTATUSEX status;
	memset(&status, 0, sizeof(status));
	status.dwLength = sizeof(status);

	GlobalMemoryStatusEx(&status);

	return status.ullTotalPhys;
}
*/

import (
	"syscall"
	"unsafe"
)

type MEMORYSTATUSEX struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

var GlobalMemoryStatusExProc *syscall.Proc

func init() {
	m := MEMORYSTATUSEX{}
	if unsafe.Sizeof(m) != 64 {
		panic("sizeof(MEMORYSTATUSEX) != 64")
	}

	kernel32 := syscall.MustLoadDLL("kernel32.dll")
	GlobalMemoryStatusExProc = kernel32.MustFindProc("GlobalMemoryStatusEx")
}

func mem_GetAvailable() uint64 {

	status := MEMORYSTATUSEX{}
	status.dwLength = uint32(unsafe.Sizeof(status))

	GlobalMemoryStatusExProc.Call(uintptr(unsafe.Pointer(&status)))

	return status.ullAvailPhys
}

func mem_GetTotal() uint64 {
	status := MEMORYSTATUSEX{}
	status.dwLength = uint32(unsafe.Sizeof(status))

	GlobalMemoryStatusExProc.Call(uintptr(unsafe.Pointer(&status)))

	return status.ullTotalPhys
}
