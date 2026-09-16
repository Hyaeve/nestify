package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// filterMetadataMultistatus 是假 OpenList 在 Depth: infinity 下返回的子树，
// 专门用来锁定「过滤名单要同时拦住 strm 与元数据」：
//
//	/dav/影视/a.mkv       媒体，未被过滤            → 生成 strm
//	/dav/影视/poster.jpg  元数据后缀，但文件名被过滤 → 不下载
//	/dav/影视/movie.nfo   扩展名被过滤              → 不下载
//	/dav/影视/cover.jpg   元数据后缀，未被过滤      → 下载（对照组）
const filterMetadataMultistatus = `<?xml version="1.0" encoding="utf-8"?>
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
    <d:href>/dav/%E5%BD%B1%E8%A7%86/poster.jpg</d:href>
    <d:propstat><d:prop><d:resourcetype/></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/movie.nfo</d:href>
    <d:propstat><d:prop><d:resourcetype/></d:prop></d:propstat>
  </d:response>
  <d:response>
    <d:href>/dav/%E5%BD%B1%E8%A7%86/cover.jpg</d:href>
    <d:propstat><d:prop><d:resourcetype/></d:prop></d:propstat>
  </d:response>
</d:multistatus>`

// newFilterMetadataServer 起假 OpenList：PROPFIND 返回上面那棵树，GET 返回文件字节。
// getCount 用来核对「被过滤的元数据不应产生任何下载请求」。
func newFilterMetadataServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	var mu sync.Mutex
	getCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodGet {
			getCount++
			_, _ = w.Write([]byte("metadata-bytes"))
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(filterMetadataMultistatus))
	}))
	t.Cleanup(server.Close)
	return server, &getCount
}

// TestExecuteStrmRuleFilterListSuppressesMetadata 锁定本地源：
// 过滤名单命中「文件名」或「扩展名」时，该文件既不生成 strm，也不作为元数据复制。
// 文件名过滤按「主干名」匹配（poster 命中 poster.jpg），不用写扩展名。
func TestExecuteStrmRuleFilterListSuppressesMetadata(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeStrmFixture(t, filepath.Join(sourceDir, "影视", "a.mkv"), "video-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "影视", "poster.jpg"), "poster-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "影视", "movie.nfo"), "nfo-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "影视", "cover.jpg"), "cover-bytes")

	service := NewService(nil)
	stats, err := service.executeStrmRule("run-local-filter-metadata", ExecuteRuleRequest{
		Filters:         []string{"mkv"},
		MetadataFilters: []string{"jpg", "nfo"},
		Whitelist:       []string{"poster", ".nfo"},
	}, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("executeStrmRule 出错: %v", err)
	}

	// 媒体照常生成 strm。
	if got := readStrmFixture(t, filepath.Join(targetDir, "影视", "a.strm")); !strings.Contains(got, "a.mkv") {
		t.Fatalf("a.strm 内容 = %q，应指向源文件", got)
	}

	// 被过滤的元数据：实体文件与 strm 都不该出现。
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "poster.jpg"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "poster.strm"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "movie.nfo"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "movie.strm"))

	// 对照：未被过滤的元数据仍按实体文件复制。
	if got := readStrmFixture(t, filepath.Join(targetDir, "影视", "cover.jpg")); got != "cover-bytes" {
		t.Fatalf("cover.jpg 内容 = %q，应原样复制", got)
	}

	if stats.MetadataCount != 1 {
		t.Fatalf("MetadataCount = %d, want 1（只有 cover.jpg）", stats.MetadataCount)
	}
}

// TestWalkWebdavStrmFilterListSuppressesMetadata 锁定逐目录列举（通用 WebDAV 的回退路径）：
// 与递归列举同一套语义，被过滤的文件名 / 扩展名既不出 strm 也不下载元数据。
func TestWalkWebdavStrmFilterListSuppressesMetadata(t *testing.T) {
	server, getCount := newFilterMetadataServer(t)
	client := newFilterClient(server)
	targetDir := t.TempDir()

	strmExtensions, metadataExtensions := splitStrmExtensionSets([]string{"mkv"}, []string{"jpg", "nfo"})
	service := NewService(nil)
	stats := &executionStats{}

	if err := service.walkWebdavStrm(context.Background(), "run-dir-filter-metadata",
		client, "", targetDir, strmExtensions, metadataExtensions,
		buildFileNameMatchers([]string{"poster", ".nfo"}), false, 0, 1, stats); err != nil {
		t.Fatalf("walkWebdavStrm 出错: %v", err)
	}

	if got := readStrmFixture(t, filepath.Join(targetDir, "影视", "a.strm")); !strings.Contains(got, "/d/影视/a.mkv") {
		t.Fatalf("a.strm 内容 = %q，应为直链", got)
	}

	requireStrmMissing(t, filepath.Join(targetDir, "影视", "poster.jpg"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "movie.nfo"))

	if got := readStrmFixture(t, filepath.Join(targetDir, "影视", "cover.jpg")); got != "metadata-bytes" {
		t.Fatalf("cover.jpg 内容 = %q，应下载成功", got)
	}

	if *getCount != 1 {
		t.Fatalf("只应为 cover.jpg 发起 1 次下载，实际 %d 次", *getCount)
	}
	if stats.MetadataCount != 1 {
		t.Fatalf("MetadataCount = %d, want 1", stats.MetadataCount)
	}
}

// TestWalkOpenListStrmRecursiveFilterListSuppressesMetadata 锁定 OpenList 原生递归列举：
// 过滤名单命中文件名 / 扩展名时，远端元数据不应被下载。
func TestWalkOpenListStrmRecursiveFilterListSuppressesMetadata(t *testing.T) {
	server, getCount := newFilterMetadataServer(t)
	client := newFilterClient(server)
	targetDir := t.TempDir()

	strmExtensions, metadataExtensions := splitStrmExtensionSets([]string{"mkv"}, []string{"jpg", "nfo"})
	service := NewService(nil)
	stats := &executionStats{}

	listed, err := service.walkOpenListStrmRecursive(context.Background(), "run-remote-filter-metadata",
		client, "", targetDir, strmExtensions, metadataExtensions,
		buildFileNameMatchers([]string{"poster", ".nfo"}), false, 0, stats)
	if err != nil {
		t.Fatalf("walkOpenListStrmRecursive 出错: %v", err)
	}
	if !listed {
		t.Fatal("递归列举应成功")
	}

	if got := readStrmFixture(t, filepath.Join(targetDir, "影视", "a.strm")); !strings.Contains(got, "/d/影视/a.mkv") {
		t.Fatalf("a.strm 内容 = %q，应为直链", got)
	}

	requireStrmMissing(t, filepath.Join(targetDir, "影视", "poster.jpg"))
	requireStrmMissing(t, filepath.Join(targetDir, "影视", "movie.nfo"))

	if got := readStrmFixture(t, filepath.Join(targetDir, "影视", "cover.jpg")); got != "metadata-bytes" {
		t.Fatalf("cover.jpg 内容 = %q，应下载成功", got)
	}

	if *getCount != 1 {
		t.Fatalf("只应为 cover.jpg 发起 1 次下载，实际 %d 次", *getCount)
	}
	if stats.MetadataCount != 1 {
		t.Fatalf("MetadataCount = %d, want 1", stats.MetadataCount)
	}
}
