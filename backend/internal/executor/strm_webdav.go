package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"nestify/backend/internal/model"
	"nestify/backend/internal/webdav"
)

// isWebdavSource 判断源路径是否指向 WebDAV 挂载目录。
func isWebdavSource(sourceDir string) bool {
	return strings.HasPrefix(strings.TrimSpace(sourceDir), model.MountPathScheme)
}

func parseWebdavSource(sourceDir string) (int64, string, error) {
	trimmed := strings.TrimSpace(sourceDir)
	rest := strings.TrimPrefix(trimmed, model.MountPathScheme)
	parts := strings.SplitN(rest, "/", 2)

	id, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || id <= 0 {
		return 0, "", fmt.Errorf("无效的 WebDAV 挂载源路径：%s", sourceDir)
	}

	internal := ""
	if len(parts) == 2 {
		internal = "/" + strings.TrimLeft(parts[1], "/")
	}
	if internal == "/" {
		internal = ""
	}
	return id, internal, nil
}

// executeWebdavStrmRule 针对 WebDAV 挂载源生成 http strm：
// strm 内容 = 挂载的 http 根地址 + 直链端点（/d）+ WebDAV 内部文件路径。
// 注意使用直链端点而非 WebDAV 端点（/dav），否则媒体服务器无法直接播放。
//
// 列举策略按挂载类型分两路：
//   - OpenList 挂载走原生递归列举，一次 PROPFIND（Depth: infinity）取回整棵子树；
//   - 通用 WebDAV 走逐目录列举：远端请求按「下载线程数」并发、每个线程遵守「API 请求间隔」，
//     避免把网盘后端打限流。
func (s *Service) executeWebdavStrmRule(runID string, req ExecuteRuleRequest, sourceDir, targetDir string, stats *executionStats) (executionStats, error) {
	extensions := normalizeStrmExtensions(req.Filters)
	if len(extensions) == 0 {
		return *stats, fmt.Errorf("strm extensions are required")
	}
	matchers := buildFileNameMatchers(req.Whitelist)

	mountID, internalPath, err := parseWebdavSource(sourceDir)
	if err != nil {
		return *stats, err
	}

	credential, err := s.store.GetMountCredential(mountID)
	if err != nil {
		return *stats, fmt.Errorf("读取 WebDAV 挂载失败: %w", err)
	}
	if credential == nil {
		return *stats, fmt.Errorf("WebDAV 挂载不存在或已被删除")
	}
	if !credential.Mount.Enabled {
		return *stats, fmt.Errorf("WebDAV 挂载「%s」已停用", credential.Mount.Name)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return *stats, fmt.Errorf("create target dir: %w", err)
	}

	client := webdav.NewClient(*credential)

	overwrite := req.Options["strm_overwrite"]
	interval := strmAPIInterval(req.OptionValues)
	threads := strmDownloadThreads(req.OptionValues)
	minVideoBytes := strmMinVideoBytes(req.OptionValues)
	client.SetRequestInterval(interval)

	if req.Options["strm_full_sync"] {
		if err := s.removeExistingStrmFiles(runID, targetDir, req.CompatibilityMode, stats); err != nil {
			return *stats, err
		}
	}

	s.appendLog(runID, "info", fmt.Sprintf("WebDAV 源：%s（%s）；请求间隔 %.1fs / 线程 %d；最小视频 %s",
		credential.Mount.Name, credential.Mount.BaseURL, interval.Seconds(), threads, describeMinVideoSize(minVideoBytes)))

	// 明确打印最终写入 strm 的直链前缀：WebDAV 端点（/dav）与直链端点（/d）容易混淆，
	// 日志里给出实际地址，出问题时一眼即可确认。
	mountEndpoint := model.NormalizeMountBasePath(credential.Mount.BasePath)
	if mountEndpoint == "" {
		mountEndpoint = "/"
	}
	s.appendLog(runID, "info", fmt.Sprintf("Strm 直链前缀：%s（挂载的 WebDAV 端点 %s 已按直链端点 /d 改写）",
		client.StrmBaseURL(), mountEndpoint))

	// OpenList 挂载优先走原生递归列举：一次 PROPFIND（Depth: infinity）取回整棵子树，
	// 远端请求数从「每个目录一次」降到一次，也不再按目录数累加节流等待。
	// 服务端不支持无限深度时回落到逐目录 + 多线程列举。
	recursiveUsed := false
	if client.SupportsRecursiveList() {
		listed, recursiveErr := s.walkOpenListStrmRecursive(context.Background(), runID, client, internalPath, targetDir, extensions, matchers, overwrite, minVideoBytes, stats)
		recursiveUsed = listed
		if recursiveErr != nil {
			s.appendLog(runID, "warn", fmt.Sprintf("OpenList 原生递归列举不可用（%v），回落为逐目录列举", recursiveErr))
		}
	}

	if !recursiveUsed {
		if err := s.walkWebdavStrm(context.Background(), runID, client, internalPath, targetDir, extensions, matchers, overwrite, minVideoBytes, threads, stats); err != nil {
			return *stats, err
		}
	}

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "未发现可生成 Strm 的媒体文件"
	} else {
		syncLabel := "增量同步"
		if req.Options["strm_full_sync"] {
			syncLabel = "全量同步"
		}
		if recursiveUsed {
			syncLabel += "·OpenList 原生递归"
		}
		if overwrite {
			syncLabel += "·覆盖生成"
			stats.Summary = fmt.Sprintf("Strm%s完成（http strm）：%s -> %s；生成/覆盖 %d 个 Strm，跳过 %d 项，失败 %d 项",
				syncLabel, sourceDir, targetDir, stats.SuccessCount, stats.SkipCount, stats.FailureCount)
		} else {
			stats.Summary = fmt.Sprintf("Strm%s完成（http strm）：%s -> %s；生成 %d 个 Strm，跳过 %d 项，失败 %d 项",
				syncLabel, sourceDir, targetDir, stats.SuccessCount, stats.SkipCount, stats.FailureCount)
		}
	}

	if stats.FailureCount > 0 {
		return *stats, fmt.Errorf("strm execution finished with %d failures", stats.FailureCount)
	}

	return *stats, nil
}

// walkOpenListStrmRecursive 用 OpenList 原生的一次性递归列举生成全部 strm。
//
// 与逐目录列举的差别：OpenList 的 handlePropfind 默认按无限深度走 walkFS，
// 服务端在同一个响应里递归遍历整棵目录树，所以这里只发一次 PROPFIND 就能拿到
// 全部文件与目录条目。strm 内容依旧由本地拼装（直链前缀 + 相对路径），
// 不需要为任何单个文件再发请求。
//
// 返回值 listed 表示「递归列举本身是否成功」：
//   - false 且 err != nil：服务端不支持 Depth: infinity，调用方应回落到逐目录列举；
//   - true：列举已成功，后续只在本地写文件，不会再产生远端请求。
func (s *Service) walkOpenListStrmRecursive(
	ctx context.Context,
	runID string,
	client *webdav.Client,
	rootInternal string,
	targetRoot string,
	extensions map[string]struct{},
	matchers []fileNameMatcher,
	overwrite bool,
	minVideoBytes int64,
	stats *executionStats,
) (bool, error) {
	// 只有启用「最小视频」时才需要文件大小，否则不索取 getcontentlength。
	started := time.Now()
	entries, err := client.ListRecursive(ctx, rootInternal, minVideoBytes > 0)
	if err != nil {
		return false, err
	}
	s.appendLog(runID, "info", fmt.Sprintf(
		"OpenList 原生递归列举：1 次 PROPFIND 取回 %d 个条目（耗时 %s）；逐目录列举需按目录数逐个往返",
		len(entries), time.Since(started).Round(time.Millisecond)))

	for _, entry := range entries {
		if matchesFileName(entry.Name, entry.IsDir, matchers) {
			stats.SkipCount++
			s.appendLog(runID, "info", fmt.Sprintf("skipped blacklisted entry %s", entry.Path))
			continue
		}

		if entry.IsDir {
			continue
		}

		if !matchesStrmExtension(entry.Name, extensions) {
			stats.SkipCount++
			continue
		}

		if shouldSkipByMinVideoSize(entry.Name, entry.Size, minVideoBytes) {
			stats.SkipCount++
			s.appendLog(runID, "info", fmt.Sprintf("skipped small video %s (%.1fMB)", entry.Path, float64(entry.Size)/(1024*1024)))
			continue
		}

		relative := strings.TrimPrefix(entry.Path, rootInternal)
		relative = strings.TrimLeft(relative, "/")
		if relative == "" {
			continue
		}

		targetPath := filepath.Join(targetRoot, strmRelativePath(relative))
		existed := false
		if _, err := os.Lstat(targetPath); err == nil {
			existed = true
			if !overwrite {
				stats.SkipCount++
				continue
			}
		}

		if err := writeStrmContent(targetPath, client.BuildStrmURL(entry.Path)); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("create strm %s failed: %v", targetPath, err))
			continue
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		if existed {
			s.appendLog(runID, "info", fmt.Sprintf("overwrote http strm %s", targetPath))
		} else {
			s.appendLog(runID, "info", fmt.Sprintf("created http strm %s", targetPath))
		}
	}

	return true, nil
}

// walkWebdavStrm 并发遍历 WebDAV 目录树并生成 strm。
// 待处理目录放进共享队列，由 threads 个工作线程消费；
// 每个线程持有独立的客户端副本，使「API 请求间隔」按线程生效，而不是全局串行等待。
func (s *Service) walkWebdavStrm(
	ctx context.Context,
	runID string,
	client *webdav.Client,
	rootInternal string,
	targetRoot string,
	extensions map[string]struct{},
	matchers []fileNameMatcher,
	overwrite bool,
	minVideoBytes int64,
	threads int,
	stats *executionStats,
) error {
	if threads < 1 {
		threads = 1
	}

	queue := newStrmDirQueue(rootInternal)
	var statsMu sync.Mutex

	workers := make([]*webdav.Client, threads)
	for index := range workers {
		if index == 0 {
			workers[index] = client
		} else {
			workers[index] = client.Fork()
		}
	}

	var waitGroup sync.WaitGroup
	for index := 0; index < threads; index++ {
		waitGroup.Add(1)
		go func(worker *webdav.Client) {
			defer waitGroup.Done()
			for {
				dir, ok := queue.pop()
				if !ok {
					return
				}
				s.processWebdavStrmDir(ctx, runID, worker, dir, rootInternal, targetRoot, extensions, matchers, overwrite, minVideoBytes, &statsMu, stats, queue)
				queue.done()
			}
		}(workers[index])
	}

	waitGroup.Wait()
	return nil
}

// processWebdavStrmDir 处理单个远端目录：列目录、过滤、为命中文件写 strm，
// 并把子目录交回队列。stats 由 statsMu 保护后修改（多个线程会同时命中这里）。
func (s *Service) processWebdavStrmDir(
	ctx context.Context,
	runID string,
	client *webdav.Client,
	currentInternal string,
	rootInternal string,
	targetRoot string,
	extensions map[string]struct{},
	matchers []fileNameMatcher,
	overwrite bool,
	minVideoBytes int64,
	statsMu *sync.Mutex,
	stats *executionStats,
	queue *strmDirQueue,
) {
	entries, err := client.List(ctx, currentInternal)
	if err != nil {
		statsMu.Lock()
		stats.FailureCount++
		statsMu.Unlock()
		s.appendLog(runID, "error", fmt.Sprintf("列出 WebDAV 目录 %s 失败：%v", currentInternal, err))
		return
	}

	for _, entry := range entries {
		if matchesFileName(entry.Name, entry.IsDir, matchers) {
			statsMu.Lock()
			stats.SkipCount++
			statsMu.Unlock()
			s.appendLog(runID, "info", fmt.Sprintf("skipped blacklisted entry %s", entry.Path))
			continue
		}

		if entry.IsDir {
			queue.push(entry.Path)
			continue
		}

		if !matchesStrmExtension(entry.Name, extensions) {
			statsMu.Lock()
			stats.SkipCount++
			statsMu.Unlock()
			continue
		}

		if shouldSkipByMinVideoSize(entry.Name, entry.Size, minVideoBytes) {
			statsMu.Lock()
			stats.SkipCount++
			statsMu.Unlock()
			s.appendLog(runID, "info", fmt.Sprintf("skipped small video %s (%.1fMB)", entry.Path, float64(entry.Size)/(1024*1024)))
			continue
		}

		relative := strings.TrimPrefix(entry.Path, rootInternal)
		relative = strings.TrimLeft(relative, "/")
		if relative == "" {
			continue
		}

		targetPath := filepath.Join(targetRoot, strmRelativePath(relative))
		existed := false
		if _, err := os.Lstat(targetPath); err == nil {
			existed = true
			if !overwrite {
				statsMu.Lock()
				stats.SkipCount++
				statsMu.Unlock()
				continue
			}
		}

		if err := writeStrmContent(targetPath, client.BuildStrmURL(entry.Path)); err != nil {
			statsMu.Lock()
			stats.FailureCount++
			statsMu.Unlock()
			s.appendLog(runID, "error", fmt.Sprintf("create strm %s failed: %v", targetPath, err))
			continue
		}

		statsMu.Lock()
		stats.ProcessedFiles++
		stats.SuccessCount++
		statsMu.Unlock()
		if existed {
			s.appendLog(runID, "info", fmt.Sprintf("overwrote http strm %s", targetPath))
		} else {
			s.appendLog(runID, "info", fmt.Sprintf("created http strm %s", targetPath))
		}
	}
}

// strmDirQueue 是并发遍历用的「待处理目录」队列。
// pending 记录尚未处理完的目录数：push 加一、done 减一、归零后所有阻塞的 pop 都会返回 false。
type strmDirQueue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	items   []string
	head    int
	pending int
}

func newStrmDirQueue(root string) *strmDirQueue {
	queue := &strmDirQueue{items: []string{root}, pending: 1}
	queue.cond = sync.NewCond(&queue.mu)
	return queue
}

func (q *strmDirQueue) push(dir string) {
	q.mu.Lock()
	q.items = append(q.items, dir)
	q.pending++
	q.mu.Unlock()
	q.cond.Signal()
}

func (q *strmDirQueue) pop() (string, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.head >= len(q.items) {
		if q.pending == 0 {
			return "", false
		}
		q.cond.Wait()
	}

	dir := q.items[q.head]
	q.head++
	if q.head >= len(q.items) {
		q.items = q.items[:0]
		q.head = 0
	}
	return dir, true
}

func (q *strmDirQueue) done() {
	q.mu.Lock()
	q.pending--
	finished := q.pending == 0
	q.mu.Unlock()
	if finished {
		q.cond.Broadcast()
	}
}

func strmRelativePath(relative string) string {
	ext := filepath.Ext(relative)
	if ext == "" {
		return relative + ".strm"
	}
	return strings.TrimSuffix(relative, ext) + ".strm"
}

func writeStrmContent(targetPath, content string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create strm parent: %w", err)
	}
	if err := os.WriteFile(targetPath, []byte(content+"\n"), 0o644); err != nil {
		return fmt.Errorf("write strm file: %w", err)
	}
	return nil
}
