package cookiemanager

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PlatformDef struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	FileName    string `json:"fileName"`
}

var SupportedPlatforms = []PlatformDef{
	{ID: "youtube", DisplayName: "YouTube", FileName: "cookies_youtube.txt"},
	{ID: "douyin", DisplayName: "抖音 (Douyin)", FileName: "cookies_douyin.txt"},
	{ID: "tiktok", DisplayName: "TikTok", FileName: "cookies_tiktok.txt"},
	{ID: "x", DisplayName: "X (Twitter)", FileName: "cookies_x.txt"},
	{ID: "bilibili", DisplayName: "哔哩哔哩 (Bilibili)", FileName: "cookies_bilibili.txt"},
	{ID: "default", DisplayName: "全局默认 (Fallback)", FileName: "cookies.txt"},
}

type PlatformCookieStatus struct {
	Platform    string `json:"platform"`
	DisplayName string `json:"displayName"`
	FileName    string `json:"fileName"`
	Configured  bool   `json:"configured"`
	Exists      bool   `json:"exists"`
	Size        int64  `json:"size"`
	LineCount   int    `json:"lineCount"`
	UpdatedAt   string `json:"updatedAt"`
	ModTime     string `json:"modTime"`
	SHA256      string `json:"sha256"`
	HasBackup   bool   `json:"hasBackup"`
}

type Config struct {
	DataDir    string
	LegacyDirs []string
}

type Manager struct {
	dataDir    string
	legacyDirs []string
	mu         sync.RWMutex
}

func New(cfg Config) (*Manager, error) {
	dir := strings.TrimSpace(cfg.DataDir)
	if dir == "" {
		return nil, errors.New("cookiemanager: dataDir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cookies data dir: %w", err)
	}

	m := &Manager{
		dataDir:    dir,
		legacyDirs: cfg.LegacyDirs,
	}

	// Auto-migrate legacy cookies if dataDir is empty
	m.initialMigration()
	return m, nil
}

func (m *Manager) initialMigration() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range SupportedPlatforms {
		targetPath := filepath.Join(m.dataDir, p.FileName)
		if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
			continue
		}
		candidateNames := []string{p.FileName}
		if p.ID == "x" {
			candidateNames = append(candidateNames, "cookies_twitter.txt")
		} else if p.ID == "douyin" {
			candidateNames = append(candidateNames, "cookies_抖音.txt")
		}

		// Try to find in legacy directories
		migrated := false
		for _, legDir := range m.legacyDirs {
			if legDir == "" || migrated {
				continue
			}
			for _, name := range candidateNames {
				srcPath := filepath.Join(legDir, name)
				if srcInfo, err := os.Stat(srcPath); err == nil && !srcInfo.IsDir() && srcInfo.Size() > 0 {
					data, err := os.ReadFile(srcPath)
					if err == nil && len(data) > 0 {
						_ = os.WriteFile(targetPath, data, 0o600)
						migrated = true
						break
					}
				}
			}
		}
	}
}

func FindPlatformDef(platform string) (PlatformDef, bool) {
	norm := strings.ToLower(strings.TrimSpace(platform))
	if norm == "twitter" {
		norm = "x"
	}
	for _, p := range SupportedPlatforms {
		if strings.ToLower(p.ID) == norm || strings.ToLower(p.FileName) == norm {
			return p, true
		}
	}
	return PlatformDef{}, false
}

// List returns the current operational status for all supported platforms.
func (m *Manager) List(ctx context.Context) ([]PlatformCookieStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]PlatformCookieStatus, 0, len(SupportedPlatforms))
	for _, p := range SupportedPlatforms {
		st := m.getPlatformStatus(p)
		res = append(res, st)
	}
	return res, nil
}

func (m *Manager) getPlatformStatus(p PlatformDef) PlatformCookieStatus {
	st := PlatformCookieStatus{
		Platform:    p.ID,
		DisplayName: p.DisplayName,
		FileName:    p.FileName,
	}

	filePath := m.locateCookieFile(p)
	if filePath == "" {
		return st
	}

	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return st
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return st
	}

	st.Configured = true
	st.Exists = true
	st.Size = info.Size()
	formattedTime := info.ModTime().Format(time.RFC3339)
	st.UpdatedAt = formattedTime
	st.ModTime = formattedTime

	hash := sha256.Sum256(data)
	st.SHA256 = hex.EncodeToString(hash[:])

	// Count lines
	lines := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines++
		}
	}
	st.LineCount = lines

	// Check if backup exists
	bakPath := filepath.Join(m.dataDir, p.FileName+".bak")
	if bInfo, err := os.Stat(bakPath); err == nil && !bInfo.IsDir() && bInfo.Size() > 0 {
		st.HasBackup = true
	}

	return st
}

func (m *Manager) locateCookieFile(p PlatformDef) string {
	candidateNames := []string{p.FileName}
	if p.ID == "x" {
		candidateNames = append(candidateNames, "cookies_twitter.txt")
	} else if p.ID == "douyin" {
		candidateNames = append(candidateNames, "cookies_抖音.txt")
	}

	// 1. Data dir
	for _, name := range candidateNames {
		primary := filepath.Join(m.dataDir, name)
		if info, err := os.Stat(primary); err == nil && !info.IsDir() && info.Size() > 0 {
			return primary
		}
	}
	// 2. Legacy dirs
	for _, legDir := range m.legacyDirs {
		if legDir == "" {
			continue
		}
		for _, name := range candidateNames {
			path := filepath.Join(legDir, name)
			if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
				return path
			}
		}
	}
	return ""
}

// GetContent returns the raw Netscape cookie text for the specified platform.
func (m *Manager) GetContent(ctx context.Context, platform string) (string, error) {
	p, ok := FindPlatformDef(platform)
	if !ok {
		return "", fmt.Errorf("unsupported platform '%s'", platform)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	filePath := m.locateCookieFile(p)
	if filePath == "" {
		return "", nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read cookie file: %w", err)
	}

	return string(data), nil
}

// SaveContent validates and saves the cookie text for the given platform.
// It automatically archives the previous content to a .bak file for rollbacks.
func (m *Manager) SaveContent(ctx context.Context, platform, content string) error {
	p, ok := FindPlatformDef(platform)
	if !ok {
		return fmt.Errorf("unsupported platform '%s'", platform)
	}

	content = strings.TrimSpace(content)
	if content != "" {
		if err := ValidateNetscapeFormat(content); err != nil {
			return err
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	targetPath := filepath.Join(m.dataDir, p.FileName)
	bakPath := filepath.Join(m.dataDir, p.FileName+".bak")

	// Create backup of current file if exists and non-empty
	if curData, err := os.ReadFile(targetPath); err == nil && len(curData) > 0 {
		_ = os.WriteFile(bakPath, curData, 0o600)
	}

	if content == "" {
		// Empty content means remove file
		_ = os.Remove(targetPath)
		for _, legDir := range m.legacyDirs {
			if legDir != "" {
				_ = os.Remove(filepath.Join(legDir, p.FileName))
			}
		}
		return nil
	}

	// Write to primary dataDir
	if err := os.WriteFile(targetPath, []byte(content+"\n"), 0o644); err != nil {
		return fmt.Errorf("write cookie file: %w", err)
	}

	// Mirror to legacyDirs for backwards compatibility if writable
	for _, legDir := range m.legacyDirs {
		if legDir == "" {
			continue
		}
		legPath := filepath.Join(legDir, p.FileName)
		_ = os.WriteFile(legPath, []byte(content+"\n"), 0o644)
	}

	return nil
}

// Restore rolls back the platform's cookie file to its previous .bak version.
func (m *Manager) Restore(ctx context.Context, platform string) error {
	p, ok := FindPlatformDef(platform)
	if !ok {
		return fmt.Errorf("unsupported platform '%s'", platform)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	bakPath := filepath.Join(m.dataDir, p.FileName+".bak")
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		return errors.New("no backup available for this platform")
	}

	targetPath := filepath.Join(m.dataDir, p.FileName)
	if err := os.WriteFile(targetPath, bakData, 0o644); err != nil {
		return fmt.Errorf("restore cookie file: %w", err)
	}

	for _, legDir := range m.legacyDirs {
		if legDir != "" {
			_ = os.WriteFile(filepath.Join(legDir, p.FileName), bakData, 0o644)
		}
	}

	return nil
}

// Delete removes the active cookie file for the specified platform.
func (m *Manager) Delete(ctx context.Context, platform string) error {
	return m.SaveContent(ctx, platform, "")
}

// ValidateNetscapeFormat validates that the content conforms to the standard
// Netscape HTTP Cookie file format, rejecting accidental HTML, JSON or garbage.
func ValidateNetscapeFormat(content string) error {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "<!doctype") ||
		strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<body") ||
		strings.Contains(lower, "<script") {
		return errors.New("检测到网页 HTML 内容，不是有效的 Netscape cookies.txt 格式")
	}

	if strings.HasPrefix(strings.TrimSpace(lower), "{") && strings.HasSuffix(strings.TrimSpace(lower), "}") {
		return errors.New("检测到 JSON 数据，请提供标准的 Netscape cookies.txt 纯文本格式")
	}

	lines := strings.Split(content, "\n")
	hasValidCookieLine := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// A valid Netscape cookie line contains at least 7 fields separated by tabs or spaces
		parts := strings.Split(trimmed, "\t")
		if len(parts) >= 6 {
			hasValidCookieLine = true
			break
		}
	}

	if !hasValidCookieLine {
		// If tabs were not preserved (e.g. pasted into a loose form with spaces), check space separation
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) >= 7 {
				hasValidCookieLine = true
				break
			}
		}
	}

	if !hasValidCookieLine {
		return errors.New("未检测到有效的 Cookie 记录行。请使用浏览器扩展（如 Get cookies.txt LOCALLY）导出的标准 Netscape 格式文本")
	}

	return nil
}
