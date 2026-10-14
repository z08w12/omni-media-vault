package catalog

import (
	"context"
	"path/filepath"
	"testing"
)

func TestTelegramLibraryMigrationPreservesFileIdentityWithoutHostPaths(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "catalog.db")
	cat, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	file := TelegramLocalFile{FileID: "tg-existing.media", JobID: "tg-existing"}
	if _, err := cat.ReserveTelegramLocalFile(ctx, file); err != nil {
		cat.Close()
		t.Fatal(err)
	}
	// Simulate the old schema and the absolute host path captured at import.
	if _, err := cat.db.Exec(`ALTER TABLE telegram_local_files ADD COLUMN root TEXT NOT NULL DEFAULT '/old/location/library'`); err != nil {
		cat.Close()
		t.Fatal(err)
	}
	if err := cat.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		cat, err = Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if hasColumn(t, cat, "telegram_local_files", "root") {
			t.Fatal("retained absolute storage root")
		}
		got, err := cat.TelegramLocalFile(ctx, file.FileID)
		if err != nil || got != file {
			t.Fatalf("migration changed identity: %+v %v", got, err)
		}
		if err := cat.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTelegramLocalFilesMultipleFilesPerJob(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "catalog.db")
	cat, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()

	jobID := "tg-job-album-1"
	f1, err := cat.ReserveTelegramLocalFile(ctx, TelegramLocalFile{FileID: "file1.media", JobID: jobID})
	if err != nil {
		t.Fatalf("f1 failed: %v", err)
	}
	f2, err := cat.ReserveTelegramLocalFile(ctx, TelegramLocalFile{FileID: "file2.media", JobID: jobID})
	if err != nil {
		t.Fatalf("f2 failed: %v", err)
	}
	if f1.FileID != "file1.media" || f2.FileID != "file2.media" {
		t.Fatalf("unexpected files: %+v %+v", f1, f2)
	}
}

