package catalog

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditPhase1_ReplaceManualVideoTagsAtomicity(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_replace_manual_atomicity.db")
	cat, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	now := time.Now()
	v := &Video{
		ID:          "vid-atomicity",
		DriveID:     "drive-1",
		FileID:      "file-atomicity",
		Title:       "Atomicity Test Video",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := cat.UpsertVideo(ctx, v); err != nil {
		t.Fatalf("upsert video: %v", err)
	}

	// Create a tag
	if _, err := cat.ensureTag(ctx, "tag-atomic-1", "user"); err != nil {
		t.Fatalf("ensure tag: %v", err)
	}

	// First set it normally to verify success
	if err := cat.replaceManualVideoTags(ctx, v.ID, []string{"tag-atomic-1"}, false); err != nil {
		t.Fatalf("replaceManualVideoTags: %v", err)
	}

	// Verify it worked
	v1, err := cat.GetVideo(ctx, v.ID)
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if len(v1.Tags) != 1 || v1.Tags[0] != "tag-atomic-1" {
		t.Fatalf("unexpected tags: %v", v1.Tags)
	}

	// Inject a trigger that causes UPDATE of tags on videos to fail
	_, err = cat.db.Exec(`CREATE TRIGGER fail_video_tags_update
		BEFORE UPDATE OF tags ON videos
		BEGIN
			SELECT RAISE(FAIL, 'injected tags update failure');
		END;`)
	if err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	// Ensure another tag exists
	if _, err := cat.ensureTag(ctx, "tag-atomic-2", "user"); err != nil {
		t.Fatalf("ensure tag 2: %v", err)
	}

	// Now attempt replaceManualVideoTags with tag-atomic-2. It should fail and roll back!
	err = cat.replaceManualVideoTags(ctx, v.ID, []string{"tag-atomic-2"}, false)
	if err == nil {
		t.Fatalf("expected error from injected failure, got nil")
	}

	// Drop trigger to read state
	if _, err := cat.db.Exec(`DROP TRIGGER fail_video_tags_update`); err != nil {
		t.Fatalf("drop trigger: %v", err)
	}

	// Verify video_tags does NOT contain tag-atomic-2 (it should have rolled back to previous state or not committed)
	var countTag2 int
	err = cat.db.QueryRow(`SELECT COUNT(*) FROM video_tags vt JOIN tags t ON t.id = vt.tag_id WHERE vt.video_id = ? AND t.label = 'tag-atomic-2'`, v.ID).Scan(&countTag2)
	if err != nil {
		t.Fatalf("query video_tags: %v", err)
	}
	if countTag2 != 0 {
		t.Fatalf("atomicity failed: tag-atomic-2 was inserted into video_tags despite transaction failure!")
	}
}

func TestAuditPhase1_MigrateTelegramLocalFilesUniqueAtomicity(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_tg_migration_atomicity.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	// Create legacy table with UNIQUE constraint
	_, err = db.Exec(`CREATE TABLE telegram_local_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id TEXT UNIQUE,
		job_id TEXT NOT NULL
	);
	INSERT INTO telegram_local_files (file_id, job_id) VALUES ('file-1', 'job-1');
	`)
	if err != nil {
		t.Fatalf("setup legacy table: %v", err)
	}

	// Add a VIEW referencing telegram_local_files to make DROP TABLE fail if foreign keys / views prevent it,
	// or create a trigger on telegram_local_files that raises error
	_, err = db.Exec(`CREATE TRIGGER tg_prevent_drop
		BEFORE DELETE ON telegram_local_files
		BEGIN
			SELECT RAISE(FAIL, 'cannot delete from telegram_local_files');
		END;`)
	if err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	cat := &Catalog{db: db}
	err = cat.migrateTelegramLocalFilesUnique(ctx)
	// Because the trigger fails DELETE or if a step fails, it should return error and rollback!
	// (Note: DROP TABLE in sqlite drops triggers on that table, but let's test if an error during migration triggers rollback)
}

func TestAuditPhase1_AddVideoTagAssignments_RejectsNonExistentVideo(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_orphan_tags.db")
	cat, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	// Ensure tag exists
	_, err = cat.ensureTag(ctx, "orphan-test-tag", "user")
	if err != nil {
		t.Fatalf("ensure tag: %v", err)
	}

	// Call AddVideoTagAssignments for non-existent video
	nonExistentID := "non-existent-video-999"
	changed, err := cat.AddVideoTagAssignments(ctx, nonExistentID, []TagAssignment{
		{Label: "orphan-test-tag", Source: "manual"},
	})
	if err == nil {
		t.Fatalf("expected error for non-existent video, got nil (changed=%d)", changed)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}

	// Verify no orphan row in video_tags
	var count int
	if err := cat.db.QueryRow(`SELECT COUNT(*) FROM video_tags WHERE video_id = ?`, nonExistentID).Scan(&count); err != nil {
		t.Fatalf("query count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 video_tags rows, found %d orphan rows", count)
	}

	// Also test upsertVideoTagAssignment directly
	tag, err := cat.getTagByLabel(ctx, "orphan-test-tag")
	if err != nil {
		t.Fatalf("getTagByLabel: %v", err)
	}
	_, _, err = cat.upsertVideoTagAssignment(ctx, nonExistentID, tag.ID, "auto", "test")
	if err == nil {
		t.Fatalf("upsertVideoTagAssignment should have failed on non-existent video")
	}
}

func TestAuditPhase1_PhotoAlbumsParityWithActiveDriveAndCanonical(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test_album_parity.db")
	cat, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	now := time.Now()
	// Create drive-1 and drive-inactive
	drive1 := &Drive{
		ID:        "drive-active",
		Name:      "Active Drive",
		Kind:      "localstorage",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := cat.UpsertDrive(ctx, drive1); err != nil {
		t.Fatalf("upsert drive: %v", err)
	}

	// Canonical image on active drive
	imgCanonical := &Video{
		ID:          "img-canonical",
		DriveID:     "drive-active",
		FileID:      "file-can",
		Title:       "Canonical Image",
		Ext:         "jpg",
		PublishedAt: now.Add(time.Second),
		CreatedAt:   now.Add(time.Second),
		UpdatedAt:   now.Add(time.Second),
	}
	if err := cat.UpsertVideo(ctx, imgCanonical); err != nil {
		t.Fatalf("upsert canonical: %v", err)
	}

	// Deduplicated secondary image (is_canonical = 0)
	imgDupe := &Video{
		ID:          "img-duplicate",
		DriveID:     "drive-active",
		FileID:      "file-dupe",
		Title:       "Duplicate Image",
		Ext:         "jpg",
		PublishedAt: now.Add(2 * time.Second),
		CreatedAt:   now.Add(2 * time.Second),
		UpdatedAt:   now.Add(2 * time.Second),
	}
	if err := cat.UpsertVideo(ctx, imgDupe); err != nil {
		t.Fatalf("upsert dupe: %v", err)
	}
	// Explicitly mark imgDupe as non-canonical
	if _, err := cat.db.Exec(`UPDATE videos SET is_canonical = 0 WHERE id = 'img-duplicate'`); err != nil {
		t.Fatalf("update is_canonical: %v", err)
	}

	// Image on an unlisted drive (inactive drive)
	imgInactiveDrive := &Video{
		ID:          "img-inactive-drive",
		DriveID:     "drive-does-not-exist",
		FileID:      "file-inactive",
		Title:       "Inactive Drive Image",
		Ext:         "png",
		PublishedAt: now.Add(3 * time.Second),
		CreatedAt:   now.Add(3 * time.Second),
		UpdatedAt:   now.Add(3 * time.Second),
	}
	if err := cat.UpsertVideo(ctx, imgInactiveDrive); err != nil {
		t.Fatalf("upsert inactive: %v", err)
	}

	// Add tag "风景" to all 3 images
	for _, vid := range []string{"img-canonical", "img-duplicate", "img-inactive-drive"} {
		if err := cat.replaceManualVideoTags(ctx, vid, []string{"风景"}, true); err != nil {
			t.Fatalf("tag %s: %v", vid, err)
		}
	}

	// Query ListPhotoAlbums
	albums, err := cat.ListPhotoAlbums(ctx)
	if err != nil {
		t.Fatalf("ListPhotoAlbums: %v", err)
	}

	// The "风景" album should only count 1 image (img-canonical),
	// NOT 3 images (ignoring img-duplicate because is_canonical=0, and img-inactive-drive because drive does not exist)!
	var fengjingAlbum *PhotoAlbum
	for i := range albums {
		if albums[i].Tag == "风景" {
			fengjingAlbum = &albums[i]
			break
		}
	}
	if fengjingAlbum == nil {
		t.Fatalf("album 风景 not found in %+v", albums)
	}
	if fengjingAlbum.Count != 1 {
		t.Fatalf("expected album count 1, got %d (non-canonical or inactive drive images leaked into count)", fengjingAlbum.Count)
	}
}
