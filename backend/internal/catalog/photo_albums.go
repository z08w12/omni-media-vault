package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type PhotoAlbum struct {
	Tag             string          `json:"tag"`
	Title           string          `json:"title"`
	Count           int             `json:"count"`
	CoverPhoto      *VideoSummary   `json:"coverPhoto,omitempty"`
	SamplePhotos    []*VideoSummary `json:"samplePhotos"`
	LatestTimestamp int64           `json:"latestTimestamp"`
}

var systemTagsToExclude = []string{
	"图片", "tg", "telegram", "x", "twitter", "youtube", "douyin", "tiktok", "bilibili", "web",
}

const photoAlbumImagePredicate = `(COALESCE(videos.media_kind, '') = 'image' OR LOWER(TRIM(COALESCE(videos.ext, ''), '.')) IN ('jpg', 'jpeg', 'png', 'webp', 'gif', 'bmp'))`

func systemTagsSQLList() string {
	quoted := make([]string, len(systemTagsToExclude))
	for i, t := range systemTagsToExclude {
		quoted[i] = fmt.Sprintf("'%s'", strings.ToLower(t))
	}
	return strings.Join(quoted, ", ")
}

func (c *Catalog) ListPhotoAlbums(ctx context.Context) ([]PhotoAlbum, error) {
	// 1. Read from in-memory cache if valid
	c.photoAlbumsMu.RLock()
	if c.photoAlbumsCache != nil && time.Now().Before(c.photoAlbumsUntil) {
		cached := make([]PhotoAlbum, len(c.photoAlbumsCache))
		copy(cached, c.photoAlbumsCache)
		c.photoAlbumsMu.RUnlock()
		return cached, nil
	}
	c.photoAlbumsMu.RUnlock()

	sysTags := systemTagsSQLList()

	// 2. Query 1: Tag-level aggregation
	tagQuery := fmt.Sprintf(`
		SELECT
			t.id,
			TRIM(t.label) AS tag_name,
			COUNT(DISTINCT videos.id) AS photo_count,
			MAX(videos.published_at) AS latest_pub
		FROM video_tags vt
		JOIN tags t ON t.id = vt.tag_id
		JOIN videos ON videos.id = vt.video_id
		WHERE COALESCE(videos.hidden, 0) = 0
		  AND %s
		  AND %s
		  AND %s
		  AND TRIM(COALESCE(t.label, '')) != ''
		  AND LOWER(TRIM(t.label)) NOT IN (%s)
		GROUP BY t.id, t.label
		ORDER BY photo_count DESC, latest_pub DESC, tag_name ASC
	`, activeDriveWhereSQL, uniqueVideoWhereSQL, photoAlbumImagePredicate, sysTags)

	tagRows, err := c.db.QueryContext(ctx, tagQuery)
	if err != nil {
		return nil, fmt.Errorf("query photo album tags: %w", err)
	}
	defer tagRows.Close()

	type tagMeta struct {
		id        int64
		label     string
		count     int
		latestPub int64
	}
	var tagMetas []tagMeta
	for tagRows.Next() {
		var tm tagMeta
		if err := tagRows.Scan(&tm.id, &tm.label, &tm.count, &tm.latestPub); err != nil {
			return nil, err
		}
		tagMetas = append(tagMetas, tm)
	}
	if err := tagRows.Err(); err != nil {
		return nil, err
	}

	// 3. Query 2: Top-4 sample photo IDs per tag using SQLite window function
	var allSampleIDs []string
	sampleIDSet := make(map[string]bool)
	tagSamples := make(map[int64][]string)

	if len(tagMetas) > 0 {
		samplesQuery := fmt.Sprintf(`
			WITH ranked_photos AS (
				SELECT
					vt.tag_id,
					videos.id AS video_id,
					ROW_NUMBER() OVER (
						PARTITION BY vt.tag_id
						ORDER BY videos.published_at DESC, videos.id ASC
					) AS rn
				FROM video_tags vt
				JOIN tags t ON t.id = vt.tag_id
				JOIN videos ON videos.id = vt.video_id
				WHERE COALESCE(videos.hidden, 0) = 0
				  AND %s
				  AND %s
				  AND %s
				  AND TRIM(COALESCE(t.label, '')) != ''
				  AND LOWER(TRIM(t.label)) NOT IN (%s)
			)
			SELECT tag_id, video_id
			FROM ranked_photos
			WHERE rn <= 4
			ORDER BY tag_id ASC, rn ASC
		`, activeDriveWhereSQL, uniqueVideoWhereSQL, photoAlbumImagePredicate, sysTags)

		sampleRows, err := c.db.QueryContext(ctx, samplesQuery)
		if err != nil {
			return nil, fmt.Errorf("query photo album samples: %w", err)
		}
		defer sampleRows.Close()

		for sampleRows.Next() {
			var tagID int64
			var videoID string
			if err := sampleRows.Scan(&tagID, &videoID); err != nil {
				return nil, err
			}
			tagSamples[tagID] = append(tagSamples[tagID], videoID)
			if !sampleIDSet[videoID] {
				sampleIDSet[videoID] = true
				allSampleIDs = append(allSampleIDs, videoID)
			}
		}
		if err := sampleRows.Err(); err != nil {
			return nil, err
		}
	}

	// 4. Query 3 & 4: Uncategorized photos (photos with no non-system tags)
	uncatCountQuery := fmt.Sprintf(`
		SELECT
			COUNT(*),
			COALESCE(MAX(videos.published_at), 0)
		FROM videos
		WHERE COALESCE(videos.hidden, 0) = 0
		  AND %s
		  AND %s
		  AND %s
		  AND NOT EXISTS (
			  SELECT 1
			  FROM video_tags vt
			  JOIN tags t ON t.id = vt.tag_id
			  WHERE vt.video_id = videos.id
				AND TRIM(COALESCE(t.label, '')) != ''
				AND LOWER(TRIM(t.label)) NOT IN (%s)
		  )
	`, activeDriveWhereSQL, uniqueVideoWhereSQL, photoAlbumImagePredicate, sysTags)

	var uncatCount int
	var uncatLatestPub int64
	if err := c.db.QueryRowContext(ctx, uncatCountQuery).Scan(&uncatCount, &uncatLatestPub); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("query uncategorized photo count: %w", err)
	}

	var uncatSampleIDs []string
	if uncatCount > 0 {
		uncatSamplesQuery := fmt.Sprintf(`
			SELECT videos.id
			FROM videos
			WHERE COALESCE(videos.hidden, 0) = 0
			  AND %s
			  AND %s
			  AND %s
			  AND NOT EXISTS (
				  SELECT 1
				  FROM video_tags vt
				  JOIN tags t ON t.id = vt.tag_id
				  WHERE vt.video_id = videos.id
					AND TRIM(COALESCE(t.label, '')) != ''
					AND LOWER(TRIM(t.label)) NOT IN (%s)
			  )
			ORDER BY videos.published_at DESC, videos.id ASC
			LIMIT 4
		`, activeDriveWhereSQL, uniqueVideoWhereSQL, photoAlbumImagePredicate, sysTags)

		uRows, err := c.db.QueryContext(ctx, uncatSamplesQuery)
		if err != nil {
			return nil, fmt.Errorf("query uncategorized samples: %w", err)
		}
		defer uRows.Close()

		for uRows.Next() {
			var vid string
			if err := uRows.Scan(&vid); err != nil {
				return nil, err
			}
			uncatSampleIDs = append(uncatSampleIDs, vid)
			if !sampleIDSet[vid] {
				sampleIDSet[vid] = true
				allSampleIDs = append(allSampleIDs, vid)
			}
		}
		if err := uRows.Err(); err != nil {
			return nil, err
		}
	}

	// 5. Batch fetch summaries for all sample photos
	summaryMap := make(map[string]*VideoSummary)
	if len(allSampleIDs) > 0 {
		summaries, err := c.VisibleVideoSummariesByIDs(ctx, allSampleIDs)
		if err != nil {
			return nil, fmt.Errorf("fetch photo album summaries: %w", err)
		}
		for _, s := range summaries {
			if s != nil {
				summaryMap[s.ID] = s
			}
		}
	}

	// 6. Build albums list
	var albums []PhotoAlbum
	for _, tm := range tagMetas {
		var sampleList []*VideoSummary
		for _, vid := range tagSamples[tm.id] {
			if s, ok := summaryMap[vid]; ok {
				sampleList = append(sampleList, s)
			}
		}
		var cover *VideoSummary
		latestTs := tm.latestPub
		if len(sampleList) > 0 {
			cover = sampleList[0]
			if latestTs == 0 && cover != nil {
				latestTs = cover.PublishedAt.UnixMilli()
			}
		}
		title := tm.label
		if !strings.HasPrefix(title, "#") {
			title = "#" + title
		}
		albums = append(albums, PhotoAlbum{
			Tag:             tm.label,
			Title:           title,
			Count:           tm.count,
			CoverPhoto:      cover,
			SamplePhotos:    sampleList,
			LatestTimestamp: latestTs,
		})
	}

	if uncatCount > 0 {
		var sampleList []*VideoSummary
		for _, vid := range uncatSampleIDs {
			if s, ok := summaryMap[vid]; ok {
				sampleList = append(sampleList, s)
			}
		}
		var cover *VideoSummary
		latestTs := uncatLatestPub
		if len(sampleList) > 0 {
			cover = sampleList[0]
			if latestTs == 0 && cover != nil {
				latestTs = cover.PublishedAt.UnixMilli()
			}
		}
		albums = append(albums, PhotoAlbum{
			Tag:             "未分类",
			Title:           "未分类图集",
			Count:           uncatCount,
			CoverPhoto:      cover,
			SamplePhotos:    sampleList,
			LatestTimestamp: latestTs,
		})
	}

	// 7. Write to in-memory cache
	c.photoAlbumsMu.Lock()
	c.photoAlbumsCache = make([]PhotoAlbum, len(albums))
	copy(c.photoAlbumsCache, albums)
	c.photoAlbumsUntil = time.Now().Add(30 * time.Second)
	c.photoAlbumsMu.Unlock()

	return albums, nil
}
