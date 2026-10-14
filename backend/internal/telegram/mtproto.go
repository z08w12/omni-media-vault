package telegram

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type MTProtoUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Username  string `json:"username"`
	Phone     string `json:"phone"`
}

type MTProtoTestResult struct {
	OK    bool         `json:"ok"`
	User  *MTProtoUser `json:"user,omitempty"`
	Error string       `json:"error,omitempty"`
}

type MTProtoSendCodeResult struct {
	OK            bool   `json:"ok"`
	PhoneCodeHash string `json:"phoneCodeHash,omitempty"`
	Session       string `json:"session,omitempty"`
	Error         string `json:"error,omitempty"`
}

type MTProtoSignInResult struct {
	OK            bool         `json:"ok"`
	NeedsPassword bool         `json:"needsPassword,omitempty"`
	Session       string       `json:"session,omitempty"`
	User          *MTProtoUser `json:"user,omitempty"`
	Error         string       `json:"error,omitempty"`
}

type MTProtoDownloadResult struct {
	Path     string   `json:"path"`
	Size     int64    `json:"size"`
	Filename string   `json:"filename"`
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
}

func FindScriptsDir() string {
	if envDir := os.Getenv("VIDEO_SCRIPTS_DIR"); envDir != "" {
		if fi, err := os.Stat(envDir); err == nil && fi.IsDir() {
			return envDir
		}
	}
	candidates := []string{
		"./scripts",
		"../scripts",
		"../../scripts",
		"/opt/video-site-91/scripts",
	}
	exe, err := os.Executable()
	if err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "scripts"))
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return "./scripts"
}

func FindPythonExecutable(scriptsDir string) string {
	candidates := []string{
		filepath.Join(scriptsDir, "venv", "bin", "python3"),
		filepath.Join(scriptsDir, "venv", "bin", "python"),
		filepath.Join(scriptsDir, "venv", "Scripts", "python.exe"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	if p, err := exec.LookPath("python3"); err == nil {
		return p
	}
	if p, err := exec.LookPath("python"); err == nil {
		return p
	}
	return "python3"
}

func FindTgDownloaderScript(scriptsDir string) (string, error) {
	scriptPath := filepath.Join(scriptsDir, "tg_downloader.py")
	if fi, err := os.Stat(scriptPath); err == nil && !fi.IsDir() {
		return scriptPath, nil
	}
	// Try fallback locations
	candidates := []string{
		"./scripts/tg_downloader.py",
		"../scripts/tg_downloader.py",
		"../../scripts/tg_downloader.py",
		"/opt/video-site-91/scripts/tg_downloader.py",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("未找到 tg_downloader.py 脚本 (查找路径: %s)", scriptPath)
}

// TestUserClient tests whether the given MTProto User credentials are valid.
func TestUserClient(ctx context.Context, apiID, apiHash, session string) (*MTProtoTestResult, error) {
	scriptsDir := FindScriptsDir()
	pythonPath := FindPythonExecutable(scriptsDir)
	scriptPath, err := FindTgDownloaderScript(scriptsDir)
	if err != nil {
		return nil, err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, pythonPath, scriptPath, "test",
		"--api-id", apiID,
		"--api-hash", apiHash,
		"--session", session,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	outBytes := stdout.Bytes()

	var pyResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		User  *struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
			Phone     string `json:"phone"`
		} `json:"user"`
	}

	if jsonErr := json.Unmarshal(outBytes, &pyResp); jsonErr == nil {
		res := &MTProtoTestResult{
			OK:    pyResp.OK,
			Error: pyResp.Error,
		}
		if pyResp.User != nil {
			res.User = &MTProtoUser{
				ID:        pyResp.User.ID,
				FirstName: pyResp.User.FirstName,
				LastName:  pyResp.User.LastName,
				Username:  pyResp.User.Username,
				Phone:     pyResp.User.Phone,
			}
		}
		if !res.OK && res.Error != "" {
			return res, errors.New(res.Error)
		}
		return res, nil
	}

	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = strings.TrimSpace(stdout.String())
		}
		if errStr == "" {
			errStr = err.Error()
		}
		return nil, errors.New(errStr)
	}

	return nil, errors.New("无法解析测试连接响应")
}

// SendUserClientCode requests a login verification code via SMS/Telegram app.
func SendUserClientCode(ctx context.Context, apiID, apiHash, phone string) (*MTProtoSendCodeResult, error) {
	scriptsDir := FindScriptsDir()
	pythonPath := FindPythonExecutable(scriptsDir)
	scriptPath, err := FindTgDownloaderScript(scriptsDir)
	if err != nil {
		return nil, err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, pythonPath, scriptPath, "send-code",
		"--api-id", apiID,
		"--api-hash", apiHash,
		"--phone", phone,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	outBytes := stdout.Bytes()

	var pyResp struct {
		OK            bool   `json:"ok"`
		PhoneCodeHash string `json:"phone_code_hash"`
		Session       string `json:"session"`
		Error         string `json:"error"`
	}

	if jsonErr := json.Unmarshal(outBytes, &pyResp); jsonErr == nil {
		res := &MTProtoSendCodeResult{
			OK:            pyResp.OK,
			PhoneCodeHash: pyResp.PhoneCodeHash,
			Session:       pyResp.Session,
			Error:         pyResp.Error,
		}
		if !res.OK && res.Error != "" {
			return res, errors.New(res.Error)
		}
		return res, nil
	}

	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = strings.TrimSpace(stdout.String())
		}
		if errStr == "" {
			errStr = err.Error()
		}
		return nil, errors.New(errStr)
	}

	return nil, errors.New("无法解析发送验证码响应")
}

// SignInUserClient signs in using the received code (and optional 2FA password).
func SignInUserClient(ctx context.Context, apiID, apiHash, phone, code, phoneCodeHash, session, password string) (*MTProtoSignInResult, error) {
	scriptsDir := FindScriptsDir()
	pythonPath := FindPythonExecutable(scriptsDir)
	scriptPath, err := FindTgDownloaderScript(scriptsDir)
	if err != nil {
		return nil, err
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{
		scriptPath, "sign-in",
		"--api-id", apiID,
		"--api-hash", apiHash,
		"--phone", phone,
		"--code", code,
		"--phone-code-hash", phoneCodeHash,
		"--session", session,
	}
	if password != "" {
		args = append(args, "--password", password)
	}

	cmd := exec.CommandContext(cmdCtx, pythonPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	outBytes := stdout.Bytes()

	var pyResp struct {
		OK            bool   `json:"ok"`
		NeedsPassword bool   `json:"needs_password"`
		Session       string `json:"session"`
		Error         string `json:"error"`
		User          *struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
			Phone     string `json:"phone"`
		} `json:"user"`
	}

	if jsonErr := json.Unmarshal(outBytes, &pyResp); jsonErr == nil {
		res := &MTProtoSignInResult{
			OK:            pyResp.OK,
			NeedsPassword: pyResp.NeedsPassword,
			Session:       pyResp.Session,
			Error:         pyResp.Error,
		}
		if pyResp.User != nil {
			res.User = &MTProtoUser{
				ID:        pyResp.User.ID,
				FirstName: pyResp.User.FirstName,
				LastName:  pyResp.User.LastName,
				Username:  pyResp.User.Username,
				Phone:     pyResp.User.Phone,
			}
		}
		if !res.OK && !res.NeedsPassword && res.Error != "" {
			return res, errors.New(res.Error)
		}
		return res, nil
	}

	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if errStr == "" {
			errStr = strings.TrimSpace(stdout.String())
		}
		if errStr == "" {
			errStr = err.Error()
		}
		return nil, errors.New(errStr)
	}

	return nil, errors.New("无法解析登录响应")
}

// DownloadMTProtoPost downloads media from a telegram channel post using MTProto User API.
func DownloadMTProtoPost(
	ctx context.Context,
	apiID, apiHash, session, channel string,
	msgID int64,
	outDir string,
	maxSizeBytes int64,
	onProgress func(done, total int64),
) (*MTProtoDownloadResult, error) {
	scriptsDir := FindScriptsDir()
	pythonPath := FindPythonExecutable(scriptsDir)
	scriptPath, err := FindTgDownloaderScript(scriptsDir)
	if err != nil {
		return nil, err
	}

	args := []string{
		scriptPath, "download",
		"--api-id", apiID,
		"--api-hash", apiHash,
		"--session", session,
		"--channel", channel,
		"--msg-id", strconv.FormatInt(msgID, 10),
		"--out-dir", outDir,
	}
	if maxSizeBytes > 0 {
		args = append(args, "--max-size", strconv.FormatInt(maxSizeBytes, 10))
	}

	cmd := exec.CommandContext(ctx, pythonPath, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建输出管道失败: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 MTProto 下载器失败: %w", err)
	}

	var result *MTProtoDownloadResult
	var lastErrorMessage string

	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "progress":
			if onProgress != nil {
				var bytesVal, totalVal int64
				if b, ok := msg["bytes"].(float64); ok {
					bytesVal = int64(b)
				}
				if t, ok := msg["total"].(float64); ok {
					totalVal = int64(t)
				}
				onProgress(bytesVal, totalVal)
			}
		case "done":
			res := &MTProtoDownloadResult{}
			if p, ok := msg["path"].(string); ok {
				res.Path = p
			}
			if s, ok := msg["size"].(float64); ok {
				res.Size = int64(s)
			}
			if f, ok := msg["filename"].(string); ok {
				res.Filename = f
			}
			if t, ok := msg["title"].(string); ok {
				res.Title = t
			}
			if rawTags, ok := msg["tags"].([]any); ok {
				for _, rt := range rawTags {
					if ts, ok := rt.(string); ok && ts != "" {
						res.Tags = append(res.Tags, ts)
					}
				}
			}
			result = res
		case "error":
			if m, ok := msg["message"].(string); ok {
				lastErrorMessage = m
			}
		}
	}

	waitErr := cmd.Wait()
	if waitErr != nil {
		if lastErrorMessage != "" {
			return nil, errors.New(lastErrorMessage)
		}
		errStr := strings.TrimSpace(stderr.String())
		if errStr != "" {
			return nil, errors.New(errStr)
		}
		return nil, fmt.Errorf("MTProto 下载进程异常退出: %w", waitErr)
	}

	if result == nil {
		if lastErrorMessage != "" {
			return nil, errors.New(lastErrorMessage)
		}
		return nil, errors.New("MTProto 下载完成但未返回有效结果")
	}

	return result, nil
}
