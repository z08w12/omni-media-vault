package ytdlp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	// XStatusPattern matches status URLs from x.com or twitter.com
	XStatusPattern = regexp.MustCompile(`(?i)https?://(?:www\.)?(?:x\.com|twitter\.com)/(?:(?:i/web|i)/status|[A-Za-z0-9_]{1,32}/status)/(?P<id>[0-9]{5,20})(?:/(?:video|photo)/[0-9]+)?(?:[?#][^\s]*)?`)
	// YouTubePattern matches video and shorts URLs from youtube.com or youtu.be
	YouTubePattern = regexp.MustCompile(`(?i)https?://(?:(?:www|m|music)\.)?(?:youtube\.com/(?:watch\?(?:[^ \t\r\n&]+&)*v=|shorts/|embed/|v/|live/)|youtu\.be/)(?P<id>[A-Za-z0-9_-]{11})(?:[^A-Za-z0-9_-]|$)`)
	// HashtagPattern matches tags like #fun #news #中文
	HashtagPattern = regexp.MustCompile(`(?i)(?:^|[^\w])#([\p{L}\p{N}_-]{1,32})`)
)

type DownloadedVideo struct {
	MediaID     string   `json:"mediaId"`
	StatusID    string   `json:"statusId"`
	Path        string   `json:"path"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Uploader    string   `json:"uploader"`
	UploaderID  string   `json:"uploaderId"`
	WebpageURL  string   `json:"webpageUrl"`
	Size        int64    `json:"size"`
	Tags        []string `json:"tags"`
}

// ExtractXStatusID parses text and returns the status ID and normalized URL if found.
func ExtractXStatusID(text string) (statusID, statusURL string, found bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	match := XStatusPattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return "", "", false
	}
	idIndex := XStatusPattern.SubexpIndex("id")
	if idIndex > 0 && idIndex < len(match) {
		statusID = match[idIndex]
	} else if len(match) > 1 {
		statusID = match[1]
	}
	if statusID == "" {
		return "", "", false
	}
	return statusID, fmt.Sprintf("https://x.com/i/status/%s", statusID), true
}

// ExtractYouTubeVideoID parses text and returns the YouTube video ID and normalized URL if found.
func ExtractYouTubeVideoID(text string) (videoID, videoURL string, found bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	match := YouTubePattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return "", "", false
	}
	idIndex := YouTubePattern.SubexpIndex("id")
	if idIndex > 0 && idIndex < len(match) {
		videoID = match[idIndex]
	} else if len(match) > 1 {
		videoID = match[1]
	}
	if videoID == "" {
		return "", "", false
	}
	return videoID, fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID), true
}

// ExtractHashtags extracts hashtags from caption or description text.
func ExtractHashtags(text string) []string {
	if text == "" {
		return nil
	}
	matches := HashtagPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var tags []string
	for _, m := range matches {
		if len(m) > 1 {
			tag := strings.TrimSpace(m[1])
			if tag != "" && !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func cleanTitle(title, fallback string) string {
	title = strings.TrimSpace(title)
	for _, line := range strings.Split(title, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			title = line
			break
		}
	}
	title = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return ' '
		}
		return r
	}, title)
	title = strings.Trim(strings.Join(strings.Fields(title), " "), ". ")
	if title == "" {
		title = fallback
	}
	for len([]byte(title)) > 180 {
		runes := []rune(title)
		if len(runes) == 0 {
			break
		}
		title = strings.Trim(string(runes[:len(runes)-1]), ". ")
	}
	if title == "" {
		title = fallback
	}
	return title
}

// BinaryPath looks up yt-dlp in the PATH or known paths.
func BinaryPath() string {
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p
	}
	known := []string{
		"/usr/local/bin/yt-dlp",
		"/usr/bin/yt-dlp",
		"/opt/telegram-media/venv/bin/yt-dlp",
	}
	for _, k := range known {
		if info, err := os.Stat(k); err == nil && !info.IsDir() {
			return k
		}
	}
	return "yt-dlp"
}

// IsInstalled checks if yt-dlp can be found and executed.
func IsInstalled() bool {
	cmd := exec.Command(BinaryPath(), "--version")
	return cmd.Run() == nil
}

type ytdlpJSON struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Uploader    string   `json:"uploader"`
	UploaderID  string   `json:"uploader_id"`
	WebpageURL  string   `json:"webpage_url"`
	Filename    string   `json:"_filename"`
	Ext         string   `json:"ext"`
	Tags        []string `json:"tags"`
	Entries     []ytdlpJSON `json:"entries"`
}

// DownloadVideo downloads the video from the supported public URL into jobDir.
func DownloadVideo(ctx context.Context, videoURL, jobDir string, maxSizeBytes int64) ([]DownloadedVideo, error) {
	return DownloadVideoWithProgress(ctx, videoURL, jobDir, maxSizeBytes, nil)
}

// DownloadXPost downloads the video from the public X/Twitter post into jobDir.
func DownloadXPost(ctx context.Context, postURL, jobDir string, maxSizeBytes int64) ([]DownloadedVideo, error) {
	return DownloadVideoWithProgress(ctx, postURL, jobDir, maxSizeBytes, nil)
}

// DownloadXPostWithProgress downloads the video and reports live progress if available.
func DownloadXPostWithProgress(ctx context.Context, postURL, jobDir string, maxSizeBytes int64, onProgress func(done, total int64)) ([]DownloadedVideo, error) {
	return DownloadVideoWithProgress(ctx, postURL, jobDir, maxSizeBytes, onProgress)
}

// DownloadVideoWithProgress downloads the video from a public platform URL (X, YouTube, etc.) and reports live progress.
func DownloadVideoWithProgress(ctx context.Context, videoURL, jobDir string, maxSizeBytes int64, onProgress func(done, total int64)) ([]DownloadedVideo, error) {
	if err := os.MkdirAll(jobDir, 0750); err != nil {
		return nil, fmt.Errorf("创建下载目录失败: %w", err)
	}

	platform := "视频"
	lowerURL := strings.ToLower(videoURL)
	if strings.Contains(lowerURL, "x.com") || strings.Contains(lowerURL, "twitter.com") {
		platform = "X"
	} else if strings.Contains(lowerURL, "youtube.com") || strings.Contains(lowerURL, "youtu.be") {
		platform = "YouTube"
	}

	bin := BinaryPath()
	outTmpl := filepath.Join(jobDir, "%(id)s.%(ext)s")

	args := []string{
		"-f", "bestvideo*+bestaudio/best",
		"--merge-output-format", "mp4",
		"-o", outTmpl,
		"--no-playlist",
		"--socket-timeout", "25",
		"--retries", "3",
		"--fragment-retries", "3",
		"--newline",
		"--progress-delta", "1",
		"--progress-template", "download:%(progress.downloaded_bytes)s/%(progress.total_bytes)s/%(progress.total_bytes_estimate)s",
		"--print-json",
	}
	if maxSizeBytes > 0 {
		args = append(args, "--max-filesize", fmt.Sprintf("%d", maxSizeBytes))
	}
	args = append(args, videoURL)

	cmd := exec.CommandContext(ctx, bin, args...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建下载输出流失败: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动下载器失败: %w", err)
	}

	var stdoutBuf bytes.Buffer
	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "download:") {
			parts := strings.Split(strings.TrimPrefix(trimmed, "download:"), "/")
			if len(parts) >= 2 {
				done, _ := strconv.ParseInt(parts[0], 10, 64)
				total, _ := strconv.ParseInt(parts[1], 10, 64)
				if total <= 0 && len(parts) >= 3 {
					total, _ = strconv.ParseInt(parts[2], 10, 64)
				}
				if done > 0 && onProgress != nil {
					onProgress(done, total)
				}
			}
			continue
		}
		stdoutBuf.WriteString(line + "\n")
	}

	if err := cmd.Wait(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = err.Error()
		}
		errStr = strings.TrimPrefix(errStr, "ERROR: ")
		return nil, fmt.Errorf("下载 %s 视频失败: %s", platform, errStr)
	}

	// Parse JSON lines from stdoutBuf
	var entries []ytdlpJSON
	for _, line := range strings.Split(stdoutBuf.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var parsed ytdlpJSON
		if err := json.Unmarshal([]byte(line), &parsed); err == nil && parsed.ID != "" {
			entries = append(entries, parsed)
		}
	}

	if len(entries) == 0 {
		// Fallback: inspect directory for any downloaded mp4 files
		files, _ := filepath.Glob(filepath.Join(jobDir, "*.mp4"))
		if len(files) == 0 {
			files, _ = filepath.Glob(filepath.Join(jobDir, "*.*"))
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("公开 %s 链接中没有可下载的视频，或视频超过大小限制", platform)
		}
		for _, f := range files {
			stat, _ := os.Stat(f)
			var size int64
			if stat != nil {
				size = stat.Size()
			}
			id := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
			entries = append(entries, ytdlpJSON{
				ID:       id,
				Filename: f,
			})
			_ = size
		}
	}

	var results []DownloadedVideo
	for _, entry := range entries {
		expectedFile := filepath.Join(jobDir, entry.ID+".mp4")
		if _, err := os.Stat(expectedFile); err != nil {
			// Try finding any file with stem matching entry.ID
			matches, _ := filepath.Glob(filepath.Join(jobDir, entry.ID+".*"))
			found := false
			for _, m := range matches {
				ext := strings.ToLower(filepath.Ext(m))
				if ext != ".part" && ext != ".ytdl" {
					expectedFile = m
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		info, err := os.Stat(expectedFile)
		if err != nil || info.Size() <= 0 {
			continue
		}
		if maxSizeBytes > 0 && info.Size() > maxSizeBytes {
			_ = os.Remove(expectedFile)
			return nil, fmt.Errorf("%s 视频超过单文件大小上限", platform)
		}

		uploader := strings.TrimSpace(entry.Uploader)
		uploaderID := strings.TrimPrefix(strings.TrimSpace(entry.UploaderID), "@")
		if uploader == "" && uploaderID != "" {
			uploader = uploaderID
		}

		fallbackTitle := fmt.Sprintf("%s-%s", platform, entry.ID)
		title := cleanTitle(entry.Title, "")
		if title == "" {
			title = cleanTitle(entry.Description, "")
		}
		if title == "" {
			title = fallbackTitle
		}

		desc := strings.TrimSpace(entry.Description)
		hashtags := ExtractHashtags(desc)

		results = append(results, DownloadedVideo{
			MediaID:     entry.ID,
			Path:        expectedFile,
			Title:       title,
			Description: desc,
			Uploader:    uploader,
			UploaderID:  uploaderID,
			WebpageURL:  entry.WebpageURL,
			Size:        info.Size(),
			Tags:        hashtags,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("公开 %s 链接中没有可下载的视频，或下载失败", platform)
	}

	return results, nil
}
