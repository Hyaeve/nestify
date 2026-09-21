package executor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"nestify/backend/internal/model"
	"nestify/backend/internal/store/sqlite"
)

type Service struct {
	mu      sync.RWMutex
	store   *sqlite.Store
	runs    map[string]*model.RunInstance
	logs    map[string][]model.RunLogEntry
	history []model.RunHistoryItem

	automationMu     sync.Mutex
	automationCtx    context.Context
	automationCancel context.CancelFunc
	cronRunner       *cron.Cron
	watchCancels     map[int64]context.CancelFunc
	activeRules      map[int64]struct{}

	// 手动停止：runID -> 可取消上下文（见 cancel.go）。
	runContexts map[string]context.Context
	runCancels  map[string]context.CancelFunc
}

func NewService(store *sqlite.Store) *Service {
	return &Service{
		store:        store,
		runs:         make(map[string]*model.RunInstance),
		logs:         make(map[string][]model.RunLogEntry),
		history:      make([]model.RunHistoryItem, 0),
		watchCancels: make(map[int64]context.CancelFunc),
		activeRules:  make(map[int64]struct{}),
		runContexts:  make(map[string]context.Context),
		runCancels:   make(map[string]context.CancelFunc),
	}
}

func (s *Service) ListHistory() []model.RunHistoryItem {
	if s.store != nil {
		items, err := s.store.ListRunHistory()
		if err == nil {
			return items
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// s.history 内部按写入顺序存放（见 recordHistory），这里翻回「最新在最前」，
	// 与 store 侧 ListRunHistory 的顺序保持一致。
	items := make([]model.RunHistoryItem, 0, len(s.history))
	for index := len(s.history) - 1; index >= 0; index-- {
		items = append(items, s.history[index])
	}
	return items
}

func (s *Service) ClearHistory() error {
	if s.store != nil {
		if err := s.store.ClearRunHistory(); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.history = make([]model.RunHistoryItem, 0)
	s.mu.Unlock()

	return nil
}

func (s *Service) PrepareRuleRun(req ExecuteRuleRequest) (*model.RunInstance, error) {
	archiveMode := strings.TrimSpace(req.ArchiveMode)
	if archiveMode != "package" && archiveMode != "collect" && archiveMode != "cleanup" && archiveMode != "transform" && archiveMode != "link" && archiveMode != "naming" {
		return nil, fmt.Errorf("unsupported archive mode: %s", archiveMode)
	}

	req.SourceDirs = normalizeExecuteSourceDirs(req.SourceDir, req.SourceDirs)
	if req.SourceDir == "" && len(req.SourceDirs) > 0 {
		req.SourceDir = req.SourceDirs[0]
	}

	triggerMode := strings.TrimSpace(req.TriggerMode)
	if triggerMode == "" {
		triggerMode = model.TriggerModeOnce
	}

	ruleID := req.RuleID
	if ruleID > 0 && !s.markRuleActive(ruleID) {
		return nil, fmt.Errorf("rule is already running")
	}
	run := s.newRun(triggerMode, archiveMode, strings.TrimSpace(req.LinkMode), &ruleID, req.RuleName)
	s.appendLog(run.ID, "info", fmt.Sprintf("规则“%s”已进入执行队列（模式：%s）", req.RuleName, archiveMode))
	if len(req.SourceDirs) > 1 && (archiveMode == "cleanup" || archiveMode == "transform") {
		s.appendLog(run.ID, "info", fmt.Sprintf("源路径：%s；目标路径：%s", strings.Join(req.SourceDirs, "；"), req.TargetDir))
	} else {
		s.appendLog(run.ID, "info", fmt.Sprintf("源路径：%s；目标路径：%s", req.SourceDir, req.TargetDir))
	}
	s.runExecution(run.ID, req)

	return s.cloneRun(run), nil
}

func (s *Service) RecordManualExtractRun(sourcePaths []string, outputDir string, extractedPaths []string) {
	cleanSources := make([]string, 0, len(sourcePaths))
	for _, path := range sourcePaths {
		trimmed := strings.TrimSpace(path)
		if trimmed != "" {
			cleanSources = append(cleanSources, trimmed)
		}
	}

	run := s.newRun(model.TriggerModeManual, "extract", "", nil, "manual-extract")
	now := time.Now().UTC()

	s.mu.Lock()
	if currentRun, ok := s.runs[run.ID]; ok {
		currentRun.Status = model.RunStatusSucceeded
		currentRun.Stage = model.RunStageFinalizing
		currentRun.ProcessedFiles = len(cleanSources)
		currentRun.SuccessCount = len(extractedPaths)
		currentRun.SkipCount = 0
		currentRun.FailureCount = 0
		currentRun.UpdatedAt = now
		currentRun.FinishedAt = &now
	}
	s.mu.Unlock()

	if len(cleanSources) > 0 {
		s.appendLog(run.ID, "info", fmt.Sprintf("手动解压任务已提交，共 %d 个压缩包", len(cleanSources)))
		for _, path := range cleanSources {
			s.appendLog(run.ID, "info", fmt.Sprintf("源压缩包：%s", path))
		}
	}
	if strings.TrimSpace(outputDir) != "" {
		s.appendLog(run.ID, "info", fmt.Sprintf("解压输出目录：%s", outputDir))
	}
	for _, path := range extractedPaths {
		s.appendLog(run.ID, "info", fmt.Sprintf("已解压到：%s", path))
	}

	// 手动解压是一次性任务：这条就是它的代表行，用带明细的版本落（当前 stats 没有明细，
	// 但语义上「收尾的那一条」就该走这里，免得以后加了明细被无声丢掉）。
	s.persistRunHistoryWithDetail(run.ID, fmt.Sprintf("手动解压完成：%d 个压缩包，输出 %d 个目录", len(cleanSources), len(extractedPaths)), &executionStats{
		ProcessedFiles: len(cleanSources),
		SuccessCount:   len(extractedPaths),
		Summary:        fmt.Sprintf("手动解压完成：%d 个压缩包，输出 %d 个目录", len(cleanSources), len(extractedPaths)),
	})
}

func (s *Service) GetRun(runID string) (*model.RunInstance, bool) {
	s.mu.RLock()
	run, ok := s.runs[runID]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}

	return s.cloneRun(run), true
}

func (s *Service) ListRuns() []*model.RunInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*model.RunInstance, 0, len(s.runs))
	for _, run := range s.runs {
		items = append(items, s.cloneRun(run))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].StartedAt.After(items[j].StartedAt)
	})
	return items
}

func (s *Service) ListRunLogs(runID string) []model.RunLogEntry {
	s.mu.RLock()
	entries := s.logs[runID]
	s.mu.RUnlock()

	cloned := make([]model.RunLogEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

func (s *Service) newRun(triggerMode, archiveMode, linkMode string, ruleID *int64, ruleName string) *model.RunInstance {
	now := time.Now().UTC()
	run := &model.RunInstance{
		ID:          mustRandomID(),
		RuleID:      ruleID,
		RuleName:    ruleName,
		TriggerMode: triggerMode,
		ArchiveMode: archiveMode,
		LinkMode:    linkMode,
		Status:      model.RunStatusPending,
		Stage:       model.RunStageQueued,
		StartedAt:   now,
		UpdatedAt:   now,
	}

	s.mu.Lock()
	s.runs[run.ID] = run
	s.mu.Unlock()

	return run
}

func (s *Service) appendLog(runID, level, message string) {
	now := time.Now().UTC()
	entry := model.RunLogEntry{
		ID:        mustRandomID(),
		RunID:     runID,
		Level:     level,
		Message:   message,
		CreatedAt: now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if run, ok := s.runs[runID]; ok {
		run.UpdatedAt = now
	}
	s.logs[runID] = append(s.logs[runID], entry)
}

func (s *Service) runExecution(runID string, req ExecuteRuleRequest) {
	// 为这次执行登记一个可取消上下文：卡片上的执行按钮再次点击时，
	// CancelRun 会 cancel 它，执行器在循环检查点退出。
	runCtx, runCancel := context.WithCancel(context.Background())
	// 必须在起 goroutine 之前登记：否则刚提交就被点「停止」时还查不到上下文。
	s.registerRunContext(runID, runCtx, runCancel)

	go func() {
		defer s.releaseRunContext(runID)
		defer runCancel()
		defer s.unmarkRuleActive(req.RuleID)

		s.mu.Lock()
		if run, ok := s.runs[runID]; ok {
			run.Status = model.RunStatusRunning
			run.Stage = model.RunStageDispatch
			run.UpdatedAt = time.Now().UTC()
		}
		s.mu.Unlock()

		s.appendLog(runID, "info", "dispatching execution")
		prepared, err := PrepareMode(req)
		if err != nil {
			s.finishRun(runID, model.RunStatusFailed, model.RunStageFinalizing, fmt.Sprintf("执行失败：%v", err))
			s.persistRunHistory(runID, fmt.Sprintf("执行失败：%v", err), nil)
			return
		}

		stats, execErr := s.executeRuleWithSourceDirs(runID, req)
		// 手动停止优先于「失败 / 成功」判定：取消信号会让执行器返回 errRunCancelled，
		// 那不是失败，不能计进失败数，否则历史里会显示成一片红。
		cancelled := s.runAborted(runID) || errors.Is(execErr, errRunCancelled)
		if !cancelled && execErr != nil && stats.FailureCount == 0 {
			stats.FailureCount = 1
		}

		finalStatus := model.RunStatusSucceeded
		if cancelled {
			finalStatus = model.RunStatusCancelled
		} else if stats.FailureCount > 0 || execErr != nil {
			finalStatus = model.RunStatusFailed
		}

		s.mu.Lock()
		if run, ok := s.runs[runID]; ok {
			run.Stage = model.RunStageFinalizing
			run.ProcessedFiles = stats.ProcessedFiles
			run.SuccessCount = stats.SuccessCount
			run.SkipCount = stats.SkipCount
			run.FailureCount = stats.FailureCount
			run.Status = finalStatus
			run.FinishedAt = ptrTime(time.Now().UTC())
			run.UpdatedAt = time.Now().UTC()
		}
		s.mu.Unlock()

		switch {
		case cancelled:
			stats.Summary = fmt.Sprintf("已手动停止：成功 %d，跳过 %d，失败 %d", stats.SuccessCount, stats.SkipCount, stats.FailureCount)
			s.appendLog(runID, "warn", stats.Summary)
		case execErr != nil:
			s.appendLog(runID, "error", execErr.Error())
		default:
			s.appendLog(runID, "info", prepared.Summary)
			s.appendLog(runID, "info", stats.Summary)
			s.appendLog(runID, "info", "执行完成")
		}

		if req.RuleID > 0 && s.store != nil {
			// 规则卡片的「上次执行结果」仍按实际处理的文件数落库（停止不是失败）。
			// 级联删除也算「这次干了活」：一次只做了清理的执行不该被显示成「跳过」。
			_ = s.store.UpdateRuleExecutionStats(req.RuleID, mapRunStatusByCounts(stats.SuccessCount, stats.SkipCount, stats.FailureCount, stats.cascadeDeletedTotal()), stats.SuccessCount, stats.SkipCount, stats.FailureCount)
		}
		// 收尾固定补一条「带完整明细」的记录：中间那些逐项记录刻意不带明细（见
		// persistRunHistory 的说明），前端折叠组按「组内明细最长的一份」取代表行，
		// 取到的就是这一条 —— 明细展示口径与从前一致，写入量却从 O(n²) 降到 O(n)。
		// 多监控目录的合并明细也走这里：stats 就是聚合后的那份（Detail 已 merge）。
		s.persistRunHistoryWithDetail(runID, stats.Summary, &stats)
		if execErr != nil && !cancelled {
			return
		}
	}()
}

func (s *Service) executeRuleWithSourceDirs(runID string, req ExecuteRuleRequest) (executionStats, error) {
	sourceDirs := normalizeExecuteSourceDirs(req.SourceDir, req.SourceDirs)
	if len(sourceDirs) <= 1 || (strings.TrimSpace(req.ArchiveMode) != "cleanup" && strings.TrimSpace(req.ArchiveMode) != "transform" && strings.TrimSpace(req.ArchiveMode) != "naming") {
		if len(sourceDirs) == 1 {
			req.SourceDir = sourceDirs[0]
		}
		return s.executeRule(runID, req)
	}

	aggregated := executionStats{}
	var lastErr error
	for index, sourceDir := range sourceDirs {
		// 多监控目录是一个个串行跑的：每个目录开始前检查一次取消信号。
		if s.runAborted(runID) {
			break
		}
		currentReq := req
		currentReq.SourceDir = sourceDir
		currentReq.SourceDirs = []string{sourceDir}
		s.appendLog(runID, "info", fmt.Sprintf("开始处理第 %d/%d 个监控目录：%s", index+1, len(sourceDirs), sourceDir))
		stats, err := s.executeRule(runID, currentReq)
		aggregated.ProcessedFiles += stats.ProcessedFiles
		aggregated.SuccessCount += stats.SuccessCount
		aggregated.SkipCount += stats.SkipCount
		aggregated.FailureCount += stats.FailureCount
		aggregated.PackedVolumes += stats.PackedVolumes
		aggregated.MovedFiles += stats.MovedFiles
		aggregated.CleanupRemovedFiles += stats.CleanupRemovedFiles
		aggregated.CleanupRemovedDirs += stats.CleanupRemovedDirs
		aggregated.SizeBytes += stats.SizeBytes
		aggregated.HistoryEvents += stats.HistoryEvents
		// 明细也跨目录合并：前端折叠组取的是「明细最完整的那一行」，
		// 不合并就只能看到条目最多的那个目录，其余目录的删除 / 生成会凭空消失。
		aggregated.mergeDetail(&stats)
		if err != nil {
			lastErr = err
			s.appendLog(runID, "error", fmt.Sprintf("监控目录 %s 执行失败：%v", sourceDir, err))
		}
	}
	aggregated.Summary = fmt.Sprintf("多监控目录执行完成：%d 个目录，成功 %d，跳过 %d，失败 %d", len(sourceDirs), aggregated.SuccessCount, aggregated.SkipCount, aggregated.FailureCount)
	// 这里不再单独补一条「合并明细」记录：调用方（runExecution）收尾时用的就是**同一个**
	// aggregated（Detail 已经 merge 过），落的那一条已带完整明细 —— 合并后的条目一条不少，
	// 反而少写一行。轮 109 之前需要它，是因为当时的逐项记录各自都带着全量明细。
	return aggregated, lastErr
}

// normalizeExecuteSourceDirs 归一化执行请求里的源目录：去空白、去重。
//
// 还要展开「多监控目录」的 JSON 数组串：规则表里多目录是存进 source_dir 的
// （`["D:/a","D:/b"]`，见 store.normalizeRuleSourceDir），而自动触发与手动执行
// 都会把 source_dir 原样塞进请求 —— 不展开的话那个 JSON 串会被当成一个路径，
// 于是每次执行都多出一个必然失败的「监控目录」，记录直接变成一条失败。
func normalizeExecuteSourceDirs(sourceDir string, sourceDirs []string) []string {
	seen := make(map[string]struct{}, len(sourceDirs)+1)
	items := make([]string, 0, len(sourceDirs)+1)

	var appendItem func(value string, depth int)
	appendItem = func(value string, depth int) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		// 只在最外层做一次 JSON 展开（depth 兜底，避免畸形数据把递归带飞）。
		if depth < 2 && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			var parsed []string
			if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil && len(parsed) > 0 {
				for _, item := range parsed {
					appendItem(item, depth+1)
				}
				return
			}
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}

	for _, item := range sourceDirs {
		appendItem(item, 0)
	}
	appendItem(sourceDir, 0)
	return items
}

func (s *Service) finishRun(runID, status, stage, logMsg string) {
	s.mu.Lock()
	if run, ok := s.runs[runID]; ok {
		run.Status = status
		run.Stage = stage
		run.UpdatedAt = time.Now().UTC()
		run.FinishedAt = ptrTime(time.Now().UTC())
	}
	s.mu.Unlock()
	s.appendLog(runID, "error", logMsg)
}

func ptrTime(t time.Time) *time.Time { return &t }

func ParseBoolOptionsJSON(raw string) map[string]bool {
	value := strings.TrimSpace(raw)
	if value == "" || value == "{}" {
		return map[string]bool{}
	}

	parsed := make(map[string]bool)
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return map[string]bool{}
	}

	return parsed
}

func ParseIntOptionsJSON(raw string) map[string]int {
	value := strings.TrimSpace(raw)
	if value == "" || value == "{}" {
		return map[string]int{}
	}

	parsed := make(map[string]int)
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return map[string]int{}
	}

	return parsed
}

func ParseStringListJSON(raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" || value == "[]" || value == "{}" {
		return []string{}
	}

	parsed := make([]string, 0)
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return []string{}
	}

	items := make([]string, 0, len(parsed))
	for _, item := range parsed {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}

	return items
}

func ParseTransformRulesJSON(raw string) []string {
	return ParseStringListJSON(raw)
}

// persistRunHistory 落一条运行记录 —— **每处理一项都会调一次**，所以这里刻意不带明细。
//
// 明细是「本次执行的完整累积快照」（长度随处理项数单调增长，轮 96 起不再有每动作 200 条上限）。
// 若每落一行都序列化一次全量明细、再把整份写进 sqlite，单轮上万项就是 O(n²) 的 JSON 序列化
// 与磁盘写入 —— 用户看到的「容器内存涨到几个 G」正是这么来的。
// 完整明细只在执行收尾时落一次，见 persistRunHistoryWithDetail；前端折叠组按
// 「组内 detail_json 最长的一份」取代表行，取到的还是那一份，展示口径不变。
func (s *Service) persistRunHistory(runID, summary string, stats *executionStats) {
	s.persistRunHistoryItem(runID, summary, stats, false)
}

// persistRunHistoryWithDetail 收尾（以及手动解压 / 收集这类单次任务）落的那一条：带完整明细。
func (s *Service) persistRunHistoryWithDetail(runID, summary string, stats *executionStats) {
	s.persistRunHistoryItem(runID, summary, stats, true)
}

func (s *Service) persistRunHistoryItem(runID, summary string, stats *executionStats, withDetail bool) {
	item := s.recordHistory(runID, summary, stats, withDetail)
	if item == nil {
		// 运行实例已不存在（例如已被回收）：既无法落库，也不能拿 nil 回填统计，
		// 否则会直接空指针 panic。
		return
	}
	if stats != nil {
		stats.ProcessedFiles = item.ProcessedFiles
		stats.SuccessCount = item.SuccessCount
		stats.SkipCount = item.SkipCount
		stats.FailureCount = item.FailureCount
		stats.SizeBytes = item.SizeBytes
		stats.HistoryEvents++
	}
	if s.store == nil {
		return
	}
	_ = s.store.UpsertRunHistory(*item)
}

func (s *Service) recordHistory(runID, summary string, stats *executionStats, withDetail bool) *model.RunHistoryItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[runID]
	if !ok || run == nil {
		return nil
	}

	status := mapRunStatus(run)
	processedFiles := run.ProcessedFiles
	successCount := run.SuccessCount
	skipCount := run.SkipCount
	failureCount := run.FailureCount
	var sizeBytes int64
	if stats != nil {
		status = mapRunStatusByCounts(stats.SuccessCount, stats.SkipCount, stats.FailureCount, stats.cascadeDeletedTotal())
		processedFiles = stats.ProcessedFiles
		successCount = stats.SuccessCount
		skipCount = stats.SkipCount
		failureCount = stats.FailureCount
		sizeBytes = stats.SizeBytes
	}
	// 手动停止优先：历史里要能一眼看出「这条是我按停的」，而不是被算成成功或失败。
	if run.Status == model.RunStatusCancelled {
		status = model.RunStatusCancelled
	}

	item := model.RunHistoryItem{
		ID:             mustRandomID(),
		RuleID:         run.RuleID,
		RuleName:       run.RuleName,
		TriggerMode:    run.TriggerMode,
		ArchiveMode:    run.ArchiveMode,
		LinkMode:       run.LinkMode,
		Status:         status,
		ProcessedFiles: processedFiles,
		SuccessCount:   successCount,
		SkipCount:      skipCount,
		FailureCount:   failureCount,
		SizeBytes:      sizeBytes,
		Summary:        summary,
		StartedAt:      run.StartedAt,
		UpdatedAt:      run.UpdatedAt,
		FinishedAt:     run.FinishedAt,
	}
	// 明细载荷（strm 生成了哪些文件、下载了哪些元数据、打包了哪些文件夹）：
	// 列表接口会统一清空，详情弹窗按需走 /run-history/detail 拉取。
	// 只有收尾那一条带全量明细 —— 中间的逐项记录带了也没人看（折叠组只取最长的一份），
	// 却会让 JSON 序列化、磁盘写入、内存占用统统退化成 O(n²)。
	if stats != nil && withDetail {
		item.DetailJSON = stats.buildDetailJSON()
	}

	// 内存镜像只是 store 不可用时的兜底，所以不驻留明细：一份累积明细可达 MB 级，
	// 留在堆里会随执行项数线性累积。另外改为顺序追加 —— 原来的「往前插」
	// （append([]T{item}, s.history...)）每写一行都要把整个切片复制一遍，是另一处 O(n²)。
	// 读取侧（ListHistory）再翻回「最新在最前」。
	mirror := item
	mirror.DetailJSON = ""
	s.history = append(s.history, mirror)
	return &item
}

func mapRunStatus(run *model.RunInstance) string {
	if run == nil {
		return "failed"
	}
	if run.FailureCount > 0 || run.Status == model.RunStatusFailed {
		return "failed"
	}
	if run.SkipCount > 0 {
		return "skip"
	}
	if run.SuccessCount > 0 || run.Status == model.RunStatusSucceeded {
		return "success"
	}
	return "skip"
}

// mapRunStatusByCounts 由计数推导这次执行的结论（规则卡片的「上次执行结果」与运行历史共用）。
//
// cascadeDeleted 是 strm「级联删除」从目标端移走的条目数（见 strm_cascade.go）：
// 它同样是「这次真的干了活」，所以只清理了什么都没生成的执行不该被显示成「跳过」。
func mapRunStatusByCounts(successCount, skipCount, failureCount, cascadeDeleted int) string {
	if failureCount > 0 {
		return "failed"
	}
	if skipCount > 0 && successCount == 0 && cascadeDeleted == 0 {
		return "skip"
	}
	if successCount > 0 || cascadeDeleted > 0 {
		return "success"
	}
	return "skip"
}

func (s *Service) markRuleActive(ruleID int64) bool {
	if ruleID <= 0 {
		return true
	}

	s.automationMu.Lock()
	defer s.automationMu.Unlock()
	if _, exists := s.activeRules[ruleID]; exists {
		return false
	}
	s.activeRules[ruleID] = struct{}{}
	return true
}

func (s *Service) unmarkRuleActive(ruleID int64) {
	if ruleID <= 0 {
		return
	}

	s.automationMu.Lock()
	delete(s.activeRules, ruleID)
	s.automationMu.Unlock()
}

func (s *Service) cloneRun(run *model.RunInstance) *model.RunInstance {
	if run == nil {
		return nil
	}

	cloned := *run
	return &cloned
}

func mustRandomID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}

	return hex.EncodeToString(buf)
}
