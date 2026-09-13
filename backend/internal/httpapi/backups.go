package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"nestify/backend/internal/model"
)

func (a *apiHandler) handleReorderBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeMethodNotAllowed(w)
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	var items []model.BackupReorderItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	if err := a.store.ReorderBackups(items); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "备份规则顺序已更新"})
}

func (a *apiHandler) handleBackups(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := a.store.ListBackups()
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "备份规则已加载",
			Data:    map[string]any{"items": items},
		})
	case http.MethodPost:
		var input model.CreateBackupInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
			return
		}

		if message := validateBackupInput(input.Name, input.SourceDirs, input.TargetDirs); message != "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_BACKUP", Message: message})
			return
		}

		task, err := a.store.CreateBackup(input)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		if a.backups != nil {
			_ = a.backups.Reload()
		}

		writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "备份规则已创建", Data: task})
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *apiHandler) handleBackupByID(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}

	raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/backups/"), "/")
	parts := strings.Split(raw, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_BACKUP_ID", Message: "缺少备份规则标识"})
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_BACKUP_ID", Message: "备份规则标识必须为数字"})
		return
	}

	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		a.handleBackupResource(w, r, id)
	case "run":
		a.handleBackupRun(w, r, id)
	case "enabled":
		a.handleBackupEnabled(w, r, id)
	case "status":
		a.handleBackupStatus(w, r, id)
	default:
		writeJSON(w, http.StatusNotFound, jsonResponse{Success: false, Code: "NOT_FOUND", Message: "未知的备份规则操作"})
	}
}

func (a *apiHandler) handleBackupResource(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		task, err := a.store.GetBackup(id)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		if task == nil {
			writeJSON(w, http.StatusNotFound, jsonResponse{Success: false, Code: "BACKUP_NOT_FOUND", Message: "备份规则不存在"})
			return
		}
		writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "备份规则已加载", Data: task})
	case http.MethodPut:
		var input model.UpdateBackupInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
			return
		}

		if message := validateBackupInput(input.Name, input.SourceDirs, input.TargetDirs); message != "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_BACKUP", Message: message})
			return
		}

		task, err := a.store.UpdateBackup(id, input)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		if task == nil {
			writeJSON(w, http.StatusNotFound, jsonResponse{Success: false, Code: "BACKUP_NOT_FOUND", Message: "备份规则不存在"})
			return
		}
		if a.backups != nil {
			_ = a.backups.Reload()
		}

		writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "备份规则已更新", Data: task})
	case http.MethodDelete:
		if err := a.store.DeleteBackup(id); err != nil {
			writeInternalError(w, err)
			return
		}
		if a.backups != nil {
			_ = a.backups.Reload()
		}

		writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "备份规则已移除"})
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *apiHandler) handleBackupRun(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if a.backups == nil {
		writeJSON(w, http.StatusServiceUnavailable, jsonResponse{Success: false, Code: "BACKUP_DISABLED", Message: "备份服务不可用"})
		return
	}

	var input struct {
		ForceFull bool `json:"force_full"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}

	if err := a.backups.RunTask(id, input.ForceFull); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "BACKUP_RUN_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "备份任务已启动", Data: a.backups.Status(id)})
}

func (a *apiHandler) handleBackupEnabled(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var input struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	task, err := a.store.SetBackupEnabled(id, input.Enabled)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if task == nil {
		writeJSON(w, http.StatusNotFound, jsonResponse{Success: false, Code: "BACKUP_NOT_FOUND", Message: "备份规则不存在"})
		return
	}
	if a.backups != nil {
		_ = a.backups.Reload()
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "备份规则状态已更新", Data: task})
}

func (a *apiHandler) handleBackupStatus(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if a.backups == nil {
		writeJSON(w, http.StatusServiceUnavailable, jsonResponse{Success: false, Code: "BACKUP_DISABLED", Message: "备份服务不可用"})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "备份状态已加载", Data: a.backups.Status(id)})
}

func validateBackupInput(name string, sourceDirs, targetDirs []string) string {
	if strings.TrimSpace(name) == "" {
		return "备份任务名称不能为空"
	}

	sources := 0
	for _, item := range sourceDirs {
		if strings.TrimSpace(item) != "" {
			sources++
		}
	}
	if sources == 0 {
		return "请至少选择一个源路径"
	}

	targets := 0
	for _, item := range targetDirs {
		if strings.TrimSpace(item) != "" {
			targets++
		}
	}
	if targets == 0 {
		return "请至少添加一个目标路径"
	}

	return ""
}
