package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"nestify/backend/internal/model"
	"nestify/backend/internal/webdav"
)

// parseMountVirtualPath 解析形如 webdav://12/移动云盘/电视剧 的虚拟挂载路径。
func parseMountVirtualPath(value string) (int64, string, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, model.MountPathScheme) {
		return 0, "", fmt.Errorf("不是 WebDAV 挂载路径")
	}

	rest := strings.TrimPrefix(trimmed, model.MountPathScheme)
	parts := strings.SplitN(rest, "/", 2)

	id, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || id <= 0 {
		return 0, "", fmt.Errorf("无效的挂载标识")
	}

	internal := ""
	if len(parts) == 2 {
		internal = parts[1]
	}
	return id, internal, nil
}

func (a *apiHandler) handleMounts(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := a.store.ListMounts()
		if err != nil {
			writeInternalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "WebDAV 挂载列表已加载",
			Data:    map[string]any{"items": items},
		})
	case http.MethodPost:
		var input model.CreateMountInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Success: false,
				Code:    "INVALID_JSON",
				Message: "Invalid request body",
			})
			return
		}

		if message := validateMountInput(input.Name, input.Host, input.Port); message != "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Success: false,
				Code:    "INVALID_MOUNT",
				Message: message,
			})
			return
		}

		mount, err := a.store.CreateMount(input)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		a.reloadPathBrowse()

		writeJSON(w, http.StatusOK, jsonResponse{
			Success: true,
			Code:    "OK",
			Message: "WebDAV 挂载已创建",
			Data:    mount,
		})
	default:
		writeMethodNotAllowed(w)
	}
}

func (a *apiHandler) handleMountByID(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, "/api/v1/mounts/")
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_MOUNT_ID", Message: "缺少挂载标识"})
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_MOUNT_ID", Message: "挂载标识必须为数字"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		credential, err := a.store.GetMountCredential(id)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		if credential == nil {
			writeJSON(w, http.StatusNotFound, jsonResponse{Success: false, Code: "MOUNT_NOT_FOUND", Message: "挂载不存在"})
			return
		}
		// 编辑场景需要回显密码（本机管理工具，仅 admin 登录后可见）。
		detail := credential.Mount
		detail.Password = credential.Password
		writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "WebDAV 挂载已加载", Data: detail})
	case http.MethodPut:
		var input model.UpdateMountInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
			return
		}

		if message := validateMountInput(input.Name, input.Host, input.Port); message != "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_MOUNT", Message: message})
			return
		}

		mount, err := a.store.UpdateMount(id, input)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		if mount == nil {
			writeJSON(w, http.StatusNotFound, jsonResponse{Success: false, Code: "MOUNT_NOT_FOUND", Message: "挂载不存在"})
			return
		}
		a.reloadPathBrowse()

		writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "WebDAV 挂载已更新", Data: mount})
	case http.MethodDelete:
		if err := a.store.DeleteMount(id); err != nil {
			writeInternalError(w, err)
			return
		}
		a.reloadPathBrowse()

		writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "WebDAV 挂载已删除"})
	default:
		writeMethodNotAllowed(w)
	}
}

// handleMountBrowse 列出 WebDAV 挂载中的远端目录内容。
func (a *apiHandler) handleMountBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	virtualPath := r.URL.Query().Get("path")
	mountID, internalPath, err := parseMountVirtualPath(virtualPath)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_MOUNT_PATH", Message: err.Error()})
		return
	}

	credential, err := a.store.GetMountCredential(mountID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if credential == nil {
		writeJSON(w, http.StatusNotFound, jsonResponse{Success: false, Code: "MOUNT_NOT_FOUND", Message: "挂载不存在"})
		return
	}
	if !credential.Mount.Enabled {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "MOUNT_DISABLED", Message: "该 WebDAV 挂载已停用"})
		return
	}

	client := webdav.NewClient(credential.Mount, credential.Password)
	entries, err := client.List(r.Context(), internalPath)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, jsonResponse{Success: false, Code: "MOUNT_BROWSE_FAILED", Message: err.Error()})
		return
	}

	response := model.BrowseDirectoriesResponse{
		CurrentPath: model.JoinMountVirtualPath(mountID, internalPath),
		Entries:     make([]model.DirectoryEntry, 0, len(entries)),
	}
	if normalized := strings.Trim(strings.TrimSpace(internalPath), "/"); normalized != "" {
		// 用 path.Dir 求父级：webdav://12/移动云盘/电视剧 -> webdav://12/移动云盘，
		// webdav://12/移动云盘 -> webdav://12（挂载根，parent_path 为空表示已在根）。
		response.ParentPath = model.MountParentVirtualPath(mountID, normalized)
	}

	for _, entry := range entries {
		childPath := model.JoinMountVirtualPath(mountID, entry.Path)
		item := model.DirectoryEntry{
			Name:       entry.Name,
			Path:       childPath,
			IsDir:      entry.IsDir,
			Size:       entry.Size,
			ModifiedAt: model.FormatTimeRFC3339(entry.ModifiedAt),
		}
		if entry.IsDir {
			item.Size = 0
			item.HasChildren = true
		}
		response.Entries = append(response.Entries, item)
	}

	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Message: "WebDAV 目录已加载", Data: response})
}

// handleMountTest 用给定参数（无需保存）发起一次 PROPFIND，验证 WebDAV 连接是否可用。
func (a *apiHandler) handleMountTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if !a.requireSession(w, r) {
		return
	}

	var input model.CreateMountInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_JSON", Message: "Invalid request body"})
		return
	}

	if message := validateMountInput(input.Name, input.Host, input.Port); message != "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Success: false, Code: "INVALID_MOUNT", Message: message})
		return
	}

	mount := model.WebdavMount{
		Name:     strings.TrimSpace(input.Name),
		Scheme:   model.NormalizeMountScheme(input.Scheme),
		Host:     strings.TrimSpace(input.Host),
		Port:     input.Port,
		Username: strings.TrimSpace(input.Username),
		BasePath: model.NormalizeMountBasePath(input.BasePath),
		Enabled:  true,
	}

	client := webdav.NewClient(mount, input.Password)
	entries, err := client.List(r.Context(), "")
	if err != nil {
		writeJSON(w, http.StatusOK, jsonResponse{
			Success: false,
			Code:    "MOUNT_TEST_FAILED",
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, jsonResponse{
		Success: true,
		Code:    "OK",
		Message: "连接成功",
		Data:    map[string]any{"count": len(entries)},
	})
}

func validateMountInput(name, host string, port int) string {
	if strings.TrimSpace(name) == "" {
		return "挂载名称不能为空"
	}
	if strings.TrimSpace(host) == "" {
		return "域名或 IP 不能为空"
	}
	if port < 0 || port > 65535 {
		return "端口必须在 0 - 65535 之间"
	}
	return ""
}

func (a *apiHandler) reloadPathBrowse() {
	if a.pathBrowse == nil {
		return
	}
	mounts, err := a.store.ListMounts()
	if err != nil {
		return
	}
	a.pathBrowse.SetMounts(mounts)
}
