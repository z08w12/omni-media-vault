package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// XStatusPattern matches status URLs from x.com or twitter.com
	XStatusPattern = regexp.MustCompile(`(?i)https?://(?:www\.)?(?:x\.com|twitter\.com)/(?:(?:i/web|i)/status|[A-Za-z0-9_]{1,32}/status)/(?P<id>[0-9]{5,20})(?:/(?:video|photo)/[0-9]+)?(?:[?#][^\s]*)?`)
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
	// Extract named or captured ID
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
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Uploader    string `json:"uploader"`
	UploaderID  string `json:"uploader_id"`
	WebpageURL  string `json:"webpage_url"`
	Filename    string `json:"_filename"`
	Ext         string `json:"ext"`
	Entries     []ytdlpJSON `json:"entries"`
}

// DownloadXPost downloads the video from the public X/Twitter post into jobDir.
func DownloadXPost(ctx context.Context, postURL, jobDir string, maxSizeBytes int64) ([]DownloadedVideo, error) {
	if err := os.MkdirAll(jobDir, 0750); err != nil {
		return nil, fmt.Errorf("创建下载目录失败: %w", err)
	}

	bin := BinaryPath()
	outTmpl := filepath.Join(jobDir, "%(id)s.%(ext)s")

	args := []string{
		"--allowed-extractors", "twitter",
		"-f", "bestvideo*+bestaudio/best",
		"--merge-output-format", "mp4",
		"-o", outTmpl,
		"--no-playlist",
		"--socket-timeout", "25",
		"--retries", "3",
		"--fragment-retries", "3",
		"--print-json",
	}
	if maxSizeBytes > 0 {
		args = append(args, "--max-filesize", fmt.Sprintf("%d", maxSizeBytes))
	}
	args = append(args, postURL)

	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = err.Error()
		}
		// Clean up error message
		errStr = strings.TrimPrefix(errStr, "ERROR: ")
		return nil, fmt.Errorf("下载 X 视频失败: %s", errStr)
	}

	// Parse JSON lines from stdout
	var entries []ytdlpJSON
	for _, line := range strings.Split(stdout.String(), "\n") {
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
			return nil, errors.New("公开 X 帖子中没有可下载的视频，或视频超过大小限制")
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
			return nil, errors.New("X 视频超过单文件大小上限")
		}

		uploader := strings.TrimSpace(entry.Uploader)
		uploaderID := strings.TrimPrefix(strings.TrimSpace(entry.UploaderID), "@")
		if uploader == "" && uploaderID != "" {
			uploader = uploaderID
		}

		title := strings.TrimSpace(entry.Title)
		if title == "" {
			title = strings.TrimSpace(entry.Description)
		}
		if title == "" {
			title = fmt.Sprintf("X-%s", entry.ID)
		}
		// Truncate title if multi-line
		for _, line := range strings.Split(title, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				title = line
				break
			}
		}
		if len([]rune(title)) > 100 {
			rs := []rune(title)
			title = string(rs[:100])
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
		return nil, errors.New("公开 X 帖子中没有可下载的视频，或下载失败")
	}

	return results, nil
}
