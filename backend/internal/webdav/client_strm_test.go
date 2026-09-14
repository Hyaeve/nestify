package webdav

import (
	"testing"

	"nestify/backend/internal/model"
)

// TestDirectLinkBasePath 锁定「WebDAV 端点 -> 直链端点」的映射：
// strm 必须写 /d，绝不能出现 /dav。
func TestDirectLinkBasePath(t *testing.T) {
	cases := map[string]string{
		"":                             "/d",
		"/":                            "/d",
		"/dav":                         "/d",
		"/DAV":                         "/d",
		"/dav/":                        "/d",
		"/d":                           "/d",
		"/dav/移动云盘":                    "/d/移动云盘",
		"/xxx/dav":                     "/xxx/d",
		"/d/移动云盘":                      "/d/移动云盘",
		"http://10.0.0.31:5244/dav":    "/d",
		"http://10.0.0.31:5244/dav/xx": "/d/xx",
		"10.0.0.31:5244/dav":           "/d",
		"https://pan.example.com/dav":  "/d",
		"/webdav":                      "/webdav/d",
	}
	for input, want := range cases {
		if got := directLinkBasePath(input); got != want {
			t.Errorf("directLinkBasePath(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestBuildStrmURL 验证最终写入 strm 的完整地址。
func TestBuildStrmURL(t *testing.T) {
	client := NewClient(model.WebdavMount{
		Scheme:   "http",
		Host:     "10.0.0.31",
		Port:     5244,
		BasePath: "/dav",
	}, "")

	got := client.BuildStrmURL("/移动云盘/剧集/a.mkv")
	want := "http://10.0.0.31:5244/d/移动云盘/剧集/a.mkv"
	if got != want {
		t.Fatalf("BuildStrmURL = %q, want %q", got, want)
	}
	if client.StrmBaseURL() != "http://10.0.0.31:5244/d" {
		t.Fatalf("StrmBaseURL = %q", client.StrmBaseURL())
	}
}

// TestBuildStrmURLWithEndpointInHost 覆盖「端点被误填进域名栏」的情况。
func TestBuildStrmURLWithEndpointInHost(t *testing.T) {
	client := NewClient(model.WebdavMount{
		Scheme: "http",
		Host:   "10.0.0.31/dav",
		Port:   0,
	}, "")

	got := client.BuildStrmURL("/a/b.mkv")
	want := "http://10.0.0.31/d/a/b.mkv"
	if got != want {
		t.Fatalf("BuildStrmURL = %q, want %q", got, want)
	}
}
