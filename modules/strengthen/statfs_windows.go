//go:build windows

package strengthen

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

const (
	pathLength = windows.MAX_PATH + 1
)

func GetDiskFreeSpaceEx(mountPath string) (*DiskFreeSpace, error) {
	absPath, err := filepath.Abs(mountPath)
	if err != nil {
		return nil, err
	}
	windowsPath, err := windows.UTF16PtrFromString(absPath)
	if err != nil {
		return nil, err
	}
	var freeBytesAvailableToCaller, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	if err = windows.GetDiskFreeSpaceEx(windowsPath,
		&freeBytesAvailableToCaller,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes); err != nil {
		return nil, err
	}
	root := filepath.VolumeName(absPath) + "\\"
	driveUTF16, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return nil, err
	}
	volumeNameBuffer := make([]uint16, pathLength)
	fileSystemNameBuffer := make([]uint16, pathLength)
	di := &DiskFreeSpace{
		Total: totalNumberOfBytes,
		Free:  totalNumberOfFreeBytes,
		Used:  totalNumberOfBytes - totalNumberOfFreeBytes,
		Avail: totalNumberOfFreeBytes,
	}
	if err = windows.GetVolumeInformation(driveUTF16, &volumeNameBuffer[0], pathLength, nil, nil, nil, &fileSystemNameBuffer[0], pathLength); err == nil {
		di.FS = windows.UTF16PtrToString(&fileSystemNameBuffer[0])
	}
	return di, nil
}
