package ytdlp

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractXStatusID(t *testing.T) {
	tests := []struct {
		input       string
		wantID      string
		wantURL     string
		wantFound   bool
	}{
		{
			input:     "https://x.com/user/status/1838848123456789012",
			wantID:    "1838848123456789012",
			wantURL:   "https://x.com/i/status/1838848123456789012",
			wantFound: true,
		},
		{
			input:     "Check this out: https://twitter.com/elonmusk/status/1789123456789012345?s=20 nice video",
			wantID:    "1789123456789012345",
			wantURL:   "https://x.com/i/status/1789123456789012345",
			wantFound: true,
		},
		{
			input:     "https://x.com/i/status/123456789012345",
			wantID:    "123456789012345",
			wantURL:   "https://x.com/i/status/123456789012345",
			wantFound: true,
		},
		{
			input:     "no link here",
			wantID:    "",
			wantURL:   "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		id, url, found := ExtractXStatusID(tt.input)
		if found != tt.wantFound || id != tt.wantID || url != tt.wantURL {
			t.Errorf("ExtractXStatusID(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, id, url, found, tt.wantID, tt.wantURL, tt.wantFound)
		}
	}
}

func TestExtractHashtags(t *testing.T) {
	text := "Great day #music #fun #每日精选 #music"
	want := []string{"music", "fun", "每日精选"}
	got := ExtractHashtags(text)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractHashtags(%q) = %v, want %v", text, got, want)
	}
}

func TestExtractYouTubeVideoID(t *testing.T) {
	tests := []struct {
		input     string
		wantID    string
		wantURL   string
		wantFound bool
	}{
		{
			input:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://youtube.com/watch?v=dQw4w9WgXcQ",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://m.youtube.com/watch?v=dQw4w9WgXcQ",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://music.youtube.com/watch?v=dQw4w9WgXcQ",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://youtu.be/dQw4w9WgXcQ",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://www.youtube.com/shorts/dQw4w9WgXcQ",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://youtube.com/shorts/dQw4w9WgXcQ?feature=share",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://www.youtube.com/live/dQw4w9WgXcQ?feature=share",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://youtube.com/watch?feature=shared&v=dQw4w9WgXcQ&t=10",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "Check this out: https://youtu.be/dQw4w9WgXcQ?si=abcdef, nice music!",
			wantID:    "dQw4w9WgXcQ",
			wantURL:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantFound: true,
		},
		{
			input:     "https://youtube.com/channel/UC1234567890",
			wantID:    "",
			wantURL:   "",
			wantFound: false,
		},
		{
			input:     "no youtube here",
			wantID:    "",
			wantURL:   "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		id, url, found := ExtractYouTubeVideoID(tt.input)
		if found != tt.wantFound || id != tt.wantID || url != tt.wantURL {
			t.Errorf("ExtractYouTubeVideoID(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, id, url, found, tt.wantID, tt.wantURL, tt.wantFound)
		}
	}
}

func TestCleanTitle(t *testing.T) {
	got := cleanTitle("Artist - Song | Official MV / 1080P: Best?", "Fallback")
	want := "Artist - Song Official MV 1080P Best"
	if got != want {
		t.Errorf("cleanTitle() = %q, want %q", got, want)
	}
}

func TestFindCookiesFile(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(emptyPath, []byte(""), 0600); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}
	// Empty file should not be returned
	if got := FindCookiesFile(dir); got == emptyPath {
		t.Errorf("FindCookiesFile should not return empty file, got %q", got)
	}

	// Non-empty file should be detected
	nonEmptyPath := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(nonEmptyPath, []byte("# Netscape HTTP Cookie File\n.youtube.com\tTRUE\t/\tTRUE\t0\tTEST\t1\n"), 0600); err != nil {
		t.Fatalf("failed to write non-empty file: %v", err)
	}
	if got := FindCookiesFile(dir); got != nonEmptyPath {
		t.Errorf("FindCookiesFile(%q) = %q, want %q", dir, got, nonEmptyPath)
	}
}

func TestExtractTikTokVideoID(t *testing.T) {
	tests := []struct {
		input     string
		wantID    string
		wantFound bool
	}{
		{
			input:     "https://www.tiktok.com/@creator/video/7123456789012345678?is_from_webapp=1",
			wantID:    "7123456789012345678",
			wantFound: true,
		},
		{
			input:     "Check this TikTok: https://vt.tiktok.com/ZS2xY7abc/ cool!",
			wantID:    "ZS2xY7abc",
			wantFound: true,
		},
		{
			input:     "not a tiktok url",
			wantID:    "",
			wantFound: false,
		},
	}
	for _, tt := range tests {
		id, _, found := ExtractTikTokVideoID(tt.input)
		if found != tt.wantFound || id != tt.wantID {
			t.Errorf("ExtractTikTokVideoID(%q) = (%q, %v), want (%q, %v)", tt.input, id, found, tt.wantID, tt.wantFound)
		}
	}
}

func TestExtractDouyinVideoID(t *testing.T) {
	tests := []struct {
		input     string
		wantID    string
		wantFound bool
	}{
		{
			input:     "https://www.douyin.com/video/7234567890123456789",
			wantID:    "7234567890123456789",
			wantFound: true,
		},
		{
			input:     "来看看这个：https://v.douyin.com/iJklmno/ 复制此链接",
			wantID:    "iJklmno",
			wantFound: true,
		},
	}
	for _, tt := range tests {
		id, _, found := ExtractDouyinVideoID(tt.input)
		if found != tt.wantFound || id != tt.wantID {
			t.Errorf("ExtractDouyinVideoID(%q) = (%q, %v), want (%q, %v)", tt.input, id, found, tt.wantID, tt.wantFound)
		}
	}
}

func TestExtractBilibiliVideoID(t *testing.T) {
	tests := []struct {
		input     string
		wantID    string
		wantFound bool
	}{
		{
			input:     "https://www.bilibili.com/video/BV1xx411c7xx?spm_id_from=333",
			wantID:    "BV1xx411c7xx",
			wantFound: true,
		},
		{
			input:     "【视频】https://b23.tv/av123456",
			wantID:    "av123456",
			wantFound: true,
		},
	}
	for _, tt := range tests {
		id, _, found := ExtractBilibiliVideoID(tt.input)
		if found != tt.wantFound || id != tt.wantID {
			t.Errorf("ExtractBilibiliVideoID(%q) = (%q, %v), want (%q, %v)", tt.input, id, found, tt.wantID, tt.wantFound)
		}
	}
}

func TestExtractPlatformLink(t *testing.T) {
	link, ok := ExtractPlatformLink("https://www.youtube.com/watch?v=dQw4w9WgXcQ #tag")
	if !ok || link.Platform != "YouTube" || link.ID != "dQw4w9WgXcQ" {
		t.Fatalf("ExtractPlatformLink youtube failed: %+v", link)
	}

	link, ok = ExtractPlatformLink("https://x.com/user/status/1838848123456789012")
	if !ok || link.Platform != "X" || link.ID != "1838848123456789012" {
		t.Fatalf("ExtractPlatformLink X failed: %+v", link)
	}

	link, ok = ExtractPlatformLink("https://www.tiktok.com/@video/video/7123456789012345678")
	if !ok || link.Platform != "TikTok" || link.ID != "7123456789012345678" {
		t.Fatalf("ExtractPlatformLink tiktok failed: %+v", link)
	}

	link, ok = ExtractPlatformLink("https://v.douyin.com/iJklmno/")
	if !ok || link.Platform != "Douyin" || link.ID != "iJklmno" {
		t.Fatalf("ExtractPlatformLink douyin failed: %+v", link)
	}

	link, ok = ExtractPlatformLink("https://www.bilibili.com/video/BV1xx411c7xx")
	if !ok || link.Platform != "Bilibili" || link.ID != "BV1xx411c7xx" {
		t.Fatalf("ExtractPlatformLink bilibili failed: %+v", link)
	}

	link, ok = ExtractPlatformLink("https://example.com/videos/stream.mp4")
	if !ok || link.Platform != "Web" || link.Prefix != "url:" {
		t.Fatalf("ExtractPlatformLink generic failed: %+v", link)
	}
}

func TestFindPlatformCookiesFile(t *testing.T) {
	dir := t.TempDir()
	ytCookie := filepath.Join(dir, "cookies_youtube.txt")
	defaultCookie := filepath.Join(dir, "cookies.txt")

	_ = os.WriteFile(defaultCookie, []byte("default cookie"), 0600)
	_ = os.WriteFile(ytCookie, []byte("youtube cookie"), 0600)

	// YouTube uses specific cookie
	if got := FindPlatformCookiesFile(dir, "YouTube"); got != ytCookie {
		t.Errorf("YouTube cookie = %q, want %q", got, ytCookie)
	}

	// X falls back to default cookie when cookies_x.txt is absent
	if got := FindPlatformCookiesFile(dir, "X"); got != defaultCookie {
		t.Errorf("X cookie = %q, want %q", got, defaultCookie)
	}
}


