package executor

import (
	"encoding/json"
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
}

func newRunDetailCollector(kind string) *runDetailCollector {
	return &runDetailCollector{
		kind:   normalizeDetailKind(kind),
		counts: make(map[string]int, 4),
	}
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
