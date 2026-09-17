package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 静态资源缓存策略要锁住：
// ① 带内容 hash 的构建产物（/assets/*）长缓存——冷缓存期间浏览器要重新下载全部资源，
//
//	「图还没下完就先按遮罩画出来」会让侧栏图标在左上角闪出一块色块；
//
// ② 入口 html 每次回源——它引用的是带新 hash 的资源，缓存住前端就更新不了；
// ③ 不带 hash 的 public 资源（logo / favicon）给一天。
func TestStaticRoutesCacheControl(t *testing.T) {
	webDir := t.TempDir()

	writeFile := func(relative, content string) {
		full := filepath.Join(webDir, relative)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("创建目录失败：%v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("写入 %s 失败：%v", relative, err)
		}
	}

	writeFile("index.html", "<!doctype html><title>Nestify</title>")
	writeFile(filepath.Join("assets", "app-abc123.js"), "console.log(1)")
	writeFile("nestify-logo.png", "png-bytes")

	mux := http.NewServeMux()
	registerStaticRoutes(mux, webDir)

	cases := []struct {
		name     string
		path     string
		expected string
		body     string
	}{
		{"入口 html 每次回源", "/", "no-cache", "<!doctype html>"},
		{"hash 产物长缓存", "/assets/app-abc123.js", "public, max-age=31536000, immutable", "console.log(1)"},
		{"public 资源缓存一天", "/nestify-logo.png", "public, max-age=86400", "png-bytes"},
		{"未知路径回落到入口 html", "/rules", "no-cache", "<!doctype html>"},
	}

	for _, item := range cases {
		request := httptest.NewRequest(http.MethodGet, item.path, nil)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("%s：状态码期望 200，实际 %d", item.name, recorder.Code)
		}
		if actual := recorder.Header().Get("Cache-Control"); actual != item.expected {
			t.Fatalf("%s：Cache-Control 期望 %q，实际 %q", item.name, item.expected, actual)
		}
		if body := recorder.Body.String(); !strings.Contains(body, item.body) {
			t.Fatalf("%s：响应体期望包含 %q，实际 %q", item.name, item.body, body)
		}
	}
}
