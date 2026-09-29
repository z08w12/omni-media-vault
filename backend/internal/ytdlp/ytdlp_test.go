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
