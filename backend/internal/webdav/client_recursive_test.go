package webdav

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nestify/backend/internal/model"
)

// recursiveMultistatus 模拟 OpenList 在 Depth: infinity 下返回的整棵子树。
// 其中「字幕」目录刻意省略 resourcetype/collection，只靠 href 尾斜杠表达目录身份，
// 用来锁定「目录判定必须有尾斜杠回退」这条规则。
const recursiveMultistatus = `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/dav/</d:href>
    <d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/</d:href>
    <d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/a.mkv</d:href>
    <d:propstat><d:prop><d:resourcetype/><d:getcontentlength>1234</d:getcontentlength></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/%E5%AD%97%E5%B9%95/</d:href>
    <d:propstat><d:prop><d:resourcetype/></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/%E5%AD%97%E5%B9%95/b.srt</d:href>
    <d:propstat><d:prop><d:resourcetype/></d:prop></d:propstat>
  </d:response>
</d:multistatus>`

type capturedRequest struct {
	method string
	depth  string
	auth   string
	body   string
}

// newPropfindServer 起一个只认 PROPFIND 的假 OpenList，并把收到的请求记录下来。
func newPropfindServer(t *testing.T, body string) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	requests := make([]capturedRequest, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)
		requests = append(requests, capturedRequest{
			method: r.Method,
			depth:  r.Header.Get("Depth"),
			auth:   r.Header.Get("Authorization"),
			body:   string(payload),
		})
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func newTestMount(server *httptest.Server, provider, authType string) model.WebdavMount {
	host := strings.TrimPrefix(server.URL, "http://")
	return model.WebdavMount{
		Provider: provider,
		AuthType: authType,
		Scheme:   "http",
		Host:     host,
		BasePath: "/dav",
	}
}

// TestListRecursiveSingleRequest 锁定 OpenList 原生递归列举的关键承诺：
// 整棵子树只发一次 PROPFIND，且 Depth 必须是 infinity。
func TestListRecursiveSingleRequest(t *testing.T) {
	server, requests := newPropfindServer(t, recursiveMultistatus)
	client := NewClient(model.MountCredential{Mount: newTestMount(server, model.MountProviderOpenList, model.MountAuthPassword)})
	client.SetRequestInterval(0)

	entries, err := client.ListRecursive(context.Background(), "", true)
	if err != nil {
		t.Fatalf("ListRecursive 出错: %v", err)
	}

	if len(*requests) != 1 {
		t.Fatalf("递归列举应只发 1 次请求，实际 %d 次", len(*requests))
	}
	if (*requests)[0].depth != depthInfinity {
		t.Fatalf("Depth = %q, want %q", (*requests)[0].depth, depthInfinity)
	}
	if (*requests)[0].method != "PROPFIND" {
		t.Fatalf("method = %q", (*requests)[0].method)
	}
	if len(entries) != 4 {
		t.Fatalf("条目数 = %d, want 4（%+v）", len(entries), entries)
	}

	byPath := map[string]Entry{}
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}

	dir, ok := byPath["/影视"]
	if !ok || !dir.IsDir {
		t.Fatalf("「影视」应是目录：%+v", byPath)
	}
	file, ok := byPath["/影视/a.mkv"]
	if !ok || file.IsDir || file.Size != 1234 {
		t.Fatalf("a.mkv 应是 1234 字节的文件：%+v", file)
	}
	// 该目录的响应省略了 collection，只能靠 href 尾斜杠识别。
	sub, ok := byPath["/影视/字幕"]
	if !ok || !sub.IsDir {
		t.Fatalf("「字幕」应靠尾斜杠被识别为目录：%+v", byPath)
	}
	if srt, ok := byPath["/影视/字幕/b.srt"]; !ok || srt.IsDir {
		t.Fatalf("b.srt 应是文件：%+v", srt)
	}
}

// TestListRecursiveSkipsSizeProp 验证不需要文件大小时不索取 getcontentlength。
func TestListRecursiveSkipsSizeProp(t *testing.T) {
	server, requests := newPropfindServer(t, recursiveMultistatus)
	client := NewClient(model.MountCredential{Mount: newTestMount(server, model.MountProviderOpenList, model.MountAuthPassword)})
	client.SetRequestInterval(0)

	if _, err := client.ListRecursive(context.Background(), "", false); err != nil {
		t.Fatalf("ListRecursive 出错: %v", err)
	}
	if strings.Contains((*requests)[0].body, "getcontentlength") {
		t.Fatalf("withSize=false 时不应索取 getcontentlength：%s", (*requests)[0].body)
	}
	if !strings.Contains((*requests)[0].body, "getlastmodified") {
		t.Fatalf("仍应保留 getlastmodified：%s", (*requests)[0].body)
	}
}

// TestTokenAuthHeader 锁定令牌认证使用 OpenList 的 `Bearer <永久令牌>` 形式。
func TestTokenAuthHeader(t *testing.T) {
	server, requests := newPropfindServer(t, recursiveMultistatus)
	client := NewClient(model.MountCredential{
		Mount: newTestMount(server, model.MountProviderOpenList, model.MountAuthToken),
		Token: "  my-permanent-token  ",
	})
	client.SetRequestInterval(0)

	if _, err := client.List(context.Background(), ""); err != nil {
		t.Fatalf("List 出错: %v", err)
	}
	if got, want := (*requests)[0].auth, "Bearer my-permanent-token"; got != want {
		t.Fatalf("Authorization = %q, want %q", got, want)
	}
	if !client.SupportsRecursiveList() {
		t.Fatal("openlist 挂载应支持递归列举")
	}
}

// TestPasswordAuthKeepsBasic 确认密码模式没有被令牌改动影响。
func TestPasswordAuthKeepsBasic(t *testing.T) {
	server, requests := newPropfindServer(t, recursiveMultistatus)
	client := NewClient(model.MountCredential{
		Mount:    newTestMount(server, model.MountProviderWebdav, model.MountAuthPassword),
		Password: "secret",
	})
	client.username = "alice"
	client.SetRequestInterval(0)

	if _, err := client.List(context.Background(), ""); err != nil {
		t.Fatalf("List 出错: %v", err)
	}
	if !strings.HasPrefix((*requests)[0].auth, "Basic ") {
		t.Fatalf("密码模式应走 Basic 认证，实际 %q", (*requests)[0].auth)
	}
	if client.SupportsRecursiveList() {
		t.Fatal("通用 webdav 挂载不应启用递归列举")
	}
}

// TestListFiltersDirectChildren 确认 Depth: 1 的结果仍只保留直接子项。
func TestListFiltersDirectChildren(t *testing.T) {
	// 同一个响应体（含深层子项）走 Depth: 1 路径，应被过滤掉孙子层。
	server, _ := newPropfindServer(t, recursiveMultistatus)
	client := NewClient(model.MountCredential{Mount: newTestMount(server, model.MountProviderWebdav, model.MountAuthPassword)})
	client.SetRequestInterval(0)

	entries, err := client.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List 出错: %v", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	if len(entries) != 1 || paths[0] != "/影视" {
		t.Fatalf("根目录的直接子项应只有 /影视，实际 %v", paths)
	}
}
