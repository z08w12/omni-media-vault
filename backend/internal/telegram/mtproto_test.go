package telegram

import (
	"encoding/json"
	"testing"
)

func TestMTProtoDownloadResultParsing(t *testing.T) {
	rawJSON := `{
		"type": "done",
		"path": "/tmp/tg_renqi85_53569.mp4",
		"size": 10485760,
		"filename": "tg_renqi85_53569.mp4",
		"mime": "video/mp4",
		"is_photo": false,
		"title": "测试相册 #美女 #视频",
		"tags": ["美女", "视频"],
		"files": [
			{
				"id": 53567,
				"path": "/tmp/tg_renqi85_53567.jpg",
				"filename": "tg_renqi85_53567.jpg",
				"size": 204800,
				"mime": "image/jpeg",
				"is_photo": true
			},
			{
				"id": 53568,
				"path": "/tmp/tg_renqi85_53568.jpg",
				"filename": "tg_renqi85_53568.jpg",
				"size": 307200,
				"mime": "image/jpeg",
				"is_photo": true
			},
			{
				"id": 53569,
				"path": "/tmp/tg_renqi85_53569.mp4",
				"filename": "tg_renqi85_53569.mp4",
				"size": 10485760,
				"mime": "video/mp4",
				"is_photo": false
			}
		]
	}`

	var msg map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &msg); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	res := &MTProtoDownloadResult{}
	if p, ok := msg["path"].(string); ok {
		res.Path = p
	}
	if s, ok := msg["size"].(float64); ok {
		res.Size = int64(s)
	}
	if f, ok := msg["filename"].(string); ok {
		res.Filename = f
	}
	if m, ok := msg["mime"].(string); ok {
		res.MIME = m
	}
	if ip, ok := msg["is_photo"].(bool); ok {
		res.IsPhoto = ip
	}
	if tStr, ok := msg["title"].(string); ok {
		res.Title = tStr
	}
	if rawTags, ok := msg["tags"].([]any); ok {
		for _, rt := range rawTags {
			if ts, ok := rt.(string); ok && ts != "" {
				res.Tags = append(res.Tags, ts)
			}
		}
	}
	if rawFiles, ok := msg["files"].([]any); ok {
		for _, rf := range rawFiles {
			if fm, ok := rf.(map[string]any); ok {
				item := MTProtoFileItem{}
				if id, ok := fm["id"].(float64); ok {
					item.ID = int64(id)
				}
				if p, ok := fm["path"].(string); ok {
					item.Path = p
				}
				if fn, ok := fm["filename"].(string); ok {
					item.Filename = fn
				}
				if sz, ok := fm["size"].(float64); ok {
					item.Size = int64(sz)
				}
				if m, ok := fm["mime"].(string); ok {
					item.MIME = m
				}
				if ip, ok := fm["is_photo"].(bool); ok {
					item.IsPhoto = ip
				}
				res.Files = append(res.Files, item)
			}
		}
	}

	if len(res.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(res.Files))
	}
	if res.Files[0].IsPhoto != true || res.Files[0].ID != 53567 {
		t.Errorf("file 0 mismatch: %+v", res.Files[0])
	}
	if res.Files[1].IsPhoto != true || res.Files[1].ID != 53568 {
		t.Errorf("file 1 mismatch: %+v", res.Files[1])
	}
	if res.Files[2].IsPhoto != false || res.Files[2].ID != 53569 {
		t.Errorf("file 2 mismatch: %+v", res.Files[2])
	}
	if res.Title != "测试相册 #美女 #视频" {
		t.Errorf("title mismatch: %s", res.Title)
	}
	if len(res.Tags) != 2 || res.Tags[0] != "美女" || res.Tags[1] != "视频" {
		t.Errorf("tags mismatch: %v", res.Tags)
	}
}

func TestIsImageExtension(t *testing.T) {
	cases := []struct {
		ext  string
		want bool
	}{
		{".jpg", true},
		{".jpeg", true},
		{".png", true},
		{".webp", true},
		{".gif", true},
		{".mp4", false},
		{".mkv", false},
		{"jpg", true},
		{"MP4", false},
	}
	for _, tc := range cases {
		if got := isImageExtension(tc.ext); got != tc.want {
			t.Errorf("isImageExtension(%q) = %v, want %v", tc.ext, got, tc.want)
		}
	}
}
