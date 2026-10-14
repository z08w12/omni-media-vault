package transcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/drives"
	"github.com/video-site/backend/internal/drives/localupload"
	"github.com/video-site/backend/internal/drives/telegramstorage"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusProcessing TaskStatus = "processing"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

type Task struct {
	VideoID    string     `json:"videoId"`
	VideoTitle string     `json:"videoTitle"`
	Status     TaskStatus `json:"status"`
	Reason     string     `json:"reason,omitempty"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

type Config struct {
	FFmpegPath     string
	FFprobePath    string
	TempDir        string
	UploadDir      string
	Catalog        *catalog.Catalog
	GetDrive       func(driveID string) (drives.Drive, bool)
	OnRegenPreview func(videoID string)
}

type Service struct {
	cfg    Config
	mu     sync.Mutex
	tasks  map[string]*Task
	queue  []string
	active *Task
	taskCh chan string
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(cfg Config) *Service {
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = "ffmpeg"
	}
	if cfg.FFprobePath == "" {
		cfg.FFprobePath = "ffprobe"
	}
	if cfg.TempDir == "" {
		cfg.TempDir = filepath.Join(os.TempDir(), "omni-transcode-tmp")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		cfg:    cfg,
		tasks:  make(map[string]*Task),
		queue:  make([]string, 0),
		taskCh: make(chan string, 100),
		ctx:    ctx,
		cancel: cancel,
	}

	s.wg.Add(1)
	go s.worker()

	return s
}

func (s *Service) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *Service) Enqueue(ctx context.Context, videoID string) (*Task, error) {
	return s.EnqueueWithReason(ctx, videoID, "")
}

func (s *Service) EnqueueWithReason(ctx context.Context, videoID, reason string) (*Task, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return nil, errors.New("invalid video id")
	}

	if s.cfg.Catalog == nil {
		return nil, errors.New("catalog unavailable")
	}

	v, err := s.cfg.Catalog.GetVideo(ctx, videoID)
	if err != nil {
		return nil, fmt.Errorf("video not found: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.tasks[videoID]; ok {
		if existing.Status == StatusPending || existing.Status == StatusProcessing {
			cp := *existing
			return &cp, nil
		}
	}

	task := &Task{
		VideoID:    videoID,
		VideoTitle: v.Title,
		Status:     StatusPending,
		Reason:     reason,
		CreatedAt:  time.Now(),
	}
	s.tasks[videoID] = task
	s.queue = append(s.queue, videoID)

	select {
	case s.taskCh <- videoID:
	default:
	}

	cp := *task
	return &cp, nil
}

// Prioritize moves a pending transcode task to the front of the queue,
// giving it immediate precedence once the current active job completes.
func (s *Service) Prioritize(videoID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[videoID]
	if !ok || task.Status != StatusPending {
		return false
	}

	newQueue := make([]string, 0, len(s.queue))
	newQueue = append(newQueue, videoID)
	for _, id := range s.queue {
		if id != videoID {
			newQueue = append(newQueue, id)
		}
	}
	s.queue = newQueue

	select {
	case s.taskCh <- videoID:
	default:
	}

	return true
}

func (s *Service) GetStatus(videoID string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.tasks[videoID]; ok {
		cp := *t
		return &cp
	}
	return nil
}

type QueueStatus struct {
	Active  *Task   `json:"active,omitempty"`
	Pending []*Task `json:"pending"`
	Recent  []*Task `json:"recent"`
}

func (s *Service) GetQueueStatus() QueueStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	var activeCopy *Task
	if s.active != nil {
		cp := *s.active
		activeCopy = &cp
	}

	pending := make([]*Task, 0)
	for _, id := range s.queue {
		if t, ok := s.tasks[id]; ok && t.Status == StatusPending {
			cp := *t
			pending = append(pending, &cp)
		}
	}

	recent := make([]*Task, 0)
	for _, t := range s.tasks {
		if t.Status == StatusCompleted || t.Status == StatusFailed {
			cp := *t
			recent = append(recent, &cp)
		}
	}

	return QueueStatus{
		Active:  activeCopy,
		Pending: pending,
		Recent:  recent,
	}
}

func (s *Service) worker() {
	defer s.wg.Done()

	for {
		var nextVideoID string

		s.mu.Lock()
		for _, id := range s.queue {
			if t, ok := s.tasks[id]; ok && t.Status == StatusPending {
				nextVideoID = id
				break
			}
		}
		s.mu.Unlock()

		if nextVideoID != "" {
			s.runNext(nextVideoID)
			continue
		}

		select {
		case <-s.ctx.Done():
			return
		case videoID := <-s.taskCh:
			s.runNext(videoID)
		}
	}
}

// CheckAndEnqueue performs an immediate fast probe on an ingested video.
// If the video is detected as incompatible with modern browser playback,
// it is automatically enqueued into the transcode background worker.
func (s *Service) CheckAndEnqueue(ctx context.Context, videoID string) (bool, string, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" || s.cfg.Catalog == nil {
		return false, "", errors.New("invalid video or catalog unavailable")
	}

	v, err := s.cfg.Catalog.GetVideo(ctx, videoID)
	if err != nil {
		return false, "", fmt.Errorf("lookup video %s: %w", videoID, err)
	}

	// 1. If storage kind is not local-writable, skip
	if !IsSupportedLocalDrive(v.DriveID) {
		return false, "unsupported storage kind for in-place transcode", nil
	}

	// 2. Resolve source path
	sourcePath, isLocal, err := s.resolveSourcePath(ctx, v)
	if err != nil || sourcePath == "" || !isLocal {
		return false, "source file not accessible locally", nil
	}

	// 3. Fast probe file
	info, err := ProbeFile(ctx, s.cfg.FFprobePath, sourcePath)
	if err != nil {
		return false, "", fmt.Errorf("probe file: %w", err)
	}

	// 4. Check compatibility
	result := CheckCompatibility(info, v.Ext)
	if result.Compatible {
		return false, "", nil
	}

	// 5. Incompatible: enqueue for transcode
	_, err = s.EnqueueWithReason(ctx, videoID, result.Reason)
	if err != nil {
		return false, "", fmt.Errorf("auto-enqueue transcode: %w", err)
	}

	log.Printf("[transcode] auto-enqueued video %s (%s, ext: %s) for transcode: %s", videoID, v.Title, v.Ext, result.Reason)
	return true, result.Reason, nil
}

func (s *Service) resolveSourcePath(ctx context.Context, v *catalog.Video) (string, bool, error) {
	if s.cfg.GetDrive != nil {
		if drv, ok := s.cfg.GetDrive(v.DriveID); ok && drv != nil {
			if lfp, ok := drv.(drives.LocalFileProvider); ok {
				if p, err := lfp.LocalPath(ctx, v.FileID); err == nil && p != "" {
					return p, true, nil
				}
			}
		}
	}

	if v.DriveID == localupload.DriveID && s.cfg.UploadDir != "" {
		p := filepath.Join(s.cfg.UploadDir, v.FileID)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, true, nil
		}
	}

	if s.cfg.GetDrive != nil {
		if drv, ok := s.cfg.GetDrive(v.DriveID); ok && drv != nil {
			link, err := drv.StreamURL(ctx, v.FileID)
			if err != nil {
				return "", false, fmt.Errorf("resolve stream url: %w", err)
			}
			if link != nil && link.URL != "" {
				if filepath.IsAbs(link.URL) {
					if info, err := os.Stat(link.URL); err == nil && !info.IsDir() {
						return link.URL, true, nil
					}
				}
				return link.URL, false, nil
			}
		}
	}

	return "", false, fmt.Errorf("unable to locate video source path for %s", v.ID)
}

func (s *Service) runNext(videoID string) {
	s.mu.Lock()
	task, ok := s.tasks[videoID]
	if !ok || task.Status != StatusPending {
		s.mu.Unlock()
		return
	}

	// Remove from pending queue
	for i, id := range s.queue {
		if id == videoID {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			break
		}
	}

	now := time.Now()
	task.Status = StatusProcessing
	task.StartedAt = &now
	s.active = task
	s.mu.Unlock()

	err := s.executeTranscode(s.ctx, videoID)

	s.mu.Lock()
	finished := time.Now()
	task.FinishedAt = &finished
	if err != nil {
		task.Status = StatusFailed
		task.Error = err.Error()
		log.Printf("[transcode] video %s failed: %v", videoID, err)
	} else {
		task.Status = StatusCompleted
		task.Error = ""
		log.Printf("[transcode] video %s completed successfully", videoID)
	}
	s.active = nil
	s.mu.Unlock()
}

func (s *Service) executeTranscode(ctx context.Context, videoID string) error {
	v, err := s.cfg.Catalog.GetVideo(ctx, videoID)
	if err != nil {
		return fmt.Errorf("lookup video %s: %w", videoID, err)
	}

	sourcePath, isLocalFile, err := s.resolveSourcePath(ctx, v)
	if err != nil {
		return err
	}

	// 2. Prepare temporary output path
	if err := os.MkdirAll(s.cfg.TempDir, 0o755); err != nil {
		return fmt.Errorf("create temp transcode dir: %w", err)
	}
	tempDest := filepath.Join(s.cfg.TempDir, fmt.Sprintf("tc_%s_%d.mp4", v.ID, time.Now().UnixNano()))
	defer os.Remove(tempDest)

	// 3. Execute FFmpeg encoding to H.264 + AAC + faststart MP4
	ffmpegPath := s.cfg.FFmpegPath
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-threads", "2",
		"-i", sourcePath,
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "44100",
		"-ac", "2",
		"-movflags", "+faststart",
		tempDest,
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg encode failed: %w (stderr: %s)", err, strings.TrimSpace(string(out)))
	}

	newStat, err := os.Stat(tempDest)
	if err != nil || newStat.Size() == 0 {
		return errors.New("transcoded file is missing or empty")
	}

	// 4. Probe duration of transcoded file
	durationSec := v.DurationSeconds
	ffprobePath := s.cfg.FFprobePath
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	probeCmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		tempDest,
	)
	probeOut, err := probeCmd.CombinedOutput()
	if err == nil {
		durStr := strings.TrimSpace(string(probeOut))
		if durFloat, err := strconv.ParseFloat(durStr, 64); err == nil && durFloat > 0 {
			durationSec = int(durFloat + 0.5)
		}
	}

	// 5. Replace source file and delete the original unplayable file
	newFileID := v.FileID
	switch {
	case v.DriveID == telegramstorage.DriveID:
		// Telegram local library file
		baseID := strings.TrimSuffix(v.FileID, filepath.Ext(v.FileID))
		newFileID = baseID + ".mp4"
		targetPath := filepath.Join(filepath.Dir(sourcePath), newFileID)

		if err := moveOrCopyFile(tempDest, targetPath); err != nil {
			return fmt.Errorf("place transcoded telegram file: %w", err)
		}
		if sourcePath != targetPath {
			_ = os.Remove(sourcePath)
		}
		if err := s.cfg.Catalog.ReplaceTelegramLocalFile(ctx, v.FileID, newFileID); err != nil {
			log.Printf("[transcode] replace telegram file id: %v", err)
		}

	case v.DriveID == localupload.DriveID:
		// Local upload file
		baseID := strings.TrimSuffix(v.FileID, filepath.Ext(v.FileID))
		newFileID = baseID + ".mp4"
		targetPath := filepath.Join(s.cfg.UploadDir, newFileID)

		if err := moveOrCopyFile(tempDest, targetPath); err != nil {
			return fmt.Errorf("place transcoded upload file: %w", err)
		}
		if sourcePath != targetPath {
			_ = os.Remove(sourcePath)
		}

	case isLocalFile:
		// Other local disk storage (e.g. localstorage / scriptcrawler)
		basePath := strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath))
		targetPath := basePath + ".mp4"

		if err := moveOrCopyFile(tempDest, targetPath); err != nil {
			return fmt.Errorf("place transcoded local file: %w", err)
		}
		if sourcePath != targetPath {
			_ = os.Remove(sourcePath)
		}
		baseID := strings.TrimSuffix(v.FileID, filepath.Ext(v.FileID))
		newFileID = baseID + ".mp4"

	default:
		return fmt.Errorf("storage kind %s does not support transcode", v.DriveID)
	}

	// 6. Update video catalog entry
	newFileName := strings.TrimSuffix(v.FileName, "."+v.Ext) + ".mp4"
	if err := s.cfg.Catalog.UpdateVideoAfterTranscode(ctx, v.ID, newFileID, newFileName, "mp4", newStat.Size(), durationSec); err != nil {
		return fmt.Errorf("update video catalog after transcode: %w", err)
	}

	// 7. Regenerate preview video / thumbnail from new playable source
	if s.cfg.OnRegenPreview != nil {
		s.cfg.OnRegenPreview(v.ID)
	}

	return nil
}

func moveOrCopyFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

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

	if _, err := io.Copy(out, in); err != nil {
		_ = os.Remove(dst)
		return err
	}

	_ = in.Close()
	_ = out.Close()
	_ = os.Remove(src)
	return nil
}
