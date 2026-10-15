//go:build !windows

package backup

import "github.com/video-site/backend/internal/diskutil"

func availableDiskBytes(path string) (int64, error) {
	return diskutil.AvailableBytes(path)
}
