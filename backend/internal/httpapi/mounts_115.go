package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/SheltonZhu/115driver/pkg/driver"
	"nestify/backend/internal/model"
	"nestify/backend/internal/pan115"
)

type pan115LoginStore struct {
	mu    sync.Mutex
	items map[string]*pan115Login
}

type pan115Login struct {
	mu       sync.Mutex
	owner    string
	device   string
	interval time.Duration
	session  *driver.QRCodeSession
	expires  time.Time
	lastPoll time.Time
	status   int
	cookie   string
}

func (a *apiHandler) handle115Devices(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Data: pan115.Devices})
}

func (a *apiHandler) handle115QRCode(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		Device   string `json:"device"`
		Interval int    `json:"request_interval_ms"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil ||
		!pan115.ValidDevice(input.Device) || input.Interval < 0 || input.Interval > 60000 {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: "INVALID_115_LOGIN", Message: "设备类型或 API 间隔无效"})
		return
	}
	owner, _ := a.sessionFromRequest(r)
	interval := time.Duration(max(input.Interval, 1000)) * time.Millisecond
	sdk, err := pan115.NewDriver(r.Context(), "", input.Device, interval)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	session, err := sdk.QRCodeStart()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, jsonResponse{Code: "115_QR_FAILED", Message: "获取 115 二维码失败，请稍后重试"})
		return
	}
	image, err := session.QRCode()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		writeInternalError(w, err)
		return
	}
	id := hex.EncodeToString(token[:])
	expires := time.Now().Add(5 * time.Minute)
	a.pan115Logins.mu.Lock()
	if a.pan115Logins.items == nil {
		a.pan115Logins.items = make(map[string]*pan115Login)
	}
	for key, item := range a.pan115Logins.items {
		if time.Now().After(item.expires) || item.owner == owner.Token {
			delete(a.pan115Logins.items, key)
		}
	}
	if len(a.pan115Logins.items) >= 64 {
		a.pan115Logins.mu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, jsonResponse{Code: "115_QR_BUSY", Message: "登录请求过多，请稍后重试"})
		return
	}
	a.pan115Logins.items[id] = &pan115Login{owner: owner.Token, device: input.Device, interval: interval, session: session, expires: expires}
	a.pan115Logins.mu.Unlock()
	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Data: map[string]any{
		"session_id": id, "image": "data:image/png;base64," + base64.StdEncoding.EncodeToString(image), "expires_at": expires,
	}})
}

func (a *apiHandler) handle115QRCodeStatus(w http.ResponseWriter, r *http.Request) {
	if !a.requireSession(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	var input struct {
		ID string `json:"session_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: "INVALID_JSON", Message: "无效请求"})
		return
	}
	owner, _ := a.sessionFromRequest(r)
	a.pan115Logins.mu.Lock()
	login := a.pan115Logins.items[input.ID]
	if login != nil && time.Now().After(login.expires) {
		delete(a.pan115Logins.items, input.ID)
		login = nil
	}
	a.pan115Logins.mu.Unlock()
	if login == nil || login.owner != owner.Token {
		writeJSON(w, http.StatusGone, jsonResponse{Code: "115_QR_EXPIRED", Message: "二维码已失效，请重新获取"})
		return
	}
	if !login.mu.TryLock() {
		writeJSON(w, http.StatusConflict, jsonResponse{Code: "115_QR_POLLING", Message: "正在检查扫码状态"})
		return
	}
	defer login.mu.Unlock()
	if login.cookie == "" && login.status >= 0 && time.Since(login.lastPoll) >= max(login.interval, 2*time.Second) {
		login.lastPoll = time.Now()
		sdk, err := pan115.NewDriver(r.Context(), "", login.device, login.interval)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		status, err := sdk.QRCodeStatus(login.session)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, jsonResponse{Code: "115_QR_STATUS_FAILED", Message: "检查扫码状态失败，请重试"})
			return
		}
		login.status = status.Status
		if status.IsAllowed() {
			credential, err := sdk.QRCodeLoginWithApp(login.session, driver.LoginApp(login.device))
			if err != nil {
				writeJSON(w, http.StatusBadGateway, jsonResponse{Code: "115_QR_LOGIN_FAILED", Message: "115 登录失败，请重新扫码"})
				return
			}
			if _, err := pan115.ParseCookie(credential.Cookie()); err != nil {
				writeInternalError(w, err)
				return
			}
			login.cookie = credential.Cookie()
		}
	}
	writeJSON(w, http.StatusOK, jsonResponse{Success: true, Code: "OK", Data: map[string]any{
		"status": login.status, "cookie": login.cookie, "device": login.device,
	}})
}

func validate115Mount(provider, cookie, device, basePath string, interval int, allowEmpty bool) string {
	if provider != model.MountProvider115 {
		return ""
	}
	if !pan115.ValidDevice(device) {
		return "请选择有效的 115 设备类型"
	}
	if interval < 0 || interval > 60000 {
		return "API 请求间隔必须为 0 到 60000 毫秒"
	}
	for _, part := range strings.Split(strings.Trim(basePath, "/"), "/") {
		if part == "." || part == ".." || strings.Contains(part, "\\") {
			return "指定路径不能包含 .、.. 或反斜杠"
		}
	}
	if !allowEmpty || strings.TrimSpace(cookie) != "" {
		if _, err := pan115.ParseCookie(cookie); err != nil {
			return err.Error()
		}
	}
	return ""
}
