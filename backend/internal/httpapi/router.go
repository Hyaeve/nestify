package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nestify/backend/internal/auth"
	"nestify/backend/internal/config"
	"nestify/backend/internal/executor"
	"nestify/backend/internal/model"
	"nestify/backend/internal/pathbrowse"
	"nestify/backend/internal/store/sqlite"
)

const sessionCookieName = "nestify_session"

type jsonResponse struct {
	Success bool        `json:"success"`
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Dependencies struct {
	Env        config.Env
	Store      *sqlite.Store
	Sessions   *auth.SessionManager
	PathBrowse *pathbrowse.Service
	Executor   *executor.Service
}

type apiHandler struct {
	env        config.Env
	store      *sqlite.Store
	sessions   *auth.SessionManager
	pathBrowse *pathbrowse.Service
	executor   *executor.Service
}

func NewRouter(deps Dependencies) http.Handler {
	api := &apiHandler{
		env:        deps.Env,
		store:      deps.Store,
		sessions:   deps.Sessions,
		pathBrowse: deps.PathBrowse,
		executor:   deps.Executor,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "Nestify backend is running",
			Data: map[string]any{
				"service":  "nestify-backend",
				"database": deps.Env.DBPath,
				"time":     time.Now().UTC(),
			},
		})
	})

	mux.HandleFunc("/api/v1/system/info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "System info placeholder",
			Data: map[string]any{
				"app_name": "Nestify",
				"db_path":  deps.Env.DBPath,
				"stage":    "rule-storage-connected",
			},
		})
	})

	mux.HandleFunc("/api/v1/system/restart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}

		writeJSON(w, http.StatusAccepted, jsonResponse{
			Success: true,
			Code:    "RESTARTING",
			Message: "System restart requested",
		})

		go func() {
			time.Sleep(300 * time.Millisecond)
			os.Exit(0)
		}()
	})

	mux.HandleFunc("/api/v1/system/resource", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "System resource loaded",
			Data:    collectSystemResourceSnapshot(),
		})
	})

	mux.HandleFunc("/api/v1/auth/login", api.handleLogin)
	mux.HandleFunc("/api/v1/auth/session", api.handleCurrentSession)
	mux.HandleFunc("/api/v1/auth/logout", api.handleLogout)
	mux.HandleFunc("/api/v1/paths/roots", api.handlePathRoots)
	mux.HandleFunc("/api/v1/paths/browse", api.handlePathBrowse)
	mux.HandleFunc("/api/v1/paths/validate", api.handlePathValidate)
	mux.HandleFunc("/api/v1/files/create-folder", api.handleCreateFolder)
	mux.HandleFunc("/api/v1/files/upload", api.handleUploadFiles)
	mux.HandleFunc("/api/v1/files/rename", api.handleRenameItem)
	mux.HandleFunc("/api/v1/files/copy", api.handleCopyItems)
	mux.HandleFunc("/api/v1/files/move", api.handleMoveItems)
	mux.HandleFunc("/api/v1/files/delete", api.handleDeleteItems)
	mux.HandleFunc("/api/v1/files/pack-cbz", api.handlePackCBZ)
	mux.HandleFunc("/api/v1/manual/preflight", api.handleManualPreflight)
	mux.HandleFunc("/api/v1/executions/prepare-rule", api.handlePrepareRuleExecution)
	mux.HandleFunc("/api/v1/runs/", api.handleRuns)
	mux.HandleFunc("/api/v1/run-history", api.handleRunHistory)
	mux.HandleFunc("/api/v1/settings", api.handleSettings)
	mux.HandleFunc("/api/v1/settings/admin-account", api.handleUpdateAdminAccount)
	mux.HandleFunc("/api/v1/rules", api.handleRules)
	mux.HandleFunc("/api/v1/rules/", api.handleRuleByID)

	registerStaticRoutes(mux, deps.Env.WebDir)

	return mux
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type prepareRuleExecutionRequest struct {
	RuleID      int64  `json:"rule_id"`
	TriggerMode string `json:"trigger_mode"`
}

type createFolderRequest struct {
	ParentPath string `json:"parent_path"`
	Name       string `json:"name"`
}

type renameItemRequest struct {
	Path    string `json:"path"`
	NewName string `json:"new_name"`
}

type fileMutationRequest struct {
	Paths           []string `json:"paths"`
	DestinationPath string   `json:"destination_path"`
	OutputDir       string   `json:"output_dir"`
	ArchiveName     string   `json:"archive_name"`
}

type updateAdminAccountRequest struct {
	Username        string `json:"username"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (a *apiHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var input loginRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_JSON",
			Message: "Invalid request body",
		})
		return
	}

	admin, err := a.store.GetAdminByUsername(input.Username)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	if admin == nil || auth.ComparePassword(admin.PasswordHash, input.Password) != nil {
		writeJSON(w, http.StatusUnauthorized, jsonResponse{
			Success: false,
			Code:    "INVALID_CREDENTIALS",
			Message: "用户名或密码错误",
		})
		return
	}

	session, err := a.sessions.Create(model.SessionUser{ID: admin.ID, Username: admin.Username})
	if err != nil {
		writeInternalError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "登录成功",
		Data:    session.User,
	})
}

func (a *apiHandler) handleCurrentSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	session, ok := a.sessionFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, jsonResponse{
			Success: false,
			Code:    "UNAUTHORIZED",
			Message: "未登录",
		})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "当前会话有效",
		Data:    session.User,
	})
}

func (a *apiHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.sessions.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "已退出登录",
	})
}

func (a *apiHandler) sessionFromRequest(r *http.Request) (auth.Session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return auth.Session{}, false
	}

	return a.sessions.Get(cookie.Value)
}

func (a *apiHandler) requireSession(w http.ResponseWriter, r *http.Request) bool {
	_, ok := a.sessionFromRequest(r)
	if ok {
		return true
	}

	writeJSON(w, http.StatusUnauthorized, jsonResponse{
		Success: false,
		Code:    "UNAUTHORIZED",
		Message: "未登录",
	})
	return false
}

func (a *apiHandler) handlePathRoots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Browse roots loaded",
		Data: map[string]any{
			"items": a.pathBrowse.Roots(),
		},
	})
}

func (a *apiHandler) handlePathBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	browsePath := r.URL.Query().Get("path")
	data, err := a.pathBrowse.Browse(browsePath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_BROWSE_PATH",
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Directory entries loaded",
		Data:    data,
	})
}

func (a *apiHandler) handlePathValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	path := r.URL.Query().Get("path")
	data, err := a.pathBrowse.Validate(path)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Path validated",
		Data:    data,
	})
}

func (a *apiHandler) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	var input createFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	createdPath, err := a.pathBrowse.CreateDirectory(input.ParentPath, input.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "CREATE_FOLDER_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "Folder created", Data: model.CreateDirectoryResponse{Path: createdPath}})
}

func (a *apiHandler) handleUploadFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	if err := r.ParseMultipartForm(512 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_MULTIPART", Message: err.Error()})
		return
	}

	destinationPath := r.FormValue("destination_path")
	files := r.MultipartForm.File["files"]
	saved, err := a.pathBrowse.UploadFiles(destinationPath, files)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "UPLOAD_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "Files uploaded", Data: model.FileItemsMutationResponse{Items: saved, Total: len(saved)}})
}

func (a *apiHandler) handleRenameItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	var input renameItemRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	renamedPath, err := a.pathBrowse.RenameItem(input.Path, input.NewName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "RENAME_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "Item renamed", Data: model.FileItemsMutationResponse{Items: []string{renamedPath}, Total: 1}})
}

func (a *apiHandler) handleCopyItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	var input fileMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	items, err := a.pathBrowse.CopyItems(input.Paths, input.DestinationPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "COPY_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "Items copied", Data: model.FileItemsMutationResponse{Items: items, Total: len(items)}})
}

func (a *apiHandler) handleMoveItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	var input fileMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	items, err := a.pathBrowse.MoveItems(input.Paths, input.DestinationPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "MOVE_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "Items moved", Data: model.FileItemsMutationResponse{Items: items, Total: len(items)}})
}

func (a *apiHandler) handleDeleteItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	var input fileMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	if err := a.pathBrowse.DeleteItems(input.Paths); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "DELETE_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "Items deleted", Data: model.FileItemsMutationResponse{Total: len(input.Paths)}})
}

func (a *apiHandler) handlePackCBZ(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	var input fileMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	outputPath, err := a.pathBrowse.PackItemsAsCBZ(input.Paths, input.OutputDir, input.ArchiveName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "PACK_CBZ_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "CBZ archive created", Data: model.FileItemsMutationResponse{Total: len(input.Paths), OutputPath: outputPath}})
}

func (a *apiHandler) handleManualPreflight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	var input model.ManualPreflightRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_JSON",
			Message: "Invalid request body",
		})
		return
	}

	run, result, err := a.executor.PrepareManualPreflight(input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_MANUAL_PRECHECK",
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Manual preflight prepared",
		Data: map[string]any{
			"run":       run,
			"preflight": result,
		},
	})
}

func (a *apiHandler) handlePrepareRuleExecution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	if !a.requireSession(w, r) {
		return
	}

	var input prepareRuleExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_JSON",
			Message: "Invalid request body",
		})
		return
	}

	rule, err := a.store.GetRuleByID(input.RuleID)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	if rule == nil {
		writeJSON(w, http.StatusNotFound, jsonResponse{
			Success: false,
			Code:    "RULE_NOT_FOUND",
			Message: "Rule not found",
		})
		return
	}

	prepared, err := executor.PrepareMode(executor.ExecuteRuleRequest{
		RuleID:            rule.ID,
		RuleName:          rule.Name,
		ArchiveMode:       rule.ArchiveMode,
		TriggerMode:       input.TriggerMode,
		CompatibilityMode: rule.CompatibilityMode,
		SourceDir:         rule.SourceDir,
		TargetDir:         rule.TargetDir,
		Options:           executor.ParseBoolOptionsJSON(rule.OptionsJSON),
		PackageOptions:    executor.ParseBoolOptionsJSON(rule.PackageOptionsJSON),
		CollectOptions:    executor.ParseBoolOptionsJSON(rule.CollectOptionsJSON),
		Filters:           executor.ParseStringListJSON(rule.FiltersJSON),
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_EXECUTION_REQUEST",
			Message: err.Error(),
		})
		return
	}

	run, err := a.executor.PrepareRuleRun(executor.ExecuteRuleRequest{
		RuleID:            rule.ID,
		RuleName:          rule.Name,
		ArchiveMode:       rule.ArchiveMode,
		TriggerMode:       input.TriggerMode,
		CompatibilityMode: rule.CompatibilityMode,
		SourceDir:         rule.SourceDir,
		TargetDir:         rule.TargetDir,
		Options:           executor.ParseBoolOptionsJSON(rule.OptionsJSON),
		PackageOptions:    executor.ParseBoolOptionsJSON(rule.PackageOptionsJSON),
		CollectOptions:    executor.ParseBoolOptionsJSON(rule.CollectOptionsJSON),
		Filters:           executor.ParseStringListJSON(rule.FiltersJSON),
	})
	if err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Rule execution skeleton prepared",
		Data: map[string]any{
			"run":      run,
			"prepared": prepared,
		},
	})
}

func (a *apiHandler) handleRuns(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/runs/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_RUN_ID",
			Message: "missing run id",
		})
		return
	}

	if strings.HasSuffix(path, "/logs") {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		runID := strings.TrimSuffix(path, "/logs")
		runID = strings.TrimSuffix(runID, "/")
		a.handleRunLogs(w, runID)
		return
	}

	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	a.handleRunByID(w, path)
}

func (a *apiHandler) handleRunByID(w http.ResponseWriter, runID string) {
	run, ok := a.executor.GetRun(runID)
	if !ok {
		writeJSON(w, http.StatusNotFound, jsonResponse{
			Success: false,
			Code:    "RUN_NOT_FOUND",
			Message: "Run not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Run loaded",
		Data:    run,
	})
}

func (a *apiHandler) handleRunLogs(w http.ResponseWriter, runID string) {
	run, ok := a.executor.GetRun(runID)
	if !ok || run == nil {
		writeJSON(w, http.StatusNotFound, jsonResponse{
			Success: false,
			Code:    "RUN_NOT_FOUND",
			Message: "Run not found",
		})
		return
	}

	entries := a.executor.ListRunLogs(runID)
	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Run logs loaded",
		Data: map[string]any{
			"items": entries,
			"total": len(entries),
		},
	})
}

func (a *apiHandler) handleRunHistory(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		archiveMode := strings.TrimSpace(r.URL.Query().Get("archive_mode"))

		page, pageSize, paged, err := parsePaginationQuery(r, 25, "keyword", "status", "archive_mode")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Success: false,
				Code:    "INVALID_PAGINATION",
				Message: err.Error(),
			})
			return
		}

		if paged {
			items, total, err := a.store.ListRunHistoryPage(page, pageSize, keyword, status, archiveMode)
			if err != nil {
				writeInternalError(w, err)
				return
			}

			summary, err := a.store.GetRunHistorySummary()
			if err != nil {
				writeInternalError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, jsonResponse{
				Success: true,
				Code:    "OK",
				Message: "Run history loaded",
				Data: map[string]any{
					"items":     items,
					"total":     total,
					"page":      page,
					"page_size": pageSize,
					"summary":   summary,
				},
			})
			return
		}

		items := a.executor.ListHistory()
		summary := summarizeRunHistoryItems(items)
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "Run history loaded",
			Data: map[string]any{
				"items":     items,
				"total":     len(items),
				"page":      1,
				"page_size": len(items),
				"summary":   summary,
			},
		})
	case http.MethodDelete:
		if id := strings.TrimSpace(r.URL.Query().Get("id")); id != "" {
			if err := a.store.DeleteRunHistoryByID(id); err != nil {
				writeInternalError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, jsonResponse{
				Success: true,
				Code:    "OK",
				Message: "Run history entry deleted",
			})
			return
		}

		if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
			if status != "success" && status != "failed" && status != "skip" {
				writeJSON(w, http.StatusBadRequest, jsonResponse{
					Success: false,
					Code:    "INVALID_STATUS",
					Message: "unsupported history status",
				})
				return
			}

			if err := a.store.DeleteRunHistoryByStatus(status); err != nil {
				writeInternalError(w, err)
				return
			}

			writeJSON(w, http.StatusOK, jsonResponse{
				Success: true,
				Code:    "OK",
				Message: "Run history entries deleted",
			})
			return
		}

		if err := a.executor.ClearHistory(); err != nil {
			writeInternalError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "Run history cleared",
		})
	default:
		writeMethodNotAllowed(w)
	}
}

func registerStaticRoutes(mux *http.ServeMux, webDir string) {

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		if webDir == "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Nestify backend is running. Frontend assets are not configured."))
			return
		}

		indexPath := filepath.Join(webDir, "index.html")
		cleanPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")

		if cleanPath == "" || cleanPath == "." {
			http.ServeFile(w, r, indexPath)
			return
		}

		assetPath := filepath.Join(webDir, cleanPath)
		if info, err := os.Stat(assetPath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, assetPath)
			return
		}

		http.ServeFile(w, r, indexPath)
	})
}

func (a *apiHandler) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	settings, err := a.store.GetSettings()
	if err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Settings loaded",
		Data:    settings,
	})
}

func (a *apiHandler) handleUpdateAdminAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	session, ok := a.sessionFromRequest(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, jsonResponse{
			Success: false,
			Code:    "UNAUTHORIZED",
			Message: "未登录",
		})
		return
	}

	var input updateAdminAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_JSON",
			Message: "Invalid request body",
		})
		return
	}

	username := strings.TrimSpace(input.Username)
	currentPassword := input.CurrentPassword
	newPassword := input.NewPassword
	if username == "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_ADMIN_USERNAME",
			Message: "管理员账号不能为空",
		})
		return
	}
	if strings.TrimSpace(currentPassword) == "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "CURRENT_PASSWORD_REQUIRED",
			Message: "请输入当前密码",
		})
		return
	}
	if username == session.User.Username && strings.TrimSpace(newPassword) == "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "NO_ADMIN_CHANGES",
			Message: "请至少修改管理员账号或新密码",
		})
		return
	}

	admin, err := a.store.GetAdminByID(session.User.ID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if admin == nil {
		writeJSON(w, http.StatusNotFound, jsonResponse{
			Success: false,
			Code:    "ADMIN_NOT_FOUND",
			Message: "管理员不存在",
		})
		return
	}
	if auth.ComparePassword(admin.PasswordHash, currentPassword) != nil {
		writeJSON(w, http.StatusUnauthorized, jsonResponse{
			Success: false,
			Code:    "INVALID_CURRENT_PASSWORD",
			Message: "当前密码错误",
		})
		return
	}

	nextPasswordHash := admin.PasswordHash
	if strings.TrimSpace(newPassword) != "" {
		hash, err := auth.HashPassword(newPassword)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		nextPasswordHash = hash
	}

	updatedAdmin, err := a.store.UpdateAdminCredentials(admin.ID, username, nextPasswordHash)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "UPDATE_ADMIN_FAILED",
			Message: err.Error(),
		})
		return
	}

	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.sessions.ReplaceUser(cookie.Value, model.SessionUser{ID: updatedAdmin.ID, Username: updatedAdmin.Username})
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "管理员账号已更新",
		Data: model.SessionUser{
			ID:       updatedAdmin.ID,
			Username: updatedAdmin.Username,
		},
	})
}

func (a *apiHandler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if !a.requireSession(w, r) {
			return
		}
		a.handleListRules(w, r)
	case http.MethodPost:
		if !a.requireSession(w, r) {
			return
		}
		a.handleCreateRule(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *apiHandler) handleRuleByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDFromPath(r.URL.Path, "/api/v1/rules/")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_RULE_ID",
			Message: err.Error(),
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !a.requireSession(w, r) {
			return
		}
		a.handleGetRuleByID(w, r, id)
	case http.MethodPut:
		if !a.requireSession(w, r) {
			return
		}
		a.handleUpdateRule(w, r, id)
	case http.MethodDelete:
		if !a.requireSession(w, r) {
			return
		}
		a.handleDeleteRule(w, r, id)
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *apiHandler) handleDeleteRule(w http.ResponseWriter, r *http.Request, id int64) {
	if err := a.store.DeleteRule(id); err != nil {
		writeInternalError(w, err)
		return
	}
	if err := a.executor.ReloadAutomation(); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Rule deleted",
	})
}

func (a *apiHandler) handleGetRuleByID(w http.ResponseWriter, r *http.Request, id int64) {

	rule, err := a.store.GetRuleByID(id)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	if rule == nil {
		writeJSON(w, http.StatusNotFound, jsonResponse{
			Success: false,
			Code:    "RULE_NOT_FOUND",
			Message: "Rule not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Rule loaded",
		Data:    rule,
	})
}

func (a *apiHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request, id int64) {
	var input model.UpdateRuleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_JSON",
			Message: "Invalid request body",
		})
		return
	}

	if err := validateUpdateRuleInput(input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_RULE_INPUT",
			Message: err.Error(),
		})
		return
	}

	rule, err := a.store.UpdateRule(id, input)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if err := a.executor.ReloadAutomation(); err != nil {
		writeInternalError(w, err)
		return
	}

	if rule == nil {
		writeJSON(w, http.StatusNotFound, jsonResponse{
			Success: false,
			Code:    "RULE_NOT_FOUND",
			Message: "Rule not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Rule updated",
		Data:    rule,
	})
}

func (a *apiHandler) handleListRules(w http.ResponseWriter, r *http.Request) {
	ruleType := strings.TrimSpace(r.URL.Query().Get("rule_type"))
	page, pageSize, paged, err := parsePaginationQuery(r, 25, "rule_type")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_PAGINATION",
			Message: err.Error(),
		})
		return
	}

	if paged {
		rules, total, err := a.store.ListRulesPage(page, pageSize, ruleType)
		if err != nil {
			writeInternalError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "Rules loaded",
			Data: map[string]any{
				"items":     rules,
				"total":     total,
				"page":      page,
				"page_size": pageSize,
			},
		})
		return
	}

	rules, err := a.store.ListRules()
	if err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "Rules loaded",
		Data: map[string]any{
			"items":     rules,
			"total":     len(rules),
			"page":      1,
			"page_size": len(rules),
		},
	})
}

func parsePaginationQuery(r *http.Request, defaultPageSize int, triggerKeys ...string) (int, int, bool, error) {
	query := r.URL.Query()
	pageRaw := strings.TrimSpace(query.Get("page"))
	pageSizeRaw := strings.TrimSpace(query.Get("page_size"))
	paged := pageRaw != "" || pageSizeRaw != ""

	if !paged {
		for _, key := range triggerKeys {
			if strings.TrimSpace(query.Get(key)) != "" {
				paged = true
				break
			}
		}
	}

	if !paged {
		return 1, 0, false, nil
	}

	page := 1
	pageSize := defaultPageSize

	if pageRaw != "" {
		value, err := strconv.Atoi(pageRaw)
		if err != nil || value < 1 {
			return 0, 0, false, errors.New("page must be a positive integer")
		}
		page = value
	}

	if pageSizeRaw != "" {
		value, err := strconv.Atoi(pageSizeRaw)
		if err != nil || value < 1 {
			return 0, 0, false, errors.New("page_size must be a positive integer")
		}
		pageSize = value
	}

	if pageSize > 200 {
		pageSize = 200
	}

	return page, pageSize, true, nil
}

func summarizeRunHistoryItems(items []model.RunHistoryItem) model.RunHistorySummary {
	summary := model.RunHistorySummary{Total: len(items)}
	now := time.Now()
	todayKey := fmt.Sprintf("%04d-%02d-%02d", now.Year(), now.Month(), now.Day())

	for _, item := range items {
		startedAt := item.StartedAt.Local()
		itemKey := fmt.Sprintf("%04d-%02d-%02d", startedAt.Year(), startedAt.Month(), startedAt.Day())
		if itemKey == todayKey {
			summary.Today++
		}

		switch item.Status {
		case "success":
			summary.Success++
		case "failed":
			summary.Failed++
		case "skip":
			summary.Skipped++
		}
	}

	return summary
}

func (a *apiHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var input model.CreateRuleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_JSON",
			Message: "Invalid request body",
		})
		return
	}

	if err := validateCreateRuleInput(input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{
			Success: false,
			Code:    "INVALID_RULE_INPUT",
			Message: err.Error(),
		})
		return
	}

	rule, err := a.store.CreateRule(input)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if err := a.executor.ReloadAutomation(); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, jsonResponse{
		Success: true,
		Code:    "CREATED",
		Message: "Rule created",
		Data:    rule,
	})
}

func validateCreateRuleInput(input model.CreateRuleInput) error {
	return validateRuleFields(input.Name, input.SourceDir, input.TargetDir, input.CompatibilityMode, input.ArchiveMode, input.RunMode, input.Options, input.Filters)
}

func validateUpdateRuleInput(input model.UpdateRuleInput) error {
	return validateRuleFields(input.Name, input.SourceDir, input.TargetDir, input.CompatibilityMode, input.ArchiveMode, input.RunMode, input.Options, input.Filters)
}

func validateRuleFields(name, sourceDir, targetDir, compatibilityMode, archiveMode, runMode string, options map[string]bool, filters []string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("rule name is required")
	}
	if strings.TrimSpace(sourceDir) == "" {
		return errors.New("source_dir is required")
	}
	archiveMode = strings.TrimSpace(archiveMode)
	if archiveMode != "cleanup" && strings.TrimSpace(targetDir) == "" {
		return errors.New("target_dir is required")
	}

	compatibilityMode = strings.TrimSpace(compatibilityMode)
	if compatibilityMode == "" {
		compatibilityMode = "local"
	}
	if compatibilityMode != "local" && compatibilityMode != "compatibility" {
		return errors.New("compatibility_mode must be local or compatibility")
	}

	if archiveMode != "package" && archiveMode != "collect" && archiveMode != "cleanup" {
		return errors.New("archive_mode must be package, collect, or cleanup")
	}
	if archiveMode == "cleanup" {
		cleanupEmptyDirs := options["cleanup_empty_dirs"]
		cleanupMatchingFiles := options["cleanup_matching_files"]
		if !cleanupEmptyDirs && !cleanupMatchingFiles {
			return errors.New("cleanup rule requires at least one cleanup option")
		}
		if cleanupMatchingFiles && len(normalizeRuleFilters(filters)) == 0 {
			return errors.New("filters are required when cleanup_matching_files is enabled")
		}
	}

	runMode = strings.TrimSpace(runMode)
	if runMode != "watch" && runMode != "cron" && runMode != "once" {
		return errors.New("run_mode must be watch, cron, or once")
	}

	return nil
}

func normalizeRuleFilters(filters []string) []string {
	items := make([]string, 0, len(filters))
	for _, filter := range filters {
		trimmed := strings.TrimSpace(filter)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}

	return items
}

func parseIDFromPath(path, prefix string) (int64, error) {
	value := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if value == "" {
		return 0, errors.New("missing rule id")
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, errors.New("rule id must be numeric")
	}

	return id, nil
}

func writeInternalError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, jsonResponse{
		Success: false,
		Code:    "INTERNAL_ERROR",
		Message: err.Error(),
	})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, jsonResponse{
		Success: false,
		Code:    "METHOD_NOT_ALLOWED",
		Message: "Method not allowed",
	})
}

func writeJSON(w http.ResponseWriter, status int, payload jsonResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
