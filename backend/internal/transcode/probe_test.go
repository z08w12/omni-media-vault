package transcode

import (
	"testing"
)

func TestCheckCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		ext        string
		streams    []StreamInfo
		compatible bool
		hasReason  bool
	}{
		{
			name:   "Standard H264 MP4 with AAC",
			format: "mov,mp4,m4a,3gp,3g2,mj2",
			ext:    "mp4",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "h264", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "aac"},
			},
			compatible: true,
		},
		{
			name:   "WebM container requires transcode for iOS/Safari compatibility",
			format: "matroska,webm",
			ext:    "webm",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "vp9", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "opus"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "AV1 in MP4 requires transcode for mobile compatibility",
			format: "mov,mp4,m4a,3gp,3g2,mj2",
			ext:    "mp4",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "av1", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "aac"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "VP9 in MP4 requires transcode for mobile compatibility",
			format: "mov,mp4,m4a,3gp,3g2,mj2",
			ext:    "mp4",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "vp9", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "aac"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "Opus audio in MP4 requires transcode for iOS compatibility",
			format: "mov,mp4,m4a,3gp,3g2,mj2",
			ext:    "mp4",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "h264", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "opus"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "MKV container requires transcode",
			format: "matroska,webm",
			ext:    "mkv",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "h264", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "aac"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "AVI container requires transcode",
			format: "avi",
			ext:    "avi",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "mpeg4", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "mp3"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "HEVC/H265 in MP4 requires transcode",
			format: "mov,mp4,m4a,3gp,3g2,mj2",
			ext:    "mp4",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "hevc", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "aac"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "10-bit H264 (yuv420p10le) requires transcode",
			format: "mov,mp4,m4a,3gp,3g2,mj2",
			ext:    "mp4",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "h264", PixFmt: "yuv420p10le"},
				{CodecType: "audio", CodecName: "aac"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "DTS audio in MP4 requires transcode",
			format: "mov,mp4,m4a,3gp,3g2,mj2",
			ext:    "mp4",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "h264", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "dts"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "AC3 audio in MP4 requires transcode",
			format: "mov,mp4,m4a,3gp,3g2,mj2",
			ext:    "mp4",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "h264", PixFmt: "yuv420p"},
				{CodecType: "audio", CodecName: "ac3"},
			},
			compatible: false,
			hasReason:  true,
		},
		{
			name:   "Image format (jpg) is compatible without video transcode",
			format: "image2",
			ext:    "jpg",
			streams: []StreamInfo{
				{CodecType: "video", CodecName: "mjpeg"},
			},
			compatible: true,
		},
		{
			name:       "No video stream is incompatible",
			format:     "mp3",
			ext:        "mp4",
			streams:    []StreamInfo{{CodecType: "audio", CodecName: "mp3"}},
			compatible: false,
			hasReason:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &MediaProbeInfo{
				FormatName: tt.format,
				Streams:    tt.streams,
			}
			res := CheckCompatibility(info, tt.ext)
			if res.Compatible != tt.compatible {
				t.Fatalf("expected compatible=%v, got=%v (reason: %s)", tt.compatible, res.Compatible, res.Reason)
			}
			if tt.hasReason && res.Reason == "" {
				t.Fatalf("expected reason but got empty string")
			}
		})
	}
}

func TestIsSupportedLocalDrive(t *testing.T) {
	if !IsSupportedLocalDrive("telegram-local") {
		t.Errorf("telegram-local should be supported")
	}
	if !IsSupportedLocalDrive("local-upload") {
		t.Errorf("local-upload should be supported")
	}
	if !IsSupportedLocalDrive("local-custom") {
		t.Errorf("local-custom should be supported")
	}
	if IsSupportedLocalDrive("webdav-remote") {
		t.Errorf("webdav-remote should not be supported")
	}
}
