package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (a *AdminServer) requireCookieManager(w http.ResponseWriter, r *http.Request) bool {
	if a.Cookies == nil {
		writeErr(w, r, http.StatusServiceUnavailable, errors.New("cookie manager service is not available"))
		return false
	}
	return true
}

func (a *AdminServer) handleListCookies(w http.ResponseWriter, r *http.Request) {
	if !a.requireCookieManager(w, r) {
		return
	}
	list, err := a.Cookies.List(r.Context())
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, list)
}

func (a *AdminServer) handleGetCookieContent(w http.ResponseWriter, r *http.Request) {
	if !a.requireCookieManager(w, r) {
		return
	}
	platform := chi.URLParam(r, "platform")
	content, err := a.Cookies.GetContent(r.Context(), platform)
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]string{
		"platform": platform,
		"content":  content,
	})
}

type saveCookieReq struct {
	Content string `json:"content"`
}

func (a *AdminServer) handleSaveCookieContent(w http.ResponseWriter, r *http.Request) {
	if !a.requireCookieManager(w, r) {
		return
	}
	platform := chi.URLParam(r, "platform")

	var req saveCookieReq
	if err := json.NewDecoder(io.LimitReader(r.Body, 10<<20)).Decode(&req); err != nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("invalid request body"))
		return
	}

	if err := a.Cookies.SaveContent(r.Context(), platform, req.Content); err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"platform": platform,
		"status":   "saved",
	})
}

func (a *AdminServer) handleUploadCookieFile(w http.ResponseWriter, r *http.Request) {
	if !a.requireCookieManager(w, r) {
		return
	}
	platform := chi.URLParam(r, "platform")

	// Limit to 10MB
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("file is too large or malformed multipart request"))
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, r, http.StatusBadRequest, errors.New("file is required"))
		return
	}
	defer file.Close()

	contentBytes, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, err)
		return
	}

	if err := a.Cookies.SaveContent(r.Context(), platform, string(contentBytes)); err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"platform": platform,
		"status":   "uploaded",
	})
}

func (a *AdminServer) handleDeleteCookie(w http.ResponseWriter, r *http.Request) {
	if !a.requireCookieManager(w, r) {
		return
	}
	platform := chi.URLParam(r, "platform")

	if err := a.Cookies.Delete(r.Context(), platform); err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"platform": platform,
		"status":   "deleted",
	})
}

func (a *AdminServer) handleRestoreCookie(w http.ResponseWriter, r *http.Request) {
	if !a.requireCookieManager(w, r) {
		return
	}
	platform := chi.URLParam(r, "platform")

	if err := a.Cookies.Restore(r.Context(), platform); err != nil {
		writeErr(w, r, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"platform": platform,
		"status":   "restored",
	})
}
