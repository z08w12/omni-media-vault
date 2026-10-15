//go:build windows

package diskutil

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func AvailableBytes(path string) (int64, error) {
	stats, err := DiskStats(path)
	if err != nil {
		return 0, err
	}
	return stats.AvailableBytes, nil
}

func DiskStats(path string) (Stats, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return Stats{}, err
	}
	ptr, err := syscall.UTF16PtrFromString(absPath)
	if err != nil {
		return Stats{}, err
	}
	var freeBytesAvailable, totalBytes uint64
	r, _, callErr := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		0,
	)
	if r == 0 {
		return Stats{}, fmt.Errorf("GetDiskFreeSpaceExW failed: %w", callErr)
	}

	maxInt64 := uint64(^uint64(0) >> 1)
	avail := int64(freeBytesAvailable)
	if freeBytesAvailable > maxInt64 {
		avail = int64(maxInt64)
	}
	cap := int64(totalBytes)
	if totalBytes > maxInt64 {
		cap = int64(maxInt64)
	}

	return Stats{
		AvailableBytes: avail,
		CapacityBytes:  cap,
	}, nil
}
