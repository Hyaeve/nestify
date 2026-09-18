package executor

import (
	"context"
	"errors"
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
	strmExtensions, metadataExtensions := splitStrmExtensionSets(req.Filters, req.MetadataFilters)
	if len(strmExtensions) == 0 && len(metadataExtensions) == 0 {
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

	// 明细采集器必须在启动并发列举之前初始化（多线程各自创建会丢明细）。
	detail := stats.detail(model.RunDetailKindStrm)
	// 明细里的源 / 目标根路径：源端裁掉挂载内部路径（明细的 Path 是远端内部绝对路径，
	// 形如 /天翼云/电影/video.mkv），目标端是本地目标目录；前端据此裁成「根路径下一级」显示。
	// 源设为挂载根时 internalPath 为空，会被归一化丢掉 —— 那时整条远端路径就是相对内容。
	detail.setRoots([]string{internalPath}, []string{targetDir})

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

	if len(metadataExtensions) > 0 {
		s.appendLog(runID, "info", fmt.Sprintf("元数据后缀 %s：从挂载下载为实体文件，不生成 Strm", describeExtensionSet(metadataExtensions)))
	}

	// OpenList 挂载优先走原生递归列举：一次 PROPFIND（Depth: infinity）取回整棵子树，
	// 远端请求数从「每个目录一次」降到一次，也不再按目录数累加节流等待。
	// 服务端不支持无限深度时回落到逐目录 + 多线程列举。
	recursiveUsed := false
	if client.SupportsRecursiveList() {
		listed, recursiveErr := s.walkOpenListStrmRecursive(context.Background(), runID, client, internalPath, targetDir, strmExtensions, metadataExtensions, matchers, overwrite, minVideoBytes, stats)
		recursiveUsed = listed
		// 手动停止不是「递归列举不可用」，别回落成逐目录列举（那样会白跑一遍）。
		if errors.Is(recursiveErr, errRunCancelled) {
			return *stats, errRunCancelled
		}
		if recursiveErr != nil {
			s.appendLog(runID, "warn", fmt.Sprintf("OpenList 原生递归列举不可用（%v），回落为逐目录列举", recursiveErr))
		}
	}

	if !recursiveUsed {
		if err := s.walkWebdavStrm(context.Background(), runID, client, internalPath, targetDir, strmExtensions, metadataExtensions, matchers, overwrite, minVideoBytes, threads, stats); err != nil {
			return *stats, err
		}
	}
	// 「跳过」逐条写进明细（筛选命中 / 后缀不匹配 / 目标已存在都会落到这里）。
	// 元数据同步同样只留一条汇总（一次任务动辄成百上千个封面 / 字幕 / nfo）。
	s.recordStrmMetadataSummary(runID, detail, stats, sourceDir, targetDir)

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
			stats.Summary = fmt.Sprintf("Strm%s完成（http strm）：%s -> %s；生成/覆盖 %d 个 Strm%s，跳过 %d 项，失败 %d 项",
				syncLabel, sourceDir, targetDir, stats.SuccessCount-stats.MetadataCount, describeMetadataCount(stats.MetadataCount), stats.SkipCount, stats.FailureCount)
		} else {
			stats.Summary = fmt.Sprintf("Strm%s完成（http strm）：%s -> %s；生成 %d 个 Strm%s，跳过 %d 项，失败 %d 项",
				syncLabel, sourceDir, targetDir, stats.SuccessCount-stats.MetadataCount, describeMetadataCount(stats.MetadataCount), stats.SkipCount, stats.FailureCount)
		}
	}
	// 把「只统计到数量、拿不到路径」的整轮跳过（含上面刚置的「未发现可生成 Strm 的媒体文件」）
	// 补齐到 counts，保证 counts.skip 与运行记录里的 skip_count 一致。
	detail.reconcileSkipCount(stats.SkipCount)

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
	strmExtensions map[string]struct{},
	metadataExtensions map[string]struct{},
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
		if s.runAborted(runID) {
			return true, errRunCancelled
		}
		if matchesFileName(entry.Name, entry.IsDir, matchers) || isUnderFilteredDir(entry.Path, matchers) {
			stats.SkipCount++
			stats.Detail.recordSkip(entry.Path, skipReasonFiltered, entry.IsDir)
			s.appendLog(runID, "info", fmt.Sprintf("skipped blacklisted entry %s", entry.Path))
			continue
		}

		if entry.IsDir {
			continue
		}

		if !matchesStrmExtension(entry.Name, strmExtensions) && !matchesStrmExtension(entry.Name, metadataExtensions) {
			stats.SkipCount++
			stats.Detail.recordSkip(entry.Path, skipReasonExtension, false)
			continue
		}

		// 元数据后缀（图片 / 字幕 / nfo）：下载成实体文件，媒体服务器才能读到封面与字幕。
		// 递归列举本身只有一次 PROPFIND，这里的下载是唯一的额外远端请求。
		if isStrmMetadataFile(entry.Name, strmExtensions, metadataExtensions) {
			switch s.downloadStrmMetadata(ctx, runID, client, entry.Path, rootInternal, targetRoot, stats) {
			case strmMetadataCopied:
				stats.ProcessedFiles++
				stats.SuccessCount++
				stats.MetadataCount++
			case strmMetadataSkipped:
				stats.SkipCount++
				stats.Detail.recordSkip(entry.Path, skipReasonExistingMeta, false)
			case strmMetadataFailed:
				stats.FailureCount++
			}
			continue
		}

		if shouldSkipByMinVideoSize(entry.Name, entry.Size, minVideoBytes) {
			stats.SkipCount++
			stats.Detail.recordSkip(entry.Path, skipReasonMinVideo, false)
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
				stats.Detail.recordSkip(entry.Path, skipReasonExistingStrm, false)
				continue
			}
		}

		if err := writeStrmContent(targetPath, client.BuildStrmURL(entry.Path)); err != nil {
			stats.FailureCount++
			stats.Detail.record(model.RunFileEntry{
				Path:   entry.Path,
				Action: model.BackupFileActionFail,
				Target: targetPath,
				Note:   fmt.Sprintf("创建 Strm 失败：%v", err),
			})
			s.appendLog(runID, "error", fmt.Sprintf("create strm %s failed: %v", targetPath, err))
			continue
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		stats.Detail.record(model.RunFileEntry{
			Path:   entry.Path,
			Action: model.RunFileActionStrm,
			Target: targetPath,
			Size:   entry.Size,
			Note:   describeStrmWrite(existed),
		})
		if existed {
			s.appendLog(runID, "info", fmt.Sprintf("overwrote http strm %s", targetPath))
		} else {
			s.appendLog(runID, "info", fmt.Sprintf("created http strm %s", targetPath))
		}
	}

	return true, nil
}

// isUnderFilteredDir 判断条目是否位于「命中过滤名单的目录」之内。
//
// 递归列举把整棵树压平成一次响应，没有「不进入这个目录」的机会：命中过滤名单的目录
// 若只跳过它自己，目录内的文件仍然会被生成 strm（元数据也会被下载），
// 与逐目录列举「命中目录即整棵跳过」的行为不一致。这里按路径逐级判断祖先目录名，
// 使两种列举路径的过滤语义一致。
func isUnderFilteredDir(entryPath string, matchers []fileNameMatcher) bool {
	if len(matchers) == 0 {
		return false
	}
	normalized := strings.Trim(strings.ReplaceAll(strings.TrimSpace(entryPath), "\\", "/"), "/")
	if normalized == "" {
		return false
	}
	segments := strings.Split(normalized, "/")
	// 最后一段是条目自身，由调用方按 isDir 语义单独判断，这里只看祖先目录。
	for _, segment := range segments[:len(segments)-1] {
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		if matchesFileName(segment, true, matchers) {
			return true
		}
	}
	return false
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
	strmExtensions map[string]struct{},
	metadataExtensions map[string]struct{},
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
				// 停止时仍然要把队列里的目录逐个 pop + done 掉再退出：
				// 直接 return 会让 pending 永远归不了零，其它线程会卡死在 cond.Wait()。
				if !s.runAborted(runID) {
					s.processWebdavStrmDir(ctx, runID, worker, dir, rootInternal, targetRoot, strmExtensions, metadataExtensions, matchers, overwrite, minVideoBytes, &statsMu, stats, queue)
				}
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
	strmExtensions map[string]struct{},
	metadataExtensions map[string]struct{},
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
		if s.runAborted(runID) {
			return
		}
		if matchesFileName(entry.Name, entry.IsDir, matchers) {
			statsMu.Lock()
			stats.SkipCount++
			stats.Detail.recordSkip(entry.Path, skipReasonFiltered, entry.IsDir)
			statsMu.Unlock()
			s.appendLog(runID, "info", fmt.Sprintf("skipped blacklisted entry %s", entry.Path))
			continue
		}

		if entry.IsDir {
			queue.push(entry.Path)
			continue
		}

		if !matchesStrmExtension(entry.Name, strmExtensions) && !matchesStrmExtension(entry.Name, metadataExtensions) {
			statsMu.Lock()
			stats.SkipCount++
			stats.Detail.recordSkip(entry.Path, skipReasonExtension, false)
			statsMu.Unlock()
			continue
		}

		// 元数据后缀：下载成实体文件。下载本身不持锁（可能很慢），只把结果计入统计。
		if isStrmMetadataFile(entry.Name, strmExtensions, metadataExtensions) {
			outcome := s.downloadStrmMetadata(ctx, runID, client, entry.Path, rootInternal, targetRoot, stats)
			statsMu.Lock()
			switch outcome {
			case strmMetadataCopied:
				stats.ProcessedFiles++
				stats.SuccessCount++
				stats.MetadataCount++
			case strmMetadataSkipped:
				stats.SkipCount++
				stats.Detail.recordSkip(entry.Path, skipReasonExistingMeta, false)
			case strmMetadataFailed:
				stats.FailureCount++
			}
			statsMu.Unlock()
			continue
		}

		if shouldSkipByMinVideoSize(entry.Name, entry.Size, minVideoBytes) {
			statsMu.Lock()
			stats.SkipCount++
			stats.Detail.recordSkip(entry.Path, skipReasonMinVideo, false)
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
				stats.Detail.recordSkip(entry.Path, skipReasonExistingStrm, false)
				statsMu.Unlock()
				continue
			}
		}

		if err := writeStrmContent(targetPath, client.BuildStrmURL(entry.Path)); err != nil {
			statsMu.Lock()
			stats.FailureCount++
			stats.Detail.record(model.RunFileEntry{
				Path:   entry.Path,
				Action: model.BackupFileActionFail,
				Target: targetPath,
				Note:   fmt.Sprintf("创建 Strm 失败：%v", err),
			})
			statsMu.Unlock()
			s.appendLog(runID, "error", fmt.Sprintf("create strm %s failed: %v", targetPath, err))
			continue
		}

		statsMu.Lock()
		stats.ProcessedFiles++
		stats.SuccessCount++
		stats.Detail.record(model.RunFileEntry{
			Path:   entry.Path,
			Action: model.RunFileActionStrm,
			Target: targetPath,
			Size:   entry.Size,
			Note:   describeStrmWrite(existed),
		})
		statsMu.Unlock()
		if existed {
			s.appendLog(runID, "info", fmt.Sprintf("overwrote http strm %s", targetPath))
		} else {
			s.appendLog(runID, "info", fmt.Sprintf("created http strm %s", targetPath))
		}
	}
}

// strmMetadataOutcome 表示一次元数据下载的结果，统计交由调用方按需加锁累加。
type strmMetadataOutcome int

const (
	strmMetadataCopied strmMetadataOutcome = iota
	strmMetadataSkipped
	strmMetadataFailed
)

// downloadStrmMetadata 把远端元数据文件（图片 / 字幕 / nfo）下载到目标目录，
// 保持与源相同的相对路径与文件名（poster.jpg 依旧是 poster.jpg，而不是 poster.strm）。
//
// 元数据**不参与「覆盖生成」**：本地已经有这个文件就跳过，不再重新下载。
// 只有 0 字节的残file（上次下载中断）才会重下，见 strmMetadataTargetState。
//
// 远端请求由 webdav.Client 内部按「API 请求间隔」节流，
// 并发路径下每个线程持有独立客户端，因此间隔按线程生效。
// 明细由这里直接写入采集器（自带锁），调用方只在 statsMu 内改计数。
func (s *Service) downloadStrmMetadata(ctx context.Context, runID string, client *webdav.Client, entryPath, rootInternal, targetRoot string, stats *executionStats) strmMetadataOutcome {
	relative := strings.TrimPrefix(entryPath, rootInternal)
	relative = strings.TrimLeft(relative, "/")
	if relative == "" {
		return strmMetadataSkipped
	}
	targetPath := filepath.Join(targetRoot, filepath.FromSlash(relative))

	skip, _, stateErr := strmMetadataTargetState(targetPath)
	if stateErr != nil {
		s.appendLog(runID, "error", fmt.Sprintf("inspect metadata target %s failed: %v", targetPath, stateErr))
		stats.Detail.record(model.RunFileEntry{
			Path:   entryPath,
			Action: model.BackupFileActionFail,
			Target: targetPath,
			Note:   fmt.Sprintf("检查元数据目标失败：%v", stateErr),
		})
		return strmMetadataFailed
	}
	if skip {
		// 已存在的元数据只计数、不逐条打印：第二次跑全量时每个封面 / 字幕都会命中这里，
		// 逐条写日志只会把运行日志刷满（明细同理，见 recordStrmMetadataSummary）。
		return strmMetadataSkipped
	}

	if err := client.Download(ctx, entryPath, targetPath); err != nil {
		s.appendLog(runID, "error", fmt.Sprintf("download metadata %s -> %s failed: %v", entryPath, targetPath, err))
		stats.Detail.record(model.RunFileEntry{
			Path:   entryPath,
			Action: model.BackupFileActionFail,
			Target: targetPath,
			Note:   fmt.Sprintf("下载元数据失败：%v", err),
		})
		return strmMetadataFailed
	}

	// 元数据只累计、不逐条记明细与日志：一次 strm 任务常见成百上千个封面 / 字幕 / nfo，
	// 逐条记录会把「归巢历史 / 任务日志」的任务详情撑满（用户反馈「太占地方」）。
	// 收尾时由 executeWebdavStrmRule 压成一条汇总明细 + 一行汇总日志。
	// addAggregated 自带锁，多线程下载路径可以安全调用。
	stats.Detail.addAggregated(model.RunFileActionMetadata, fileSizeOrZero(targetPath))
	return strmMetadataCopied
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
