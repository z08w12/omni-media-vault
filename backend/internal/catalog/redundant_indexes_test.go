package catalog

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPurgeExpiredReactionVisits(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/reaction_purge_test.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	now := time.Now()
	// Insert test video
	vid := &Video{
		ID:          "vid-react-1",
		DriveID:     "drive-1",
		FileID:      "file-react-1",
		Title:       "Reaction Test Video",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := cat.UpsertVideo(ctx, vid); err != nil {
		t.Fatalf("upsert video: %v", err)
	}

	// Add 3 visits: 2 expired (> 90 days), 1 active
	expired1 := now.Add(-100 * 24 * time.Hour).UnixMilli()
	expired2 := now.Add(-95 * 24 * time.Hour).UnixMilli()
	active := now.Add(-1 * 24 * time.Hour).UnixMilli()

	if _, err := cat.db.ExecContext(ctx, `
		INSERT INTO video_reaction_visits (video_id, visit_id, reaction, created_at, updated_at)
		VALUES
			('vid-react-1', 'visit-exp-11111111', 'like', ?, ?),
			('vid-react-1', 'visit-exp-22222222', 'dislike', ?, ?),
			('vid-react-1', 'visit-act-33333333', 'like', ?, ?)
	`, expired1, expired1, expired2, expired2, active, active); err != nil {
		t.Fatalf("seed reaction visits: %v", err)
	}

	// Update video likes/dislikes manually to verify purge does not alter them
	if _, err := cat.db.ExecContext(ctx, `UPDATE videos SET likes = 5, dislikes = 2 WHERE id = 'vid-react-1'`); err != nil {
		t.Fatalf("update video counts: %v", err)
	}

	// Purge with batch size 1 to test loop logic
	deleted, err := cat.PurgeExpiredReactionVisits(ctx, 90*24*time.Hour, 1)
	if err != nil {
		t.Fatalf("PurgeExpiredReactionVisits: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 deleted rows, got %d", deleted)
	}

	// Verify only active visit remains
	var count int
	if err := cat.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM video_reaction_visits`).Scan(&count); err != nil {
		t.Fatalf("count remaining visits: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 remaining visit, got %d", count)
	}

	var remainingVisitID string
	if err := cat.db.QueryRowContext(ctx, `SELECT visit_id FROM video_reaction_visits`).Scan(&remainingVisitID); err != nil {
		t.Fatalf("scan remaining visit: %v", err)
	}
	if remainingVisitID != "visit-act-33333333" {
		t.Fatalf("expected visit-act-33333333 to remain, got %s", remainingVisitID)
	}

	// Verify video likes/dislikes counts remained untouched
	var likes, dislikes int
	if err := cat.db.QueryRowContext(ctx, `SELECT likes, dislikes FROM videos WHERE id = 'vid-react-1'`).Scan(&likes, &dislikes); err != nil {
		t.Fatalf("scan video counts: %v", err)
	}
	if likes != 5 || dislikes != 2 {
		t.Fatalf("video counts altered by purge: likes=%d, dislikes=%d, want 5/2", likes, dislikes)
	}
}

func TestRedundantIndexesExplainQueryPlan(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/explain_test.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	// 1. Verify video_reaction_visits query by video_id uses composite primary key index
	explainVisitSQL := `EXPLAIN QUERY PLAN SELECT reaction FROM video_reaction_visits WHERE video_id = ?`
	rows, err := cat.db.QueryContext(ctx, explainVisitSQL, "dummy-id")
	if err != nil {
		t.Fatalf("explain visit query: %v", err)
	}
	defer rows.Close()

	var visitPlanDetails []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan explain row: %v", err)
		}
		visitPlanDetails = append(visitPlanDetails, detail)
	}
	rows.Close()

	fullVisitPlan := strings.Join(visitPlanDetails, " | ")
	// SQLite automatically names the PRIMARY KEY index sqlite_autoindex_video_reaction_visits_1
	if !strings.Contains(fullVisitPlan, "sqlite_autoindex_video_reaction_visits") && !strings.Contains(fullVisitPlan, "USING INDEX") && !strings.Contains(fullVisitPlan, "USING COVERING INDEX") {
		t.Fatalf("visit query by video_id did not hit primary key index: %s", fullVisitPlan)
	}
	// Verify it does NOT do a SCAN TABLE
	if strings.Contains(fullVisitPlan, "SCAN TABLE video_reaction_visits") {
		t.Fatalf("visit query performed full table scan: %s", fullVisitPlan)
	}

	// 2. Verify remote_upload_jobs ORDER BY sequence DESC uses INTEGER PRIMARY KEY directly (no temp b-tree)
	explainRecentSQL := `EXPLAIN QUERY PLAN SELECT sequence FROM remote_upload_jobs ORDER BY sequence DESC LIMIT 10`
	rows2, err := cat.db.QueryContext(ctx, explainRecentSQL)
	if err != nil {
		t.Fatalf("explain recent query: %v", err)
	}
	defer rows2.Close()

	var recentPlanDetails []string
	for rows2.Next() {
		var id, parent, notused int
		var detail string
		if err := rows2.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan explain row: %v", err)
		}
		recentPlanDetails = append(recentPlanDetails, detail)
	}
	rows2.Close()

	fullRecentPlan := strings.Join(recentPlanDetails, " | ")
	// Must NOT use temporary B-tree for ORDER BY sequence DESC, proving the primary key order is used directly.
	if strings.Contains(fullRecentPlan, "USE TEMP B-TREE") {
		t.Fatalf("recent jobs query used temporary b-tree: %s", fullRecentPlan)
	}

	// 3. Verify redundant indexes do not exist in sqlite_master
	for _, idx := range []string{"idx_video_reaction_visits_video", "idx_remote_upload_jobs_recent"} {
		var exists int
		err := cat.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, idx).Scan(&exists)
		if err != nil {
			t.Fatalf("check index %s in master: %v", idx, err)
		}
		if exists > 0 {
			t.Fatalf("redundant index %s still exists in sqlite_master", idx)
		}
	}

	// 4. Verify idx_video_reaction_visits_updated exists in sqlite_master
	var updatedIdxExists int
	if err := cat.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_video_reaction_visits_updated'`).Scan(&updatedIdxExists); err != nil {
		t.Fatalf("check idx_video_reaction_visits_updated: %v", err)
	}
	if updatedIdxExists == 0 {
		t.Fatal("idx_video_reaction_visits_updated does not exist in sqlite_master")
	}
}

func TestSchemaMigrationsTracking(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/migrations_track_test.db"
	cat, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}

	// Verify schema_migrations table exists and has entries
	rows, err := cat.db.QueryContext(ctx, `SELECT version, description FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v, desc string
		if err := rows.Scan(&v, &desc); err != nil {
			t.Fatalf("scan migration row: %v", err)
		}
		versions = append(versions, v)
	}
	rows.Close()

	expectedVersions := []string{
		"20260401_0001_media_metadata",
		"20260401_0002_drop_redundant_indexes",
		"20260401_0003_reaction_visits_updated_idx",
	}
	if len(versions) < len(expectedVersions) {
		t.Fatalf("expected at least %d migrations recorded, got %d: %v", len(expectedVersions), len(versions), versions)
	}

	for _, exp := range expectedVersions {
		found := false
		for _, v := range versions {
			if v == exp {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("migration %s was not recorded in schema_migrations: %v", exp, versions)
		}
	}

	// Close and re-open to verify idempotence
	if err := cat.Close(); err != nil {
		t.Fatalf("close catalog: %v", err)
	}

	cat2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("re-open catalog: %v", err)
	}
	defer cat2.Close()

	var countAfter int
	if err := cat2.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&countAfter); err != nil {
		t.Fatalf("query count after re-open: %v", err)
	}
	if countAfter != len(versions) {
		t.Fatalf("migration count changed after re-open: before=%d, after=%d", len(versions), countAfter)
	}
}
