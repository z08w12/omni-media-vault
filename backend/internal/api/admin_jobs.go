package api

import (
	"net/http"
)

// handleRunScanAllJob 触发手动全量扫盘与维护。
func (a *AdminServer) handleRunScanAllJob(w http.ResponseWriter, r *http.Request) {
	accepted := false
	if a.OnRunScanAllJob != nil {
		accepted = a.OnRunScanAllJob()
	}
	resp := map[string]any{
		"ok":       true,
		"accepted": accepted,
		"status":   a.nightlyJobStatus(),
	}
	if !accepted {
		resp["message"] = fullScanBusyMessage
	}
	writeJSON(w, http.StatusAccepted, resp)
}

// handleRunNightlyJob 保留旧接口路径的兼容性；其行为与全量扫描一致。
func (a *AdminServer) handleRunNightlyJob(w http.ResponseWriter, r *http.Request) {
	a.handleRunScanAllJob(w, r)
}

func (a *AdminServer) handleScanAllJobStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.nightlyJobStatus())
}

// handleNightlyJobStatus 保留旧接口路径的兼容性。
func (a *AdminServer) handleNightlyJobStatus(w http.ResponseWriter, r *http.Request) {
	a.handleScanAllJobStatus(w, r)
}

func (a *AdminServer) handleStopAllTasks(w http.ResponseWriter, r *http.Request) {
	stoppedDrives := 0
	if a.OnStopAllTasks != nil {
		stoppedDrives = a.OnStopAllTasks()
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":            true,
		"stoppedDrives": stoppedDrives,
		"status":        a.nightlyJobStatus(),
	})
}

func (a *AdminServer) nightlyJobStatus() NightlyJobStatus {
	if a.GetNightlyJobStatus == nil {
		return NightlyJobStatus{State: "idle"}
	}
	status := a.GetNightlyJobStatus()
	if status.State == "" {
		status.State = "idle"
	}
	return status
}
