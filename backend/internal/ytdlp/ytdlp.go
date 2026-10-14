package ytdlp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	// TikTokPattern matches video URLs from tiktok.com, vt.tiktok.com, vm.tiktok.com
	TikTokPattern = regexp.MustCompile(`(?i)https?://(?:(?:www|m|vt|vm)\.)?tiktok\.com/(?:@[\w.-]+/video/(?P<id>[0-9]{15,25})|(?P<short_id>[A-Za-z0-9_-]{5,30}))`)
	// DouyinPattern matches video URLs from douyin.com or v.douyin.com
	DouyinPattern = regexp.MustCompile(`(?i)https?://(?:(?:www\.)?douyin\.com/(?:video|note)/(?P<id>[0-9]{15,25})|v\.douyin\.com/(?P<short_id>[A-Za-z0-9_-]{5,30}))`)
	// BilibiliPattern matches video URLs from bilibili.com or b23.tv
	BilibiliPattern = regexp.MustCompile(`(?i)https?://(?:(?:www\.)?bilibili\.com/video/(?P<id>(?:BV|bv)[A-Za-z0-9]{10}|av[0-9]{1,12})|b23\.tv/(?P<short_id>[A-Za-z0-9]{5,20}))`)
	// GenericURLPattern matches any standalone http/https URL
	GenericURLPattern = regexp.MustCompile(`(?i)https?://[A-Za-z0-9][-A-Za-z0-9.]*\.[A-Za-z]{2,}(?::[0-9]+)?/[^\s<>"'{}|\\^]+`)
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

// ExtractTikTokVideoID parses text and returns the TikTok video/short ID and URL if found.
func ExtractTikTokVideoID(text string) (videoID, videoURL string, found bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	match := TikTokPattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return "", "", false
	}
	idIndex := TikTokPattern.SubexpIndex("id")
	shortIndex := TikTokPattern.SubexpIndex("short_id")
	if idIndex > 0 && idIndex < len(match) && match[idIndex] != "" {
		videoID = match[idIndex]
		return videoID, fmt.Sprintf("https://www.tiktok.com/@video/video/%s", videoID), true
	}
	if shortIndex > 0 && shortIndex < len(match) && match[shortIndex] != "" {
		videoID = match[shortIndex]
		return videoID, match[0], true
	}
	return "", "", false
}

// ExtractDouyinVideoID parses text and returns the Douyin video/short ID and URL if found.
func ExtractDouyinVideoID(text string) (videoID, videoURL string, found bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	match := DouyinPattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return "", "", false
	}
	idIndex := DouyinPattern.SubexpIndex("id")
	shortIndex := DouyinPattern.SubexpIndex("short_id")
	if idIndex > 0 && idIndex < len(match) && match[idIndex] != "" {
		videoID = match[idIndex]
		return videoID, fmt.Sprintf("https://www.douyin.com/video/%s", videoID), true
	}
	if shortIndex > 0 && shortIndex < len(match) && match[shortIndex] != "" {
		videoID = match[shortIndex]
		return videoID, match[0], true
	}
	return "", "", false
}

// ExtractBilibiliVideoID parses text and returns the Bilibili video ID and normalized URL if found.
func ExtractBilibiliVideoID(text string) (videoID, videoURL string, found bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	match := BilibiliPattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return "", "", false
	}
	idIndex := BilibiliPattern.SubexpIndex("id")
	shortIndex := BilibiliPattern.SubexpIndex("short_id")
	if idIndex > 0 && idIndex < len(match) && match[idIndex] != "" {
		videoID = match[idIndex]
		return videoID, fmt.Sprintf("https://www.bilibili.com/video/%s", videoID), true
	}
	if shortIndex > 0 && shortIndex < len(match) && match[shortIndex] != "" {
		videoID = match[shortIndex]
		return videoID, fmt.Sprintf("https://b23.tv/%s", videoID), true
	}
	return "", "", false
}

// ExtractGenericVideoURL extracts any standalone HTTP(S) URL and produces a stable ID.
func ExtractGenericVideoURL(text string) (videoID, videoURL string, found bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	raw := GenericURLPattern.FindString(text)
	if raw == "" {
		return "", "", false
	}
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "t.me/") || strings.Contains(lower, "telegram.me/") || strings.Contains(lower, "telegram.org/") {
		return "", "", false
	}
	sum := sha256.Sum256([]byte(raw))
	videoID = hex.EncodeToString(sum[:])[:16]
	return videoID, raw, true
}

// PlatformLink holds parsed video link information across multiple platforms.
type PlatformLink struct {
	Platform string // "X", "YouTube", "TikTok", "Douyin", "Bilibili", "Web"
	Prefix   string // "x:", "yt:", "tiktok:", "douyin:", "bilibili:", "url:"
	ID       string
	URL      string
}

// ExtractPlatformLink inspects text and returns the first recognized video platform link.
func ExtractPlatformLink(text string) (*PlatformLink, bool) {
	if id, u, ok := ExtractXStatusID(text); ok {
		return &PlatformLink{Platform: "X", Prefix: "x:", ID: id, URL: u}, true
	}
	if id, u, ok := ExtractYouTubeVideoID(text); ok {
		return &PlatformLink{Platform: "YouTube", Prefix: "yt:", ID: id, URL: u}, true
	}
	if id, u, ok := ExtractTikTokVideoID(text); ok {
		return &PlatformLink{Platform: "TikTok", Prefix: "tiktok:", ID: id, URL: u}, true
	}
	if id, u, ok := ExtractDouyinVideoID(text); ok {
		return &PlatformLink{Platform: "Douyin", Prefix: "douyin:", ID: id, URL: u}, true
	}
	if id, u, ok := ExtractBilibiliVideoID(text); ok {
		return &PlatformLink{Platform: "Bilibili", Prefix: "bilibili:", ID: id, URL: u}, true
	}
	if id, u, ok := ExtractGenericVideoURL(text); ok {
		return &PlatformLink{Platform: "Web", Prefix: "url:", ID: id, URL: u}, true
	}
	return nil, false
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

// FindCookiesFile looks for a cookies.txt file in standard locations.
func FindCookiesFile(jobDir string) string {
	candidates := []string{
		filepath.Join(jobDir, "cookies.txt"),
		filepath.Join(jobDir, "cookies", "cookies.txt"),
		"/opt/video-site-91/data/cookies/cookies.txt",
		"/opt/video-site-91/cookies.txt",
		"data/cookies/cookies.txt",
		"/etc/yt-dlp/cookies.txt",
		"cookies.txt",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && !info.IsDir() && info.Size() > 0 {
			if abs, err := filepath.Abs(c); err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}

// FindPlatformCookiesFile looks for platform-specific cookies first, falling back to cookies.txt.
func FindPlatformCookiesFile(jobDir, platform string) string {
	var specific string
	switch strings.ToLower(platform) {
	case "youtube":
		specific = "cookies_youtube.txt"
	case "x", "twitter":
		specific = "cookies_x.txt"
	case "tiktok":
		specific = "cookies_tiktok.txt"
	case "douyin", "抖音":
		specific = "cookies_douyin.txt"
	case "bilibili":
		specific = "cookies_bilibili.txt"
	}
	if specific != "" {
		candidates := []string{
			filepath.Join(jobDir, specific),
			filepath.Join(jobDir, "cookies", specific),
			filepath.Join("/opt/video-site-91/data/cookies", specific),
			filepath.Join("/opt/video-site-91", specific),
			filepath.Join("data/cookies", specific),
			filepath.Join("/etc/yt-dlp", specific),
			specific,
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() && info.Size() > 0 {
				if abs, err := filepath.Abs(c); err == nil {
					return abs
				}
				return c
			}
		}
	}
	return FindCookiesFile(jobDir)
}

// platformDomains maps platform names to matching cookie domain substrings.
var platformDomains = map[string][]string{
	"youtube":  {"youtube.com", "google.com", "googlevideo.com", "youtu.be"},
	"x":        {"x.com", "twitter.com", "t.co"},
	"twitter":  {"x.com", "twitter.com", "t.co"},
	"tiktok":   {"tiktok.com", "byteoversea.com", "ibytedtos.com"},
	"douyin":   {"douyin.com", "iesdouyin.com", "amemv.com"},
	"抖音":       {"douyin.com", "iesdouyin.com", "amemv.com"},
	"bilibili": {"bilibili.com", "b23.tv", "hdslb.com"},
}

// copyFileContentsWithFilter copies cookies from src to dst. If allowed domains exist for the
// platform, it preserves comment headers (#...) and filters out cookies belonging to other platforms,
// preventing cross-platform cookie exposure and pollution.
func copyFileContentsWithFilter(src, dst, platform string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	domains := platformDomains[strings.ToLower(platform)]
	if len(domains) == 0 {
		if _, err = io.Copy(out, in); err != nil {
			return err
		}
		return out.Sync()
	}

	scanner := bufio.NewScanner(in)
	writer := bufio.NewWriter(out)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			_, _ = writer.WriteString(line + "\n")
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) > 0 {
			domain := strings.ToLower(fields[0])
			matched := false
			for _, d := range domains {
				if strings.Contains(domain, d) {
					matched = true
					break
				}
			}
			if matched {
				_, _ = writer.WriteString(line + "\n")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return writer.Flush()
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

// DownloadVideoWithProgress downloads the video from a public platform URL (X, YouTube, TikTok, Douyin, Bilibili, etc.) and reports live progress.
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
	} else if strings.Contains(lowerURL, "tiktok.com") {
		platform = "TikTok"
	} else if strings.Contains(lowerURL, "douyin.com") {
		platform = "抖音"
	} else if strings.Contains(lowerURL, "bilibili.com") || strings.Contains(lowerURL, "b23.tv") {
		platform = "Bilibili"
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
	if cookiesFile := FindPlatformCookiesFile(jobDir, platform); cookiesFile != "" {
		// 复制到任务专属临时文件，防止 yt-dlp 回写覆盖或污染原 cookies 文件
		tempCookies := filepath.Join(jobDir, ".job_cookies.txt")
		if err := copyFileContentsWithFilter(cookiesFile, tempCookies, platform); err == nil {
			defer os.Remove(tempCookies)
			args = append(args, "--cookies", tempCookies)
		} else {
			args = append(args, "--cookies", cookiesFile)
		}
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
	reader := bufio.NewReaderSize(stdoutPipe, 128*1024)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
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
			stdoutBuf.WriteString(line)
		}
		if err != nil {
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = err.Error()
		}
		if strings.Contains(errStr, "Sign in to confirm you’re not a bot") ||
			strings.Contains(errStr, "Sign in to confirm you're not a bot") ||
			strings.Contains(errStr, "Sign in to confirm your age") ||
			strings.Contains(errStr, "LOGIN_REQUIRED") ||
			strings.Contains(errStr, "Fresh cookies") {
			return nil, fmt.Errorf("该视频需要登录验证或 Cookies (可在服务器放置 cookies_%s.txt 或 cookies.txt 解锁)", strings.ToLower(platform))
		}
		for _, line := range strings.Split(errStr, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ERROR:") {
				errStr = strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
				break
			}
		}
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
		hashtags := ExtractHashtags(entry.Title + " " + desc)

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
