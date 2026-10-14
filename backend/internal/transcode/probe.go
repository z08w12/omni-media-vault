package transcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/video-site/backend/internal/drives/localupload"
	"github.com/video-site/backend/internal/drives/telegramstorage"
)

// StreamInfo represents the stream-level metadata parsed from ffprobe.
type StreamInfo struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
	PixFmt    string `json:"pix_fmt,omitempty"`
}

// MediaProbeInfo contains container and stream metadata parsed from ffprobe.
type MediaProbeInfo struct {
	FormatName string       `json:"format_name"`
	Streams    []StreamInfo `json:"streams"`
}

// CompatibilityResult holds the verdict on whether a media file is natively
// playable in mainstream desktop and mobile web browsers without server-side transcoding.
type CompatibilityResult struct {
	Compatible bool   `json:"compatible"`
	Reason     string `json:"reason,omitempty"`
}

// IsSupportedLocalDrive checks whether a drive kind supports in-place atomic transcode replacement.
func IsSupportedLocalDrive(driveID string) bool {
	switch driveID {
	case telegramstorage.DriveID, localupload.DriveID, "localstorage", "scriptcrawler":
		return true
	default:
		return strings.HasPrefix(driveID, "local-")
	}
}

// ProbeFile executes ffprobe against the target file, inspecting only the header
// metadata needed to judge stream compatibility.
func ProbeFile(ctx context.Context, ffprobePath, filePath string) (*MediaProbeInfo, error) {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		probeCtx,
		ffprobePath,
		"-v", "error",
		"-show_entries", "format=format_name",
		"-show_entries", "stream=codec_type,codec_name,pix_fmt",
		"-of", "json",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("probe file failed: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("probe file failed: %w", err)
	}

	var parsed struct {
		Format struct {
			FormatName string `json:"format_name"`
		} `json:"format"`
		Streams []StreamInfo `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("probe parse output: %w", err)
	}

	return &MediaProbeInfo{
		FormatName: parsed.Format.FormatName,
		Streams:    parsed.Streams,
	}, nil
}

// CheckCompatibility evaluates whether the given container extension and stream metadata
// can be played directly across mainstream modern web browsers (Chrome, Safari, Firefox, Edge, iOS/Android).
func CheckCompatibility(info *MediaProbeInfo, fileExt string) CompatibilityResult {
	ext := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(fileExt), "."))

	// 1. Image containers require no video transcode
	if isImageFormat(info.FormatName) || isImageExtension(ext) {
		return CompatibilityResult{Compatible: true}
	}

	// 2. Container check: only mp4, m4v, webm are universally supported web video containers
	switch ext {
	case "mp4", "m4v":
		// Allowed container, verify inner stream codecs below
	case "webm":
		// Allowed container, verify inner stream codecs below
	default:
		// mkv, avi, wmv, flv, mov, rmvb, ts, 3gp, asf etc. require transcode/remux
		return CompatibilityResult{
			Compatible: false,
			Reason:     fmt.Sprintf("container format '%s' is not natively playable in web browsers", ext),
		}
	}

	var hasVideo, hasAudio bool
	var primaryVideo StreamInfo
	var unsupportedAudioCodec string

	for _, s := range info.Streams {
		switch s.CodecType {
		case "video":
			if !hasVideo {
				primaryVideo = s
				hasVideo = true
			}
		case "audio":
			hasAudio = true
			codec := strings.ToLower(strings.TrimSpace(s.CodecName))
			switch codec {
			case "aac", "mp3", "opus", "vorbis", "":
				// Compatible web audio codecs
			case "dts", "ac3", "eac3", "truehd", "mlp", "flac":
				if unsupportedAudioCodec == "" {
					unsupportedAudioCodec = codec
				}
			}
		}
	}

	if !hasVideo {
		// No video stream found (may be audio only or corrupt)
		return CompatibilityResult{
			Compatible: false,
			Reason:     "no video stream found in media",
		}
	}

	vCodec := strings.ToLower(strings.TrimSpace(primaryVideo.CodecName))
	pixFmt := strings.ToLower(strings.TrimSpace(primaryVideo.PixFmt))

	// 3. Video Codec evaluation
	switch vCodec {
	case "h264", "avc1":
		// H.264 is the web gold standard, but check pixel format
		if isTenBitOrUnsupportedPixFmt(pixFmt) {
			return CompatibilityResult{
				Compatible: false,
				Reason:     fmt.Sprintf("H.264 with pixel format '%s' (10-bit/high-depth) causes black screen on most browsers", pixFmt),
			}
		}
	case "vp8", "vp9", "av01", "av1":
		// WebM / modern AV1
		if ext != "webm" && ext != "mp4" {
			return CompatibilityResult{
				Compatible: false,
				Reason:     fmt.Sprintf("video codec '%s' in container '%s' is not universally supported", vCodec, ext),
			}
		}
	case "hevc", "h265":
		// HEVC / H.265 in MP4 lacks hardware/software decode support in many Chrome/Windows/Android setups
		return CompatibilityResult{
			Compatible: false,
			Reason:     "video codec 'hevc/h265' is not universally supported in browsers without dedicated hardware licensing",
		}
	default:
		return CompatibilityResult{
			Compatible: false,
			Reason:     fmt.Sprintf("video codec '%s' is not supported for web playback", vCodec),
		}
	}

	// 4. Audio Codec evaluation
	if hasAudio && unsupportedAudioCodec != "" {
		return CompatibilityResult{
			Compatible: false,
			Reason:     fmt.Sprintf("audio codec '%s' lacks native decode in web browsers (causes video without sound)", unsupportedAudioCodec),
		}
	}

	return CompatibilityResult{
		Compatible: true,
	}
}

func isTenBitOrUnsupportedPixFmt(pixFmt string) bool {
	if pixFmt == "" {
		return false
	}
	switch pixFmt {
	case "yuv420p", "yuvj420p":
		return false
	default:
		// e.g. yuv420p10le, yuv444p, yuv422p, yuv420p12le etc.
		if strings.Contains(pixFmt, "10") || strings.Contains(pixFmt, "12") || strings.Contains(pixFmt, "444") || strings.Contains(pixFmt, "422") {
			return true
		}
		return false
	}
}

func isImageFormat(formatName string) bool {
	formatName = strings.ToLower(formatName)
	for _, f := range strings.Split(formatName, ",") {
		switch strings.TrimSpace(f) {
		case "image2", "jpeg_pipe", "png_pipe", "webp_pipe", "gif", "mjpeg":
			return true
		}
	}
	return false
}

func isImageExtension(ext string) bool {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "jpg", "jpeg", "png", "webp", "gif", "bmp":
		return true
	}
	return false
}
