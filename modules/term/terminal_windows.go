package term

import (
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                         = syscall.NewLazyDLL("kernel32.dll")
	procGetFileInformationByHandleEx = kernel32.NewProc("GetFileInformationByHandleEx")
)

// isCygwinPipeName checks if a pipe name indicates a Cygwin/MSYS2 pseudo-terminal.
// Cygwin/MSYS2 PTY pipe names follow the pattern:
//
//	\{cygwin,msys}-XXXXXXXXXXXXXXXX-ptyN-{from,to}-master
//
// This function is used by IsCygwinTerminal to detect these emulated terminals.
func isCygwinPipeName(name string) bool {
	token := strings.Split(name, "-")
	if len(token) < 5 {
		return false
	}

	if token[0] != `\msys` &&
		token[0] != `\cygwin` &&
		token[0] != `\Device\NamedPipe\msys` &&
		token[0] != `\Device\NamedPipe\cygwin` {
		return false
	}

	if token[1] == "" {
		return false
	}

	if !strings.HasPrefix(token[2], "pty") {
		return false
	}

	if token[3] != `from` && token[3] != `to` {
		return false
	}

	if token[4] != "master" {
		return false
	}

	return true
}

// FILE_NAME_INFO structure used by GetFileInformationByHandleEx.
// Receives the file name. Used for any handles.
type FILE_NAME_INFO struct {
	FileNameLength uint32
	FileName       [512]uint16
}

// GetFileInformationByHandleEx retrieves file information for the specified file.
// This is a wrapper around the Windows API of the same name.
func GetFileInformationByHandleEx(hFile syscall.Handle,
	fileInformationClass uint32,
	lpFileInformation unsafe.Pointer,
	dwBufferSize uint32) error {
	r1, _, err := procGetFileInformationByHandleEx.Call(
		uintptr(hFile),
		uintptr(fileInformationClass),
		uintptr(lpFileInformation),
		uintptr(dwBufferSize),
	)
	if r1 == 1 {
		return nil
	}
	return err
}

const (
	FILE_NAME_INFO_BY_HANDLE = 2
)

// IsCygwinTerminal returns true if the file descriptor is connected to a
// Cygwin or MSYS2 pseudo-terminal. These terminals use named pipes rather
// than native Windows console APIs.
func IsCygwinTerminal(fd uintptr) bool {
	var fi FILE_NAME_INFO
	bufferSize := uint32(unsafe.Sizeof(fi))
	if err := GetFileInformationByHandleEx(syscall.Handle(fd), FILE_NAME_INFO_BY_HANDLE, unsafe.Pointer(&fi), bufferSize); err != nil {
		return false
	}
	fileName := windows.UTF16ToString(fi.FileName[:fi.FileNameLength/2])
	return isCygwinPipeName(fileName)
}

// detectColorLevelHijack detects Windows console color support and enables
// virtual terminal processing if needed.
//
// This function:
//  1. Attempts to get the current console mode
//  2. Enables virtual terminal processing (VT100/ANSI escape sequences) if disabled
//  3. Determines color support based on Windows version:
//     - Windows 10 build 14931+: 16M colors (truecolor)
//     - Windows 10 build 10586+: 256 colors
//     - Earlier versions: No color support
//
// References:
//   - https://github.com/microsoft/terminal/issues/11057#issuecomment-1493118152
//   - https://github.com/microsoft/terminal/issues/13006
func detectColorLevelHijack() Level {
	var mode uint32
	handle := windows.Handle(os.Stderr.Fd())
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		handle = windows.Handle(os.Stdout.Fd())
		if err := windows.GetConsoleMode(handle, &mode); err != nil {
			return LevelNone
		}
	}
	// VT detect and vt enabled
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING {
		mode = mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
		if err := windows.SetConsoleMode(handle, mode); err != nil {
			return LevelNone
		}
	}
	major, minor, build := windows.RtlGetNtVersionNumbers()
	if major > 10 || (major == 10 && minor >= 1) || (major == 10 && minor == 0 && build > 14931) {
		return Level16M
	}
	if major == 10 && build > 10586 {
		return Level256
	}
	return LevelNone
}
