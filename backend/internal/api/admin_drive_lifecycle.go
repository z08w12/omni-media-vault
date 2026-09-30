package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (a *AdminServer) beginDriveConfigUpdate(w http.ResponseWriter, driveID string) (DriveConfigUpdateLease, bool) {
	if a.BeginDriveConfigUpdate == nil {
		return nil, true
	}
	lease, reason := a.BeginDriveConfigUpdate(driveID)
	if lease != nil {
		return lease, true
	}
	if reason == "" {
		reason = "当前存储配置正在更新，请稍后重试"
	}
	http.Error(w, reason, http.StatusConflict)
	return nil, false
}

func authorizeDriveConfigUpdate(w http.ResponseWriter, lease DriveConfigUpdateLease, scope DriveConfigUpdateScope) bool {
	if lease == nil {
		return true
	}
	if reason := lease.Authorize(scope); reason != "" {
		http.Error(w, reason, http.StatusConflict)
		return false
	}
	return true
}

func commitDriveConfigUpdate(lease DriveConfigUpdateLease, scope DriveConfigUpdateScope, apply func() error) (bool, error) {
	if lease != nil {
		return lease.Commit(scope, apply)
	}
	if apply == nil {
		return false, nil
	}
	return false, apply()
}

func (a *AdminServer) prepareDriveDelete(ctx context.Context, driveID string) error {
	if a.OnPrepareDriveDelete != nil {
		return a.OnPrepareDriveDelete(ctx, driveID)
	}
	if a.OnStopDriveTasks != nil {
		a.OnStopDriveTasks(driveID)
	}
	return nil
}

func completeDriveDelete(lease DriveConfigUpdateLease) error {
	deferred, err := commitDriveConfigUpdate(lease, DriveConfigUpdateDestructive, nil)
	if err != nil {
		return fmt.Errorf("complete drive deletion: %w", err)
	}
	if deferred {
		return errors.New("complete drive deletion: destructive operation was unexpectedly deferred")
	}
	return nil
}

func (a *AdminServer) handleStopDriveTasks(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stopped := false
	if a.OnStopDriveTasks != nil {
		stopped = a.OnStopDriveTasks(id)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"stopped": stopped,
	})
}
