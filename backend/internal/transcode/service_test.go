package transcode

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/video-site/backend/internal/catalog"
)

func TestTranscodeQueueConcurrency(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "catalog.db")
	cat, err := catalog.Open(dbPath)
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	now := time.Now()
	v1 := &catalog.Video{
		ID:          "v-1",
		DriveID:     "local-upload",
		FileID:      "test1.avi",
		FileName:    "test1.avi",
		Title:       "Test 1",
		Size:        100,
		Ext:         "avi",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	v2 := &catalog.Video{
		ID:          "v-2",
		DriveID:     "local-upload",
		FileID:      "test2.mkv",
		FileName:    "test2.mkv",
		Title:       "Test 2",
		Size:        200,
		Ext:         "mkv",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := cat.UpsertVideo(ctx, v1); err != nil {
		t.Fatalf("upsert v1: %v", err)
	}
	if err := cat.UpsertVideo(ctx, v2); err != nil {
		t.Fatalf("upsert v2: %v", err)
	}

	tempDir := t.TempDir()
	uploadDir := t.TempDir()

	// Create fake source files
	_ = os.WriteFile(filepath.Join(uploadDir, "test1.avi"), []byte("dummy-video-data-1"), 0o644)
	_ = os.WriteFile(filepath.Join(uploadDir, "test2.mkv"), []byte("dummy-video-data-2"), 0o644)

	var stubFFmpeg, stubFFprobe string
	if runtime.GOOS == "windows" {
		stubFFmpeg = filepath.Join(tempDir, "ffmpeg.bat")
		// In Windows batch, loop to get the last argument
		stubScript := `@echo off
setlocal enabledelayedexpansion
for %%a in (%*) do set last=%%a
echo fake mp4 content > "!last!"
exit /b 0
`
		if err := os.WriteFile(stubFFmpeg, []byte(stubScript), 0o755); err != nil {
			t.Fatalf("write stub ffmpeg: %v", err)
		}

		stubFFprobe = filepath.Join(tempDir, "ffprobe.bat")
		stubProbeScript := `@echo off
echo 42.0
exit /b 0
`
		if err := os.WriteFile(stubFFprobe, []byte(stubProbeScript), 0o755); err != nil {
			t.Fatalf("write stub ffprobe: %v", err)
		}
	} else {
		stubFFmpeg = filepath.Join(tempDir, "ffmpeg")
		stubScript := `#!/usr/bin/env bash
dest="${@: -1}"
echo "fake mp4 content" > "$dest"
sleep 0.1
`
		if err := os.WriteFile(stubFFmpeg, []byte(stubScript), 0o755); err != nil {
			t.Fatalf("write stub ffmpeg: %v", err)
		}

		stubFFprobe = filepath.Join(tempDir, "ffprobe")
		stubProbeScript := `#!/usr/bin/env bash
echo "42.0"
`
		if err := os.WriteFile(stubFFprobe, []byte(stubProbeScript), 0o755); err != nil {
			t.Fatalf("write stub ffprobe: %v", err)
		}
	}

	svc := New(Config{
		FFmpegPath:  stubFFmpeg,
		FFprobePath: stubFFprobe,
		TempDir:     tempDir,
		UploadDir:   uploadDir,
		Catalog:     cat,
	})
	defer svc.Close()

	t1, err := svc.Enqueue(ctx, "v-1")
	if err != nil {
		t.Fatalf("enqueue v-1: %v", err)
	}
	if t1.Status != StatusPending && t1.Status != StatusProcessing {
		t.Fatalf("unexpected t1 status: %s", t1.Status)
	}

	t2, err := svc.Enqueue(ctx, "v-2")
	if err != nil {
		t.Fatalf("enqueue v-2: %v", err)
	}
	if t2 == nil {
		t.Fatal("t2 is nil")
	}

	// Poll until both finish
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s1 := svc.GetStatus("v-1")
		s2 := svc.GetStatus("v-2")
		if s1 != nil && s2 != nil && s1.Status == StatusCompleted && s2.Status == StatusCompleted {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	s1 := svc.GetStatus("v-1")
	s2 := svc.GetStatus("v-2")
	if s1 == nil || s1.Status != StatusCompleted {
		t.Fatalf("v-1 did not complete, status: %+v", s1)
	}
	if s2 == nil || s2.Status != StatusCompleted {
		t.Fatalf("v-2 did not complete, status: %+v", s2)
	}

	// Verify catalog updated
	uv1, err := cat.GetVideo(ctx, "v-1")
	if err != nil {
		t.Fatalf("get v-1: %v", err)
	}
	if uv1.Ext != "mp4" || uv1.FileID != "test1.mp4" {
		t.Fatalf("v-1 not updated properly: ext=%s fileId=%s", uv1.Ext, uv1.FileID)
	}

	// Verify old source file deleted
	if _, err := os.Stat(filepath.Join(uploadDir, "test1.avi")); !os.IsNotExist(err) {
		t.Fatalf("old source file test1.avi still exists")
	}
	// Verify new mp4 file exists
	if _, err := os.Stat(filepath.Join(uploadDir, "test1.mp4")); err != nil {
		t.Fatalf("new file test1.mp4 missing: %v", err)
	}
}

func TestPrioritize(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "catalog.db")
	cat, err := catalog.Open(dbPath)
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	now := time.Now()
	for _, id := range []string{"v-a", "v-b", "v-c"} {
		_ = cat.UpsertVideo(ctx, &catalog.Video{
			ID:          id,
			DriveID:     "local-upload",
			FileID:      id + ".mp4",
			FileName:    id + ".mp4",
			Title:       id,
			Ext:         "mp4",
			PublishedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	svc := New(Config{
		Catalog: cat,
	})
	defer svc.Close()

	// Enqueue without running worker
	svc.mu.Lock()
	svc.tasks["v-a"] = &Task{VideoID: "v-a", Status: StatusPending}
	svc.tasks["v-b"] = &Task{VideoID: "v-b", Status: StatusPending}
	svc.tasks["v-c"] = &Task{VideoID: "v-c", Status: StatusPending}
	svc.queue = []string{"v-a", "v-b", "v-c"}
	svc.mu.Unlock()

	// Prioritize v-c
	ok := svc.Prioritize("v-c")
	if !ok {
		t.Fatalf("expected Prioritize to succeed")
	}

	qStatus := svc.GetQueueStatus()
	if len(qStatus.Pending) != 3 {
		t.Fatalf("expected 3 pending tasks, got %d", len(qStatus.Pending))
	}
	if qStatus.Pending[0].VideoID != "v-c" {
		t.Fatalf("expected v-c to be at front of queue, got %s", qStatus.Pending[0].VideoID)
	}
}

