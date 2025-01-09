//go:build windows

package version

import (
	"fmt"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/klauspost/cpuid/v2"
	"golang.org/x/sys/windows"
)

const (
	PROCESSOR_ARCHITECTURE_AMD64 = 9
	PROCESSOR_ARCHITECTURE_ARM   = 5
	PROCESSOR_ARCHITECTURE_ARM64 = 12
	PROCESSOR_ARCHITECTURE_IA64  = 6
	PROCESSOR_ARCHITECTURE_INTEL = 0
)

var (
	processorArchLists = map[uint16]string{
		PROCESSOR_ARCHITECTURE_AMD64: "x64",
		PROCESSOR_ARCHITECTURE_ARM:   "arm",
		PROCESSOR_ARCHITECTURE_ARM64: "arm64",
		PROCESSOR_ARCHITECTURE_IA64:  "ia64",
		PROCESSOR_ARCHITECTURE_INTEL: "x86",
	}
)

func machineName(i uint16) string {
	if n, ok := processorArchLists[i]; ok {
		return n
	}
	return "unknown"
}

type PROCESSOR_ARCH struct {
	ProcessorArchitecture uint16
	Reserved              uint16
}

type SYSTEM_INFO struct {
	Arch                        PROCESSOR_ARCH
	DwPageSize                  uint32
	LpMinimumApplicationAddress uintptr
	LpMaximumApplicationAddress uintptr
	DwActiveProcessorMask       uint
	DwNumberOfProcessors        uint32
	DwProcessorType             uint32
	DwAllocationGranularity     uint32
	WProcessorLevel             uint16
	WProcessorRevision          uint16
}

var (
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procGetNativeSystemInfo = kernel32.NewProc("GetNativeSystemInfo")
)

func GetNativeSystemInfo() *SYSTEM_INFO {
	var info SYSTEM_INFO
	_, _, _ = procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&info)))
	return &info
}

func GetComputerName() (string, error) {
	var bufferSize uint32 = 1024
	var buffer [1024]uint16
	if err := windows.GetComputerName(&buffer[0], &bufferSize); err != nil {
		return "", nil
	}
	return windows.UTF16ToString(buffer[:bufferSize]), nil
}

func GetSystemInfo() (*SystemInfo, error) {
	sysinfo := GetNativeSystemInfo()
	computerName, _ := GetComputerName()
	major, minor, build := windows.RtlGetNtVersionNumbers()
	return &SystemInfo{
		Name:      "WindowsNT",
		Node:      computerName,
		Release:   strconv.FormatUint(uint64(major), 10),
		Version:   fmt.Sprintf("%d.%d.%d", major, minor, build),
		Machine:   machineName(sysinfo.Arch.ProcessorArchitecture),
		OS:        runtime.GOOS,
		Processor: cpuid.CPU.BrandName,
	}, nil
}
