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

// Check pipe name is used for cygwin/msys2 pty.
// Cygwin/MSYS2 PTY has a name like:
//
//	\{cygwin,msys}-XXXXXXXXXXXXXXXX-ptyN-{from,to}-master
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

// Receives the file name. Used for any handles.
// Use only when calling GetFileInformationByHandleEx.
type FILE_NAME_INFO struct {
	FileNameLength uint32
	FileName       [512]uint16
}

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

// IsCygwinTerminal() return true if the file descriptor is a cygwin or msys2
// terminal.
func IsCygwinTerminal(fd uintptr) bool {
	var fi FILE_NAME_INFO
	bufferSize := uint32(unsafe.Sizeof(fi))
	if err := GetFileInformationByHandleEx(syscall.Handle(fd), FILE_NAME_INFO_BY_HANDLE, unsafe.Pointer(&fi), bufferSize); err != nil {
		return false
	}
	fileName := windows.UTF16ToString(fi.FileName[:fi.FileNameLength/2])
	return isCygwinPipeName(fileName)
}

// https://github.com/microsoft/terminal/issues/11057#issuecomment-1493118152
// https://github.com/microsoft/terminal/issues/13006
func detectColorLevelHijack() Level {
	var mode uint32
	handle := windows.Handle(os.Stderr.Fd())
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		handle = windows.Handle(os.Stdout.Fd())
		if err := windows.GetConsoleMode(windows.Handle(os.Stdout.Fd()), &mode); err != nil {
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
