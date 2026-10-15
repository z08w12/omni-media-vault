//go:build !windows

package api

import (
	"github.com/video-site/backend/internal/diskutil"
	"github.com/video-site/backend/internal/storageusage"
)

func localDiskStats(path string) (storageusage.DiskStats, error) {
	stats, err := diskutil.DiskStats(path)
	if err != nil {
		return storageusage.DiskStats{}, err
	}
	return storageusage.DiskStats{
		AvailableBytes: stats.AvailableBytes,
		CapacityBytes:  stats.CapacityBytes,
	}, nil
}
