//go:build !windows && !linux

package version

import (
	"runtime"

	"github.com/klauspost/cpuid/v2"
	"golang.org/x/sys/unix"
)

func GetSystemInfo() (*SystemInfo, error) {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		return nil, err
	}
	return &SystemInfo{
		Name:      unix.ByteSliceToString(utsname.Sysname[:]),
		Node:      unix.ByteSliceToString(utsname.Nodename[:]),
		Release:   unix.ByteSliceToString(utsname.Release[:]),
		Version:   unix.ByteSliceToString(utsname.Version[:]),
		Machine:   unix.ByteSliceToString(utsname.Machine[:]),
		OS:        runtime.GOOS,
		Processor: cpuid.CPU.BrandName,
	}, nil
}
