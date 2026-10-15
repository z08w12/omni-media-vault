package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

type SchemaMigration struct {
	Version     string
	Description string
	Run         func(ctx context.Context, c *Catalog) error
}

func (c *Catalog) runOrderedMigrations(ctx context.Context) error {
	if _, err := c.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     TEXT PRIMARY KEY,
			applied_at  INTEGER NOT NULL,
			description TEXT NOT NULL DEFAULT ''
		);
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	migrations := []SchemaMigration{
		{
			Version:     "20260401_0001_media_metadata",
			Description: "Add media_kind, mime_type, width, height columns and backfill",
			Run: func(ctx context.Context, c *Catalog) error {
				if err := c.addColumnIfMissing(ctx, "videos", "media_kind", "TEXT NOT NULL DEFAULT 'video'"); err != nil {
					return err
				}
				if err := c.addColumnIfMissing(ctx, "videos", "mime_type", "TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
				if err := c.addColumnIfMissing(ctx, "videos", "width", "INTEGER NOT NULL DEFAULT 0"); err != nil {
					return err
				}
				if err := c.addColumnIfMissing(ctx, "videos", "height", "INTEGER NOT NULL DEFAULT 0"); err != nil {
					return err
				}
				_, err := c.db.ExecContext(ctx, `
					UPDATE videos
					   SET media_kind = 'image',
					       mime_type = CASE LOWER(TRIM(COALESCE(ext, ''), '.'))
					           WHEN 'jpg' THEN 'image/jpeg'
					           WHEN 'jpeg' THEN 'image/jpeg'
					           WHEN 'png' THEN 'image/png'
					           WHEN 'webp' THEN 'image/webp'
					           WHEN 'gif' THEN 'image/gif'
					           WHEN 'bmp' THEN 'image/bmp'
					           ELSE 'image/jpeg'
					       END
					 WHERE LOWER(TRIM(COALESCE(ext, ''), '.')) IN ('jpg', 'jpeg', 'png', 'webp', 'gif', 'bmp')
					   AND (media_kind = 'video' OR media_kind = '')
				`)
				return err
			},
		},
		{
			Version:     "20260401_0002_drop_redundant_indexes",
			Description: "Drop redundant indexes covered by PK prefixes or rowid",
			Run: func(ctx context.Context, c *Catalog) error {
				if _, err := c.db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_video_reaction_visits_video`); err != nil {
					return err
				}
				if _, err := c.db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_remote_upload_jobs_recent`); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Version:     "20260401_0003_reaction_visits_updated_idx",
			Description: "Add index on video_reaction_visits(updated_at) for TTL purges",
			Run: func(ctx context.Context, c *Catalog) error {
				_, err := c.db.ExecContext(ctx, `
					CREATE INDEX IF NOT EXISTS idx_video_reaction_visits_updated
					    ON video_reaction_visits(updated_at);
				`)
				return err
			},
		},
		{
			Version:     "20260401_0004_remove_all_builtin_tags",
			Description: "Purge all builtin tags, disassociate video_tags, and resync affected videos",
			Run: func(ctx context.Context, c *Catalog) error {
				tx, err := c.db.BeginTx(ctx, nil)
				if err != nil {
					return err
				}
				defer tx.Rollback()

				rows, err := tx.QueryContext(ctx, `
					SELECT DISTINCT vt.video_id
					  FROM video_tags vt
					  JOIN tags t ON t.id = vt.tag_id
					 WHERE lower(trim(COALESCE(t.source, ''))) = 'builtin'
				`)
				if err != nil {
					return err
				}
				var videoIDs []string
				for rows.Next() {
					var vid string
					if err := rows.Scan(&vid); err != nil {
						rows.Close()
						return err
					}
					videoIDs = append(videoIDs, vid)
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return err
				}
				rows.Close()

				if _, err := tx.ExecContext(ctx, `
					DELETE FROM video_tags
					 WHERE tag_id IN (
					     SELECT id FROM tags WHERE lower(trim(COALESCE(source, ''))) = 'builtin'
					 )
				`); err != nil {
					return err
				}

				if _, err := tx.ExecContext(ctx, `
					DELETE FROM tags WHERE lower(trim(COALESCE(source, ''))) = 'builtin'
				`); err != nil {
					return err
				}

				if _, err := tx.ExecContext(ctx, `
					DELETE FROM settings WHERE key IN (
					    'tags.builtin_pack_enabled',
					    'tags.builtin_pack_initialized_v1'
					)
				`); err != nil {
					return err
				}

				if err := tx.Commit(); err != nil {
					return err
				}

				for _, vid := range uniqueStrings(videoIDs) {
					manual := c.hasManualTags(ctx, vid)
					if err := c.syncVideoTagsJSON(ctx, vid, manual); err != nil {
						return err
					}
				}

				return c.bumpTagRulesVersion(ctx)
			},
		},
	}

	for _, m := range migrations {
		var exists int
		err := c.db.QueryRowContext(ctx, `SELECT 1 FROM schema_migrations WHERE version = ?`, m.Version).Scan(&exists)
		if err == nil && exists == 1 {
			continue
		}
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", m.Version, err)
		}

		if err := m.Run(ctx, c); err != nil {
			return fmt.Errorf("run migration %s (%s): %w", m.Version, m.Description, err)
		}

		if _, err := c.db.ExecContext(ctx, `
			INSERT INTO schema_migrations (version, applied_at, description)
			VALUES (?, ?, ?)
		`, m.Version, time.Now().UnixMilli(), m.Description); err != nil {
			return fmt.Errorf("record migration %s: %w", m.Version, err)
		}
		log.Printf("[migration] applied schema migration %s: %s", m.Version, m.Description)
	}

	return nil
}
