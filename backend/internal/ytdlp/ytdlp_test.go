package ytdlp

import (
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

