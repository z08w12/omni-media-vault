package ytdlp

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestExtractGenericVideoURL_ExcludesTelegram(t *testing.T) {
	for _, tc := range []string{
		"https://t.me/example/1",
		"https://t.me/renqi85/53569",
		"https://telegram.me/channel/123",
		"https://telegram.org/dl",
	} {
		if _, _, found := ExtractGenericVideoURL(tc); found {
			t.Errorf("ExtractGenericVideoURL(%q) should be excluded, but was found", tc)
		}
	}

	// Non-message Telegram link should not be treated as a platform link
	if _, found := ExtractPlatformLink("https://telegram.org/dl"); found {
		t.Errorf("ExtractPlatformLink('https://telegram.org/dl') should not be recognized")
	}

	valid := "https://example.com/video.mp4"
	id, u, found := ExtractGenericVideoURL(valid)
	if !found || id == "" || u != valid {
		t.Errorf("ExtractGenericVideoURL(%q) failed: got (%q, %q, %v)", valid, id, u, found)
	}
}

func TestExtractTelegramMessageLink(t *testing.T) {
	cases := []struct {
		input       string
		wantID      string
		wantURL     string
		wantChannel string
		wantMsgID   int64
	}{
		{
			input:       "https://t.me/renqi85/53569",
			wantID:      "renqi85_53569",
			wantURL:     "https://t.me/renqi85/53569",
			wantChannel: "renqi85",
			wantMsgID:   53569,
		},
		{
			input:       "https://t.me/c/1234567890/53569?single",
			wantID:      "c_1234567890_53569",
			wantURL:     "https://t.me/c/1234567890/53569",
			wantChannel: "-1001234567890",
			wantMsgID:   53569,
		},
		{
			input:       "https://telegram.me/durov/123",
			wantID:      "durov_123",
			wantURL:     "https://t.me/durov/123",
			wantChannel: "durov",
			wantMsgID:   123,
		},
	}

	for _, tc := range cases {
		id, u, ok := ExtractTelegramMessageLink(tc.input)
		if !ok || id != tc.wantID || u != tc.wantURL {
			t.Errorf("ExtractTelegramMessageLink(%q) = (%q, %q, %v), want (%q, %q, true)", tc.input, id, u, ok, tc.wantID, tc.wantURL)
		}
		ch, msgID, err := ParseTelegramChannelAndMsgID(tc.input)
		if err != nil || ch != tc.wantChannel || msgID != tc.wantMsgID {
			t.Errorf("ParseTelegramChannelAndMsgID(%q) = (%q, %d, %v), want (%q, %d, nil)", tc.input, ch, msgID, err, tc.wantChannel, tc.wantMsgID)
		}

		link, found := ExtractPlatformLink(tc.input)
		if !found || link.Platform != "Telegram" || link.Prefix != "tg:" || link.ID != tc.wantID {
			t.Errorf("ExtractPlatformLink(%q) = %+v, want Platform Telegram with ID %q", tc.input, link, tc.wantID)
		}
	}
}

func TestCopyFileContentsWithFilter(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cookies.txt")
	dstX := filepath.Join(dir, "temp_cookies_x.txt")
	dstYT := filepath.Join(dir, "temp_cookies_yt.txt")
	content := "# Netscape HTTP Cookie File\n# Comments preserved\n.youtube.com\tTRUE\t/\tTRUE\t0\tYOUTUBE_TOKEN\t1\n.x.com\tTRUE\t/\tTRUE\t0\tAUTH_TOKEN\t2\n"
	if err := os.WriteFile(src, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Filter for X
	if err := copyFileContentsWithFilter(src, dstX, "X"); err != nil {
		t.Fatalf("copyFileContentsWithFilter X failed: %v", err)
	}
	copiedX, err := os.ReadFile(dstX)
	if err != nil {
		t.Fatal(err)
	}
	strX := string(copiedX)
	if !strings.Contains(strX, "AUTH_TOKEN") || strings.Contains(strX, "YOUTUBE_TOKEN") {
		t.Fatalf("X filter did not isolate cookies properly: %q", strX)
	}

	// Filter for YouTube
	if err := copyFileContentsWithFilter(src, dstYT, "YouTube"); err != nil {
		t.Fatalf("copyFileContentsWithFilter YouTube failed: %v", err)
	}
	copiedYT, err := os.ReadFile(dstYT)
	if err != nil {
		t.Fatal(err)
	}
	strYT := string(copiedYT)
	if !strings.Contains(strYT, "YOUTUBE_TOKEN") || strings.Contains(strYT, "AUTH_TOKEN") {
		t.Fatalf("YouTube filter did not isolate cookies properly: %q", strYT)
	}
}



