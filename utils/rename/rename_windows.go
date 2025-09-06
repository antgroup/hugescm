//go:build windows

package rename

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	FILE_RENAME_FLAG_REPLACE_IF_EXISTS = 0x00000001
	FILE_RENAME_FLAG_POSIX_SEMANTICS   = 0x00000002
)

type fileRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

// TODO: test and use this
func PosixRename(oldName, newName string) error {
	oldNameUTF16, err := windows.UTF16PtrFromString(oldName)
	if err != nil {
		return err
	}
	newNameUTF16, err := windows.UTF16FromString(newName)
	if err != nil {
		return err
	}
	fileNameLen := len(newNameUTF16)*2 - 2
	var dummyFileRenameInfo fileRenameInformation
	bufferSize := int(unsafe.Offsetof(dummyFileRenameInfo.FileName)) + fileNameLen
	buffer := make([]byte, bufferSize)
	typedBufferPtr := (*fileRenameInformation)(unsafe.Pointer(&buffer[0]))
	typedBufferPtr.ReplaceIfExists = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	typedBufferPtr.FileNameLength = uint32(fileNameLen)
	copy((*[windows.MAX_LONG_PATH]uint16)(unsafe.Pointer(&typedBufferPtr.FileName[0]))[:fileNameLen/2:fileNameLen/2], newNameUTF16)

	fd, err := windows.CreateFile(oldNameUTF16, windows.DELETE, windows.FILE_SHARE_WRITE|windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(fd) // nolint

	// https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information
	// https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_rename_info
	return windows.SetFileInformationByHandle(fd, windows.FileRenameInfoEx, &buffer[0], uint32(bufferSize))
}

type FILE_BASIC_INFO struct {
	CreationTime   int64
	LastAccessTime int64
	LastWriteTime  int64
	ChangedTime    int64
	FileAttributes uint32

	// Pad out to 8-byte alignment.
	//
	// Without this padding, TestChmod fails due to an argument validation error
	// in SetFileInformationByHandle on windows/386.
	//
	// https://learn.microsoft.com/en-us/cpp/build/reference/zp-struct-member-alignment?view=msvc-170
	// says that “The C/C++ headers in the Windows SDK assume the platform's
	// default alignment is used.” What we see here is padding rather than
	// alignment, but maybe it is related.
	_ uint32
}

type FILE_DISPOSITION_INFO struct {
	Flags uint32
}

type FILE_DISPOSITION_INFO_EX struct {
	Flags uint32
}

func removeHideAttrbutes(fd windows.Handle) error {
	var du FILE_BASIC_INFO
	if err := windows.GetFileInformationByHandleEx(fd, windows.FileBasicInfo, (*byte)(unsafe.Pointer(&du)), uint32(unsafe.Sizeof(du))); err != nil {
		return err
	}
	du.FileAttributes &^= (windows.FILE_ATTRIBUTE_HIDDEN | windows.FILE_ATTRIBUTE_READONLY)
	return windows.SetFileInformationByHandle(fd, windows.FileBasicInfo, (*byte)(unsafe.Pointer(&du)), uint32(unsafe.Sizeof(du)))
}

func removeInternal(fd windows.Handle) error {
	infoEx := FILE_DISPOSITION_INFO_EX{
		Flags: windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS,
	}
	var err error
	if err = windows.SetFileInformationByHandle(fd, windows.FileDispositionInfoEx, (*byte)(unsafe.Pointer(&infoEx)), uint32(unsafe.Sizeof(infoEx))); err == nil {
		return nil
	}
	if err == windows.ERROR_ACCESS_DENIED {
		if err := removeHideAttrbutes(fd); err != nil {
			return err
		}
		if err = windows.SetFileInformationByHandle(fd, windows.FileDispositionInfoEx, (*byte)(unsafe.Pointer(&infoEx)), uint32(unsafe.Sizeof(infoEx))); err == nil {
			return nil
		}
	}
	switch err {
	case windows.ERROR_INVALID_PARAMETER:
	case windows.ERROR_INVALID_FUNCTION:
	case windows.ERROR_NOT_SUPPORTED:
	default:
		return err
	}
	info := FILE_DISPOSITION_INFO{
		Flags: 0x13, // DELETE
	}
	if err = windows.SetFileInformationByHandle(fd, windows.FileDispositionInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err == nil {
		return nil
	}
	if err != windows.ERROR_ACCESS_DENIED {
		return err
	}
	if err := removeHideAttrbutes(fd); err != nil {
		return err
	}
	return windows.SetFileInformationByHandle(fd, windows.FileDispositionInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
}

func Remove(name string) error {
	nameUTF16, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	fd, err := windows.CreateFile(nameUTF16, windows.FILE_READ_ATTRIBUTES|windows.FILE_WRITE_ATTRIBUTES|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0,
	)
	if err == syscall.ERROR_NOT_FOUND {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(fd) // nolint
	return removeInternal(fd)
}
