package catalog

import (
	"context"
	"slices"
	"strings"
)

type PhotoAlbum struct {
	Tag             string
	Title           string
	Count           int
	CoverPhoto      *VideoSummary
	SamplePhotos    []*VideoSummary
	LatestTimestamp int64
}

var systemTagsToExclude = map[string]bool{
	"图片":       true,
	"tg":       true,
	"telegram": true,
	"x":        true,
	"twitter":  true,
	"youtube":  true,
	"douyin":   true,
	"tiktok":   true,
	"bilibili": true,
	"web":      true,
}

func isSystemTag(label string) bool {
	return systemTagsToExclude[strings.ToLower(strings.TrimSpace(label))]
}

func (c *Catalog) ListPhotoAlbums(ctx context.Context) ([]PhotoAlbum, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT v.id, COALESCE(t.label, '')
		FROM videos v
		LEFT JOIN video_tags vt ON vt.video_id = v.id
		LEFT JOIN tags t ON t.id = vt.tag_id
		WHERE COALESCE(v.hidden, 0) = 0
		  AND LOWER(TRIM(COALESCE(v.ext, ''), '.')) IN ('jpg', 'jpeg', 'png', 'webp', 'gif', 'bmp')
		ORDER BY v.published_at DESC, v.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orderedVideoIDs []string
	seenVideos := make(map[string]bool)
	videoNonSystemTags := make(map[string][]string)

	for rows.Next() {
		var vid, tagLabel string
		if err := rows.Scan(&vid, &tagLabel); err != nil {
			return nil, err
		}
		if !seenVideos[vid] {
			seenVideos[vid] = true
			orderedVideoIDs = append(orderedVideoIDs, vid)
		}
		cleanTag := strings.TrimSpace(tagLabel)
		if cleanTag != "" && !isSystemTag(cleanTag) {
			videoNonSystemTags[vid] = append(videoNonSystemTags[vid], cleanTag)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tagToVideoIDs := make(map[string][]string)
	var uncategorizedIDs []string

	for _, vid := range orderedVideoIDs {
		tags := videoNonSystemTags[vid]
		if len(tags) == 0 {
			uncategorizedIDs = append(uncategorizedIDs, vid)
		} else {
			seenTag := make(map[string]bool)
			for _, t := range tags {
				if !seenTag[t] {
					seenTag[t] = true
					tagToVideoIDs[t] = append(tagToVideoIDs[t], vid)
				}
			}
		}
	}

	sampleIDSet := make(map[string]bool)
	var sampleIDs []string

	addSampleIDs := func(ids []string) {
		limit := 4
		if len(ids) < limit {
			limit = len(ids)
		}
		for i := 0; i < limit; i++ {
			id := ids[i]
			if !sampleIDSet[id] {
				sampleIDSet[id] = true
				sampleIDs = append(sampleIDs, id)
			}
		}
	}

	for _, vids := range tagToVideoIDs {
		addSampleIDs(vids)
	}
	if len(uncategorizedIDs) > 0 {
		addSampleIDs(uncategorizedIDs)
	}

	summaries, err := c.VisibleVideoSummariesByIDs(ctx, sampleIDs)
	if err != nil {
		return nil, err
	}
	summaryMap := make(map[string]*VideoSummary, len(summaries))
	for _, s := range summaries {
		if s != nil {
			summaryMap[s.ID] = s
		}
	}

	var albums []PhotoAlbum
	for tag, vids := range tagToVideoIDs {
		if len(vids) == 0 {
			continue
		}
		var sampleList []*VideoSummary
		limit := 4
		if len(vids) < limit {
			limit = len(vids)
		}
		for i := 0; i < limit; i++ {
			if s, ok := summaryMap[vids[i]]; ok {
				sampleList = append(sampleList, s)
			}
		}
		var cover *VideoSummary
		var latestTs int64
		if len(sampleList) > 0 {
			cover = sampleList[0]
			latestTs = cover.PublishedAt.UnixMilli()
		}
		title := tag
		if !strings.HasPrefix(title, "#") {
			title = "#" + title
		}
		albums = append(albums, PhotoAlbum{
			Tag:             tag,
			Title:           title,
			Count:           len(vids),
			CoverPhoto:      cover,
			SamplePhotos:    sampleList,
			LatestTimestamp: latestTs,
		})
	}

	slices.SortFunc(albums, func(a, b PhotoAlbum) int {
		if b.Count != a.Count {
			return b.Count - a.Count
		}
		if b.LatestTimestamp > a.LatestTimestamp {
			return 1
		} else if b.LatestTimestamp < a.LatestTimestamp {
			return -1
		}
		return strings.Compare(a.Tag, b.Tag)
	})

	if len(uncategorizedIDs) > 0 {
		var sampleList []*VideoSummary
		limit := 4
		if len(uncategorizedIDs) < limit {
			limit = len(uncategorizedIDs)
		}
		for i := 0; i < limit; i++ {
			if s, ok := summaryMap[uncategorizedIDs[i]]; ok {
				sampleList = append(sampleList, s)
			}
		}
		var cover *VideoSummary
		var latestTs int64
		if len(sampleList) > 0 {
			cover = sampleList[0]
			latestTs = cover.PublishedAt.UnixMilli()
		}
		albums = append(albums, PhotoAlbum{
			Tag:             "未分类",
			Title:           "未分类图集",
			Count:           len(uncategorizedIDs),
			CoverPhoto:      cover,
			SamplePhotos:    sampleList,
			LatestTimestamp: latestTs,
		})
	}

	return albums, nil
}
