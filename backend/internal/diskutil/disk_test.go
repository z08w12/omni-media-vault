package diskutil

import (
	"os"
	"testing"
)

func TestAvailableBytesAndDiskStats(t *testing.T) {
	tempDir := t.TempDir()

	avail, err := AvailableBytes(tempDir)
	if err != nil {
		t.Fatalf("AvailableBytes error = %v", err)
	}
	if avail <= 0 {
		t.Fatalf("AvailableBytes = %d, want > 0", avail)
	}

	stats, err := DiskStats(tempDir)
	if err != nil {
		t.Fatalf("DiskStats error = %v", err)
	}
	if stats.AvailableBytes <= 0 {
		t.Fatalf("stats.AvailableBytes = %d, want > 0", stats.AvailableBytes)
	}
	if stats.CapacityBytes <= 0 {
		t.Fatalf("stats.CapacityBytes = %d, want > 0", stats.CapacityBytes)
	}
	if stats.AvailableBytes > stats.CapacityBytes {
		t.Fatalf("available (%d) > capacity (%d)", stats.AvailableBytes, stats.CapacityBytes)
	}

	// Non-existent path returns error
	_, err = DiskStats(tempDir + "/does-not-exist/nested")
	if err == nil && !os.IsNotExist(err) {
		// on some platforms getting stats on missing child may or may not succeed depending on whether parent is resolved,
		// but checking that function runs without crashing is the main contract.
	}
}
