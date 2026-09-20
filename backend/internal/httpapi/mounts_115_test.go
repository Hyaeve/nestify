package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nestify/backend/internal/auth"
	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
	"nestify/backend/internal/pan115"
	"nestify/backend/internal/store/sqlite"
)

type qrTransport func(*http.Request) (*http.Response, error)

func (transport qrTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func Test115LoginAndMountHTTP(t *testing.T) {
	store, err := sqlite.Open(config.Env{DBPath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	sessions := auth.NewSessionManager(time.Hour)
	session, err := sessions.Create(model.SessionUser{})
	if err != nil {
		t.Fatal(err)
	}
	other, err := sessions.Create(model.SessionUser{})
	if err != nil {
		t.Fatal(err)
	}
	handler := &apiHandler{store: store, sessions: sessions}
	call := func(fn http.HandlerFunc, method, endpoint, body, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, endpoint, strings.NewReader(body))
		if token != "" {
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		}
		response := httptest.NewRecorder()
		fn(response, request)
		return response
	}
	if response := call(handler.handle115QRCode, "POST", "/", `{}`, ""); response.Code != 401 {
		t.Fatal("QR endpoint not protected")
	}
	if response := call(handler.handle115QRCodeStatus, "POST", "/", `{}`, ""); response.Code != 401 {
		t.Fatal("status endpoint not protected")
	}
	if response := call(handler.handle115QRCode, "POST", "/", `{"device":"windows"}`, session.Token); response.Code != 400 {
		t.Fatal("invalid device accepted")
	}
	if response := call(handler.handle115Devices, "GET", "/", "", ""); response.Code != 401 {
		t.Fatal("device list not protected")
	}
	catalog := call(handler.handle115Devices, "GET", "/api/v1/mounts/115/devices", "", session.Token)
	var catalogResponse struct {
		Data []struct {
			Value string `json:"value"`
			Label string `json:"label"`
		} `json:"data"`
	}
	if err := json.Unmarshal(catalog.Body.Bytes(), &catalogResponse); err != nil {
		t.Fatal(err)
	}
	if catalog.Code != 200 || len(catalogResponse.Data) != len(pan115.Devices) {
		t.Fatalf("device list payload: %s", catalog.Body.String())
	}
	first := catalogResponse.Data[0]
	last := catalogResponse.Data[len(catalogResponse.Data)-1]
	if first.Value != "web" || first.Label != "115生活_网页端" || last.Value != "harmony" || last.Label != "115_鸿蒙端" {
		t.Fatalf("device list order: %s", catalog.Body.String())
	}
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()
	stage := 0
	logins := 0
	http.DefaultTransport = qrTransport(func(request *http.Request) (*http.Response, error) {
		body := `{"state":1,"data":{"uid":"qr-uid","time":123,"sign":"private-sign","qrcode":"https://115.com/scan/example"}}`
		if strings.Contains(request.URL.Path, "/get/status") {
			body = `{"state":1,"data":{"status":` + []string{"0", "1", "2"}[stage] + `}}`
		}
		if request.Method == "POST" {
			logins++
			if !strings.Contains(request.URL.Path, "/ios/") {
				t.Error("selected device not used")
			}
			body = `{"state":1,"data":{"cookie":{"UID":"42_A1","CID":"cid","SEID":"private-cookie","KID":"kid"}}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
	})
	start := call(handler.handle115QRCode, "POST", "/", `{"device":"ios","request_interval_ms":0}`, session.Token)
	if start.Code != 200 || start.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("start: %s", start.Body.String())
	}
	var response struct {
		Data struct {
			ID    string `json:"session_id"`
			Image string `json:"image"`
		} `json:"data"`
	}
	if err := json.Unmarshal(start.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(response.Data.Image, "data:image/png;base64,") || strings.Contains(start.Body.String(), "private-sign") {
		t.Fatal("invalid QR response")
	}
	body := `{"session_id":"` + response.Data.ID + `"}`
	if denied := call(handler.handle115QRCodeStatus, "POST", "/", body, other.Token); denied.Code != 410 {
		t.Fatal("cross-session QR access allowed")
	}
	login := handler.pan115Logins.items[response.Data.ID]
	for stage = 0; stage <= 2; stage++ {
		login.lastPoll = time.Time{}
		login.interval = 0
		poll := call(handler.handle115QRCodeStatus, "POST", "/", body, session.Token)
		if poll.Code != 200 {
			t.Fatalf("poll: %s", poll.Body.String())
		}
		var status struct {
			Data struct {
				Status int    `json:"status"`
				Cookie string `json:"cookie"`
			} `json:"data"`
		}
		if err := json.Unmarshal(poll.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status.Data.Status != stage {
			t.Fatalf("bad QR state: %s", poll.Body.String())
		}
		if stage == 2 && status.Data.Cookie == "" {
			t.Fatal("confirmed login missing CK")
		}
		if stage != 2 && status.Data.Cookie != "" {
			t.Fatal("CK exposed before confirmation")
		}
	}
	call(handler.handle115QRCodeStatus, "POST", "/", body, session.Token)
	if logins != 1 {
		t.Fatalf("login repeated %d times", logins)
	}
	login.expires = time.Now().Add(-time.Second)
	if expired := call(handler.handle115QRCodeStatus, "POST", "/", body, session.Token); expired.Code != 410 {
		t.Fatal("expired QR accepted")
	}
	created := call(handler.handleMounts, "POST", "/api/v1/mounts",
		`{"name":"115","provider":"115","cookie":"UID=42_A1;CID=cid;SEID=private-cookie","device":"ios","request_interval_ms":1200}`, session.Token)
	if created.Code != 200 || strings.Contains(created.Body.String(), "private-cookie") {
		t.Fatalf("create: %s", created.Body.String())
	}
	listed := call(handler.handleMounts, "GET", "/api/v1/mounts", "", session.Token)
	if listed.Code != 200 || strings.Contains(listed.Body.String(), "private-cookie") {
		t.Fatal("list exposed CK")
	}
	detail := call(handler.handleMountByID, "GET", "/api/v1/mounts/1", "", session.Token)
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), "private-cookie") || detail.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("authenticated editing cannot load CK")
	}
	invalid := call(handler.handleMounts, "POST", "/", `{"name":"bad","provider":"115","device":"web","cookie":"invalid"}`, session.Token)
	if invalid.Code != 400 {
		t.Fatal("invalid CK accepted")
	}
	// 新增的设备值（非 SDK 常量）要能过校验并原样落库。
	fresh := call(handler.handleMounts, "POST", "/api/v1/mounts",
		`{"name":"115鸿蒙","provider":"115","cookie":"UID=42_A1;CID=cid;SEID=private-cookie","device":"harmony","request_interval_ms":1000}`, session.Token)
	if fresh.Code != 200 || !strings.Contains(fresh.Body.String(), `"device":"harmony"`) {
		t.Fatalf("new device rejected: %s", fresh.Body.String())
	}
}

// 设备清单由项目自己维护，多数取值不在 SDK 的 driver.LoginApp 常量里
// （常量只有 web / android / ios / tv / alipaymini / wechatmini / qandroid）。
// SDK 把 app 当普通字符串拼进扫码登录 URL，没有白名单，所以这条用例锁住
// 「额外设备值会被原样带到 115」——只要这里绿，扩展清单就不会在代码层被拦。
func Test115QRCodePassesDeviceOutsideSDKConstants(t *testing.T) {
	store, err := sqlite.Open(config.Env{DBPath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	sessions := auth.NewSessionManager(time.Hour)
	session, err := sessions.Create(model.SessionUser{})
	if err != nil {
		t.Fatal(err)
	}
	handler := &apiHandler{store: store, sessions: sessions}
	call := func(fn http.HandlerFunc, method, endpoint, body, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, endpoint, strings.NewReader(body))
		if token != "" {
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		}
		response := httptest.NewRecorder()
		fn(response, request)
		return response
	}
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()

	for _, device := range []string{"harmony", "os_mac", "115ipad", "qios"} {
		var loginPath, loginForm string
		http.DefaultTransport = qrTransport(func(request *http.Request) (*http.Response, error) {
			body := `{"state":1,"data":{"uid":"qr-uid","time":123,"sign":"private-sign","qrcode":"https://115.com/scan/example"}}`
			if strings.Contains(request.URL.Path, "/get/status") {
				body = `{"state":1,"data":{"status":2}}`
			}
			if request.Method == http.MethodPost {
				loginPath = request.URL.Path
				if request.Body != nil {
					raw, _ := io.ReadAll(request.Body)
					loginForm = string(raw)
				}
				body = `{"state":1,"data":{"cookie":{"UID":"42_A1","CID":"cid","SEID":"private-cookie","KID":"kid"}}}`
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
		})
		started := call(handler.handle115QRCode, "POST", "/", `{"device":"`+device+`","request_interval_ms":0}`, session.Token)
		if started.Code != 200 {
			t.Fatalf("%s: start: %s", device, started.Body.String())
		}
		var response struct {
			Data struct {
				ID string `json:"session_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(started.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		login := handler.pan115Logins.items[response.Data.ID]
		if login == nil {
			t.Fatalf("%s: QR session missing", device)
		}
		// 绕开轮询节流，让这一次请求直接走到「已确认」并换取 CK。
		login.lastPoll = time.Time{}
		login.interval = 0
		poll := call(handler.handle115QRCodeStatus, "POST", "/", `{"session_id":"`+response.Data.ID+`"}`, session.Token)
		if poll.Code != 200 || !strings.Contains(poll.Body.String(), `"status":2`) {
			t.Fatalf("%s: poll: %s", device, poll.Body.String())
		}
		if loginPath != "/app/1.0/"+device+"/1.0/login/qrcode" {
			t.Fatalf("%s: login path %q", device, loginPath)
		}
		if loginForm == "" || !strings.Contains(loginForm, "app="+device) {
			t.Fatalf("%s: login form %q", device, loginForm)
		}
	}
}
