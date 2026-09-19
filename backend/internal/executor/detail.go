package executor

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"nestify/backend/internal/model"
)

// 明细不再按动作封顶（轮 96）：一次执行动了多少个文件就记多少条，
// counts 与 files 因此恒等 —— 统计里写多少，点开就能看到多少条。
// 条目再多也不怕：任务详情窗口用**分页**查看，一页条数跟随系统设置的「每页文件数」。

// 跳过原因的统一样板：同一类跳过在归档 / 打包 / 收集 / strm 链路 / 备份里必须用同一句文案，
// 否则同一个原因在明细的悬浮提示里会出现好几种说法。
const (
	skipReasonFiltered        = "命中过滤名单，已移入回收站"
	skipReasonPackageLeftover = "打包模式不处理该文件，保留在源目录"
	skipReasonEmptyDir        = "空目录，已清理"
	skipReasonExtension       = "后缀不在 Strm / 元数据名单内"
	skipReasonMinVideo        = "视频小于「最小视频」阈值"
	skipReasonExistingStrm    = "目标已有同名 Strm，按增量跳过"
	skipReasonExistingMeta    = "目标已有该元数据文件"
	skipReasonDuplicate       = "目标已有相同文件，按去重跳过"
	skipReasonNestedPackage   = "「处理嵌套文件夹」未开启，未进入该子目录"
	skipReasonNestedCollect   = "「递归收集」未开启，未进入该子目录"
	skipReasonFullSyncRemove  = "全量同步：删除了目标端旧的 Strm"
	// 命名规则：规则跑完名字没变（没命中 / 改完等于原名），或改出来的新名字已经被占用。
	skipReasonNamingUnchanged = "命名规则未产生变化"
	skipReasonNamingConflict  = "命名后目标已存在，跳过"
	// 软链 / 硬链：命中过滤名单的文件与目录不建链，与净化 / strm 的「已移入回收站」区分开。
	skipReasonLinkFiltered = "命中过滤名单，未建立链路"
	skipReasonLinkExisting = "目标已有同名文件或链接，跳过"
)

// 删除原因的统一样板：删除动作（delete）在备份删源、strm 级联删除、净化规则上都出现，
// 明细里要能一眼看出「为什么删」。
const (
	deleteNoteMatchedFile = "命中清理名单"
	deleteNoteMatchedDir  = "命中清理名单（整个目录一起删）"
	deleteNoteExpiredFile = "超过保留天数"
	deleteNoteEmptyDir    = "空目录"
)

// runDetailCollector 采集一次执行「到底动了哪些文件」的明细，序列化进
// run_history.detail_json（与备份共用同一载荷结构，只是 kind 与 action 不同）。
//
// 采集器自带锁：strm 的并发列举（下载线程数 > 1）会多线程写入。
type runDetailCollector struct {
	mu     sync.Mutex
	kind   string
	files  []model.RunFileEntry
	counts map[string]int
	total  int
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

// record 记录一条明细：累加计数并保留条目（不再有每动作条数上限）。
// 「跳过」走 recordSkip，与其它动作记在同一份明细里。
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
	c.files = append(c.files, entry)
}

// recordSkip 登记一条「跳过」明细：这个文件确实被扫到了，但没有被处理。
//
// note 写明跳过原因（筛选名单命中 / 后缀不匹配 / 目标已存在 / 小于阈值 …），
// 前端悬浮时能看到；dir 用于区分被跳过的目录与文件。
//
// 与 record 完全一致地落明细：跳过也逐条列出，counts.skip 与条目数恒等，
// 前端点「跳过」筛出来的条数与统计数字一致。
func (c *runDetailCollector) recordSkip(path, note string, dir bool) {
	if c == nil {
		return
	}
	c.record(model.RunFileEntry{
		Path:   strings.TrimSpace(path),
		Action: model.BackupFileActionSkip,
		Note:   strings.TrimSpace(note),
		Dir:    dir,
	})
}

// reconcileSkipCount 用运行统计里的 SkipCount 补齐「只知数量、拿不到路径」的跳过。
//
// 有些跳过本来就没有具体文件：源目录为空、未发现可归档项目、净化规则没开启任何动作……
// 这类整轮跳过只会把 stats.SkipCount 置 1，没有路径可记。
// 收尾时按「总数 − 已登记条数」补差，保证 counts.skip 恒等于运行记录里的 skip_count，
// 不会出现「统计写 5、点进去只有 3 条」的错位。
//
// 差额只进 counts，不产生条目：前端点「跳过」筛出来的是有路径的那部分，
// 数量上的缺口由这里兜住。
func (c *runDetailCollector) reconcileSkipCount(total int) {
	if c == nil || total <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if diff := total - c.counts[model.BackupFileActionSkip]; diff > 0 {
		c.counts[model.BackupFileActionSkip] += diff
		c.total += diff
	}
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

// merge 把另一份采集器的内容并进自己（多监控目录串行执行时逐目录合并）。
//
// 为什么必须合并：一次执行会写出多行 run_history，前端「折叠任务」组取的是
// **明细最完整的那一行**（排序兜底见 runHistoryDetailTieBreak）。多监控目录是
// 一个个串行跑的、每个目录各有一份采集器，不合并的话详情窗口只能看到
// 条目最多的那个目录 —— 其它目录删掉的文件、生成的 strm 会凭空消失。
func (c *runDetailCollector) merge(other *runDetailCollector) {
	if c == nil || other == nil || c == other {
		return
	}

	// 先把对方的内容整体拷出来（自有锁，避免拿着两把锁互相等），再并进自己。
	other.mu.Lock()
	files := append([]model.RunFileEntry(nil), other.files...)
	counts := make(map[string]int, len(other.counts))
	for action, count := range other.counts {
		counts[action] = count
	}
	total := other.total
	kind := other.kind
	sourceRoots := append([]string(nil), other.sourceRoots...)
	targetRoots := append([]string(nil), other.targetRoots...)
	other.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.kind == "" {
		c.kind = kind
	}
	c.files = append(c.files, files...)
	c.total += total
	for action, count := range counts {
		c.counts[action] += count
	}
	// 根路径取并集：多监控目录的明细要能同时裁剪出各个目录下的相对路径。
	c.sourceRoots = normalizeDetailRoots(append(append([]string{}, c.sourceRoots...), sourceRoots...))
	c.targetRoots = normalizeDetailRoots(append(append([]string{}, c.targetRoots...), targetRoots...))
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
	// files_total 与 counts 之和、files 条数三者恒等：「跳过」现在也逐条列出（轮 96 起不再上限）。
	filesTotal := c.total

	encoded, err := json.Marshal(model.RunDetail{
		Kind:        c.kind,
		Files:       c.files,
		Counts:      counts,
		FilesTotal:  filesTotal,
		SourceRoots: c.sourceRoots,
		TargetRoots: c.targetRoots,
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

// mergeDetail 把另一次执行的明细并进本次统计（多监控目录串行执行时逐目录调用）。
//
// 第一份直接接管（那次执行已经跑完，后续目录往里并即可，不会重复计数），
// 之后的目录走 collector.merge 合并条目 / 计数 / 根路径。
func (s *executionStats) mergeDetail(other *executionStats) {
	if s == nil || other == nil || other.Detail == nil {
		return
	}
	if s.Detail == nil {
		s.Detail = other.Detail
		return
	}
	s.Detail.merge(other.Detail)
}
