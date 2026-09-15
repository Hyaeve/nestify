package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"nestify/backend/internal/model"
	"nestify/backend/internal/webdav"
)

// filterMultistatus 是假 OpenList 在 Depth: infinity 下返回的整棵子树：
//
//	/dav/                        （根）
//	/dav/影视/                    （目录）
//	/dav/影视/a.mkv               （媒体）
//	/dav/影视/字幕/               （目录，将被过滤名单命中）
//	/dav/影视/字幕/b.srt          （元数据）
//	/dav/影视/字幕/b.mkv          （媒体）
const filterMultistatus = `<?xml version="1.0" encoding="utf-8"?>
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
    <d:propstat><d:prop><d:resourcetype/><d:getcontentlength>100</d:getcontentlength></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/%E5%AD%97%E5%B9%95/</d:href>
    <d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/%E5%AD%97%E5%B9%95/b.srt</d:href>
    <d:propstat><d:prop><d:resourcetype/></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/%E5%AD%97%E5%B9%95/b.mkv</d:href>
    <d:propstat><d:prop><d:resourcetype/></d:prop></d:propstat>
  </d:response>
</d:multistatus>`

// newStrmFilterServer 起一个假 OpenList：PROPFIND 返回整棵子树，GET 返回文件字节。
// 返回值里的 getCount 用来核对「被过滤的目录之下不应再产生任何下载请求」。
func newStrmFilterServer(t *testing.T) (*httptest.Server, *int, *int) {
	t.Helper()
	var mu sync.Mutex
	propfindCount, getCount := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			getCount++
			_, _ = w.Write([]byte("metadata-bytes"))
		default:
			propfindCount++
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = w.Write([]byte(filterMultistatus))
		}
	}))
	t.Cleanup(server.Close)
	return server, &propfindCount, &getCount
}

func newFilterClient(server *httptest.Server) *webdav.Client {
	client := webdav.NewClient(model.MountCredential{
		Mount: model.WebdavMount{
			Provider: model.MountProviderOpenList,
			AuthType: model.MountAuthPassword,
			Scheme:   "http",
			Host:     strings.TrimPrefix(server.URL, "http://"),
			BasePath: "/dav",
		},
	})
	client.SetRequestInterval(0)
	return client
}

// TestIsUnderFilteredDir 锁定祖先目录判定：过滤名单命中目录时，
// 递归列举必须能识别出「位于该目录之下」的条目（逐目录列举是整棵跳过的）。
func TestIsUnderFilteredDir(t *testing.T) {
	matchers := buildFileNameMatchers([]string{"/字幕"})

	cases := []struct {
		path string
		want bool
	}{
		{"/影视/字幕/b.srt", true},
		{"/影视/字幕/深层/b.mkv", true},
		{"/字幕/b.srt", true},
		{"/影视/a.mkv", false},
		{"/影视/字幕组/a.mkv", false}, // 前缀不同名，不能被误伤
		{"/影视", false},           // 祖先里没有命中目录
		{"/a.mkv", false},
		{"/字幕", false}, // 条目自身由 isDir 语义单独判断，这里不看末段
	}
	for _, item := range cases {
		if got := isUnderFilteredDir(item.path, matchers); got != item.want {
			t.Fatalf("isUnderFilteredDir(%q) = %v, want %v", item.path, got, item.want)
		}
	}

	if isUnderFilteredDir("/影视/字幕/b.srt", nil) {
		t.Fatal("没有过滤名单时不应剪枝")
	}
}

// TestWalkOpenListStrmRecursiveHonoursFilterList 锁定 OpenList 原生递归列举下的
// 过滤名单语义：命中过滤名单的目录必须整棵跳过（不生成 strm、不下载元数据），
// 与逐目录列举保持一致。
func TestWalkOpenListStrmRecursiveHonoursFilterList(t *testing.T) {
	server, propfindCount, getCount := newStrmFilterServer(t)
	client := newFilterClient(server)
	targetDir := t.TempDir()

	strmExtensions, metadataExtensions := splitStrmExtensionSets([]string{"mkv"}, []string{"srt"})
	service := NewService(nil)
	stats := &executionStats{}

	listed, err := service.walkOpenListStrmRecursive(context.Background(), "run-filter",
		client, "", targetDir, strmExtensions, metadataExtensions,
		buildFileNameMatchers([]string{"/字幕"}), false, 0, stats)
	if err != nil {
		t.Fatalf("walkOpenListStrmRecursive 出错: %v", err)
	}
	if !listed {
		t.Fatal("递归列举应成功")
	}
	if *propfindCount != 1 {
		t.Fatalf("应只发 1 次 PROPFIND，实际 %d 次", *propfindCount)
	}

	// 未命中过滤的媒体照常生成 strm。
	if got := readStrmFixture(t, filepath.Join(targetDir, "影视", "a.strm")); !strings.Contains(got, "/d/影视/a.mkv") {
		t.Fatalf("a.strm 内容 = %q，应为直链", got)
	}

	// 被过滤目录之下的内容一律不落地，也不应产生任何下载请求。
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "字幕", "b.mkv"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "字幕", "b.strm"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "字幕", "b.srt"))
	if *getCount != 0 {
		t.Fatalf("被过滤目录之下不应发起下载，实际 %d 次 GET", *getCount)
	}

	if stats.MetadataCount != 0 {
		t.Fatalf("MetadataCount = %d, want 0", stats.MetadataCount)
	}
	if stats.SuccessCount != 1 {
		t.Fatalf("SuccessCount = %d, want 1（只有 a.mkv）", stats.SuccessCount)
	}
}

// TestWalkOpenListStrmRecursiveWithoutFilterList 是对照组：不配过滤名单时，
// 「字幕」目录内的 strm 与元数据都会照常落地，说明上一条用例屏蔽的是过滤名单本身。
func TestWalkOpenListStrmRecursiveWithoutFilterList(t *testing.T) {
	server, _, getCount := newStrmFilterServer(t)
	client := newFilterClient(server)
	targetDir := t.TempDir()

	strmExtensions, metadataExtensions := splitStrmExtensionSets([]string{"mkv"}, []string{"srt"})
	service := NewService(nil)
	stats := &executionStats{}

	if _, err := service.walkOpenListStrmRecursive(context.Background(), "run-nofilter",
		client, "", targetDir, strmExtensions, metadataExtensions, nil, false, 0, stats); err != nil {
		t.Fatalf("walkOpenListStrmRecursive 出错: %v", err)
	}

	if got := readStrmFixture(t, filepath.Join(targetDir, "影视", "字幕", "b.srt")); got != "metadata-bytes" {
		t.Fatalf("b.srt 内容 = %q，元数据应下载成实体文件", got)
	}
	if *getCount != 1 {
		t.Fatalf("应有 1 次元数据下载，实际 %d 次", *getCount)
	}
	if stats.MetadataCount != 1 {
		t.Fatalf("MetadataCount = %d, want 1", stats.MetadataCount)
	}
}

// TestExecuteStrmRuleHonoursFilterListForLocalSource 锁定本地源的同一语义：
// 过滤名单命中目录时整棵跳过（strm 与元数据都不落地）。
func TestExecuteStrmRuleHonoursFilterListForLocalSource(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeStrmFixture(t, filepath.Join(sourceDir, "影视", "a.mkv"), "video-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "影视", "字幕", "b.mkv"), "video-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "影视", "字幕", "b.srt"), "subtitle-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "影视", "预告", "trailer.mkv"), "video-bytes")

	service := NewService(nil)
	stats, err := service.executeStrmRule("run-local-filter", ExecuteRuleRequest{
		Filters:         []string{"mkv"},
		MetadataFilters: []string{"srt"},
		Whitelist:       []string{"/字幕"},
	}, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("executeStrmRule 出错: %v", err)
	}

	readStrmFixture(t, filepath.Join(targetDir, "影视", "a.strm"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "字幕", "b.strm"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "字幕", "b.srt"))
	// 过滤名单只作用于「字幕」，同级的「预告」不受影响。
	readStrmFixture(t, filepath.Join(targetDir, "影视", "预告", "trailer.strm"))

	if stats.SuccessCount != 2 {
		t.Fatalf("SuccessCount = %d, want 2（a.mkv + trailer.mkv）", stats.SuccessCount)
	}
}
