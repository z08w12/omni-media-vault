package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/video-site/backend/internal/cookiemanager"
)

func TestAdminCookiesAPI(t *testing.T) {
	dataDir := t.TempDir()
	mgr, err := cookiemanager.New(cookiemanager.Config{
		DataDir: dataDir,
	})
	if err != nil {
		t.Fatalf("cookiemanager.New: %v", err)
	}

	admin := &AdminServer{
		Cookies: mgr,
	}

	r := chi.NewRouter()
	r.Get("/cookies", admin.handleListCookies)
	r.Get("/cookies/{platform}", admin.handleGetCookieContent)
	r.Put("/cookies/{platform}", admin.handleSaveCookieContent)
	r.Delete("/cookies/{platform}", admin.handleDeleteCookie)
	r.Post("/cookies/{platform}/restore", admin.handleRestoreCookie)

	// 1. List cookies
	req := httptest.NewRequest(http.MethodGet, "/cookies", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /cookies returned %d", rec.Code)
	}

	var list []cookiemanager.PlatformCookieStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(list) != 6 {
		t.Fatalf("expected 6 platforms, got %d", len(list))
	}

	// 2. Save valid cookie for youtube
	validContent := `# Netscape HTTP Cookie File
.youtube.com	TRUE	/	TRUE	1791881939	VISITOR_INFO1_LIVE	sample_val
`
	body, _ := json.Marshal(map[string]string{"content": validContent})
	req = httptest.NewRequest(http.MethodPut, "/cookies/youtube", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /cookies/youtube returned %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Get cookie content
	req = httptest.NewRequest(http.MethodGet, "/cookies/youtube", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /cookies/youtube returned %d", rec.Code)
	}
	var getResp struct {
		Platform string `json:"platform"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("unmarshal getResp: %v", err)
	}
	if getResp.Platform != "youtube" || len(getResp.Content) == 0 {
		t.Fatalf("unexpected getResp: %+v", getResp)
	}

	// 4. Delete cookie
	req = httptest.NewRequest(http.MethodDelete, "/cookies/youtube", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE /cookies/youtube returned %d", rec.Code)
	}

	// 5. Restore cookie
	req = httptest.NewRequest(http.MethodPost, "/cookies/youtube/restore", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /cookies/youtube/restore returned %d", rec.Code)
	}

	// Verify restored
	content, err := mgr.GetContent(context.Background(), "youtube")
	if err != nil || len(content) == 0 {
		t.Fatalf("expected restored content, got error: %v", err)
	}
}
