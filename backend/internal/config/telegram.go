package config

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

type Telegram struct {
	BotToken            string  `yaml:"bot_token" json:"-"`
	Enabled             bool    `yaml:"enabled" json:"enabled"`
	AllowedUserIDs      []int64 `yaml:"allowed_user_ids" json:"allowedUserIds"`
	SiteBaseURL         string  `yaml:"site_base_url" json:"siteBaseUrl"`
	MaxFileSizeBytes    int64   `yaml:"max_file_size_bytes" json:"maxFileSizeBytes"`
	MaxPendingJobs      int     `yaml:"max_pending_jobs" json:"maxPendingJobs"`
	FetchTimeoutSeconds int     `yaml:"fetch_timeout_seconds" json:"fetchTimeoutSeconds"`

	// MTProto User API client credentials (for channel/post video downloads)
	UserClientEnabled bool   `yaml:"user_client_enabled" json:"userClientEnabled"`
	APIID             string `yaml:"api_id" json:"apiId"`
	APIHash           string `yaml:"api_hash" json:"apiHash"`
	SessionString     string `yaml:"session_string" json:"sessionString"`

	// The runtime endpoint and paths come exclusively from the deployment's Compose file.
	APIBaseURL     string `yaml:"-" json:"apiBaseUrl"`
	APIFilesRoot   string `yaml:"-" json:"-"`
	LocalFilesRoot string `yaml:"-" json:"-"`
}

var telegramTokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]+$`)

func (t *Telegram) Validate() error {
	t.BotToken = strings.TrimSpace(t.BotToken)
	t.APIID = strings.TrimSpace(t.APIID)
	t.APIHash = strings.TrimSpace(t.APIHash)
	t.SessionString = strings.TrimSpace(t.SessionString)

	if t.BotToken != "" && !telegramTokenPattern.MatchString(t.BotToken) {
		return errors.New("telegram.bot_token 格式无效")
	}
	if t.Enabled && t.BotToken == "" {
		return errors.New("启用 Telegram 前请填写 bot_token")
	}

	if t.AllowedUserIDs == nil {
		t.AllowedUserIDs = []int64{}
	}
	if t.MaxFileSizeBytes == 0 {
		t.MaxFileSizeBytes = 4 << 30
	}
	if t.MaxPendingJobs == 0 {
		t.MaxPendingJobs = 100
	}
	if t.FetchTimeoutSeconds == 0 {
		t.FetchTimeoutSeconds = 1800
	}
	if t.MaxFileSizeBytes < 1 || t.MaxFileSizeBytes > 16<<30 {
		return errors.New("telegram.max_file_size_bytes 必须在 1 字节到 16 GiB 之间")
	}
	if t.MaxPendingJobs < 1 || t.MaxPendingJobs > 10000 {
		return errors.New("telegram.max_pending_jobs 必须在 1 到 10000 之间")
	}
	if t.FetchTimeoutSeconds < 30 || t.FetchTimeoutSeconds > 86400 {
		return errors.New("telegram.fetch_timeout_seconds 必须在 30 到 86400 之间")
	}
	if t.SiteBaseURL != "" {
		u, err := url.Parse(t.SiteBaseURL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("telegram.site_base_url 必须为有效的站点 HTTP(S) 地址")
		}
	}
	for _, id := range t.AllowedUserIDs {
		if id <= 0 {
			return errors.New("telegram.allowed_user_ids 必须为正整数用户 ID")
		}
	}
	return nil
}

// TelegramSettings returns an isolated copy of the validated live configuration.
func (m *Manager) TelegramSettings() Telegram {
	if m == nil {
		cfg := Telegram{}
		_ = cfg.Validate()
		return cfg
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := m.current.Telegram
	cfg.APIBaseURL = m.telegramDeployment.apiBaseURL
	cfg.APIFilesRoot = m.telegramDeployment.apiRoot
	cfg.LocalFilesRoot = m.telegramDeployment.localRoot
	cfg.AllowedUserIDs = append([]int64{}, cfg.AllowedUserIDs...)
	return cfg
}
