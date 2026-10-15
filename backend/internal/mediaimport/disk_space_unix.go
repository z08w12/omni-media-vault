//go:build !windows

package mediaimport

import "github.com/video-site/backend/internal/diskutil"

func diskAvailableBytes(path string) (int64, error) {
	return diskutil.AvailableBytes(path)
}
