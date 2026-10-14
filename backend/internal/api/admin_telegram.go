package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/telegram"
)

func (a *AdminServer) handleTelegramStatus(w http.ResponseWriter, r *http.Request) {
	if a.Telegram == nil {
		http.Error(w, "Telegram 服务未配置", http.StatusServiceUnavailable)
		return
	}
	status := a.Telegram.Status()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":    a.ConfigManager.LiveSettings().TelegramEnabled,
		"connection": status,
	})
}
func (a *AdminServer) handleTelegramTest(w http.ResponseWriter, r *http.Request) {
	if a.Telegram == nil {
		http.Error(w, "Telegram 服务未配置", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	name, err := a.Telegram.Test(ctx)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": name})
}

type userClientTestReq struct {
	APIID   string `json:"apiId"`
	APIHash string `json:"apiHash"`
	Session string `json:"session"`
}

func (a *AdminServer) handleTelegramUserClientTest(w http.ResponseWriter, r *http.Request) {
	var req userClientTestReq
	_ = json.NewDecoder(r.Body).Decode(&req)

	cfg := a.ConfigManager.TelegramSettings()
	apiID := strings.TrimSpace(req.APIID)
	if apiID == "" {
		apiID = cfg.APIID
	}
	apiHash := strings.TrimSpace(req.APIHash)
	if apiHash == "" {
		apiHash = cfg.APIHash
	}
	session := strings.TrimSpace(req.Session)
	if session == "" {
		session = cfg.SessionString
	}

	if apiID == "" || apiHash == "" || session == "" {
		writeErr(w, r, http.StatusBadRequest, errors.New("缺少 API ID、API Hash 或 Session 凭据"))
		return
	}

	res, err := telegram.TestUserClient(r.Context(), apiID, apiHash, session)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type userClientSendCodeReq struct {
	APIID   string `json:"apiId"`
	APIHash string `json:"apiHash"`
	Phone   string `json:"phone"`
}

func (a *AdminServer) handleTelegramUserClientSendCode(w http.ResponseWriter, r *http.Request) {
	var req userClientSendCodeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("请求体格式错误"))
		return
	}

	cfg := a.ConfigManager.TelegramSettings()
	apiID := strings.TrimSpace(req.APIID)
	if apiID == "" {
		apiID = cfg.APIID
	}
	apiHash := strings.TrimSpace(req.APIHash)
	if apiHash == "" {
		apiHash = cfg.APIHash
	}
	phone := strings.TrimSpace(req.Phone)

	if apiID == "" || apiHash == "" {
		writeErr(w, r, http.StatusBadRequest, errors.New("请先填写 API ID 和 API Hash"))
		return
	}
	if phone == "" {
		writeErr(w, r, http.StatusBadRequest, errors.New("请填写手机号码（含国家区号，如 +8613800000000）"))
		return
	}

	res, err := telegram.SendUserClientCode(r.Context(), apiID, apiHash, phone)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type userClientSignInReq struct {
	APIID         string `json:"apiId"`
	APIHash       string `json:"apiHash"`
	Phone         string `json:"phone"`
	Code          string `json:"code"`
	PhoneCodeHash string `json:"phoneCodeHash"`
	Session       string `json:"session"`
	Password      string `json:"password"`
}

func (a *AdminServer) handleTelegramUserClientSignIn(w http.ResponseWriter, r *http.Request) {
	var req userClientSignInReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("请求体格式错误"))
		return
	}

	cfg := a.ConfigManager.TelegramSettings()
	apiID := strings.TrimSpace(req.APIID)
	if apiID == "" {
		apiID = cfg.APIID
	}
	apiHash := strings.TrimSpace(req.APIHash)
	if apiHash == "" {
		apiHash = cfg.APIHash
	}
	phone := strings.TrimSpace(req.Phone)
	code := strings.TrimSpace(req.Code)
	phoneCodeHash := strings.TrimSpace(req.PhoneCodeHash)
	session := strings.TrimSpace(req.Session)
	password := strings.TrimSpace(req.Password)

	if code == "" {
		writeErr(w, r, http.StatusBadRequest, errors.New("请填写验证码"))
		return
	}

	res, err := telegram.SignInUserClient(r.Context(), apiID, apiHash, phone, code, phoneCodeHash, session, password)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	if res.OK && res.Session != "" && a.ConfigManager != nil {
		if err := a.ConfigManager.UpdateTelegramUserClientSession(res.Session, apiID, apiHash); err != nil {
			log.Printf("[telegram] failed to auto-persist user client session: %v", err)
		} else {
			log.Printf("[telegram] user client session automatically persisted to config.yaml")
		}
	}
	writeJSON(w, http.StatusOK, res)
}
func (a *AdminServer) handleTelegramPrepare(w http.ResponseWriter, r *http.Request) {
	a.telegramAction(w, r, false)
}
func (a *AdminServer) handleTelegramResume(w http.ResponseWriter, r *http.Request) {
	a.telegramAction(w, r, true)
}
func (a *AdminServer) telegramAction(w http.ResponseWriter, r *http.Request, resume bool) {
	if a.Telegram == nil {
		http.Error(w, "Telegram 服务未配置", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	var err error
	if resume {
		err = a.Telegram.Resume(ctx)
	} else {
		err = a.Telegram.PreparePolling(ctx)
	}
	if err != nil {
		writeErr(w, r, http.StatusConflict, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ImportJobDTO struct {
	RemoteUploadJobDTO
	SourceKind  string `json:"sourceKind"`
	Stage       string `json:"stage"`
	RetryCount  int    `json:"retryCount"`
	NextAttempt int64  `json:"nextAttempt"`
	Sequence    string `json:"sequence"`
	SenderID    string `json:"senderId,omitempty"`
	CanRetry    bool   `json:"canRetry"`
}

func importDTO(j *catalog.RemoteUploadJob) ImportJobDTO {
	dto := ImportJobDTO{RemoteUploadJobDTO: mapRemoteUploadJob(j), SourceKind: j.SourceKind, Stage: j.Stage, RetryCount: j.RetryCount, NextAttempt: j.NextAttempt, Sequence: strconv.FormatInt(j.Sequence, 10)}
	if j.SourceKind == "telegram" {
		if source, err := catalog.DecodeTelegramSource(j); err == nil {
			dto.SenderID = strconv.FormatInt(source.SenderID, 10)
			dto.CanRetry = j.State == catalog.RemoteUploadFailed || j.State == catalog.RemoteUploadCanceled
		}
	}
	return dto
}
func (a *AdminServer) handleImportList(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	state := r.URL.Query().Get("state")
	if source != "" && source != "telegram" && source != "http" {
		writeErr(w, r, 400, errors.New("无效的任务来源"))
		return
	}
	switch state {
	case "", catalog.ImportJobFilterActive, catalog.RemoteUploadQueued, catalog.RemoteUploadDownloading, catalog.RemoteUploadValidating, catalog.RemoteUploadSaving, catalog.RemoteUploadCompleted, catalog.RemoteUploadFailed, catalog.RemoteUploadCanceled:
	default:
		writeErr(w, r, 400, errors.New("无效的任务状态"))
		return
	}
	limit := 30
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			writeErr(w, r, 400, errors.New("limit 必须为 1 到 100 的整数"))
			return
		}
	}
	var before int64
	if raw := r.URL.Query().Get("before"); raw != "" {
		var err error
		before, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || before < 1 {
			writeErr(w, r, 400, errors.New("无效的分页游标"))
			return
		}
	}
	jobs, err := a.Catalog.ListImportJobs(r.Context(), source, state, before, limit)
	if err != nil {
		writeErr(w, r, 500, errors.New("无法读取导入任务"))
		return
	}
	out := []ImportJobDTO{}
	for _, j := range jobs {
		out = append(out, importDTO(j))
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, out)
}
func (a *AdminServer) handleImportCancel(w http.ResponseWriter, r *http.Request) {
	if a.Imports == nil {
		writeErr(w, r, 503, errors.New("导入服务未配置"))
		return
	}
	id := routeParam(r, "jobId")
	job, err := a.Imports.Cancel(r.Context(), id)
	if errors.Is(err, catalog.ErrRemoteUploadTerminal) {
		job, err = a.Catalog.GetRemoteUploadJob(r.Context(), id)
	}
	if err != nil {
		status := 500
		if errors.Is(err, sql.ErrNoRows) {
			status = 404
		}
		writeErr(w, r, status, errors.New("无法取消导入任务"))
		return
	}
	writeJSON(w, 200, importDTO(job))
}
func (a *AdminServer) handleImportRetry(w http.ResponseWriter, r *http.Request) {
	if a.Telegram == nil || a.Imports == nil || !a.Telegram.Available() {
		writeErr(w, r, 409, errors.New("请先连接 Telegram 机器人"))
		return
	}
	id := routeParam(r, "jobId")
	j, err := a.Catalog.GetRemoteUploadJob(r.Context(), id)
	if err != nil {
		writeErr(w, r, 404, errors.New("任务不存在"))
		return
	}
	s, err := catalog.DecodeTelegramSource(j)
	if err != nil || !a.Telegram.Allowed(s.SenderID) {
		writeErr(w, r, 409, errors.New("该任务无法重试或发送者已无权限"))
		return
	}
	if err = a.Catalog.RetryTelegramImport(r.Context(), id, a.Telegram.BotID()); err != nil {
		writeErr(w, r, 409, errors.New("该文件已有其他任务、已保存或属于其他机器人"))
		return
	}
	a.Imports.Wake()
	w.WriteHeader(http.StatusNoContent)
}
