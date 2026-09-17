package executor

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"nestify/backend/internal/model"
)

// maxDetailEntriesPerAction 每个动作最多保留的明细条数，与备份链路的
// maxFileEntriesPerAction 保持一致：明细是用来「看清这次到底动了哪些文件」的，
// 不做完整账本，超出部分由 counts 里的真实数量体现。
const maxDetailEntriesPerAction = 200

// runDetailCollector 采集一次执行「到底动了哪些文件」的明细，序列化进
// run_history.detail_json（与备份共用同一载荷结构，只是 kind 与 action 不同）。
//
// 采集器自带锁：strm 的并发列举（下载线程数 > 1）会多线程写入。
type runDetailCollector struct {
	mu        sync.Mutex
	kind      string
	files     []model.RunFileEntry
	counts    map[string]int
	total     int
	truncated bool
	// aggregate 记录「逐条列出太占地方」的动作的累计值（数量 + 总大小），
	// 收尾时由 summarizeAggregated 压成一条汇总明细。当前用于 strm 的元数据同步：
	// 一次任务常见成百上千个封面 / 字幕 / nfo，逐条列出只会把明细面板撑满。
	aggregate map[string]runAggregate
	// sourceRoots / targetRoots 是本次执行的源 / 目标根路径（规则里配置的路径）。
	// 前端据此刻成「根路径下一级」显示，避免长绝对路径把明细的两列挤爆（见 model.RunDetail）。
	sourceRoots []string
	targetRoots []string
}

// runAggregate 是某个动作的汇总累计值。
type runAggregate struct {
	count int
	bytes int64
}

func newRunDetailCollector(kind string) *runDetailCollector {
	return &runDetailCollector{
		kind:   normalizeDetailKind(kind),
		counts: make(map[string]int, 4),
	}
}

// setRoots 记录本次执行的源 / 目标根路径，收尾时写进明细载荷（见 model.RunDetail）。
//
// 在启动并发枚举之前调用一次即可；传空切片表示不裁剪，前端照旧渲染完整路径。
func (c *runDetailCollector) setRoots(sourceRoots, targetRoots []string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sourceRoots = normalizeDetailRoots(sourceRoots)
	c.targetRoots = normalizeDetailRoots(targetRoots)
}

// normalizeDetailRoots 归一化根路径：去空白、统一成斜杠分隔、去掉尾部斜杠与重复项。
//
// 根路径只用来做前缀裁剪，所以「/」这类只剩空的根会被丢掉（无可裁内容），
// 免得前端把整条路径都裁没。
func normalizeDetailRoots(roots []string) []string {
	if len(roots) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(roots))
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		trimmed := filepath.ToSlash(strings.TrimSpace(root))
		trimmed = strings.TrimRight(trimmed, "/")
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// record 记录一条明细：累加计数，同一动作超过上限后只计数不再追加。
// 「跳过」不应走这里（见 recordSkip），否则海量跳过会把明细撑爆。
func (c *runDetailCollector) record(entry model.RunFileEntry) {
	if c == nil {
		return
	}
	path := strings.TrimSpace(entry.Path)
	if path == "" {
		return
	}
	action := strings.TrimSpace(entry.Action)
	if action == "" {
		action = model.BackupFileActionUpload
	}
	entry.Path = path
	entry.Action = action

	c.mu.Lock()
	defer c.mu.Unlock()

	c.counts[action]++
	c.total++
	if c.counts[action] > maxDetailEntriesPerAction {
		c.truncated = true
		return
	}
	c.files = append(c.files, entry)
}

// recordSkip 只累计「跳过」数量，不产生逐条明细：
// 筛选名单命中的文件既没有被生成 strm 也没有被复制，逐条列出没有意义。
func (c *runDetailCollector) recordSkip(count int) {
	if c == nil || count <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[model.BackupFileActionSkip] += count
	c.total += count
}

// addAggregated 累计一个「逐条列出太占地方」的动作（当前用于 strm 的元数据同步）。
//
// 与 record 的区别：record 一条文件一条明细，这里只累加数量与总大小，
// 收尾时由 summarizeAggregated 压成一条汇总明细 —— 一次 strm 任务可能同步
// 成百上千个封面 / 字幕 / nfo，逐条列出来会把明细面板整个撑满。
// 自带锁：strm 的多线程下载路径会并发调用。
func (c *runDetailCollector) addAggregated(action string, size int64) {
	if c == nil {
		return
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.aggregate == nil {
		c.aggregate = make(map[string]runAggregate, 1)
	}
	current := c.aggregate[action]
	current.count++
	current.bytes += size
	c.aggregate[action] = current
}

// summarizeAggregated 把累计结果写成一条汇总明细，并在采集器里清掉该动作的累计值。
//
// counts 仍按真实数量累计（前端动作页签照旧显示「同步元数据 N」），
// files 里只留 build 造出来的这一条，明细面板因此只占一行。
func (c *runDetailCollector) summarizeAggregated(action string, build func(count int, bytes int64) model.RunFileEntry) {
	if c == nil || build == nil {
		return
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	aggregated, ok := c.aggregate[action]
	if !ok || aggregated.count <= 0 {
		return
	}
	delete(c.aggregate, action)

	entry := build(aggregated.count, aggregated.bytes)
	entry.Action = action
	entry.Path = strings.TrimSpace(entry.Path)
	if entry.Path == "" {
		// 汇总条目必须有 Path（前端按它渲染「文件」列），缺了就给个语义占位。
		entry.Path = "元数据文件"
	}

	c.counts[action] += aggregated.count
	c.total += aggregated.count
	c.files = append(c.files, entry)
}

// buildJSON 序列化明细载荷；没有任何明细时返回空串，
// 前端据此隐藏明细面板（历史记录与无明细的链路同样走这条路）。
func (c *runDetailCollector) buildJSON() string {
	if c == nil {
		return ""
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.files) == 0 && c.total == 0 {
		return ""
	}

	counts := make(map[string]int, len(c.counts))
	for action, count := range c.counts {
		counts[action] = count
	}
	// files_total 只统计会被列出的明细（跳过是纯计数，只出现在 counts 里）。
	filesTotal := c.total - counts[model.BackupFileActionSkip]
	if filesTotal < 0 {
		filesTotal = 0
	}

	encoded, err := json.Marshal(model.RunDetail{
		Kind:           c.kind,
		Files:          c.files,
		Counts:         counts,
		FilesTotal:     filesTotal,
		FilesTruncated: c.truncated,
		SourceRoots:    c.sourceRoots,
		TargetRoots:    c.targetRoots,
	})
	if err != nil {
		return ""
	}
	return string(encoded)
}

// normalizeDetailKind 归一化明细载荷类型，未知类型回落到归档，
// 保证前端总能拿到一个可识别的 kind。
func normalizeDetailKind(kind string) string {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return model.RunDetailKindArchive
	}
	return trimmed
}

// describeArchiveDetailKind 把归档模式映射成明细载荷类型：
// package → 打包、collect → 收集、其余（archive）→ 归档。前端据此决定面板标题。
func describeArchiveDetailKind(archiveMode string) string {
	switch strings.TrimSpace(archiveMode) {
	case "package":
		return model.RunDetailKindPackage
	case "collect":
		return model.RunDetailKindCollect
	default:
		return model.RunDetailKindArchive
	}
}

// detail 返回本次执行的明细采集器，未初始化时按链路类型创建。
//
// 注意：必须在启动并发枚举（如 strm 的多线程列举）之前调用一次，
// 否则两个线程可能各自创建采集器，先创建的那份会丢掉后一条明细。
func (s *executionStats) detail(kind string) *runDetailCollector {
	if s == nil {
		return nil
	}
	if s.Detail == nil {
		s.Detail = newRunDetailCollector(kind)
	}
	return s.Detail
}

// buildDetailJSON 序列化本次执行的明细载荷；没有明细时返回空串。
func (s *executionStats) buildDetailJSON() string {
	if s == nil || s.Detail == nil {
		return ""
	}
	return s.Detail.buildJSON()
}
