package executor

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"nestify/backend/internal/model"
	"nestify/backend/internal/webdav"
)

// 远程挂载（WebDAV）上的净化。
//
// 净化规则的监控目录可以直接指向一个远程挂载（webdav://<id>/...，见 DirectoryPickerDialog
// 的「远程挂载」页），而本地那条实现全是 os.* 调用 —— 对一个虚拟路径只会得到
// 「系统找不到指定的路径」，于是整轮必然失败。这里把同样的动作改用 WebDAV 协议跑：
//
//	命中清理名单的文件  → DELETE
//	命中清理名单的目录  → 先逐条删掉里面的内容，再删目录本身
//	                    （WebDAV 的 DELETE 打在集合上是否递归由服务端决定，不能指望）
//	空目录              → 子项被清空后 DELETE
//	超过保留天数的文件  → 按 PROPFIND 的 getlastmodified 判断后 DELETE
//
// 口径与本地实现严格对齐：同一套匹配器 / 白名单 / 删除原因文案与统计口径
// （delete / fail 明细，CleanupRemovedFiles / CleanupRemovedDirs / SizeBytes）。
// 明细里的路径是「挂载内部绝对路径」（形如 /影视/广告.txt），根路径挂进 source_roots，
// 前端据此裁成相对路径显示 —— 与 strm 链路对 WebDAV 源的做法完全一致。

// executeWebdavCleanupRule 是净化规则在 WebDAV 挂载上的实现（sourceDir 形如 webdav://3/影视）。
func (s *Service) executeWebdavCleanupRule(runID string, req ExecuteRuleRequest, sourceDir string) (executionStats, error) {
	stats := executionStats{}

	plan, skipReason := parseCleanupPlan(req)
	if skipReason != "" {
		stats.SkipCount = 1
		stats.Summary = skipReason
		return stats, nil
	}

	mountID, internalPath, err := parseWebdavSource(sourceDir)
	if err != nil {
		return stats, err
	}

	credential, err := s.store.GetMountCredential(mountID)
	if err != nil {
		return stats, fmt.Errorf("读取 WebDAV 挂载失败: %w", err)
	}
	if credential == nil {
		return stats, fmt.Errorf("WebDAV 挂载不存在或已被删除")
	}
	if !credential.Mount.Enabled {
		return stats, fmt.Errorf("WebDAV 挂载「%s」已停用", credential.Mount.Name)
	}

	client := webdav.NewClient(*credential)
	// 「API 请求间隔」沿用 strm 的同名参数（净化表单里暂时没有这个字段，
	// 缺省即客户端的 250ms）：限流严格的网盘可以调大，别把后端打炸。
	client.SetRequestInterval(strmAPIInterval(req.OptionValues))

	// 先确认监控目录确实存在且是目录：远程不像本地，没有 stat 就只能 PROPFIND 一次。
	root, found, err := client.Stat(context.Background(), internalPath)
	if err != nil {
		return stats, fmt.Errorf("读取监控目录失败: %w", err)
	}
	if !found {
		return stats, fmt.Errorf("监控目录在挂载「%s」里不存在：%s", credential.Mount.Name, describeRemoteCleanupRoot(internalPath))
	}
	if !root.IsDir {
		return stats, fmt.Errorf("监控目录不是文件夹：%s", sourceDir)
	}

	// 明细先建好再开跑：每条运行记录带的是「写它那一刻」的明细快照。
	stats.detail(model.RunDetailKindCleanup)
	stats.Detail.setRoots([]string{internalPath}, nil)

	s.appendLog(runID, "info", fmt.Sprintf("净化远程挂载：%s（%s%s）；请求间隔 %.2fs",
		credential.Mount.Name, credential.Mount.BaseURL, describeRemoteCleanupRoot(internalPath), client.RequestInterval().Seconds()))

	target := &webdavCleanup{
		service: s,
		runID:   runID,
		client:  client,
		root:    internalPath,
		plan:    plan,
		stats:   &stats,
		removed: make(map[string]struct{}),
	}
	target.prepareListing()
	target.walk(internalPath)

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "未发现可清理项目"
	} else {
		stats.Summary = cleanupSummary(&stats)
	}

	if stats.FailureCount > 0 {
		return stats, fmt.Errorf("cleanup finished with %d failures", stats.FailureCount)
	}

	return stats, nil
}

// webdavCleanup 是一次远程净化的执行状态。
type webdavCleanup struct {
	service *Service
	runID   string
	client  *webdav.Client
	// root 是监控目录在挂载内的绝对路径（"" 表示挂载根）。
	root string
	plan cleanupPlan
	// stats 指向本次执行的统计与明细采集器。
	stats *executionStats
	// dirs 是「一次 PROPFIND 取回整棵子树」的缓存（仅 OpenList 这类支持
	// Depth: infinity 的服务端）；nil 表示逐目录列举。
	dirs map[string][]webdav.Entry
	// removed 记录本次已经删掉的路径，用来判断目录是否已经变空 ——
	// 只看列举快照不行：子项可能在这次执行里刚被删掉。
	removed map[string]struct{}
}

// prepareListing 决定这次用哪种列举方式。
//
// OpenList / Alist 支持一次 PROPFIND（Depth: infinity）取回整棵子树，先整棵缓存下来：
// 删除仍然一个条目一次请求（无法避免），但「找东西」的请求数从「目录数」降到 1，
// 对挂在公网、有限流的网盘差别很大。服务端不支持时静默回落到逐目录列举，边删边列。
func (c *webdavCleanup) prepareListing() {
	if !c.client.SupportsRecursiveList() {
		return
	}

	entries, err := c.client.ListRecursive(context.Background(), c.root, true)
	if err != nil {
		c.service.appendLog(c.runID, "warn", fmt.Sprintf("OpenList 原生递归列举不可用（%v），回落为逐目录列举", err))
		return
	}

	dirs := make(map[string][]webdav.Entry, len(entries)+1)
	dirs[cleanupInternalPath(c.root)] = []webdav.Entry{}
	for _, entry := range entries {
		parent := cleanupParentPath(entry.Path)
		dirs[parent] = append(dirs[parent], entry)
		if entry.IsDir {
			// 空目录也要有键：children 命中不到键就等于「没有子项」是错的，
			// 这里显式登记，空的即为空目录。
			if _, ok := dirs[entry.Path]; !ok {
				dirs[entry.Path] = []webdav.Entry{}
			}
		}
	}
	c.dirs = dirs
}

// children 返回某个目录的直接子项（已按「目录在前 + 自然顺序」排序，与本地一致）。
func (c *webdavCleanup) children(dir string) ([]webdav.Entry, error) {
	if c.dirs != nil {
		entries := c.dirs[cleanupInternalPath(dir)]
		sorted := append([]webdav.Entry(nil), entries...)
		sortWebdavEntriesNaturally(sorted)
		return sorted, nil
	}

	entries, err := c.client.List(context.Background(), dir)
	if err != nil {
		return nil, err
	}
	sortWebdavEntriesNaturally(entries)
	return entries, nil
}

// walk 递归对齐一个目录，动作与本地实现一一对应。
func (c *webdavCleanup) walk(dir string) {
	// 手动停止：递归到这里直接不再往下走（取消信号由 runExecution 统一收尾）。
	if c.service.runAborted(c.runID) {
		return
	}

	children, err := c.children(dir)
	if err != nil {
		c.recordFailure(dir, true, fmt.Sprintf("读取目录失败：%v", err))
		return
	}

	for _, child := range children {
		if c.service.runAborted(c.runID) {
			return
		}
		if c.isRemoved(child.Path) {
			continue
		}

		if child.IsDir {
			if c.plan.matchingFiles && matchesFileName(child.Name, true, c.plan.matchers) {
				c.removeMatchedDir(child)
				continue
			}

			c.walk(child.Path)

			// 空目录：递归回来之后再判断，其间子项可能已被本次清理删掉。
			// 监控目录自身不删（与本地 sameCleanPath 那道判断同义）。
			if c.plan.emptyDirs && !c.isRoot(child.Path) {
				if isWhitelistedDirectoryName(child.Name, c.plan.whitelist) {
					c.skipWhitelistedDir(child)
				} else {
					c.removeIfEmpty(child)
				}
			}
			continue
		}

		if c.plan.matchingFiles && matchesFileName(child.Name, false, c.plan.matchers) {
			c.removeFile(child, deleteNoteMatchedFile)
			continue
		}
		if !c.plan.expiredFiles {
			continue
		}
		if child.ModifiedAt.IsZero() {
			// 服务端没给 getlastmodified，过期与否判不了：文件留着，并记一条跳过明细
			// （与本地 stat 失败同款）—— 否则运行记录里多一次「跳过」，点开却什么都没有。
			c.recordSkip(child.Path, skipReasonCleanupUnknownAge, false)
			continue
		}
		if isExpiredRemoteEntry(child, c.plan.retentionDays) {
			c.removeFile(child, deleteNoteExpiredFile)
		}
	}
}

// removeFile 删掉一个命中的文件并落明细。
func (c *webdavCleanup) removeFile(entry webdav.Entry, note string) {
	if err := c.client.DeleteFile(context.Background(), entry.Path); err != nil {
		c.recordFailure(entry.Path, false, fmt.Sprintf("删除失败：%v", err))
		return
	}

	c.markRemoved(entry.Path)
	c.stats.ProcessedFiles++
	c.stats.SuccessCount++
	c.stats.CleanupRemovedFiles++
	c.stats.SizeBytes += entry.Size

	// 明细先于 persistRunHistory 落：每条运行记录带的是「写它那一刻」的明细快照，
	// 顺序反了这条删除就不在快照里。
	c.stats.Detail.record(model.RunFileEntry{
		Path:   entry.Path,
		Action: model.BackupFileActionDelete,
		Note:   note,
	})
	c.finishEntry(fmt.Sprintf("已删除远程文件 %s", entry.Path), "info")
}

// removeMatchedDir 删掉一个命中清理名单的目录：连内容一起删。
//
// 大小口径与本地略有一处差别（有意为之）：本地是 RemoveAll 之后再 dirSizeOrZero，
// 目录已经没了，所以整目录的大小实际记成 0；远程这里是**删之前**逐层统计出来的真实字节数。
// 数字更准，别为了「对齐」把它改回去。
func (c *webdavCleanup) removeMatchedDir(entry webdav.Entry) {
	size, failures := c.purge(entry.Path)
	if failures > 0 {
		// purge 已经逐条记了失败；目录里还有东西没删掉，不能报成功，
		// 也不落这条 delete 明细（与本地 RemoveAll 失败时只记失败的语义一致）。
		return
	}

	if err := c.client.DeleteCollection(context.Background(), entry.Path); err != nil {
		c.recordFailure(entry.Path, true, fmt.Sprintf("删除失败：%v", err))
		return
	}

	c.markRemoved(entry.Path)
	c.stats.ProcessedFiles++
	c.stats.SuccessCount++
	c.stats.CleanupRemovedDirs++
	c.stats.SizeBytes += size

	c.stats.Detail.record(model.RunFileEntry{
		Path:   entry.Path,
		Action: model.BackupFileActionDelete,
		Dir:    true,
		Note:   deleteNoteMatchedDir,
	})
	c.finishEntry(fmt.Sprintf("已删除远程目录 %s", entry.Path), "info")
}

// purge 递归清掉 dir 里的全部内容（不含 dir 自身），返回内容总大小与失败项数。
//
// 一条失败一条明细：远程每次 DELETE 都是独立请求，能报准是哪一条没删掉
// （本地 os.RemoveAll 失败只能整目录报一条）。计数口径不变：失败进 FailureCount，
// 不算成功，目录本身也不会被记成已删除。
func (c *webdavCleanup) purge(dir string) (int64, int) {
	children, err := c.children(dir)
	if err != nil {
		c.recordFailure(dir, true, fmt.Sprintf("读取目录失败：%v", err))
		return 0, 1
	}

	var total int64
	failures := 0
	for _, child := range children {
		if c.isRemoved(child.Path) {
			continue
		}

		if child.IsDir {
			childSize, childFailures := c.purge(child.Path)
			total += childSize
			failures += childFailures
			if childFailures > 0 {
				continue
			}
			if err := c.client.DeleteCollection(context.Background(), child.Path); err != nil {
				c.recordFailure(child.Path, true, fmt.Sprintf("删除失败：%v", err))
				failures++
				continue
			}
			c.markRemoved(child.Path)
			continue
		}

		total += child.Size
		if err := c.client.DeleteFile(context.Background(), child.Path); err != nil {
			c.recordFailure(child.Path, false, fmt.Sprintf("删除失败：%v", err))
			failures++
			continue
		}
		c.markRemoved(child.Path)
	}

	return total, failures
}

// isEmptyAfterCleanup 判断目录在本次清理之后是否已经空了。
//
// 只看列举快照不行：子项可能是被这次执行刚删掉的（c.removed 记着这些路径）。
func (c *webdavCleanup) isEmptyAfterCleanup(dir string) (bool, error) {
	children, err := c.children(dir)
	if err != nil {
		return false, err
	}
	for _, child := range children {
		if c.isRemoved(child.Path) {
			continue
		}
		// 还有东西留着 —— 不是空目录。
		return false, nil
	}
	return true, nil
}

// skipWhitelistedDir 记一条「白名单保护」的跳过明细。
//
// 与本地 recordWhitelistedDirSkip 同一口径：只有目录**确实空了**才算一次跳过 ——
// 空目录本该被清掉，是白名单把它留下了；非空目录本来就不在空目录清理的范围内，
// 记进去只会凭空多出一堆跳过条目。
func (c *webdavCleanup) skipWhitelistedDir(entry webdav.Entry) {
	empty, err := c.isEmptyAfterCleanup(entry.Path)
	if err != nil || !empty {
		return
	}
	c.recordSkip(entry.Path, skipReasonCleanupWhitelist, true)
}

// removeIfEmpty 在一个目录已经空了（本次清理把它里面该删的都删了）时删掉它。
func (c *webdavCleanup) removeIfEmpty(entry webdav.Entry) {
	empty, err := c.isEmptyAfterCleanup(entry.Path)
	if err != nil {
		c.recordFailure(entry.Path, true, fmt.Sprintf("读取目录失败：%v", err))
		return
	}
	if !empty {
		return
	}

	if err := c.client.DeleteCollection(context.Background(), entry.Path); err != nil {
		c.recordFailure(entry.Path, true, fmt.Sprintf("删除失败：%v", err))
		return
	}

	c.markRemoved(entry.Path)
	c.stats.ProcessedFiles++
	c.stats.SuccessCount++
	c.stats.CleanupRemovedDirs++

	c.stats.Detail.record(model.RunFileEntry{
		Path:   entry.Path,
		Action: model.BackupFileActionDelete,
		Dir:    true,
		Note:   deleteNoteEmptyDir,
	})
	c.finishEntry(fmt.Sprintf("已删除远程空目录 %s", entry.Path), "info")
}

// recordFailure 记一条失败：明细 + 计数 + 运行记录 + 日志（与本地失败分支同款）。
func (c *webdavCleanup) recordFailure(path string, dir bool, note string) {
	c.stats.FailureCount++
	c.stats.Detail.record(model.RunFileEntry{
		Path:   path,
		Action: model.BackupFileActionFail,
		Dir:    dir,
		Note:   note,
	})
	c.finishEntry(fmt.Sprintf("清理远程路径 %s 失败：%s", path, note), "error")
}

// recordSkip 记一条跳过：明细 + 计数 + 运行记录 + 日志（与 recordFailure 同款，只是进 SkipCount）。
//
// 明细同样要先落再写运行记录 —— 每条运行记录带的是「写它那一刻」的明细快照。
func (c *webdavCleanup) recordSkip(path string, note string, dir bool) {
	c.stats.SkipCount++
	c.stats.Detail.recordSkip(path, note, dir)
	c.finishEntry(fmt.Sprintf("跳过远程路径 %s：%s", path, note), "info")
}

// finishEntry 落运行记录 + 写日志。
// 明细已经在调用点落过了 —— 这里的顺序不能调换（每条运行记录带的是当下的明细快照）。
func (c *webdavCleanup) finishEntry(message, level string) {
	c.service.persistRunHistory(c.runID, message, c.stats)
	c.service.appendLog(c.runID, level, message)
}

func (c *webdavCleanup) markRemoved(internalPath string) {
	c.removed[cleanupInternalPath(internalPath)] = struct{}{}
}

func (c *webdavCleanup) isRemoved(internalPath string) bool {
	_, ok := c.removed[cleanupInternalPath(internalPath)]
	return ok
}

func (c *webdavCleanup) isRoot(internalPath string) bool {
	return cleanupInternalPath(internalPath) == cleanupInternalPath(c.root)
}

// isExpiredRemoteEntry 按 PROPFIND 的 getlastmodified 判断是否过期。
//
// 时间拿不到（部分服务端不返回这个属性）时一律按「不过期」处理：宁可留着，
// 也不能拿零值比出一个 1970 年、把用户的文件当过期删掉。
func isExpiredRemoteEntry(entry webdav.Entry, retentionDays int) bool {
	if retentionDays < 1 || entry.IsDir || entry.ModifiedAt.IsZero() {
		return false
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	return entry.ModifiedAt.Before(cutoff)
}

// cleanupInternalPath 归一化挂载内部路径：统一斜杠、去掉尾部斜杠，挂载根统一成空串。
func cleanupInternalPath(value string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if trimmed == "" || trimmed == "/" {
		return ""
	}
	cleaned := path.Clean("/" + strings.TrimLeft(trimmed, "/"))
	if cleaned == "/" {
		return ""
	}
	return cleaned
}

// cleanupParentPath 求内部路径的父目录（同样归一化后返回）。
func cleanupParentPath(value string) string {
	return cleanupInternalPath(path.Dir(cleanupInternalPath(value)))
}

// describeRemoteCleanupRoot 把内部路径渲染成日志里好读的形式（挂载根显示成 /）。
func describeRemoteCleanupRoot(internalPath string) string {
	if normalized := cleanupInternalPath(internalPath); normalized != "" {
		return normalized
	}
	return "/"
}

// sortWebdavEntriesNaturally 与本地 sortEntriesNaturally 同一口径：
// 目录排在文件前面，同类按自然顺序（第 2 集排在第 10 集之前）。
func sortWebdavEntriesNaturally(entries []webdav.Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return naturalLess(entries[i].Name, entries[j].Name)
	})
}
