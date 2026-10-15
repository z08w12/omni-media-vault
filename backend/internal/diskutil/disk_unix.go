//go:build !windows

package diskutil

import (
	"path/filepath"

	"golang.org/x/sys/unix"
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
	var stat unix.Statfs_t
	if err := unix.Statfs(absPath, &stat); err != nil {
		return Stats{}, err
	}

	maxInt64 := uint64(^uint64(0) >> 1)

	availU := uint64(stat.Bavail) * uint64(stat.Bsize)
	avail := int64(availU)
	if availU > maxInt64 {
		avail = int64(maxInt64)
	}

	totalU := uint64(stat.Blocks) * uint64(stat.Bsize)
	cap := int64(totalU)
	if totalU > maxInt64 {
		cap = int64(maxInt64)
	}

	return Stats{
		AvailableBytes: avail,
		CapacityBytes:  cap,
	}, nil
}
