package httpapi

import (
	"encoding/json"
	"errors"
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

	mux.HandleFunc("/api/v1/auth/login", api.handleLogin)
	mux.HandleFunc("/api/v1/auth/session", api.handleCurrentSession)
	mux.HandleFunc("/api/v1/auth/logout", api.handleLogout)
	mux.HandleFunc("/api/v1/paths/roots", api.handlePathRoots)
	mux.HandleFunc("/api/v1/paths/browse", api.handlePathBrowse)
	mux.HandleFunc("/api/v1/paths/validate", api.handlePathValidate)
	mux.HandleFunc("/api/v1/manual/preflight", api.handleManualPreflight)
	mux.HandleFunc("/api/v1/executions/prepare-rule", api.handlePrepareRuleExecution)
	mux.HandleFunc("/api/v1/runs/", api.handleRuns)
	mux.HandleFunc("/api/v1/settings", api.handleSettings)
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
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		ArchiveMode: rule.ArchiveMode,
		TriggerMode: input.TriggerMode,
		SourceDir:   rule.SourceDir,
		TargetDir:   rule.TargetDir,
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
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		ArchiveMode: rule.ArchiveMode,
		TriggerMode: input.TriggerMode,
		SourceDir:   rule.SourceDir,
		TargetDir:   rule.TargetDir,
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

func (a *apiHandler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleListRules(w, r)
	case http.MethodPost:
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
		a.handleGetRuleByID(w, r, id)
	case http.MethodPut:
		a.handleUpdateRule(w, r, id)
	default:
		writeMethodNotAllowed(w)
	}
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

	writeJSON(w, http.StatusCreated, jsonResponse{
		Success: true,
		Code:    "CREATED",
		Message: "Rule created",
		Data:    rule,
	})
}

func validateCreateRuleInput(input model.CreateRuleInput) error {
	return validateRuleFields(input.Name, input.SourceDir, input.TargetDir, input.ArchiveMode, input.RunMode)
}

func validateUpdateRuleInput(input model.UpdateRuleInput) error {
	return validateRuleFields(input.Name, input.SourceDir, input.TargetDir, input.ArchiveMode, input.RunMode)
}

func validateRuleFields(name, sourceDir, targetDir, archiveMode, runMode string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("rule name is required")
	}
	if strings.TrimSpace(sourceDir) == "" {
		return errors.New("source_dir is required")
	}
	if strings.TrimSpace(targetDir) == "" {
		return errors.New("target_dir is required")
	}

	archiveMode = strings.TrimSpace(archiveMode)
	if archiveMode != "package" && archiveMode != "collect" {
		return errors.New("archive_mode must be package or collect")
	}

	runMode = strings.TrimSpace(runMode)
	if runMode != "watch" && runMode != "cron" && runMode != "once" {
		return errors.New("run_mode must be watch, cron, or once")
	}

	return nil
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
