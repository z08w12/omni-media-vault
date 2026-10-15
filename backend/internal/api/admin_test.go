package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/video-site/backend/internal/auth"
	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/drives/scriptcrawler"
)

type adminTestDriveConfigLease struct {
	busyReason     string
	deferred       bool
	scope          DriveConfigUpdateScope
	committedScope DriveConfigUpdateScope
	pendingApply   []func() error
	released       bool
}

func (lease *adminTestDriveConfigLease) Authorize(scope DriveConfigUpdateScope) string {
	lease.scope = scope
	return lease.busyReason
}

func (lease *adminTestDriveConfigLease) Commit(scope DriveConfigUpdateScope, apply func() error) (bool, error) {
	lease.committedScope |= scope
	if lease.deferred {
		if apply != nil {
			lease.pendingApply = append(lease.pendingApply, apply)
		}
		return true, nil
	}
	if apply == nil {
		return false, nil
	}
	return false, apply()
}

func (lease *adminTestDriveConfigLease) Release() {
	lease.released = true
}

func TestHandleUpdateVideoRejectsReadOnlyTitleAndAuthor(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()
	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID: "video-read-only", DriveID: "drive", FileID: "file", FileName: "file.mp4",
		Title: "file", Author: "author", PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/admin/api/videos/video-read-only", strings.NewReader(`{"title":"changed"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "video-read-only")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleUpdateVideo(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	got, err := cat.GetVideo(ctx, "video-read-only")
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if got.Title != "file" || got.Author != "author" {
		t.Fatalf("metadata changed: title=%q author=%q", got.Title, got.Author)
	}
}

func TestHandleLoginReturnsForbiddenForBannedIP(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	createSessionUser(t, cat)
	if err := cat.BanLoginIP(ctx, "203.0.113.20", "test"); err != nil {
		t.Fatalf("ban ip: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.RemoteAddr = "203.0.113.20:12345"
	rr := httptest.NewRecorder()

	(&AdminServer{
		Catalog: cat,
		Auth:    &auth.Authenticator{Catalog: cat},
	}).handleLogin(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rr.Code, rr.Body.String())
	}
}

func TestAuthenticationEndpointsReturnServiceUnavailableWhenCatalogFails(t *testing.T) {
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	if err := cat.Close(); err != nil {
		t.Fatalf("close catalog: %v", err)
	}
	server := &AdminServer{
		Catalog: cat,
		Auth:    &auth.Authenticator{Catalog: cat},
	}

	tests := []struct {
		name   string
		handle http.HandlerFunc
		req    *http.Request
	}{
		{
			name:   "login",
			handle: server.handleLogin,
			req:    httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin","password":"secret"}`)),
		},
		{
			name:   "me",
			handle: server.handleMe,
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/admin/api/me", nil)
				req.AddCookie(&http.Cookie{Name: "vs_admin", Value: "valid-looking-token"})
				return req
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tt.handle(rr, tt.req)

			if rr.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503; body = %s", rr.Code, rr.Body.String())
			}
			if strings.Contains(rr.Body.String(), "database") || strings.Contains(rr.Body.String(), "interrupted") {
				t.Fatalf("response exposed the catalog error: %q", rr.Body.String())
			}
		})
	}
}

func TestHandleLoginRequiresSetupBeforeDefaultLogin(t *testing.T) {
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin","password":"admin123"}`))
	rr := httptest.NewRecorder()
	(&AdminServer{
		Catalog: cat,
		Auth:    &auth.Authenticator{Catalog: cat},
	}).handleLogin(rr, req)

	if rr.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want 428; body = %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSetupStoresCredentialsAndCreatesSession(t *testing.T) {
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	authr := &auth.Authenticator{Catalog: cat}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/setup", strings.NewReader(`{"username":"owner","password":"secret123"}`))
	rr := httptest.NewRecorder()

	(&AdminServer{
		Catalog: cat,
		Auth:    authr,
	}).handleSetup(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	user, err := cat.GetUserByUsername(context.Background(), "owner")
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != "admin" || user.Password == "secret123" {
		t.Fatal("administrator was not stored with a password hash")
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("setup did not set a session cookie")
	}
	ok, _, err := cat.ValidateSession(context.Background(), cookies[0].Value)
	if err != nil || !ok {
		t.Fatalf("setup session valid=%v err=%v", ok, err)
	}
}

func createSessionUser(t *testing.T, cat *catalog.Catalog) int64 {
	t.Helper()
	id, err := cat.CreateUser(context.Background(), "session-admin", "unused-password-hash", "admin")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestHandleBanUserDeletesSessions(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	userID, err := cat.CreateUser(ctx, "viewer", hash, "user")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := cat.CreateSession(ctx, "viewer-token", time.Hour, userID); err != nil {
		t.Fatalf("create session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/users/1/ban", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(userID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleBanUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	ok, _, err := cat.ValidateSession(ctx, "viewer-token")
	if err != nil {
		t.Fatalf("validate session: %v", err)
	}
	if ok {
		t.Fatal("banned user session is still valid")
	}
}

func TestHandleBanUserRejectsLastActiveAdmin(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	hash, err := auth.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	adminID, err := cat.CreateUser(ctx, "owner", hash, "admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/users/1/ban", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.FormatInt(adminID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleBanUser(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteVideoDefaultsDeleteSourceFalse(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/videos/video-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "video-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	called := false
	(&AdminServer{
		OnDeleteVideo: func(ctx context.Context, videoID string, deleteSource bool) (DeleteVideoResult, error) {
			called = true
			if videoID != "video-1" {
				t.Fatalf("videoID = %q, want video-1", videoID)
			}
			if deleteSource {
				t.Fatal("deleteSource defaulted to true")
			}
			return DeleteVideoResult{OK: true}, nil
		},
	}).handleDeleteVideo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("OnDeleteVideo was not called")
	}
}

func TestHandleDeleteVideoPassesDeleteSourceOption(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/admin/api/videos/video-1", strings.NewReader(`{"deleteSource":true}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "video-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{
		OnDeleteVideo: func(ctx context.Context, videoID string, deleteSource bool) (DeleteVideoResult, error) {
			if !deleteSource {
				t.Fatal("deleteSource = false, want true")
			}
			return DeleteVideoResult{OK: true, DeletedSource: true}, nil
		},
	}).handleDeleteVideo(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	var got DeleteVideoResult
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.DeletedSource {
		t.Fatalf("DeletedSource = false, want true; response = %s", rr.Body.String())
	}
}

func TestHandleRemoveBlacklistRejectsNonRestorableVideo(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	// 被去重删掉的视频不可恢复，应当去看保留下来的那一条。
	now := time.Now()
	for _, id := range []string{"canonical-video", "duplicate-video"} {
		if err := cat.UpsertVideo(ctx, &catalog.Video{
			ID: id, DriveID: "remote", FileID: id + ".mp4",
			Title: id, PublishedAt: now, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	if err := cat.DeleteVideoWithTombstoneOptions(ctx, "duplicate-video", catalog.DeleteVideoTombstoneOptions{
		Reason:           catalog.DeletedVideoReasonDuplicate,
		CanonicalVideoID: "canonical-video",
	}); err != nil {
		t.Fatalf("tombstone video: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/blacklist/duplicate-video", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "duplicate-video")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleRemoveBlacklist(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rr.Code, rr.Body.String())
	}
	if deleted, err := cat.IsVideoDeleted(ctx, "duplicate-video"); err != nil || !deleted {
		t.Fatalf("non-restorable tombstone was removed: deleted=%v err=%v", deleted, err)
	}
}

// 本地上传的视频没有任何重新发现的途径，取消拉黑必须当场把记录放回媒体库。
func TestHandleRemoveBlacklistRestoresLocalUploadImmediately(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID: "local-upload-video", DriveID: "local-upload", FileID: "upload.mp4",
		FileName: "upload.mp4", Title: "Upload", PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := cat.DeleteVideoWithTombstone(ctx, "local-upload-video"); err != nil {
		t.Fatalf("tombstone video: %v", err)
	}
	beforeRestore, err := cat.CountDriveAssetStats(ctx)
	if err != nil {
		t.Fatalf("count asset stats before restore: %v", err)
	}
	if got := beforeRestore.Teasers["local-upload"].Pending; got != 0 {
		t.Fatalf("pre-restore pending teasers = %d, want 0", got)
	}

	verified := ""
	server := &AdminServer{
		Catalog: cat,
		OnRemoveBlacklist: func(ctx context.Context, id string) error {
			_, err := cat.RestoreDeletedVideo(ctx, id, func(driveID, fileID string) (catalog.DeletedVideoSourceInfo, error) {
				verified = driveID + "/" + fileID
				return catalog.DeletedVideoSourceInfo{Size: 1024, ModTime: now}, nil
			})
			return err
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/blacklist/local-upload-video", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "local-upload-video")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	server.handleRemoveBlacklist(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	if verified != "local-upload/upload.mp4" {
		t.Fatalf("verified source = %q, want local-upload/upload.mp4", verified)
	}
	if _, err := cat.GetVideo(ctx, "local-upload-video"); err != nil {
		t.Fatalf("video was not restored: %v", err)
	}
	if deleted, err := cat.IsVideoDeleted(ctx, "local-upload-video"); err != nil || deleted {
		t.Fatalf("tombstone still present: deleted=%v err=%v", deleted, err)
	}
	afterRestore, err := cat.CountDriveAssetStats(ctx)
	if err != nil {
		t.Fatalf("count asset stats after restore: %v", err)
	}
	if got := afterRestore.Teasers["local-upload"].Pending; got != 1 {
		t.Fatalf("post-restore pending teasers = %d, want 1", got)
	}
}

// 源文件已经不在时必须拒绝恢复，否则库里会多出一条点开就播放失败的视频。
func TestHandleRemoveBlacklistRejectsDirectRestoreWhenSourceMissing(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID: "local-upload-video", DriveID: "local-upload", FileID: "upload.mp4",
		FileName: "upload.mp4", Title: "Upload", PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := cat.DeleteVideoWithTombstone(ctx, "local-upload-video"); err != nil {
		t.Fatalf("tombstone video: %v", err)
	}

	server := &AdminServer{
		Catalog: cat,
		OnRemoveBlacklist: func(ctx context.Context, id string) error {
			_, err := cat.RestoreDeletedVideo(ctx, id, func(string, string) (catalog.DeletedVideoSourceInfo, error) {
				return catalog.DeletedVideoSourceInfo{}, errors.New("stat upload.mp4: no such file")
			})
			return err
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/blacklist/local-upload-video", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "local-upload-video")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	server.handleRemoveBlacklist(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rr.Code, rr.Body.String())
	}
	if deleted, err := cat.IsVideoDeleted(ctx, "local-upload-video"); err != nil || !deleted {
		t.Fatalf("tombstone was removed despite missing source: deleted=%v err=%v", deleted, err)
	}
	if _, err := cat.GetVideo(ctx, "local-upload-video"); err == nil {
		t.Fatalf("video must not be restored when the source file is gone")
	}
}

// 可扫描来源不该触发源文件校验：它们本来就要等下一轮扫盘，此刻文件在不在
// 都不影响「删掉墓碑」这个动作。
func TestHandleRemoveBlacklistSkipsSourceCheckForScannableDrive(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID: "remote-video", DriveID: "remote", FileID: "remote.mp4",
		FileName: "remote.mp4", Title: "Remote", PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := cat.DeleteVideoWithTombstone(ctx, "remote-video"); err != nil {
		t.Fatalf("tombstone video: %v", err)
	}

	server := &AdminServer{
		Catalog: cat,
		OnRemoveBlacklist: func(ctx context.Context, id string) error {
			_, err := cat.RestoreDeletedVideo(ctx, id, func(string, string) (catalog.DeletedVideoSourceInfo, error) {
				t.Fatalf("scannable drive must not be source-checked")
				return catalog.DeletedVideoSourceInfo{}, nil
			})
			return err
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/blacklist/remote-video", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "remote-video")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	server.handleRemoveBlacklist(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	// 扫盘来源只是移除墓碑，不会当场重建记录。
	if _, err := cat.GetVideo(ctx, "remote-video"); err == nil {
		t.Fatalf("scannable restore must wait for the next scan")
	}
	if deleted, err := cat.IsVideoDeleted(ctx, "remote-video"); err != nil || deleted {
		t.Fatalf("tombstone not removed: deleted=%v err=%v", deleted, err)
	}
}

func TestHandleStartBlacklistSourceDeleteReturnsBackgroundStatus(t *testing.T) {
	started := false
	server := &AdminServer{
		OnStartBlacklistSourceDelete: func(req BlacklistSourceDeleteRequest) bool {
			if !req.DeleteAllSources || len(req.IDs) != 0 {
				t.Fatalf("request = %#v, want all sources", req)
			}
			started = true
			return true
		},
		GetBlacklistSourceDeleteStatus: func() BlacklistSourceDeleteStatus {
			return BlacklistSourceDeleteStatus{
				State: "running", Running: true, Total: 12, Processed: 3, Deleted: 2, Failed: 1,
			}
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/blacklist/source-delete", strings.NewReader(`{"deleteAllSources":true}`))
	rr := httptest.NewRecorder()

	server.handleStartBlacklistSourceDelete(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rr.Code, rr.Body.String())
	}
	if !started {
		t.Fatal("source delete hook was not called")
	}
	var got struct {
		Accepted bool                        `json:"accepted"`
		Status   BlacklistSourceDeleteStatus `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.Accepted ||
		!got.Status.Running ||
		got.Status.Total != 12 ||
		got.Status.Processed != 3 ||
		got.Status.Deleted != 2 ||
		got.Status.Failed != 1 {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleStartBlacklistSourceDeleteRequiresExplicitConfirmation(t *testing.T) {
	called := false
	server := &AdminServer{
		OnStartBlacklistSourceDelete: func(_ BlacklistSourceDeleteRequest) bool {
			called = true
			return true
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/blacklist/source-delete", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	server.handleStartBlacklistSourceDelete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
	if called {
		t.Fatal("source delete hook ran without explicit confirmation")
	}
}

func TestHandleStartBlacklistSourceDeleteAcceptsExplicitIDs(t *testing.T) {
	var got BlacklistSourceDeleteRequest
	server := &AdminServer{
		OnStartBlacklistSourceDelete: func(req BlacklistSourceDeleteRequest) bool {
			got = req
			return true
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/blacklist/source-delete", strings.NewReader(`{"ids":[" a ","","b","a"]}`))
	rr := httptest.NewRecorder()

	server.handleStartBlacklistSourceDelete(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rr.Code, rr.Body.String())
	}
	if got.DeleteAllSources || len(got.IDs) != 2 || got.IDs[0] != "a" || got.IDs[1] != "b" {
		t.Fatalf("request = %#v", got)
	}
}

func TestHandleCheckUpdateReportsNewRelease(t *testing.T) {
	dir := t.TempDir()
	versionFile := filepath.Join(dir, ".version")
	if err := os.WriteFile(versionFile, []byte("v0.1.0\n2026-05-29 12:00:00\n"), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}
	releaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "missing user agent", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tag_name": "v0.2.0",
			"html_url": "https://github.com/z08w12/omni-media-vault/releases/tag/v0.2.0",
			"body":     "## Changes\n\n- Added update notes dialog",
		})
	}))
	t.Cleanup(releaseServer.Close)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/update/check", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{
		VersionFilePath: versionFile,
		ReleaseAPIURL:   releaseServer.URL,
	}).handleCheckUpdate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got updateCheckDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CurrentVersion != "v0.1.0" {
		t.Fatalf("currentVersion = %q, want v0.1.0", got.CurrentVersion)
	}
	if got.LatestVersion != "v0.2.0" {
		t.Fatalf("latestVersion = %q, want v0.2.0", got.LatestVersion)
	}
	if !got.HasUpdate {
		t.Fatalf("hasUpdate = false, want true")
	}
	if got.ReleaseURL == "" {
		t.Fatalf("releaseUrl is empty")
	}
	if got.ReleaseNotes != "## Changes\n\n- Added update notes dialog" {
		t.Fatalf("releaseNotes = %q", got.ReleaseNotes)
	}
}

func TestHandleCheckUpdateReportsUpToDate(t *testing.T) {
	dir := t.TempDir()
	versionFile := filepath.Join(dir, ".version")
	if err := os.WriteFile(versionFile, []byte("v0.2.0\n"), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}
	releaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"tag_name": "v0.2.0",
			"html_url": "https://github.com/z08w12/omni-media-vault/releases/tag/v0.2.0",
		})
	}))
	t.Cleanup(releaseServer.Close)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/update/check", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{
		VersionFilePath: versionFile,
		ReleaseAPIURL:   releaseServer.URL,
	}).handleCheckUpdate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got updateCheckDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.HasUpdate {
		t.Fatalf("hasUpdate = true, want false")
	}
}

func TestHandleCheckUpdateUsesDockerImageVersion(t *testing.T) {
	releaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"tag_name": "v0.2.0",
			"html_url": "https://github.com/z08w12/omni-media-vault/releases/tag/v0.2.0",
		})
	}))
	t.Cleanup(releaseServer.Close)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/update/check", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{
		ImageVersion:  "v0.1.0",
		ReleaseAPIURL: releaseServer.URL,
	}).handleCheckUpdate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got updateCheckDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CurrentVersion != "v0.1.0" {
		t.Fatalf("currentVersion = %q, want v0.1.0", got.CurrentVersion)
	}
	if !got.HasUpdate {
		t.Fatalf("hasUpdate = false, want true")
	}
}

func TestInstalledVersionPrefersDockerImageVersionOverVersionFile(t *testing.T) {
	dir := t.TempDir()
	versionFile := filepath.Join(dir, ".version")
	if err := os.WriteFile(versionFile, []byte("v0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}

	got := (&AdminServer{
		VersionFilePath: versionFile,
		ImageVersion:    "v0.2.0",
	}).installedVersion()

	if got != "v0.2.0" {
		t.Fatalf("installedVersion = %q, want v0.2.0", got)
	}
}

func TestHandleRunScanAllJobReturnsAcceptedStatus(t *testing.T) {
	called := false
	req := httptest.NewRequest(http.MethodPost, "/admin/api/jobs/scan-all/run", nil)
	rr := httptest.NewRecorder()

	(&AdminServer{
		OnRunScanAllJob: func() bool {
			called = true
			return true
		},
		GetNightlyJobStatus: func() NightlyJobStatus {
			return NightlyJobStatus{State: "queued", Queued: true}
		},
	}).handleRunScanAllJob(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("OnRunScanAllJob was not called")
	}
	var got struct {
		OK       bool             `json:"ok"`
		Accepted bool             `json:"accepted"`
		Status   NightlyJobStatus `json:"status"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.OK || !got.Accepted || got.Status.State != "queued" || !got.Status.Queued {
		t.Fatalf("response = %#v, want accepted queued status", got)
	}
}

func TestHandleRunScanAllJobReturnsBusyMessageWhenRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/admin/api/jobs/scan-all/run", nil)
	rr := httptest.NewRecorder()

	(&AdminServer{
		OnRunScanAllJob: func() bool {
			return false
		},
		GetNightlyJobStatus: func() NightlyJobStatus {
			return NightlyJobStatus{State: "running", Running: true}
		},
	}).handleRunScanAllJob(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		OK       bool             `json:"ok"`
		Accepted bool             `json:"accepted"`
		Message  string           `json:"message"`
		Status   NightlyJobStatus `json:"status"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.OK || got.Accepted || got.Message != fullScanBusyMessage || !got.Status.Running {
		t.Fatalf("response = %#v, want rejected busy message", got)
	}
}

func TestHandleScanAllJobStatusDefaultsToIdle(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/api/jobs/scan-all/status", nil)
	rr := httptest.NewRecorder()

	(&AdminServer{}).handleScanAllJobStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	var got NightlyJobStatus
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.State != "idle" || got.Running || got.Queued {
		t.Fatalf("status = %#v, want idle", got)
	}
}

func TestHandleStopDriveTasksInvokesHookWithDriveID(t *testing.T) {
	calledWith := ""
	server := &AdminServer{
		OnStopDriveTasks: func(driveID string) bool {
			calledWith = driveID
			return true
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/PikPak/tasks/stop", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "PikPak")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	server.handleStopDriveTasks(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if calledWith != "PikPak" {
		t.Fatalf("hook called with %q, want PikPak", calledWith)
	}
	var got struct {
		OK      bool `json:"ok"`
		Stopped bool `json:"stopped"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.OK || !got.Stopped {
		t.Fatalf("response = %#v, want stopped", got)
	}
}

func TestHandleStopAllTasksInvokesHookAndReturnsStatus(t *testing.T) {
	called := false
	server := &AdminServer{
		OnStopAllTasks: func() int {
			called = true
			return 2
		},
		GetNightlyJobStatus: func() NightlyJobStatus {
			return NightlyJobStatus{State: "running", Running: true}
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/tasks/stop", nil)
	rr := httptest.NewRecorder()
	server.handleStopAllTasks(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("OnStopAllTasks was not called")
	}
	var got struct {
		OK            bool             `json:"ok"`
		StoppedDrives int              `json:"stoppedDrives"`
		Status        NightlyJobStatus `json:"status"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.OK || got.StoppedDrives != 2 || got.Status.State != "running" || !got.Status.Running {
		t.Fatalf("response = %#v, want stopped drives and status", got)
	}
}

func TestHandleListCrawlersOnlyIncludesCrawlerPageScripts(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	cat, err := catalog.Open(filepath.Join(tmp, "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	scriptPath := filepath.Join(tmp, "demo_crawler.py")
	if err := os.WriteFile(scriptPath, []byte("CRAWLER_NAME = \"Demo Crawler\"\nCRAWLER_PROTOCOL = \"crawler.v2\"\n"), 0o644); err != nil {
		t.Fatalf("write crawler script: %v", err)
	}

	for _, d := range []*catalog.Drive{
		{
			ID:     "crawler-main",
			Kind:   "scriptcrawler",
			Name:   "Crawler",
			RootID: "/",
			Credentials: map[string]string{
				"last_crawl_at":   "1800000000",
				"proxy":           " http://127.0.0.1:7890 ",
				"upload_proxy":    " socks5h://upload-proxy.example:1080 ",
				"script_path":     scriptPath,
				"upload_drive_id": "p115-target",
				"paused":          "true",
			},
			Status: "ok",
		},
		{
			ID:          "p115-target",
			Kind:        "p115",
			Name:        "115",
			RootID:      "0",
			Credentials: map[string]string{"cookie": "x"},
			Status:      "ok",
		},
		{
			ID:     "onedrive-main",
			Kind:   "onedrive",
			Name:   "OneDrive",
			RootID: "root",
			Credentials: map[string]string{
				"proxy": "http://should-not-leak.local:7890",
			},
			Status: "ok",
		},
		{
			ID:          "crawler-script-deleted",
			Kind:        "scriptcrawler",
			Name:        "Deleted Script",
			RootID:      "/",
			Credentials: map[string]string{},
			Status:      "disconnected",
		},
	} {
		if err := cat.UpsertDrive(ctx, d); err != nil {
			t.Fatalf("seed drive %s: %v", d.ID, err)
		}
	}
	for _, v := range []*catalog.Video{
		{
			ID:              "scriptcrawler-crawler-main-local",
			DriveID:         "crawler-main",
			FileID:          "local.mp4",
			FileName:        "local.mp4",
			Title:           "Local",
			Size:            123,
			Ext:             "mp4",
			ThumbnailURL:    "/p/thumb/scriptcrawler-crawler-main-local",
			PreviewStatus:   "ready",
			DurationSeconds: 12,
			PublishedAt:     time.Now(),
		},
		{
			ID:              "scriptcrawler-crawler-main-migrated",
			DriveID:         "p115-target",
			FileID:          "uploaded-id",
			FileName:        "migrated.mp4",
			Title:           "Migrated",
			Size:            456,
			Ext:             "mp4",
			ThumbnailURL:    "/p/thumb/scriptcrawler-crawler-main-migrated",
			PreviewStatus:   "ready",
			DurationSeconds: 34,
			PublishedAt:     time.Now(),
		},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed crawler video %s: %v", v.ID, err)
		}
		if err := cat.UpdateVideoFingerprint(ctx, v.ID, "sha-"+v.ID, "ready", ""); err != nil {
			t.Fatalf("seed crawler fingerprint %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/crawlers", nil)
	rr := httptest.NewRecorder()
	srv := &AdminServer{Catalog: cat}
	srv.handleListCrawlers(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got []struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Protocol         string `json:"protocol"`
		Kind             string `json:"kind"`
		Proxy            string `json:"proxy"`
		UploadProxy      string `json:"uploadProxy"`
		UploadDriveID    string `json:"uploadDriveId"`
		Paused           bool   `json:"paused"`
		LastCrawlAt      int64  `json:"lastCrawlAt"`
		TotalCrawled     int    `json:"totalCrawledCount"`
		LocalVideos      int    `json:"localVideoCount"`
		MigratedVideo    int    `json:"migratedVideoCount"`
		ThumbnailReady   int    `json:"thumbnailReadyCount"`
		TeaserReady      int    `json:"teaserReadyCount"`
		FingerprintReady int    `json:"fingerprintReadyCount"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	type crawlerListRow struct {
		Name             string
		Protocol         string
		Kind             string
		Proxy            string
		Paused           bool
		LastCrawlAt      int64
		TotalCrawled     int
		LocalVideos      int
		MigratedVideo    int
		ThumbnailReady   int
		TeaserReady      int
		FingerprintReady int
	}
	byID := map[string]crawlerListRow{}
	for _, d := range got {
		byID[d.ID] = crawlerListRow{
			Name:             d.Name,
			Protocol:         d.Protocol,
			Kind:             d.Kind,
			Proxy:            d.Proxy,
			Paused:           d.Paused,
			LastCrawlAt:      d.LastCrawlAt,
			TotalCrawled:     d.TotalCrawled,
			LocalVideos:      d.LocalVideos,
			MigratedVideo:    d.MigratedVideo,
			ThumbnailReady:   d.ThumbnailReady,
			TeaserReady:      d.TeaserReady,
			FingerprintReady: d.FingerprintReady,
		}
	}
	if _, ok := byID["crawler-script-deleted"]; ok {
		t.Fatal("crawler without script_path should not be returned by crawler list")
	}
	if byID["crawler-main"].Kind != "scriptcrawler" {
		t.Fatalf("crawler kind = %q, want scriptcrawler", byID["crawler-main"].Kind)
	}
	if byID["crawler-main"].Name != "Demo Crawler" {
		t.Fatalf("crawler name = %q, want script metadata name", byID["crawler-main"].Name)
	}
	if byID["crawler-main"].Protocol != scriptcrawler.ProtocolV2 {
		t.Fatalf("crawler protocol = %q, want %q", byID["crawler-main"].Protocol, scriptcrawler.ProtocolV2)
	}
	if byID["crawler-main"].Proxy != "http://127.0.0.1:7890" {
		t.Fatalf("crawler proxy = %q, want trimmed proxy", byID["crawler-main"].Proxy)
	}
	if !byID["crawler-main"].Paused {
		t.Fatal("paused = false, want true from crawler drive")
	}
	if byID["crawler-main"].LastCrawlAt != 1800000000 {
		t.Fatalf("lastCrawlAt = %d, want 1800000000", byID["crawler-main"].LastCrawlAt)
	}
	if byID["crawler-main"].TotalCrawled != 2 || byID["crawler-main"].LocalVideos != 1 || byID["crawler-main"].MigratedVideo != 1 {
		t.Fatalf("crawler counts = total %d local %d migrated %d, want 2/1/1", byID["crawler-main"].TotalCrawled, byID["crawler-main"].LocalVideos, byID["crawler-main"].MigratedVideo)
	}
	if byID["crawler-main"].ThumbnailReady != 2 || byID["crawler-main"].TeaserReady != 2 || byID["crawler-main"].FingerprintReady != 2 {
		t.Fatalf("asset ready counts = thumb %d teaser %d fingerprint %d, want 2/2/2", byID["crawler-main"].ThumbnailReady, byID["crawler-main"].TeaserReady, byID["crawler-main"].FingerprintReady)
	}
	if _, ok := byID["onedrive-main"]; ok {
		t.Fatal("onedrive should not be returned by crawler list")
	}

}

func TestHandleUpsertCrawlerRequiresScriptPath(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	cat, err := catalog.Open(filepath.Join(tmp, "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	srv := &AdminServer{Catalog: cat}
	scriptPath := filepath.Join(tmp, "custom.py")
	if err := os.WriteFile(scriptPath, []byte("CRAWLER_NAME = \"Demo Crawler\"\n"), 0o644); err != nil {
		t.Fatalf("write crawler script: %v", err)
	}

	// 不再内置任何爬虫：没有脚本路径的保存请求必须被拒绝，
	// 旧的 builtin 字段也不再有"免脚本"特权。
	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers", strings.NewReader(`{
		"id": "crawler-main",
		"builtin": "legacy",
		"scriptPath": "",
		"targetNew": "15"
	}`))
	rr := httptest.NewRecorder()
	srv.handleUpsertCrawler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s, want 400", rr.Code, rr.Body.String())
	}

	// 带脚本路径时正常保存，且请求中的 builtin 字段被忽略，不会写入凭证。
	req = httptest.NewRequest(http.MethodPost, "/admin/api/crawlers", strings.NewReader(`{
		"id": "crawler-main",
		"builtin": "legacy",
		"scriptPath": "`+scriptPath+`",
		"targetNew": "15"
	}`))
	rr = httptest.NewRecorder()
	srv.handleUpsertCrawler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	got, err := cat.GetDrive(ctx, "crawler-main")
	if err != nil {
		t.Fatalf("get crawler drive: %v", err)
	}
	if got.Kind != "scriptcrawler" || got.Credentials["builtin"] != "" {
		t.Fatalf("kind/builtin = %q/%q, want scriptcrawler with no builtin credential", got.Kind, got.Credentials["builtin"])
	}
	if got.Credentials["python_path"] != "" || got.Credentials["config_json"] != "" {
		t.Fatalf("legacy hidden credentials should not be saved: %+v", got.Credentials)
	}
	if got.Name != "Demo Crawler" {
		t.Fatalf("name = %q, want script metadata name", got.Name)
	}
	if got.Credentials["script_path"] != scriptPath {
		t.Fatalf("script_path = %q, want %q", got.Credentials["script_path"], scriptPath)
	}
}

func TestHandleUpsertCrawlerGeneratesIDFromScriptName(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	cat, err := catalog.Open(filepath.Join(tmp, "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	if err := cat.UpsertDrive(ctx, &catalog.Drive{
		ID:          "crawler-my-spider",
		Kind:        scriptcrawler.Kind,
		Name:        "Existing",
		RootID:      "/",
		Credentials: map[string]string{"script_path": "/opt/crawlers/existing.py"},
	}); err != nil {
		t.Fatalf("seed crawler: %v", err)
	}
	scriptPath := filepath.Join(tmp, "custom.py")
	if err := os.WriteFile(scriptPath, []byte("CRAWLER_NAME = \"My Spider\"\n"), 0o644); err != nil {
		t.Fatalf("write crawler script: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers", strings.NewReader(`{
		"scriptPath": "`+scriptPath+`",
		"targetNew": "15"
	}`))
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleUpsertCrawler(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		OK bool   `json:"ok"`
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK || resp.ID != "crawler-my-spider-2" {
		t.Fatalf("response = %+v, want generated suffix id", resp)
	}
	got, err := cat.GetDrive(ctx, resp.ID)
	if err != nil {
		t.Fatalf("get generated crawler: %v", err)
	}
	if got.Name != "My Spider" || got.Kind != scriptcrawler.Kind {
		t.Fatalf("generated crawler = %+v", got)
	}
}

func TestHandleImportCrawlerScriptFile(t *testing.T) {
	tmp := t.TempDir()
	script := "CRAWLER_NAME = \"Demo Crawler\"\nCRAWLER_PROTOCOL = \"crawler.v2\"\nprint('crawler')\n"
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "../demo crawler.py")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(script)); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers/import-file", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	(&AdminServer{LocalPreviewDir: filepath.Join(tmp, "previews")}).handleImportCrawlerScriptFile(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		ScriptPath string `json:"scriptPath"`
		Name       string `json:"name"`
		Protocol   string `json:"protocol"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantRoot := filepath.Join(tmp, "crawler-scripts")
	if !strings.HasPrefix(got.ScriptPath, wantRoot+string(os.PathSeparator)) {
		t.Fatalf("script path = %q, want under %q", got.ScriptPath, wantRoot)
	}
	if filepath.Ext(got.ScriptPath) != ".py" {
		t.Fatalf("script path = %q, want .py", got.ScriptPath)
	}
	if filepath.Base(got.ScriptPath) != "demo_crawler.py" {
		t.Fatalf("script filename = %q, want original sanitized filename", filepath.Base(got.ScriptPath))
	}
	data, err := os.ReadFile(got.ScriptPath)
	if err != nil {
		t.Fatalf("read imported script: %v", err)
	}
	if got.Name != "Demo Crawler" {
		t.Fatalf("name = %q, want script metadata name", got.Name)
	}
	if got.Protocol != scriptcrawler.ProtocolV2 {
		t.Fatalf("protocol = %q, want %q", got.Protocol, scriptcrawler.ProtocolV2)
	}
	if string(data) != script {
		t.Fatalf("script content = %q", string(data))
	}
}

func TestHandleImportCrawlerScriptFileDoesNotCreateCrawlerTagWithoutVideos(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	cat, err := catalog.Open(filepath.Join(tmp, "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	script := "CRAWLER_NAME = \"Imported Crawler\"\nprint('crawler')\n"
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "imported.py")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(script)); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers/import-file", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	(&AdminServer{
		Catalog:         cat,
		LocalPreviewDir: filepath.Join(tmp, "previews"),
	}).handleImportCrawlerScriptFile(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	label, ok, err := cat.LookupTagLabel(ctx, "Imported Crawler")
	if err != nil {
		t.Fatalf("lookup crawler tag: %v", err)
	}
	if ok {
		t.Fatalf("lookup tag = %q/%v, want no crawler tag before any video exists", label, ok)
	}
}

func TestHandleImportCrawlerScriptFileRejectsMissingName(t *testing.T) {
	tmp := t.TempDir()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "crawler.py")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("print('crawler')\n")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers/import-file", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	(&AdminServer{LocalPreviewDir: filepath.Join(tmp, "previews")}).handleImportCrawlerScriptFile(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "CRAWLER_NAME") {
		t.Fatalf("body = %s, want CRAWLER_NAME error", rr.Body.String())
	}
}

func TestHandleImportCrawlerScriptFileRejectsNonPython(t *testing.T) {
	tmp := t.TempDir()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "crawler.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("print('crawler')\n")); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers/import-file", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	(&AdminServer{LocalPreviewDir: filepath.Join(tmp, "previews")}).handleImportCrawlerScriptFile(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ".py") {
		t.Fatalf("body = %s, want .py error", rr.Body.String())
	}
}

func TestHandleImportCrawlerScriptURL(t *testing.T) {
	tmp := t.TempDir()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crawler.py" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("CRAWLER_NAME = \"URL Crawler\"\n# crawler from url\n"))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers/import-url", strings.NewReader(`{
		"url": "`+upstream.URL+`/crawler.py"
	}`))
	rr := httptest.NewRecorder()
	(&AdminServer{LocalPreviewDir: filepath.Join(tmp, "previews")}).handleImportCrawlerScriptURL(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		ScriptPath string `json:"scriptPath"`
		Name       string `json:"name"`
		Protocol   string `json:"protocol"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantRoot := filepath.Join(tmp, "crawler-scripts")
	if !strings.HasPrefix(got.ScriptPath, wantRoot+string(os.PathSeparator)) {
		t.Fatalf("script path = %q, want under %q", got.ScriptPath, wantRoot)
	}
	data, err := os.ReadFile(got.ScriptPath)
	if err != nil {
		t.Fatalf("read imported script: %v", err)
	}
	if got.Name != "URL Crawler" {
		t.Fatalf("name = %q, want script metadata name", got.Name)
	}
	if got.Protocol != scriptcrawler.ProtocolV1 {
		t.Fatalf("protocol = %q, want %q", got.Protocol, scriptcrawler.ProtocolV1)
	}
	if filepath.Base(got.ScriptPath) != "crawler.py" {
		t.Fatalf("script filename = %q, want original filename", filepath.Base(got.ScriptPath))
	}
	if string(data) != "CRAWLER_NAME = \"URL Crawler\"\n# crawler from url\n" {
		t.Fatalf("script content = %q", string(data))
	}
}

func TestCrawlerScriptDownloadURLConvertsGitHubBlob(t *testing.T) {
	input, err := url.Parse("https://github.com/Just-Spider/SpiderFor91/blob/main/91Porn/91Porn.py")
	if err != nil {
		t.Fatalf("parse input: %v", err)
	}
	got := crawlerScriptDownloadURL(input)
	want := "https://raw.githubusercontent.com/Just-Spider/SpiderFor91/main/91Porn/91Porn.py"
	if got.String() != want {
		t.Fatalf("download URL = %q, want %q", got.String(), want)
	}
}

func TestCrawlerScriptDownloadURLKeepsNonGitHubURL(t *testing.T) {
	input, err := url.Parse("https://example.com/crawlers/demo.py")
	if err != nil {
		t.Fatalf("parse input: %v", err)
	}
	got := crawlerScriptDownloadURL(input)
	if got.String() != input.String() {
		t.Fatalf("download URL = %q, want original %q", got.String(), input.String())
	}
}

func TestHandleDeleteCrawlerRemovesScriptLocalVideosAndDrive(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	cat, err := catalog.Open(filepath.Join(tmp, "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	scriptDir := filepath.Join(tmp, "crawler-scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("mkdir script dir: %v", err)
	}
	scriptPath := filepath.Join(scriptDir, "crawler.py")
	if err := os.WriteFile(scriptPath, []byte("CRAWLER_NAME = \"Delete Me\"\n"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := cat.UpsertDrive(ctx, &catalog.Drive{
		ID:     "crawler-delete-me",
		Kind:   scriptcrawler.Kind,
		Name:   "Delete Me",
		RootID: "/",
		Credentials: map[string]string{
			"script_path": scriptPath,
			"proxy":       "http://127.0.0.1:7890",
			"target_new":  "10",
			"paused":      "true",
		},
	}); err != nil {
		t.Fatalf("seed crawler: %v", err)
	}
	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID:          "video-from-crawler",
		DriveID:     "crawler-delete-me",
		FileID:      "video.mp4",
		Title:       "Keep Me",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID:          "migrated-video-from-crawler",
		DriveID:     "pikpak",
		FileID:      "remote-file-id",
		Title:       "Keep Migrated Video",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("seed migrated video: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/api/crawlers/crawler-delete-me", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "crawler-delete-me")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	stopped := false
	removed := false
	(&AdminServer{
		Catalog:         cat,
		LocalPreviewDir: filepath.Join(tmp, "previews"),
		OnDriveDeleteCleanup: func(cleanupCtx context.Context, driveID string) (int, error) {
			if driveID != "crawler-delete-me" {
				t.Fatalf("cleanup drive = %q, want crawler-delete-me", driveID)
			}
			if err := cat.DeleteVideo(cleanupCtx, "video-from-crawler"); err != nil {
				t.Fatalf("delete local crawler video: %v", err)
			}
			return 1, nil
		},
		OnPrepareDriveDelete: func(_ context.Context, driveID string) error {
			stopped = driveID == "crawler-delete-me"
			return nil
		},
		OnDriveRemoved: func(driveID string) {
			removed = driveID == "crawler-delete-me"
		},
	}).handleDeleteCrawler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(scriptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("script stat error = %v, want not exist", err)
	}
	if !stopped {
		t.Fatal("stop hook was not called")
	}
	if !removed {
		t.Fatal("drive removed hook was not called")
	}
	if _, err := cat.GetDrive(ctx, "crawler-delete-me"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("crawler drive lookup error = %v, want sql.ErrNoRows", err)
	}
	if _, err := cat.GetVideo(ctx, "video-from-crawler"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("local crawler video lookup error = %v, want sql.ErrNoRows", err)
	}
	if _, err := cat.GetVideo(ctx, "migrated-video-from-crawler"); err != nil {
		t.Fatalf("migrated crawler video should remain: %v", err)
	}
	var got struct {
		OK            bool `json:"ok"`
		DeletedVideos int  `json:"deletedVideos"`
		DeletedScript bool `json:"deletedScript"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.OK || got.DeletedVideos != 1 || !got.DeletedScript {
		t.Fatalf("response = %#v", got)
	}
}

func TestHandleSetCrawlerPausedPersistsCredential(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	cat, err := catalog.Open(filepath.Join(tmp, "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	if err := cat.UpsertDrive(ctx, &catalog.Drive{
		ID:          "crawler-main",
		Kind:        scriptcrawler.Kind,
		Name:        "Crawler",
		RootID:      "/",
		Credentials: map[string]string{"script_path": "/tmp/crawler.py"},
	}); err != nil {
		t.Fatalf("seed crawler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers/crawler-main/paused", strings.NewReader(`{"paused":true}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "crawler-main")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleSetCrawlerPaused(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		OK     bool `json:"ok"`
		Paused bool `json:"paused"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK || !resp.Paused {
		t.Fatalf("response = %#v, want paused true", resp)
	}
	drive, err := cat.GetDrive(ctx, "crawler-main")
	if err != nil {
		t.Fatalf("get crawler: %v", err)
	}
	if drive.Credentials["paused"] != "true" {
		t.Fatalf("paused credential = %q, want true", drive.Credentials["paused"])
	}
}

func TestHandleImportCrawlerScriptURLRejectsNonPython(t *testing.T) {
	tmp := t.TempDir()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crawler.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("# crawler from url\n"))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers/import-url", strings.NewReader(`{
		"url": "`+upstream.URL+`/crawler.txt"
	}`))
	rr := httptest.NewRecorder()
	(&AdminServer{LocalPreviewDir: filepath.Join(tmp, "previews")}).handleImportCrawlerScriptURL(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), ".py") {
		t.Fatalf("body = %s, want .py error", rr.Body.String())
	}
}

func TestHandleTestCrawlerScriptRunsImportedScript(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required for crawler script dry-run")
	}
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/video.mp4" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		if r.Header.Get("Range") == "bytes=0-0" {
			w.Header().Set("Content-Range", "bytes 0-0/2048")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0})
			return
		}
		_, _ = w.Write([]byte("video"))
	}))
	defer media.Close()

	script := filepath.Join(t.TempDir(), "crawler.py")
	body := `CRAWLER_NAME = "Dry Run Test"
import json
print(json.dumps({"title": "Dry Run Video", "source_id": "dry-1", "media_url": "` + media.URL + `/video.mp4", "thumbnail_url": "` + media.URL + `/thumb.jpg", "detail_url": "` + media.URL + `/detail"}))
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	reqBody, err := json.Marshal(map[string]string{
		"scriptPath": script,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/api/crawlers/test-script", bytes.NewReader(reqBody))
	rr := httptest.NewRecorder()
	(&AdminServer{}).handleTestCrawlerScript(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got struct {
		OK    bool `json:"ok"`
		Items []struct {
			Title    string `json:"title"`
			SourceID string `json:"sourceId"`
			MediaURL string `json:"mediaUrl"`
		} `json:"items"`
		MediaCheck *struct {
			OK            bool   `json:"ok"`
			Status        int    `json:"status"`
			ContentType   string `json:"contentType"`
			ContentLength int64  `json:"contentLengthBytes"`
		} `json:"mediaCheck"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.OK {
		t.Fatalf("ok = false, body = %s", rr.Body.String())
	}
	if len(got.Items) != 1 || got.Items[0].Title != "Dry Run Video" || got.Items[0].SourceID != "dry-1" {
		t.Fatalf("items = %#v", got.Items)
	}
	if got.Items[0].MediaURL != media.URL+"/video.mp4" {
		t.Fatalf("mediaUrl = %q", got.Items[0].MediaURL)
	}
	if got.MediaCheck == nil || !got.MediaCheck.OK || got.MediaCheck.Status != http.StatusPartialContent {
		t.Fatalf("mediaCheck = %#v", got.MediaCheck)
	}
	if got.MediaCheck.ContentLength != 2048 {
		t.Fatalf("contentLength = %d, want 2048", got.MediaCheck.ContentLength)
	}
}

func TestHandleRegenFailedFingerprintsInvokesHook(t *testing.T) {
	called := ""
	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/drive-one/fingerprints/failed/regenerate", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "drive-one")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{
		OnRegenFailedFingerprints: func(driveID string) {
			called = driveID
		},
	}).handleRegenFailedFingerprints(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if called != "drive-one" {
		t.Fatalf("called drive = %q, want drive-one", called)
	}
}

func TestHandleDriveStorageReportsLocalMediaUsage(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cat, err := catalog.Open(filepath.Join(root, "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	localDir := filepath.Join(root, "previews")
	thumbDir := filepath.Join(localDir, "thumbs")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		t.Fatalf("mkdir thumbs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "drive-one-video.mp4"), []byte("teaser-one"), 0o644); err != nil {
		t.Fatalf("write teaser one: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "drive-two-video.mp4"), []byte("teaser-two!!"), 0o644); err != nil {
		t.Fatalf("write teaser two: %v", err)
	}
	if err := os.WriteFile(filepath.Join(thumbDir, "drive-one-video.jpg"), []byte("jpg-one"), 0o644); err != nil {
		t.Fatalf("write thumb one: %v", err)
	}
	if err := os.WriteFile(filepath.Join(thumbDir, "drive-two-video.jpg"), []byte("jpg-two!!"), 0o644); err != nil {
		t.Fatalf("write thumb two: %v", err)
	}

	for _, d := range []*catalog.Drive{
		{ID: "drive-one", Kind: "onedrive", Name: "Drive One", RootID: "root", Status: "ok"},
		{ID: "drive-two", Kind: "pikpak", Name: "Drive Two", RootID: "", Status: "ok"},
	} {
		if err := cat.UpsertDrive(ctx, d); err != nil {
			t.Fatalf("seed drive %s: %v", d.ID, err)
		}
	}
	now := time.Now()
	for _, v := range []*catalog.Video{
		{
			ID:            "drive-one-video",
			DriveID:       "drive-one",
			FileID:        "file-one",
			Title:         "Video One",
			PreviewLocal:  filepath.Join(localDir, "drive-one-video.mp4"),
			PreviewStatus: "ready",
			PublishedAt:   now,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            "drive-two-video",
			DriveID:       "drive-two",
			FileID:        "file-two",
			Title:         "Video Two",
			PreviewLocal:  filepath.Join(localDir, "drive-two-video.mp4"),
			PreviewStatus: "ready",
			PublishedAt:   now,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/drives/storage", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat, LocalPreviewDir: localDir}).handleDriveStorage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		ThumbnailBytes int64 `json:"thumbnailBytes"`
		TeaserBytes    int64 `json:"teaserBytes"`
		TotalBytes     int64 `json:"totalBytes"`
		AvailableBytes int64 `json:"availableBytes"`
		Drives         map[string]struct {
			ThumbnailBytes int64 `json:"thumbnailBytes"`
			TeaserBytes    int64 `json:"teaserBytes"`
			TotalBytes     int64 `json:"totalBytes"`
		} `json:"drives"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ThumbnailBytes != int64(len("jpg-one")+len("jpg-two!!")) {
		t.Fatalf("thumbnail bytes = %d, want %d", got.ThumbnailBytes, len("jpg-one")+len("jpg-two!!"))
	}
	if got.TeaserBytes != int64(len("teaser-one")+len("teaser-two!!")) {
		t.Fatalf("teaser bytes = %d, want %d", got.TeaserBytes, len("teaser-one")+len("teaser-two!!"))
	}
	if got.TotalBytes != got.ThumbnailBytes+got.TeaserBytes {
		t.Fatalf("total bytes = %d, want thumbnail + teaser", got.TotalBytes)
	}
	if got.AvailableBytes <= 0 {
		t.Fatalf("available bytes = %d, want positive", got.AvailableBytes)
	}
	if got.Drives["drive-one"].ThumbnailBytes != int64(len("jpg-one")) ||
		got.Drives["drive-one"].TeaserBytes != int64(len("teaser-one")) {
		t.Fatalf("drive-one usage = %#v", got.Drives["drive-one"])
	}
	if got.Drives["drive-two"].TotalBytes != int64(len("jpg-two!!")+len("teaser-two!!")) {
		t.Fatalf("drive-two total = %d, want %d", got.Drives["drive-two"].TotalBytes, len("jpg-two!!")+len("teaser-two!!"))
	}
}

func TestHandleCreateTagClassifiesExistingVideos(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID:          "video-1",
		DriveID:     "drive",
		FileID:      "file-1",
		Title:       "清纯短发",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/tags", strings.NewReader(`{"label":"清纯"}`))
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleCreateTag(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Label      string `json:"label"`
		Classified int    `json:"classified"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Label != "清纯" || got.Classified != 1 {
		t.Fatalf("response = %#v, want 清纯 classified 1", got)
	}

	video, err := cat.GetVideo(ctx, "video-1")
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if len(video.Tags) != 1 || video.Tags[0] != "清纯" {
		t.Fatalf("video tags = %#v, want 清纯", video.Tags)
	}
}

func TestHandleUpdateTagSavesMatchRulesAndClassifies(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })
	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID: "rule-video", DriveID: "drive", FileID: "file", Title: "custom matching phrase",
		PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if _, err := cat.CreateTagAndClassify(ctx, "展示标签", "user"); err != nil {
		t.Fatalf("create tag: %v", err)
	}
	tags, err := cat.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	var tagID int64
	for _, tag := range tags {
		if tag.Label == "展示标签" {
			tagID = tag.ID
		}
	}
	req := requestWithRouteParam(
		http.MethodPut,
		"/admin/api/tags/1",
		"id",
		strconv.FormatInt(tagID, 10),
		strings.NewReader(`{"matchRules":{"keywords":["custom matching phrase"]}}`),
	)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleUpdateTag(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Tag catalog.Tag `json:"tag"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Tag.MatchRules.Keywords) != 1 {
		t.Fatalf("response = %#v", got)
	}
	video, err := cat.GetVideo(ctx, "rule-video")
	if err != nil || len(video.Tags) != 1 || video.Tags[0] != "展示标签" {
		t.Fatalf("classified video = %#v, %v", video, err)
	}

	req = requestWithRouteParam(
		http.MethodPut,
		"/admin/api/tags/1",
		"id",
		strconv.FormatInt(tagID, 10),
		strings.NewReader(`{"matchRules":{"keywords":["other phrase"]}}`),
	)
	rr = httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleUpdateTag(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("second update status = %d, body = %s", rr.Code, rr.Body.String())
	}
	video, err = cat.GetVideo(ctx, "rule-video")
	if err != nil {
		t.Fatalf("get video after second update: %v", err)
	}
	if len(video.Tags) != 0 {
		t.Fatalf("tag rule deletion did not refresh video tags: %#v", video.Tags)
	}
}

func TestTagJobHandlersExposeStatus(t *testing.T) {
	server := &AdminServer{
		OnStartTagRetag: func() bool { return true },
		GetTagJobStatus: func() TagJobStatus {
			return TagJobStatus{State: "running", Running: true, Kind: "retag", Total: 10, Processed: 4}
		},
	}
	rr := httptest.NewRecorder()
	server.handleStartTagRetag(rr, httptest.NewRequest(http.MethodPost, "/admin/api/tags/retag", nil))
	if rr.Code != http.StatusAccepted {
		t.Fatalf("retag status = %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	server.handleTagJobStatus(rr, httptest.NewRequest(http.MethodGet, "/admin/api/tags/jobs/status", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"processed":4`) {
		t.Fatalf("job status = %d, %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDeleteTagRemovesTagFromVideos(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID:          "video-1",
		DriveID:     "drive",
		FileID:      "file-1",
		Title:       "清纯短发",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if _, err := cat.CreateTagAndClassify(ctx, "清纯", "user"); err != nil {
		t.Fatalf("create tag: %v", err)
	}
	tags, err := cat.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	var tagID int64
	for _, tag := range tags {
		if tag.Label == "清纯" {
			tagID = tag.ID
			break
		}
	}
	if tagID == 0 {
		t.Fatal("created tag not found")
	}

	req := requestWithRouteParam(http.MethodDelete, "/admin/api/tags/1", "id", strconv.FormatInt(tagID, 10), strings.NewReader(``))
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleDeleteTag(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	video, err := cat.GetVideo(ctx, "video-1")
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if len(video.Tags) != 0 {
		t.Fatalf("video tags = %#v, want none", video.Tags)
	}
}

func TestHandleDeleteTagRemovesTag(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })
	tag, err := cat.EnsureTag(ctx, "测试标签", "user")
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}
	req := requestWithRouteParam(
		http.MethodDelete,
		"/admin/api/tags/1",
		"id",
		strconv.FormatInt(tag.ID, 10),
		strings.NewReader(""),
	)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleDeleteTag(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestHandleAdminListVideosFiltersByDriveID(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	videos := []*catalog.Video{
		{
			ID:          "od-video",
			DriveID:     "OneDrive",
			FileID:      "od-file",
			Title:       "OneDrive video",
			PublishedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "pp-video",
			DriveID:     "PikPak",
			FileID:      "pp-file",
			Title:       "PikPak video",
			PublishedAt: now.Add(-time.Hour),
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	for _, v := range videos {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/videos?driveId=OneDrive", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []catalog.Video `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 {
		t.Fatalf("response total/items = %d/%d, want 1/1: %#v", got.Total, len(got.Items), got.Items)
	}
	if got.Items[0].DriveID != "OneDrive" || got.Items[0].ID != "od-video" {
		t.Fatalf("item = %#v, want OneDrive od-video", got.Items[0])
	}
}

func TestHandleAdminListVideosTreatsUploadsAsOneSource(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	for _, video := range []*catalog.Video{
		{
			ID:          "uploaded-from-file",
			DriveID:     "local-upload",
			FileID:      "from-file.mp4",
			Title:       "uploaded from file",
			PublishedAt: now,
			CreatedAt:   now,
		},
		{
			ID:          "uploaded-from-link",
			DriveID:     "local-upload",
			FileID:      "from-link.mp4",
			Title:       "uploaded from link",
			PublishedAt: now.Add(-time.Second),
			CreatedAt:   now.Add(-time.Second),
		},
		{
			ID:          "cloud-video",
			DriveID:     "cloud",
			FileID:      "cloud-file",
			Title:       "cloud video",
			PublishedAt: now.Add(-2 * time.Second),
			CreatedAt:   now.Add(-2 * time.Second),
		},
	} {
		if err := cat.UpsertVideo(ctx, video); err != nil {
			t.Fatalf("seed %s: %v", video.ID, err)
		}
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/admin/api/videos?driveId=local-upload",
		nil,
	)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []catalog.Video `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 2 || len(got.Items) != 2 {
		t.Fatalf("response total/items = %d/%d, want both uploads: %#v", got.Total, len(got.Items), got.Items)
	}
	for _, item := range got.Items {
		if item.DriveID != "local-upload" {
			t.Fatalf("unexpected non-upload item: %#v", item)
		}
	}
}

func TestHandleAdminListVideosAppliesAdvancedFilters(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	seed := func(id, driveID string, createdAt, publishedAt time.Time, durationSeconds int) {
		t.Helper()
		if err := cat.UpsertVideo(ctx, &catalog.Video{
			ID:              id,
			DriveID:         driveID,
			FileID:          "file-" + id,
			Title:           id,
			DurationSeconds: durationSeconds,
			PublishedAt:     publishedAt,
			CreatedAt:       createdAt,
			UpdatedAt:       createdAt,
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	seed(
		"scriptcrawler-crawler-a-source-1",
		"cloud-a",
		time.Date(2026, time.July, 10, 23, 30, 0, 0, time.Local),
		time.Date(2026, time.June, 1, 23, 30, 0, 0, time.Local),
		5*60,
	)
	seed(
		"scriptcrawler-crawler-a-source-2",
		"cloud-a",
		time.Date(2026, time.July, 11, 0, 30, 0, 0, time.Local),
		time.Date(2026, time.June, 2, 0, 30, 0, 0, time.Local),
		15*60,
	)
	seed(
		"scriptcrawler-crawler-b-source-3",
		"cloud-b",
		time.Date(2026, time.July, 10, 12, 0, 0, 0, time.Local),
		time.Date(2026, time.June, 1, 12, 0, 0, 0, time.Local),
		5*60,
	)
	for _, source := range []struct {
		crawlerID string
		sourceID  string
		videoID   string
	}{
		{"crawler-a", "source-1", "scriptcrawler-crawler-a-source-1"},
		{"crawler-a", "source-2", "scriptcrawler-crawler-a-source-2"},
		{"crawler-b", "source-3", "scriptcrawler-crawler-b-source-3"},
	} {
		if err := cat.MarkCrawlerSourceSeen(ctx, "scriptcrawler", source.crawlerID, source.sourceID, "imported", source.videoID, "", 0); err != nil {
			t.Fatalf("mark crawler source %s: %v", source.sourceID, err)
		}
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/admin/api/videos?driveId=cloud-a&crawlerId=crawler-a&createdFrom=2026-07-10&createdTo=2026-07-10&durationMinMinutes=4&durationMaxMinutes=6",
		nil,
	)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []catalog.Video `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != "scriptcrawler-crawler-a-source-1" {
		t.Fatalf("filtered response = total %d items %#v, want source-1", got.Total, got.Items)
	}
}

func TestHandleAdminListVideosRejectsInvalidAdvancedRanges(t *testing.T) {
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	tomorrow := time.Now().In(time.Local).AddDate(0, 0, 1).Format(adminVideoDateLayout)
	for _, target := range []string{
		"/admin/api/videos?createdFrom=2026/07/10",
		"/admin/api/videos?createdFrom=" + tomorrow,
		"/admin/api/videos?createdTo=" + tomorrow,
		"/admin/api/videos?durationMinMinutes=10&durationMaxMinutes=5",
		"/admin/api/videos?durationMinMinutes=1.5",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rr := httptest.NewRecorder()
		(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, body = %s", target, rr.Code, rr.Body.String())
		}
	}
}

func TestHandleAdminListVideosDoesNotExposeCategory(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID:          "video-1",
		DriveID:     "drive",
		FileID:      "file-1",
		Title:       "Video",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}
	if _, err := cat.EnsureTag(ctx, "source-tag", "user"); err != nil {
		t.Fatalf("ensure source tag: %v", err)
	}
	if _, err := cat.AddVideoTagAssignments(ctx, "video-1", []catalog.TagAssignment{{
		Label: "source-tag", Source: "crawler", Evidence: "脚本标签",
	}}); err != nil {
		t.Fatalf("assign source tag: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/videos", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(got.Items))
	}
	if _, ok := got.Items[0]["category"]; ok {
		t.Fatalf("admin video response exposed category: %#v", got.Items[0])
	}
	sources, ok := got.Items[0]["tagSources"].(map[string]any)
	if !ok || sources["source-tag"] != "crawler" {
		t.Fatalf("tag sources = %#v", got.Items[0]["tagSources"])
	}
	evidence, ok := got.Items[0]["tagEvidence"].(map[string]any)
	if !ok || evidence["source-tag"] != "脚本标签" {
		t.Fatalf("tag evidence = %#v", got.Items[0]["tagEvidence"])
	}
}

func TestHandleAdminListVideosPaginates(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	for i, title := range []string{"first", "second", "third"} {
		v := &catalog.Video{
			ID:          title,
			DriveID:     "OneDrive",
			FileID:      title + "-file",
			Title:       title,
			PublishedAt: now.Add(-time.Duration(i) * time.Hour),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/videos?driveId=OneDrive&page=2&size=2", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []catalog.Video `json:"items"`
		Total int             `json:"total"`
		Page  int             `json:"page"`
		Size  int             `json:"size"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 3 || got.Page != 2 || got.Size != 2 {
		t.Fatalf("pagination meta = total:%d page:%d size:%d, want 3/2/2", got.Total, got.Page, got.Size)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "third" {
		t.Fatalf("items = %#v, want only third", got.Items)
	}
}

func TestHandleAdminListVideosMarksActivePreviewGeneration(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	for _, v := range []*catalog.Video{
		{
			ID:            "active-video",
			DriveID:       "OneDrive",
			FileID:        "active-file",
			Title:         "Active video",
			PreviewStatus: "ready",
			PublishedAt:   now,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            "idle-video",
			DriveID:       "OneDrive",
			FileID:        "idle-file",
			Title:         "Idle video",
			PreviewStatus: "ready",
			PublishedAt:   now.Add(-time.Hour),
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/videos?driveId=OneDrive", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{
		Catalog: cat,
		GetPreviewGenerationVideoIDs: func() map[string]bool {
			return map[string]bool{"active-video": true}
		},
	}).handleAdminListVideos(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []catalog.Video `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 2 || len(got.Items) != 2 {
		t.Fatalf("response total/items = %d/%d, want 2/2", got.Total, len(got.Items))
	}
	statusByID := map[string]string{}
	for _, item := range got.Items {
		statusByID[item.ID] = item.PreviewStatus
	}
	if statusByID["active-video"] != "generating" {
		t.Fatalf("active status = %q, want generating", statusByID["active-video"])
	}
	if statusByID["idle-video"] != "ready" {
		t.Fatalf("idle status = %q, want ready", statusByID["idle-video"])
	}
}

func TestHandleRegenAllPreviewsInvokesHook(t *testing.T) {
	called := false
	server := &AdminServer{
		OnRegenAllPreviews: func() {
			called = true
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/videos/regen-preview", nil)
	rr := httptest.NewRecorder()
	server.handleRegenAllPreviews(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("regen all previews hook was not called")
	}
}

func TestHandleRegenFailedPreviewsInvokesHookWithDriveID(t *testing.T) {
	calledWith := ""
	server := &AdminServer{
		OnRegenFailedPreviews: func(driveID string) {
			calledWith = driveID
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/PikPak/previews/failed/regenerate", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "PikPak")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	server.handleRegenFailedPreviews(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if calledWith != "PikPak" {
		t.Fatalf("hook called with %q, want PikPak", calledWith)
	}
}

func TestHandleAdminListVideosFiltersByMediaType(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	for _, video := range []*catalog.Video{
		{
			ID:          "sample-video",
			DriveID:     "local-upload",
			FileID:      "sample.mp4",
			Ext:         "mp4",
			Title:       "Sample MP4 Video",
			PublishedAt: now,
			CreatedAt:   now,
		},
		{
			ID:          "sample-photo",
			DriveID:     "local-upload",
			FileID:      "sample.jpg",
			Ext:         "jpg",
			Title:       "Sample JPG Photo",
			PublishedAt: now.Add(-time.Second),
			CreatedAt:   now.Add(-time.Second),
		},
	} {
		if err := cat.UpsertVideo(ctx, video); err != nil {
			t.Fatalf("seed video %s: %v", video.ID, err)
		}
	}

	server := &AdminServer{Catalog: cat}

	fetchMedia := func(mediaType string) ([]catalog.Video, int) {
		url := "/admin/api/videos"
		if mediaType != "" {
			url += "?mediaType=" + mediaType
		}
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rr := httptest.NewRecorder()
		server.handleAdminListVideos(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
		}
		var got struct {
			Items []catalog.Video `json:"items"`
			Total int             `json:"total"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return got.Items, got.Total
	}

	// 1. mediaType=video -> only sample-video
	vItems, vTotal := fetchMedia("video")
	if vTotal != 1 || len(vItems) != 1 || vItems[0].ID != "sample-video" {
		t.Fatalf("video filter = total:%d items:%+v, want 1 sample-video", vTotal, vItems)
	}

	// 2. mediaType=image -> only sample-photo
	pItems, pTotal := fetchMedia("image")
	if pTotal != 1 || len(pItems) != 1 || pItems[0].ID != "sample-photo" {
		t.Fatalf("image filter = total:%d items:%+v, want 1 sample-photo", pTotal, pItems)
	}

	// 3. mediaType=all -> both
	allIt, allTot := fetchMedia("all")
	if allTot != 2 || len(allIt) != 2 {
		t.Fatalf("all filter = total:%d items:%+v, want 2 items", allTot, allIt)
	}

	// 4. mediaType="" -> both
	defIt, defTot := fetchMedia("")
	if defTot != 2 || len(defIt) != 2 {
		t.Fatalf("default filter = total:%d items:%+v, want 2 items", defTot, defIt)
	}
}

