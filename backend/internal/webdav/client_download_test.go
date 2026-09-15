package webdav

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"nestify/backend/internal/model"
)

type capturedDownload struct {
	method string
	path   string
	auth   string
}

func newDownloadServer(t *testing.T, payload string) (*httptest.Server, *[]capturedDownload) {
	t.Helper()
	requests := make([]capturedDownload, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedDownload{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
		})
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

// TestDownloadWritesFileWithTokenAuth 锁定元数据下载：GET 走 DAV 端点、
// 带令牌认证、内容写入目标文件，且临时文件被改名而不是留在原地。
func TestDownloadWritesFileWithTokenAuth(t *testing.T) {
	payload := "poster-bytes"
	server, requests := newDownloadServer(t, payload)

	client := NewClient(model.MountCredential{
		Mount: newTestMount(server, model.MountProviderOpenList, model.MountAuthToken),
		Token: "token-123",
	})
	client.SetRequestInterval(0)

	targetPath := filepath.Join(t.TempDir(), "剧集", "poster.jpg")
	if err := client.Download(context.Background(), "/剧集/poster.jpg", targetPath); err != nil {
		t.Fatalf("Download 出错: %v", err)
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}
	if string(content) != payload {
		t.Fatalf("下载内容 = %q, want %q", string(content), payload)
	}
	if _, err := os.Stat(targetPath + ".download"); !os.IsNotExist(err) {
		t.Fatalf("临时文件应被改名为目标文件，不应残留（err=%v）", err)
	}

	if len(*requests) != 1 {
		t.Fatalf("下载应只发 1 次请求，实际 %d 次", len(*requests))
	}
	if (*requests)[0].method != http.MethodGet {
		t.Fatalf("method = %q, want GET", (*requests)[0].method)
	}
	if got, want := (*requests)[0].auth, "Bearer token-123"; got != want {
		t.Fatalf("Authorization = %q, want %q", got, want)
	}
	// 协议请求仍按挂载的 WebDAV 端点（/dav）拼接，而不是 strm 用的直链端点（/d）。
	if !strings.HasPrefix((*requests)[0].path, "/dav/") {
		t.Fatalf("请求路径 = %q，应指向挂载的 WebDAV 端点", (*requests)[0].path)
	}
}

// TestDownloadReportsAuthFailure 覆盖非令牌认证失败的提示。
// 该分支曾因错误提示函数自递归而在运行时栈溢出，这里作为回归保护。
func TestDownloadReportsAuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client := NewClient(model.MountCredential{
		Mount:    newTestMount(server, model.MountProviderWebdav, model.MountAuthPassword),
		Password: "wrong-password",
	})
	client.SetRequestInterval(0)

	err := client.Download(context.Background(), "/poster.jpg", filepath.Join(t.TempDir(), "poster.jpg"))
	if err == nil {
		t.Fatal("401 应返回错误")
	}
	if !strings.Contains(err.Error(), "用户名与密码") {
		t.Fatalf("密码认证失败应提示核对用户名与密码，实际 %q", err.Error())
	}
}

// TestDownloadFailsOnNonSuccessStatus 确认非 2xx 会报错且不产生目标文件。
func TestDownloadFailsOnNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	t.Cleanup(server.Close)

	client := NewClient(model.MountCredential{
		Mount: newTestMount(server, model.MountProviderOpenList, model.MountAuthPassword),
	})
	client.SetRequestInterval(0)

	targetPath := filepath.Join(t.TempDir(), "poster.jpg")
	err := client.Download(context.Background(), "/poster.jpg", targetPath)
	if err == nil {
		t.Fatal("404 应返回错误")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("错误里应带上状态码，实际 %q", err.Error())
	}
	if _, statErr := os.Stat(targetPath); !os.IsNotExist(statErr) {
		t.Fatalf("失败时不应留下目标文件（err=%v）", statErr)
	}
}
