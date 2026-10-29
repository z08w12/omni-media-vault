package telegram

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/drives/telegramstorage"
	"github.com/video-site/backend/internal/mediaasset"
	"github.com/video-site/backend/internal/mediaimport"
	"github.com/video-site/backend/internal/persistence"
	"github.com/video-site/backend/internal/videoname"
	"github.com/video-site/backend/internal/ytdlp"
)

// Check both new messages and persisted jobs before getFile can start a download.
func validateVideoSize(size, limit int64) error {
	if size <= 0 {
		return errors.New("无法确认视频大小，已拒绝下载，请重新发送视频")
	}
	if size > limit {
		return errors.New("视频超过配置的单文件大小限制")
	}
	return nil
}

func (s *Service) Fetch(ctx context.Context, j *catalog.RemoteUploadJob, progress func(string, int64, int64) error) (mediaimport.SourceFile, error) {
	// A configuration change cancels downloads using the old bot identity.
	if s.runCtx != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		stop := context.AfterFunc(s.runCtx, cancel)
		defer stop()
		defer cancel()
	}
	var out mediaimport.SourceFile
	source, err := catalog.DecodeTelegramSource(j)
	if err != nil {
		return out, &mediaimport.SourceError{Message: "TG 文件信息无效"}
	}
	if source.BotID != s.BotID() {
		return out, &mediaimport.SourceError{Message: "请连接提交任务的机器人", RetryAfter: time.Minute, WaitForAvailability: true}
	}
	if !s.Allowed(source.SenderID) {
		return out, &mediaimport.SourceError{Message: "发送者已不在允许用户列表中"}
	}
	isExternal := isExternalSource(source.UniqueID)
	size := source.Size
	if !isExternal {
		if err := validateVideoSize(size, s.cfg.MaxFileSizeBytes); err != nil {
			return out, &mediaimport.SourceError{Message: err.Error()}
		}
	}
	// Persist the destination before moving bytes, including across restarts.
	if err = persistence.RLockContext(ctx); err != nil {
		return out, err
	}
	local, localErr := s.cat.TelegramLocalFile(ctx, j.ID+".media")
	if errors.Is(localErr, sql.ErrNoRows) {
		local, err = s.cat.ReserveTelegramLocalFile(ctx, catalog.TelegramLocalFile{FileID: j.ID + ".media", JobID: j.ID})
	} else {
		err = localErr
	}
	if err == nil {
		err = os.MkdirAll(filepath.Join(s.cfg.LocalFilesRoot, "library"), 0750)
	}
	persistence.RUnlock()
	if err != nil {
		return out, &mediaimport.SourceError{Message: "无法准备 TG 视频存储目录", WaitForAvailability: true}
	}
	destination, err := telegramstorage.Path(s.cfg.LocalFilesRoot, local.FileID)
	if err != nil {
		return out, &mediaimport.SourceError{Message: "TG 视频存储目录不可用", WaitForAvailability: true}
	}
	if info, e := os.Stat(destination); e == nil {
		if info.Size() <= 0 || info.Size() > s.cfg.MaxFileSizeBytes || (source.Size > 0 && source.Size != info.Size()) {
			return out, &mediaimport.SourceError{Message: "TG 视频大小无效"}
		}
		if err = syncLibrary(destination, filepath.Dir(destination)); err != nil {
			return out, &mediaimport.SourceError{Message: "无法确认 TG 视频已落盘", WaitForAvailability: true}
		}
		return acquiredFile(source, local, destination, info.Size()), nil
	}
	if isExternal {
		// 检查本地可用存储配额（预留安全磁盘空间）
		if s.cfg.LocalFilesRoot != "" {
			reserve := s.getReserve()
			if avail, err := mediaimport.AvailableBytes(s.cfg.LocalFilesRoot); err == nil && avail > 0 && avail < reserve {
				return out, &mediaimport.SourceError{Message: fmt.Sprintf("服务器可用存储空间不足 (%s)，低于预留阈值 (%s)", formatBytes(avail), formatBytes(reserve))}
			}
		}
		cleanStaleTempJobs(filepath.Join(s.cfg.LocalFilesRoot, "temp"), time.Hour)

		tempJobDir := filepath.Join(s.cfg.LocalFilesRoot, "temp", j.ID)
		defer os.RemoveAll(tempJobDir)
		stageName, platformName := externalPlatformInfo(source.UniqueID)
		if err = progress(stageName, 0, 0); err != nil {
			return out, err
		}
		monitorCtx, cancelMonitor := context.WithCancel(ctx)
		defer cancelMonitor()
		var (
			progressMu sync.Mutex
			maxDone    int64
			curTotal   int64
		)
		updateProgress := func(done, total int64) {
			progressMu.Lock()
			if done > maxDone {
				maxDone = done
			}
			if total > 0 {
				curTotal = total
			}
			d, t := maxDone, curTotal
			progressMu.Unlock()
			_ = progress(stageName, d, t)
		}
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-monitorCtx.Done():
					return
				case <-ticker.C:
					bytes := findDirectoryBytes(tempJobDir)
					if bytes > 0 {
						updateProgress(bytes, 0)
					}
				}
			}
		}()
		if strings.HasPrefix(source.UniqueID, "tg:") {
			if !s.cfg.UserClientEnabled || s.cfg.APIID == "" || s.cfg.APIHash == "" || s.cfg.SessionString == "" {
				cancelMonitor()
				return out, &mediaimport.SourceError{Message: "未配置或未启用 Telegram MTProto 客户端凭据，请在后台系统设置 -> Telegram 中配置"}
			}
			channel, msgID, err := ytdlp.ParseTelegramChannelAndMsgID(source.UniqueID)
			if err != nil {
				channel, msgID, err = ytdlp.ParseTelegramChannelAndMsgID(source.FileName)
			}
			if err != nil {
				cancelMonitor()
				return out, &mediaimport.SourceError{Message: fmt.Sprintf("无法解析 Telegram 消息链接: %v", err)}
			}

			dlResult, err := DownloadMTProtoPost(ctx, s.cfg.APIID, s.cfg.APIHash, s.cfg.SessionString, channel, msgID, tempJobDir, s.cfg.MaxFileSizeBytes, func(done, total int64) {
				updateProgress(done, total)
			})
			cancelMonitor()
			if err != nil {
				return out, &mediaimport.SourceError{Message: err.Error()}
			}

			// 确定主文件 (Primary File)
			var primaryItem MTProtoFileItem
			if len(dlResult.Files) > 0 {
				for _, f := range dlResult.Files {
					if f.Path == dlResult.Path {
						primaryItem = f
						break
					}
				}
				if primaryItem.Path == "" {
					primaryItem = dlResult.Files[0]
				}
			} else {
				primaryItem = MTProtoFileItem{
					Path:     dlResult.Path,
					Filename: dlResult.Filename,
					Size:     dlResult.Size,
					MIME:     dlResult.MIME,
					IsPhoto:  dlResult.IsPhoto,
				}
			}

			_ = progress(stageName, primaryItem.Size, primaryItem.Size)
			if err := os.Rename(primaryItem.Path, destination); err != nil {
				data, readErr := os.ReadFile(primaryItem.Path)
				if readErr != nil {
					return out, &mediaimport.SourceError{Message: "无法读取下载的 Telegram 媒体"}
				}
				if writeErr := os.WriteFile(destination, data, 0644); writeErr != nil {
					return out, &mediaimport.SourceError{Message: "无法保存下载的 Telegram 媒体"}
				}
			}
			if err = syncLibrary(destination, filepath.Dir(destination)); err != nil {
				return out, &mediaimport.SourceError{Message: "无法确认 TG 媒体已落盘", WaitForAvailability: true}
			}
			source.Size = primaryItem.Size
			source.FileName = filepath.Base(destination)
			if primaryItem.MIME != "" {
				source.MIME = primaryItem.MIME
			}

			finalTitle := dlResult.Title
			if len(dlResult.Files) > 1 {
				primaryIdx := 1
				for i, item := range dlResult.Files {
					if item.Path == primaryItem.Path || (item.ID != 0 && item.ID == primaryItem.ID) {
						primaryIdx = i + 1
						break
					}
				}
				primaryBase := strings.TrimSuffix(primaryItem.Filename, filepath.Ext(primaryItem.Filename))
				if primaryBase != "" && !strings.HasPrefix(primaryBase, "tg_") {
					finalTitle = fmt.Sprintf("%s - %s", dlResult.Title, primaryBase)
				} else {
					finalTitle = fmt.Sprintf("%s - %d", dlResult.Title, primaryIdx)
				}
			}
			if finalTitle != "" {
				j.RequestedTitle = finalTitle
				j.ResolvedTitle = finalTitle
			}
			if primaryItem.IsPhoto {
				j.Tags = append([]string{"图片"}, j.Tags...)
				primaryVideoID := telegramstorage.DriveID + "-" + j.ID
				previewDir := filepath.Join(filepath.Dir(s.cfg.LocalFilesRoot), "previews")
				thumbDst := mediaasset.ThumbnailPath(previewDir, primaryVideoID)
				if err := os.MkdirAll(filepath.Dir(thumbDst), 0755); err == nil {
					if linkErr := os.Link(destination, thumbDst); linkErr != nil {
						if data, readErr := os.ReadFile(destination); readErr == nil {
							_ = os.WriteFile(thumbDst, data, 0644)
						}
					}
				}
			}
			if len(dlResult.Tags) > 0 {
				seen := make(map[string]bool)
				for _, t := range j.Tags {
					seen[t] = true
				}
				for _, t := range dlResult.Tags {
					if !seen[t] {
						seen[t] = true
						j.Tags = append(j.Tags, t)
					}
				}
			}

			// 保存相册中的其余附属文件（图片或额外视频）
			if len(dlResult.Files) > 1 {
				for i, extra := range dlResult.Files {
					if extra.Path == primaryItem.Path || (extra.ID != 0 && extra.ID == primaryItem.ID) {
						continue
					}
					extraIndex := i + 1
					extraBase := strings.TrimSuffix(extra.Filename, filepath.Ext(extra.Filename))
					extraTitle := fmt.Sprintf("%s - %d", dlResult.Title, extraIndex)
					if extraBase != "" && !strings.HasPrefix(extraBase, "tg_") {
						extraTitle = fmt.Sprintf("%s - %s", dlResult.Title, extraBase)
					}
					extraExt := filepath.Ext(extra.Filename)
					if extraExt == "" {
						if extra.IsPhoto {
							extraExt = ".jpg"
						} else {
							extraExt = ".mp4"
						}
					}
					extraFileID := fmt.Sprintf("%s-%d.media", j.ID, extra.ID)
					if extra.ID == 0 {
						extraFileID = fmt.Sprintf("%s-extra-%d.media", j.ID, extraIndex)
					}
					extraDest, err := telegramstorage.Path(s.cfg.LocalFilesRoot, extraFileID)
					if err != nil {
						continue
					}
					if renameErr := os.Rename(extra.Path, extraDest); renameErr != nil {
						data, readErr := os.ReadFile(extra.Path)
						if readErr == nil {
							_ = os.WriteFile(extraDest, data, 0644)
						}
					}
					_ = syncLibrary(extraDest, filepath.Dir(extraDest))

					_, _ = s.cat.ReserveTelegramLocalFile(ctx, catalog.TelegramLocalFile{
						FileID: extraFileID,
						JobID:  j.ID,
					})

					extraTags := make([]string, 0, len(dlResult.Tags)+3)
					if extra.IsPhoto {
						extraTags = append(extraTags, "图片")
					}
					extraTags = append(extraTags, "Telegram")
					extraTags = append(extraTags, dlResult.Tags...)
					seenTag := make(map[string]bool)
					var cleanTags []string
					for _, t := range extraTags {
						t = strings.TrimSpace(t)
						if t != "" && !seenTag[t] {
							seenTag[t] = true
							cleanTags = append(cleanTags, t)
						}
					}

					storedName := videoname.UploadFileName(extraTitle, extraExt, extraFileID, false)
					previewStatus := "pending"
					thumbnailURL := ""
					extraVideoID := telegramstorage.DriveID + "-" + extraFileID

					if isImageExtension(extraExt) || extra.IsPhoto {
						previewStatus = "disabled"
						previewDir := filepath.Join(filepath.Dir(s.cfg.LocalFilesRoot), "previews")
						thumbDst := mediaasset.ThumbnailPath(previewDir, extraVideoID)
						if err := os.MkdirAll(filepath.Dir(thumbDst), 0755); err == nil {
							if linkErr := os.Link(extraDest, thumbDst); linkErr != nil {
								if data, readErr := os.ReadFile(extraDest); readErr == nil {
									_ = os.WriteFile(thumbDst, data, 0644)
								}
							}
							if _, statErr := os.Stat(thumbDst); statErr == nil {
								thumbnailURL = "/p/thumb/" + extraVideoID
							}
						}
					}
					now := time.Now()
					isImg := isImageExtension(extraExt) || extra.IsPhoto
					mediaKind := "video"
					if isImg {
						mediaKind = "image"
						previewStatus = "disabled"
					}
					extraVideo := &catalog.Video{
						ID:              extraVideoID,
						DriveID:         telegramstorage.DriveID,
						FileID:          extraFileID,
						FileName:        storedName,
						Title:           extraTitle,
						Tags:            cleanTags,
						Size:            extra.Size,
						Ext:             strings.TrimPrefix(extraExt, "."),
						PreviewStatus:   previewStatus,
						ThumbnailURL:    thumbnailURL,
						MediaKind:       mediaKind,
						MIMEType:        extra.MIME,
						DurationSeconds: 0,
						PublishedAt:     now,
						CreatedAt:       now,
						UpdatedAt:       now,
					}
					if err := s.cat.UpsertVideo(ctx, extraVideo); err == nil {
						extraUniqueID := fmt.Sprintf("tg:%s/%d", channel, extra.ID)
						_ = s.cat.LinkTelegramVideo(ctx, s.BotID(), extraUniqueID, extraFileID, j.ID, extraVideoID)
						s.mu.RLock()
						notifyFn := s.onVideoUploaded
						s.mu.RUnlock()
						if notifyFn != nil {
							notifyFn(extraVideo)
						}
					}
				}
			}

			return acquiredFile(source, local, destination, primaryItem.Size), nil
		}

		videos, err := ytdlp.DownloadVideoWithProgress(ctx, source.FileName, tempJobDir, s.cfg.MaxFileSizeBytes, func(done, total int64) {
			updateProgress(done, total)
		})
		cancelMonitor()
		if err != nil {
			return out, &mediaimport.SourceError{Message: err.Error()}
		}
		if len(videos) == 0 {
			return out, &mediaimport.SourceError{Message: fmt.Sprintf("未在 %s 链接中检测到视频", platformName)}
		}
		v := videos[0]
		_ = progress(stageName, v.Size, v.Size)
		if err := os.Rename(v.Path, destination); err != nil {
			data, readErr := os.ReadFile(v.Path)
			if readErr != nil {
				return out, &mediaimport.SourceError{Message: fmt.Sprintf("无法读取下载的 %s 视频", platformName)}
			}
			if writeErr := os.WriteFile(destination, data, 0644); writeErr != nil {
				return out, &mediaimport.SourceError{Message: fmt.Sprintf("无法保存下载的 %s 视频", platformName)}
			}
		}
		if err = syncLibrary(destination, filepath.Dir(destination)); err != nil {
			return out, &mediaimport.SourceError{Message: "无法确认 TG 视频已落盘", WaitForAvailability: true}
		}
		source.Size = v.Size
		source.FileName = filepath.Base(destination)
		if v.Title != "" {
			j.RequestedTitle = v.Title
			j.ResolvedTitle = v.Title
		}
		extraTags := ytdlp.ExtractHashtags(v.Title + " " + v.Description)
		if len(extraTags) > 0 {
			seen := make(map[string]bool)
			for _, t := range j.Tags {
				seen[t] = true
			}
			for _, t := range extraTags {
				if !seen[t] {
					seen[t] = true
					j.Tags = append(j.Tags, t)
				}
			}
		}
		return acquiredFile(source, local, destination, v.Size), nil
	}
	free, err := mediaimport.AvailableBytes(s.cfg.LocalFilesRoot)
	reserve := s.getReserve()
	if err != nil || free < reserve || size > free-reserve {
		return out, &mediaimport.SourceError{Message: fmt.Sprintf("TG 共享目录空间不足或低于预留阈值 (%s)", formatBytes(reserve)), WaitForAvailability: true}
	}
	if err = progress("telegram_download", 0, source.Size); err != nil {
		return out, err
	}
	fileID, err := s.cat.LatestTelegramFileID(ctx, source)
	if err != nil {
		return out, err
	}
	s.mu.RLock()
	c := s.client
	s.mu.RUnlock()
	if c == nil {
		return out, &mediaimport.SourceError{Message: "Telegram 尚未连接", RetryAfter: time.Minute}
	}
	fetchCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.FetchTimeoutSeconds)*time.Second)
	defer cancel()

	downloadStart := time.Now()
	monitorCtx, cancelMonitor := context.WithCancel(fetchCtx)
	defer cancelMonitor()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var lastBytes int64
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				bytes := findActiveDownloadBytes(s.cfg.LocalFilesRoot, source.Size, downloadStart)
				if bytes > 0 && bytes > lastBytes {
					lastBytes = bytes
					_ = progress("telegram_download", bytes, source.Size)
				}
			}
		}
	}()

	var file struct {
		Path string `json:"file_path"`
		Size int64  `json:"file_size"`
	}
	err = c.call(fetchCtx, "getFile", map[string]string{"file_id": fileID}, &file)
	cancelMonitor()
	if err != nil {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if apiErr.Code == 401 || apiErr.Code == 403 {
				s.setState("error", errors.New("机器人鉴权失败，请检查 Token"))
			}
			return out, &mediaimport.SourceError{Message: apiErr.Error(), RetryAfter: apiErr.RetryAfter}
		}
		return out, &mediaimport.SourceError{Message: "TG 文件获取超时或响应无效", RetryAfter: 10 * time.Second}
	}
	localPath, err := mapAPIFilePath(s.cfg.APIFilesRoot, s.cfg.LocalFilesRoot, file.Path)
	if err != nil {
		return out, &mediaimport.SourceError{Message: "TG 返回的文件路径不在 Bot API 文件目录内，请检查目录映射"}
	}
	f, info, err := openCachedFile(s.cfg.LocalFilesRoot, localPath)
	if err != nil {
		return out, &mediaimport.SourceError{Message: "TG 返回的文件路径不可用，请检查共享目录挂载"}
	}
	defer f.Close()
	libraryRoot, _ := filepath.EvalSymlinks(filepath.Join(s.cfg.LocalFilesRoot, "library"))
	if rel, e := filepath.Rel(libraryRoot, f.Name()); e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return out, &mediaimport.SourceError{Message: "TG 返回了已接管的视频路径"}
	}
	if info.Size() <= 0 || info.Size() > s.cfg.MaxFileSizeBytes {
		return out, &mediaimport.SourceError{Message: "视频为空或超过当前大小限制"}
	}
	if info.Size() != source.Size || (file.Size > 0 && info.Size() != file.Size) {
		return out, &mediaimport.SourceError{Message: "TG 文件大小不一致，请重新获取", RetryAfter: 10 * time.Second}
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	// Both locations must be on the same filesystem. Never fall back to copying.
	if err = persistence.RLockContext(ctx); err != nil {
		return out, err
	}
	defer persistence.RUnlock()
	current, e := os.Lstat(f.Name())
	if e != nil || !os.SameFile(info, current) {
		return out, &mediaimport.SourceError{Message: "TG 下载文件发生变化，请重试", RetryAfter: time.Second}
	}
	if err = f.Close(); err != nil {
		return out, &mediaimport.SourceError{Message: "无法关闭 TG 下载文件"}
	}
	if err = os.Rename(f.Name(), destination); err != nil {
		return out, &mediaimport.SourceError{Message: "无法接管 TG 视频，请检查共享目录写入权限及 library 是否位于同一文件系统", WaitForAvailability: true}
	}
	if err = syncLibrary(destination, filepath.Dir(destination), filepath.Dir(f.Name())); err != nil {
		return out, &mediaimport.SourceError{Message: "无法确认 TG 视频已落盘", WaitForAvailability: true}
	}
	if err = progress("telegram_download", info.Size(), info.Size()); err != nil {
		return out, err
	}
	return acquiredFile(source, local, destination, info.Size()), nil
}
func openCachedFile(root, path string) (*os.File, os.FileInfo, error) {
	if !filepath.IsAbs(path) {
		return nil, nil, errors.New("expected local Bot API absolute path")
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, nil, err
	}
	actual, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, nil, err
	}
	rel, err := filepath.Rel(base, actual)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return nil, nil, errors.New("file outside cache")
	}
	before, err := os.Stat(actual)
	if err != nil || !before.Mode().IsRegular() {
		return nil, nil, errors.New("cache file is not regular")
	}
	f, err := os.Open(actual)
	if err != nil {
		return nil, nil, err
	}
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		f.Close()
		return nil, nil, errors.New("cache file changed")
	}
	return f, after, nil
}

func acquiredFile(source catalog.TelegramSource, local catalog.TelegramLocalFile, path string, size int64) mediaimport.SourceFile {
	name := source.FileName
	if name == "" {
		name = "telegram-video.mp4"
	}
	return mediaimport.SourceFile{Name: name, MIME: source.MIME, Size: size, Path: path, DriveID: telegramstorage.DriveID, FileID: local.FileID}
}
func (s *Service) Discard(ctx context.Context, j *catalog.RemoteUploadJob) error {
	return telegramstorage.New(s.cat, func() string { return s.cfg.LocalFilesRoot }).Remove(ctx, j.ID+".media")
}

func findActiveDownloadBytes(root string, expectedSize int64, startedAt time.Time) int64 {
	if root == "" {
		return 0
	}
	var bestSize int64
	var bestModTime time.Time

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "library" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".binlog") || strings.HasSuffix(name, ".lock") {
			return nil
		}
		size := info.Size()
		if size <= 0 {
			return nil
		}
		if expectedSize > 0 && size > expectedSize {
			return nil
		}
		modTime := info.ModTime()
		if modTime.Before(startedAt.Add(-5*time.Second)) || time.Since(modTime) > 30*time.Second {
			return nil
		}
		if modTime.After(bestModTime.Add(time.Second)) {
			bestModTime = modTime
			bestSize = size
		} else if modTime.After(bestModTime.Add(-time.Second)) {
			if size > bestSize {
				bestModTime = modTime
				bestSize = size
			}
		}
		return nil
	})

	return bestSize
}

func findDirectoryBytes(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func isExternalSource(uniqueID string) bool {
	return strings.HasPrefix(uniqueID, "x:") ||
		strings.HasPrefix(uniqueID, "yt:") ||
		strings.HasPrefix(uniqueID, "tiktok:") ||
		strings.HasPrefix(uniqueID, "douyin:") ||
		strings.HasPrefix(uniqueID, "bilibili:") ||
		strings.HasPrefix(uniqueID, "tg:") ||
		strings.HasPrefix(uniqueID, "url:")
}

func externalPlatformInfo(uniqueID string) (stageName, platformName string) {
	switch {
	case strings.HasPrefix(uniqueID, "x:"):
		return "x_download", "X"
	case strings.HasPrefix(uniqueID, "yt:"):
		return "youtube_download", "YouTube"
	case strings.HasPrefix(uniqueID, "tiktok:"):
		return "tiktok_download", "TikTok"
	case strings.HasPrefix(uniqueID, "douyin:"):
		return "douyin_download", "抖音"
	case strings.HasPrefix(uniqueID, "bilibili:"):
		return "bilibili_download", "Bilibili"
	case strings.HasPrefix(uniqueID, "tg:"):
		return "telegram_download", "Telegram"
	default:
		return "external_download", "外部"
	}
}

func cleanStaleTempJobs(tempRootDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(tempRootDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(tempRootDir, entry.Name()))
		}
	}
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MiB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GiB", float64(bytes)/(1024*1024*1024))
}

func isImageExtension(ext string) bool {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), ".")) {
	case "jpg", "jpeg", "png", "webp", "gif", "bmp":
		return true
	}
	return false
}
