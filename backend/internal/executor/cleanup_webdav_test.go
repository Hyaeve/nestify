package executor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"nestify/backend/internal/model"
	"nestify/backend/internal/store/sqlite"
)

// 远程挂载（WebDAV）上的净化：这一组用例把「净化规则监控目录 = webdav://N/... 」
// 这条以前必然整轮失败的链路钉住。
//
// 假服务端刻意贴着真实网盘的行为走，用来锁三件事：
//  ① 目录级请求必须带尾斜杠 —— 不带就回 301，而 Go 的 http.Client 会把 301 上的
//     DELETE 降级成 GET（表面 200，实际什么都没删）；
//  ② 非空集合的 DELETE 一律拒绝 —— 各家服务端「删除目录是否递归」实现不一，
//     净化必须先自己把内容清空再删目录；
//  ③ getlastmodified 参与「过期文件」判定。

type fakeDavNode struct {
	isDir    bool
	size     int64
	modified time.Time
	children map[string]*fakeDavNode
}

type fakeDav struct {
	mu       sync.Mutex
	basePath string
	user     string
	password string
	root     *fakeDavNode
	requests []string
	// denyDelete 让服务端对所有 DELETE 回 403，用来验证「删不掉要记失败、不能报成功」。
	denyDelete bool
	// omitModified 让响应里不带 getlastmodified，模拟不返回该属性的服务端：
	// 时间拿不到时「过期文件」必须一条都不删（宁可留着，也不能按 1970 年比）。
	omitModified bool
}

func newFakeDav(basePath, user, password string) *fakeDav {
	return &fakeDav{
		basePath: basePath,
		user:     user,
		password: password,
		root:     &fakeDavNode{isDir: true, children: map[string]*fakeDavNode{}},
	}
}

func fakeSegments(internal string) []string {
	trimmed := strings.Trim(strings.TrimSpace(internal), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func (d *fakeDav) childDir(parent *fakeDavNode, name string) *fakeDavNode {
	if existing, ok := parent.children[name]; ok {
		return existing
	}
	created := &fakeDavNode{isDir: true, modified: time.Now(), children: map[string]*fakeDavNode{}}
	parent.children[name] = created
	return created
}

// addFile 在挂载内部路径上放一个文件（自动补出父目录）。
func (d *fakeDav) addFile(internal string, size int64, modified time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	segments := fakeSegments(internal)
	parent := d.root
	for _, segment := range segments[:len(segments)-1] {
		parent = d.childDir(parent, segment)
	}
	parent.children[segments[len(segments)-1]] = &fakeDavNode{size: size, modified: modified}
}

// addDir 放一个空目录。
func (d *fakeDav) addDir(internal string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	segments := fakeSegments(internal)
	parent := d.root
	for _, segment := range segments {
		parent = d.childDir(parent, segment)
	}
}

// snapshot 返回当前还活着的全部路径（目录带尾斜杠），排序后用于断言。
func (d *fakeDav) snapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	paths := make([]string, 0)
	var walk func(node *fakeDavNode, prefix string)
	walk = func(node *fakeDavNode, prefix string) {
		for _, name := range sortedFakeNames(node) {
			child := node.children[name]
			current := prefix + "/" + name
			if child.isDir {
				paths = append(paths, current+"/")
				walk(child, current)
				continue
			}
			paths = append(paths, current)
		}
	}
	walk(d.root, "")
	sort.Strings(paths)
	return paths
}

func (d *fakeDav) requestLog() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.requests...)
}

func sortedFakeNames(node *fakeDavNode) []string {
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (d *fakeDav) internalPath(urlPath string) string {
	clean := strings.TrimSuffix(urlPath, "/")
	if d.basePath != "" && strings.HasPrefix(clean, d.basePath) {
		clean = strings.TrimPrefix(clean, d.basePath)
	}
	return clean
}

func (d *fakeDav) lookup(internal string) (*fakeDavNode, bool) {
	if internal == "" {
		return d.root, true
	}
	node := d.root
	for _, segment := range fakeSegments(internal) {
		child, ok := node.children[segment]
		if !ok {
			return nil, false
		}
		node = child
	}
	return node, true
}

func (d *fakeDav) remove(internal string) bool {
	if internal == "" {
		return false
	}
	segments := fakeSegments(internal)
	parent := d.root
	for _, segment := range segments[:len(segments)-1] {
		child, ok := parent.children[segment]
		if !ok {
			return false
		}
		parent = child
	}
	name := segments[len(segments)-1]
	if _, ok := parent.children[name]; !ok {
		return false
	}
	delete(parent.children, name)
	return true
}

func (d *fakeDav) href(internal string, isDir bool) string {
	value := d.basePath + internal
	if value == "" {
		value = "/"
	}
	if isDir && !strings.HasSuffix(value, "/") {
		value += "/"
	}
	return (&url.URL{Path: value}).EscapedPath()
}

func (d *fakeDav) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, password, ok := r.BasicAuth()
	if !ok || user != d.user || password != d.password {
		w.Header().Set("WWW-Authenticate", `Basic realm="nestify-test"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.requests = append(d.requests, fmt.Sprintf("%s %s depth=%s", r.Method, r.URL.Path, r.Header.Get("Depth")))

	internal := d.internalPath(r.URL.Path)
	node, found := d.lookup(internal)

	switch r.Method {
	case "PROPFIND":
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// 目录不带尾斜杠 → 301 重定向，与 OpenList / Alist 一致。
		if node.isDir && !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}
		d.writePropfind(w, r, internal, node)
	case "DELETE":
		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if d.denyDelete {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if node.isDir {
			if !strings.HasSuffix(r.URL.Path, "/") {
				http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
				return
			}
			// 非空集合拒绝删除：服务端是否递归删除集合各家实现不同，
			// 这里取最保守的一种，逼调用方先把内容清干净。
			if len(node.children) > 0 {
				w.WriteHeader(http.StatusConflict)
				return
			}
		}
		d.remove(internal)
		w.WriteHeader(http.StatusNoContent)
	case "GET":
		// 301 / 302 上的 DELETE 会被 Go 的 http.Client 降级成 GET，所以这里刻意回 200
		// （而不是 405）：调用方一旦漏掉集合的尾斜杠，删除就会「看起来成功、实际什么都没删」——
		// 正是真实网盘上那个坑。用例靠事后比对存活路径来抓它，而不是靠错误码。
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// fakePropfindItem 是假服务端要写进 multistatus 的一条资源。
type fakePropfindItem struct {
	internal string
	node     *fakeDavNode
}

func (d *fakeDav) writePropfind(w http.ResponseWriter, r *http.Request, internal string, node *fakeDavNode) {
	depth := strings.TrimSpace(r.Header.Get("Depth"))

	items := []fakePropfindItem{{internal: internal, node: node}}

	if depth != "0" {
		for _, name := range sortedFakeNames(node) {
			child := node.children[name]
			childPath := internal + "/" + name
			items = append(items, fakePropfindItem{internal: childPath, node: child})
			if depth == "infinity" {
				collectFakeSubtree(&items, childPath, child)
			}
		}
	}

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	builder.WriteString("\n<d:multistatus xmlns:d=\"DAV:\">")
	for _, item := range items {
		builder.WriteString("<d:response><d:href>")
		builder.WriteString(d.href(item.internal, item.node.isDir))
		builder.WriteString("</d:href><d:propstat><d:prop><d:resourcetype>")
		if item.node.isDir {
			builder.WriteString("<d:collection/>")
		}
		builder.WriteString("</d:resourcetype>")
		if !item.node.isDir {
			builder.WriteString(fmt.Sprintf("<d:getcontentlength>%d</d:getcontentlength>", item.node.size))
		}
		if !d.omitModified {
			modified := item.node.modified
			if modified.IsZero() {
				modified = time.Now()
			}
			builder.WriteString("<d:getlastmodified>" + modified.UTC().Format(http.TimeFormat) + "</d:getlastmodified>")
		}
		builder.WriteString("</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>")
	}
	builder.WriteString("</d:multistatus>")

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write([]byte(builder.String()))
}

func collectFakeSubtree(items *[]fakePropfindItem, dir string, node *fakeDavNode) {
	for _, name := range sortedFakeNames(node) {
		child := node.children[name]
		childPath := dir + "/" + name
		*items = append(*items, fakePropfindItem{internal: childPath, node: child})
		if child.isDir {
			collectFakeSubtree(items, childPath, child)
		}
	}
}

// startFakeDavMount 起一个假 WebDAV 服务，并把它登记成一条启用中的挂载。
func startFakeDavMount(t *testing.T, store *sqlite.Store, provider string) (*fakeDav, *model.WebdavMount) {
	t.Helper()

	const user = "nestify"
	const password = "secret"
	dav := newFakeDav("/dav", user, password)
	server := httptest.NewServer(dav)
	t.Cleanup(server.Close)

	host, port := splitHostPort(t, server.URL)
	mount, err := store.CreateMount(model.CreateMountInput{
		Name:     "测试挂载",
		Provider: provider,
		AuthType: model.MountAuthPassword,
		Scheme:   "http",
		Host:     host,
		Port:     port,
		Username: user,
		Password: password,
		BasePath: "/dav",
	})
	if err != nil {
		t.Fatalf("创建测试挂载失败: %v", err)
	}
	return dav, mount
}

// splitHostPort 从 httptest 的 URL（http://127.0.0.1:PORT）里取出挂载要填的 host 与 port。
func splitHostPort(t *testing.T, rawURL string) (string, int) {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("解析测试服务地址失败: %v", err)
	}
	port := 0
	if _, err := fmt.Sscanf(parsed.Port(), "%d", &port); err != nil {
		t.Fatalf("解析测试服务端口失败: %v", err)
	}
	return parsed.Hostname(), port
}

// remoteCleanupRequest 造一个「监控目录在挂载里」的净化请求。
// strm_api_interval_ms=100 只是把请求间隔压到允许的最小值，免得用例被节流拖慢。
func remoteCleanupRequest(mount *model.WebdavMount, internalDir string, options map[string]bool, filters, whitelist []string, retentionDays int) ExecuteRuleRequest {
	optionValues := map[string]int{"strm_api_interval_ms": 100}
	if retentionDays > 0 {
		optionValues["cleanup_retention_days"] = retentionDays
	}
	return ExecuteRuleRequest{
		ArchiveMode:  "cleanup",
		SourceDir:    model.JoinMountVirtualPath(mount.ID, internalDir),
		Options:      options,
		OptionValues: optionValues,
		Filters:      filters,
		Whitelist:    whitelist,
	}
}

// ---------------------------------------------------------------------------
// 断言辅助
// ---------------------------------------------------------------------------

// executeRemoteCleanup 跑一次「监控目录在挂载里」的净化。
// 直接调 executeCleanupRule（与本地那几条用例同一入口），确认它认得出虚拟路径并分流到 WebDAV 实现。
func executeRemoteCleanup(t *testing.T, store *sqlite.Store, runID string, req ExecuteRuleRequest) (executionStats, error) {
	t.Helper()
	return NewService(store).executeCleanupRule(runID, req)
}

// assertFakeDavPaths 比对服务端上「还剩什么」：删没删掉、有没有误删，全看这一条。
// 只看接口返回的状态码是不够的 —— 漏尾斜杠的 DELETE 会被 301 降级成 GET，
// 表面成功、实际什么都没删（假服务端刻意回 200 复现这一点）。
func assertFakeDavPaths(t *testing.T, dav *fakeDav, want []string) {
	t.Helper()

	got := dav.snapshot()
	expected := append([]string(nil), want...)
	sort.Strings(expected)
	if strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Fatalf("挂载上剩余内容 = %v, want %v", got, expected)
	}
}

// remoteCleanupDetail 取本次执行的明细载荷（详情窗口就是靠它渲染「删除」那一项）。
func remoteCleanupDetail(t *testing.T, stats executionStats) model.RunDetail {
	t.Helper()

	payload := stats.buildDetailJSON()
	if strings.TrimSpace(payload) == "" {
		t.Fatalf("远程净化的明细载荷为空：详情窗口不会出现任何明细（summary=%q）", stats.Summary)
	}
	var detail model.RunDetail
	if err := json.Unmarshal([]byte(payload), &detail); err != nil {
		t.Fatalf("明细载荷不是合法 JSON: %v", err)
	}
	return detail
}

// fakeDavRequests 取某一方法的请求流水（"DELETE /dav/影视/空目录/ depth="）。
func fakeDavRequests(dav *fakeDav, method string) []string {
	matched := make([]string, 0)
	for _, line := range dav.requestLog() {
		if strings.HasPrefix(line, method+" ") {
			matched = append(matched, line)
		}
	}
	return matched
}

// deleteEntryByPath 从删除明细里按路径取一条。
func deleteEntryByPath(entries []model.RunFileEntry, path string) (model.RunFileEntry, bool) {
	for _, entry := range entries {
		if entry.Path == path {
			return entry, true
		}
	}
	return model.RunFileEntry{}, false
}

// ---------------------------------------------------------------------------
// 用例
// ---------------------------------------------------------------------------

// TestWebdavCleanupRemovesMatchedFilesAndEmptyDirs 远程净化的基本盘：
// 命中清理名单的文件删掉、子项被删空的目录删掉、没命中的原样留着、监控目录自身永远不删。
func TestWebdavCleanupRemovesMatchedFilesAndEmptyDirs(t *testing.T) {
	store := openRunHistoryTestStore(t)
	dav, mount := startFakeDavMount(t, store, model.MountProviderWebdav)

	now := time.Now()
	dav.addFile("/影视/广告.txt", 12, now)
	dav.addFile("/影视/影片.mkv", 2048, now)
	dav.addFile("/影视/剧集/第 01 集.mkv", 1024, now)
	dav.addDir("/影视/空目录")
	dav.addDir("/影视/白名单空目录")

	stats, err := executeRemoteCleanup(t, store, "run-webdav-basic", remoteCleanupRequest(mount, "/影视",
		map[string]bool{"cleanup_matching_files": true, "cleanup_empty_dirs": true},
		[]string{"*广告"}, []string{"白名单空目录"}, 0))
	if err != nil {
		t.Fatalf("远程净化出错（这就是以前那条「系统找不到指定的路径」的整轮失败）：%v", err)
	}

	// 空目录删掉、命中的文件删掉；白名单目录与非空目录（含其中文件）留着，
	// 监控目录自身也不在删除范围内。
	assertFakeDavPaths(t, dav, []string{
		"/影视/",
		"/影视/影片.mkv",
		"/影视/剧集/",
		"/影视/剧集/第 01 集.mkv",
		"/影视/白名单空目录/",
	})

	if stats.CleanupRemovedFiles != 1 || stats.CleanupRemovedDirs != 1 {
		t.Fatalf("删除统计 = %d 文件 / %d 目录, want 1 / 1", stats.CleanupRemovedFiles, stats.CleanupRemovedDirs)
	}
	if stats.SuccessCount != 2 || stats.FailureCount != 0 || stats.SkipCount != 1 {
		t.Fatalf("成功/失败/跳过 = %d/%d/%d, want 2/0/1（白名单空目录要算一次跳过）", stats.SuccessCount, stats.FailureCount, stats.SkipCount)
	}
	if stats.SizeBytes != 12 {
		t.Fatalf("累计大小 = %d, want 12（只有被删的广告.txt 计入）", stats.SizeBytes)
	}

	// 集合 DELETE 必须带尾斜杠：漏了会被 301 降级成 GET，表面成功实际没删。
	for _, line := range fakeDavRequests(dav, "DELETE") {
		if strings.Contains(line, "/影视/空目录") && !strings.Contains(line, "/影视/空目录/") {
			t.Fatalf("集合 DELETE 没有尾斜杠（会被 301 降级成 GET，什么都删不掉）：%s", line)
		}
	}

	detail := remoteCleanupDetail(t, stats)
	if detail.Kind != model.RunDetailKindCleanup {
		t.Fatalf("明细类型 = %q, want %q", detail.Kind, model.RunDetailKindCleanup)
	}
	if got := detail.Counts[model.BackupFileActionDelete]; got != 2 {
		t.Fatalf("counts.delete = %d, want 2（命中文件 + 空目录）", got)
	}
	if len(detail.SourceRoots) != 1 || detail.SourceRoots[0] != "/影视" {
		t.Fatalf("source_roots 应为监控目录的挂载内路径，实际 %+v", detail.SourceRoots)
	}

	deletes := detailEntriesByAction(detail, model.BackupFileActionDelete)
	fileEntry, ok := deleteEntryByPath(deletes, "/影视/广告.txt")
	if !ok || fileEntry.Dir || fileEntry.Note != deleteNoteMatchedFile {
		t.Fatalf("命中文件的删除明细不对: %+v", fileEntry)
	}
	dirEntry, ok := deleteEntryByPath(deletes, "/影视/空目录")
	if !ok || !dirEntry.Dir || dirEntry.Note != deleteNoteEmptyDir {
		t.Fatalf("空目录的删除明细不对: %+v", dirEntry)
	}

	// 白名单保护的目录：它确实是空的，本该被「清理空目录」删掉，是白名单把它留下了 ——
	// 这要落一条跳过明细，否则运行记录里写着「跳过 1」，任务详情窗口点开什么都没有。
	skips := detailEntriesByAction(detail, model.BackupFileActionSkip)
	if len(skips) != 1 || skips[0].Path != "/影视/白名单空目录" || !skips[0].Dir || skips[0].Note != skipReasonCleanupWhitelist {
		t.Fatalf("白名单目录的跳过明细不对: %+v", skips)
	}
	if got := detail.Counts[model.BackupFileActionSkip]; got != 1 {
		t.Fatalf("counts.skip = %d, want 1", got)
	}
	if !strings.Contains(stats.Summary, "跳过 1 项") {
		t.Fatalf("摘要要带出跳过数，实际 %q", stats.Summary)
	}
}

// TestWebdavCleanupPurgesMatchedDirectoryRecursively 命中清理名单的目录要「连内容一起删」。
//
// 不能直接 DELETE 集合就完事：各家服务端的 DELETE 打在非空集合上行为不一
// （假服务端就按最保守的一种回 409），净化必须自己先把内容清干净再删目录。
func TestWebdavCleanupPurgesMatchedDirectoryRecursively(t *testing.T) {
	store := openRunHistoryTestStore(t)
	dav, mount := startFakeDavMount(t, store, model.MountProviderWebdav)

	now := time.Now()
	dav.addFile("/影视/剧集/第 01 集.mkv", 1000, now)
	dav.addFile("/影视/剧集/子目录/第 02 集.mkv", 2000, now)
	dav.addDir("/影视/剧集/空子目录")
	dav.addFile("/影视/保留.mkv", 10, now)

	stats, err := executeRemoteCleanup(t, store, "run-webdav-purge", remoteCleanupRequest(mount, "/影视",
		map[string]bool{"cleanup_matching_files": true},
		[]string{"/剧集"}, nil, 0))
	if err != nil {
		t.Fatalf("远程净化出错: %v", err)
	}

	assertFakeDavPaths(t, dav, []string{"/影视/", "/影视/保留.mkv"})

	// 口径与本地一致：整棵目录算「1 个目录」，里面的文件不单独计成功（本地走 os.RemoveAll）。
	if stats.CleanupRemovedDirs != 1 || stats.CleanupRemovedFiles != 0 {
		t.Fatalf("删除统计 = %d 文件 / %d 目录, want 0 / 1", stats.CleanupRemovedFiles, stats.CleanupRemovedDirs)
	}
	if stats.SuccessCount != 1 || stats.FailureCount != 0 {
		t.Fatalf("成功/失败 = %d/%d, want 1/0", stats.SuccessCount, stats.FailureCount)
	}
	if stats.SizeBytes != 3000 {
		t.Fatalf("累计大小 = %d, want 3000（目录内容的总大小）", stats.SizeBytes)
	}

	detail := remoteCleanupDetail(t, stats)
	deletes := detailEntriesByAction(detail, model.BackupFileActionDelete)
	if len(deletes) != 1 {
		t.Fatalf("删除明细条数 = %d, want 1（整目录一条）", len(deletes))
	}
	if deletes[0].Path != "/影视/剧集" || !deletes[0].Dir || deletes[0].Note != deleteNoteMatchedDir {
		t.Fatalf("命中目录的删除明细不对: %+v", deletes[0])
	}
}

// TestWebdavCleanupRemovesExpiredFiles 过期文件按 PROPFIND 的 getlastmodified 判定。
func TestWebdavCleanupRemovesExpiredFiles(t *testing.T) {
	store := openRunHistoryTestStore(t)
	dav, mount := startFakeDavMount(t, store, model.MountProviderWebdav)

	dav.addFile("/影视/老文件.mkv", 7, time.Now().Add(-40*24*time.Hour))
	dav.addFile("/影视/新文件.mkv", 9, time.Now().Add(-2*24*time.Hour))

	stats, err := executeRemoteCleanup(t, store, "run-webdav-expired", remoteCleanupRequest(mount, "/影视",
		map[string]bool{"cleanup_expired_files": true}, nil, nil, 30))
	if err != nil {
		t.Fatalf("远程净化出错: %v", err)
	}

	assertFakeDavPaths(t, dav, []string{"/影视/", "/影视/新文件.mkv"})
	if stats.CleanupRemovedFiles != 1 || stats.FailureCount != 0 {
		t.Fatalf("删除 %d 个文件 / 失败 %d, want 1 / 0", stats.CleanupRemovedFiles, stats.FailureCount)
	}

	detail := remoteCleanupDetail(t, stats)
	expired, ok := deleteEntryByPath(detailEntriesByAction(detail, model.BackupFileActionDelete), "/影视/老文件.mkv")
	if !ok || expired.Note != deleteNoteExpiredFile {
		t.Fatalf("过期文件的删除明细不对: %+v", expired)
	}
}

// TestWebdavCleanupKeepsFilesWhenServerOmitsModifiedTime 服务端不返回 getlastmodified 时，
// 「过期文件」必须一条都不删：拿零值去比会算出 1970 年，把用户的文件当过期清掉。
//
// 同时要留一条跳过明细（note = 无法读取修改时间）：这类文件不是「没被扫到」，而是
// 「扫到了但判不了过期」，运行记录里的「跳过 N」得能点开看到具体是哪些、为什么没动。
func TestWebdavCleanupKeepsFilesWhenServerOmitsModifiedTime(t *testing.T) {
	store := openRunHistoryTestStore(t)
	dav, mount := startFakeDavMount(t, store, model.MountProviderWebdav)
	dav.omitModified = true

	dav.addFile("/影视/老文件.mkv", 7, time.Now().Add(-400*24*time.Hour))

	stats, err := executeRemoteCleanup(t, store, "run-webdav-no-mtime", remoteCleanupRequest(mount, "/影视",
		map[string]bool{"cleanup_expired_files": true}, nil, nil, 1))
	if err != nil {
		t.Fatalf("远程净化出错: %v", err)
	}

	assertFakeDavPaths(t, dav, []string{"/影视/", "/影视/老文件.mkv"})
	if stats.CleanupRemovedFiles != 0 || stats.FailureCount != 0 {
		t.Fatalf("删除 %d 个文件 / 失败 %d, want 0 / 0（时间拿不到就不该动）", stats.CleanupRemovedFiles, stats.FailureCount)
	}
	// 这次不是「整轮跳过」，而是「扫到了、但判不了」：留一条跳过明细说明是哪个文件、为什么没动。
	if stats.SkipCount != 1 {
		t.Fatalf("时间拿不到时应记一次跳过，实际 skip=%d", stats.SkipCount)
	}
	detail := remoteCleanupDetail(t, stats)
	skips := detailEntriesByAction(detail, model.BackupFileActionSkip)
	if len(skips) != 1 || skips[0].Path != "/影视/老文件.mkv" || skips[0].Note != skipReasonCleanupUnknownAge {
		t.Fatalf("「时间拿不到」的跳过明细不对: %+v", skips)
	}
	if got := detail.Counts[model.BackupFileActionSkip]; got != 1 {
		t.Fatalf("counts.skip = %d, want 1", got)
	}
	if !strings.Contains(stats.Summary, "跳过 1 项") {
		t.Fatalf("摘要要带出跳过数，实际 %q", stats.Summary)
	}
}

// TestWebdavCleanupRecordsFailureWithoutCountingSuccess 删不掉要「记失败、不记成功、
// 不落 delete 明细、不把东西当成已删」。本地失败分支就是这个口径。
func TestWebdavCleanupRecordsFailureWithoutCountingSuccess(t *testing.T) {
	store := openRunHistoryTestStore(t)
	dav, mount := startFakeDavMount(t, store, model.MountProviderWebdav)
	dav.denyDelete = true

	dav.addFile("/影视/广告.txt", 12, time.Now())

	stats, err := executeRemoteCleanup(t, store, "run-webdav-fail", remoteCleanupRequest(mount, "/影视",
		map[string]bool{"cleanup_matching_files": true}, []string{"*广告"}, nil, 0))
	if err == nil {
		t.Fatal("有失败项时整轮应返回错误（历史里会显示成失败）")
	}

	assertFakeDavPaths(t, dav, []string{"/影视/", "/影视/广告.txt"})
	if stats.FailureCount != 1 || stats.SuccessCount != 0 || stats.CleanupRemovedFiles != 0 {
		t.Fatalf("失败/成功/删除 = %d/%d/%d, want 1/0/0", stats.FailureCount, stats.SuccessCount, stats.CleanupRemovedFiles)
	}

	detail := remoteCleanupDetail(t, stats)
	if got := detail.Counts[model.BackupFileActionDelete]; got != 0 {
		t.Fatalf("counts.delete = %d, want 0（没删掉就不能记删除）", got)
	}
	failures := detailEntriesByAction(detail, model.BackupFileActionFail)
	if len(failures) != 1 || failures[0].Path != "/影视/广告.txt" {
		t.Fatalf("失败明细不对: %+v", failures)
	}
}

// TestWebdavCleanupValidatesMonitorDirectoryBeforeRunning 执行前先确认监控目录存在且是目录：
// 远程没有本地那种 stat，只能 PROPFIND 一次，但绝不能直接开跑再把每条都记成失败。
func TestWebdavCleanupValidatesMonitorDirectoryBeforeRunning(t *testing.T) {
	store := openRunHistoryTestStore(t)
	dav, mount := startFakeDavMount(t, store, model.MountProviderWebdav)

	dav.addFile("/其他/文件.mkv", 1, time.Now())
	dav.addFile("/影视", 1, time.Now())

	options := map[string]bool{"cleanup_matching_files": true}
	filters := []string{"*广告"}

	_, err := executeRemoteCleanup(t, store, "run-webdav-missing", remoteCleanupRequest(mount, "/不存在的目录", options, filters, nil, 0))
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("监控目录不存在时应报错，实际 %v", err)
	}

	_, err = executeRemoteCleanup(t, store, "run-webdav-not-dir", remoteCleanupRequest(mount, "/影视", options, filters, nil, 0))
	if err == nil || !strings.Contains(err.Error(), "不是文件夹") {
		t.Fatalf("监控目录指向文件时应报错，实际 %v", err)
	}

	// 一个动作都没开：整轮跳过，连一次请求都不该发出去。
	before := len(dav.requestLog())
	stats, err := executeRemoteCleanup(t, store, "run-webdav-noop", remoteCleanupRequest(mount, "/影视", nil, nil, nil, 0))
	if err != nil {
		t.Fatalf("未开启清理动作时不该报错: %v", err)
	}
	if stats.SkipCount != 1 || stats.Summary != "no cleanup actions enabled" {
		t.Fatalf("未开启清理动作时应整轮跳过，实际 skip=%d / summary=%q", stats.SkipCount, stats.Summary)
	}
	if after := len(dav.requestLog()); after != before {
		t.Fatalf("整轮跳过的执行发了 %d 次请求，应为 0", after-before)
	}
}

// TestWebdavCleanupOpenListRecursiveListing 挂载类型是 OpenList 时改走原生递归列举：
// 一次 PROPFIND（Depth: infinity）取回整棵子树，删除仍然一条一次请求，
// 但「找东西」的请求数从「目录数」降到 1 —— 这是挂在公网、有限流的网盘的关键差别。
func TestWebdavCleanupOpenListRecursiveListing(t *testing.T) {
	store := openRunHistoryTestStore(t)
	dav, mount := startFakeDavMount(t, store, model.MountProviderOpenList)

	now := time.Now()
	dav.addFile("/影视/A/B/C/广告.txt", 33, now)
	dav.addFile("/影视/保留.mkv", 11, now)

	stats, err := executeRemoteCleanup(t, store, "run-webdav-openlist", remoteCleanupRequest(mount, "/影视",
		map[string]bool{"cleanup_matching_files": true, "cleanup_empty_dirs": true},
		[]string{"*广告"}, nil, 0))
	if err != nil {
		t.Fatalf("远程净化出错: %v", err)
	}

	// A / B / C 在文件被删空后逐层变成空目录，一并清掉；保留.mkv 与监控目录不动。
	assertFakeDavPaths(t, dav, []string{"/影视/", "/影视/保留.mkv"})
	if stats.CleanupRemovedFiles != 1 || stats.CleanupRemovedDirs != 3 {
		t.Fatalf("删除统计 = %d 文件 / %d 目录, want 1 / 3", stats.CleanupRemovedFiles, stats.CleanupRemovedDirs)
	}

	propfinds := fakeDavRequests(dav, "PROPFIND")
	if len(propfinds) != 2 {
		t.Fatalf("PROPFIND 次数 = %d, want 2（1 次 Stat + 1 次递归列举）；逐目录列举会随目录数增长：%v", len(propfinds), propfinds)
	}
	recursive := false
	for _, line := range propfinds {
		if strings.Contains(line, "depth=infinity") {
			recursive = true
		}
	}
	if !recursive {
		t.Fatalf("OpenList 挂载没有走递归列举（Depth: infinity）：%v", propfinds)
	}
}

// TestWebdavCleanupRuleRunsThroughExecutionAndPersistsDetail 整链路：库里的一条净化规则，
// 监控目录指向挂载 → 手动触发 → 真实落库 → 按前端折叠表的取法回读明细。
//
// 用户报的「清理远程挂载的目录总是失败」就发生在这条链路的最后一步：以前整轮返回
// 「系统找不到指定的路径」，记录直接变失败。这条用例把它钉住，同时确认远程删除同样会
// 在详情窗口里长出「删除 N」那一项。
func TestWebdavCleanupRuleRunsThroughExecutionAndPersistsDetail(t *testing.T) {
	store := openRunHistoryTestStore(t)
	dav, mount := startFakeDavMount(t, store, model.MountProviderWebdav)

	now := time.Now()
	dav.addFile("/影视/广告.txt", 5, now)
	dav.addFile("/影视/保留.mkv", 5, now)
	dav.addDir("/影视/空目录")

	sourceDir := model.JoinMountVirtualPath(mount.ID, "影视")
	rule, err := store.CreateRule(model.CreateRuleInput{
		Name:        "清理模式-远程挂载",
		ArchiveMode: "cleanup",
		RuleType:    "cleanup",
		SourceDir:   sourceDir,
		SourceDirs:  []string{sourceDir},
		Options: map[string]bool{
			"cleanup_matching_files": true,
			"cleanup_empty_dirs":     true,
		},
		Filters: []string{"*广告"},
	})
	if err != nil {
		t.Fatalf("创建清理模式规则失败: %v", err)
	}

	service := NewService(store)
	run, err := service.PrepareRuleRun(buildRuleExecuteRequest(*rule, model.TriggerModeManual))
	if err != nil {
		t.Fatalf("启动清理模式执行失败: %v", err)
	}
	waitRunSettled(t, service, run.ID)
	assertRunSucceeded(t, service, run.ID)

	assertFakeDavPaths(t, dav, []string{"/影视/", "/影视/保留.mkv"})

	detail := representativeDetailFromHistory(t, store, "cleanup")
	if detail.Kind != model.RunDetailKindCleanup {
		t.Fatalf("明细类型 = %q, want %q", detail.Kind, model.RunDetailKindCleanup)
	}
	if got := detail.Counts[model.BackupFileActionDelete]; got != 2 {
		t.Fatalf("counts.delete = %d, want 2（命中文件 + 空目录）", got)
	}
	// source_roots 存的是「挂载内路径」：前端据此把明细里的路径裁成相对路径显示。
	if len(detail.SourceRoots) != 1 || detail.SourceRoots[0] != "/影视" {
		t.Fatalf("source_roots = %+v, want [/影视]", detail.SourceRoots)
	}
}
