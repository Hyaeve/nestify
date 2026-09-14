// Package backup 提供「备份规则」的执行引擎：
// 把源目录同步到目标目录，支持完成规则、替换规则、筛选规则、
// 完整扫描 / 扫描间隔 / Cron 计划扫描，以及实时监控。
package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"nestify/backend/internal/model"
	"nestify/backend/internal/store/sqlite"
	"nestify/backend/internal/webdav"
)

const (
	monitorPollInterval = 5 * time.Second
	maxRecentLogs       = 60
)

type taskState struct {
	Running      bool
	Status       string
	Phase        string
	Progress     string
	Scanned      int
	Copied       int
	Skipped      int
	Deleted      int
	Failed       int
	LastBackupAt string
	RecentLogs   []string
}

type Service struct {
	store *sqlite.Store

	mu      sync.RWMutex
	states  map[int64]*taskState
	cancels map[int64]context.CancelFunc

	cronRunner *cron.Cron
	cronIDs    map[int64]cron.EntryID

	ctx    context.Context
	cancel context.CancelFunc
}

func NewService(store *sqlite.Store) *Service {
	return &Service{
		store:   store,
		states:  make(map[int64]*taskState),
		cancels: make(map[int64]context.CancelFunc),
		cronIDs: make(map[int64]cron.EntryID),
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.cronRunner = cron.New()
	s.cronRunner.Start()
	return s.Reload()
}

func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.cronRunner != nil {
		s.cronRunner.Stop()
	}
}

// Reload 重新装载所有备份任务的调度（实时监控 / 扫描间隔 / Cron）。
func (s *Service) Reload() error {
	s.mu.Lock()
	for id, cancel := range s.cancels {
		cancel()
		delete(s.cancels, id)
	}
	if s.cronRunner != nil {
		for id, entryID := range s.cronIDs {
			s.cronRunner.Remove(entryID)
			delete(s.cronIDs, id)
		}
	}
	s.mu.Unlock()

	tasks, err := s.store.ListBackups()
	if err != nil {
		return err
	}

	for _, task := range tasks {
		s.scheduleTask(task)
	}

	return nil
}

func (s *Service) scheduleTask(task model.BackupTask) {
	if !task.Enabled {
		return
	}

	baseCtx := s.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	if task.MonitorEnabled || task.ScanIntervalSeconds > 0 {
		interval := monitorPollInterval
		if task.ScanIntervalSeconds > 0 {
			interval = time.Duration(task.ScanIntervalSeconds) * time.Second
			if interval < time.Second {
				interval = time.Second
			}
		}

		ctx, cancel := context.WithCancel(baseCtx)
		s.mu.Lock()
		s.cancels[task.ID] = cancel
		s.mu.Unlock()

		go s.pollLoop(ctx, task.ID, interval, task.MonitorEnabled)
	}

	if strings.TrimSpace(task.CronExpression) != "" && s.cronRunner != nil {
		taskID := task.ID
		entryID, err := s.cronRunner.AddFunc(task.CronExpression, func() {
			_ = s.runTask(taskID, false, model.TriggerModeCron)
		})
		if err == nil {
			s.mu.Lock()
			s.cronIDs[task.ID] = entryID
			s.mu.Unlock()
		}
	}
}

func (s *Service) pollLoop(ctx context.Context, taskID int64, interval time.Duration, monitor bool) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lastFingerprint := ""
	first := true

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			task, err := s.store.GetBackup(taskID)
			if err != nil || task == nil || !task.Enabled {
				continue
			}

			if monitor {
				fingerprint := fingerprintDirs(task.SourceDirs)
				if first {
					lastFingerprint = fingerprint
					first = false
					continue
				}
				if fingerprint == lastFingerprint {
					continue
				}
				lastFingerprint = fingerprint
			}

		if s.isRunning(taskID) {
			continue
		}
		_ = s.runTask(taskID, false, model.TriggerModeWatch)
		}
	}
}

func (s *Service) isRunning(taskID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.states[taskID]
	return ok && state.Running
}

// isWebdavPath 判断路径是否为 WebDAV 虚拟挂载路径（webdav://）。
func isWebdavPath(p string) bool {
	return strings.HasPrefix(strings.TrimSpace(p), model.MountPathScheme)
}

// parseWebdavTarget 解析 webdav://id[/internal] 形式的目标路径，返回挂载 ID 与内部相对路径。
func parseWebdavTarget(p string) (int64, string, error) {
	trimmed := strings.TrimSpace(p)
	rest := strings.TrimPrefix(trimmed, model.MountPathScheme)
	parts := strings.SplitN(rest, "/", 2)
	id, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || id <= 0 {
		return 0, "", fmt.Errorf("无效的 WebDAV 目标路径：%s", p)
	}
	internal := ""
	if len(parts) == 2 {
		internal = strings.Trim(strings.TrimLeft(parts[1], "/"), "/")
	}
	return id, internal, nil
}

// RunTask 触发一次备份。forceFull 为真时忽略增量判断，执行完整扫描。
func (s *Service) RunTask(taskID int64, forceFull bool) error {
	return s.runTask(taskID, forceFull, model.TriggerModeManual)
}

// runTask 与 RunTask 相同，但显式指定触发方式（手动 / 定时 / 监听），
// 用于在运行日志里标记任务来源。
func (s *Service) runTask(taskID int64, forceFull bool, triggerMode string) error {
	task, err := s.store.GetBackup(taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("备份任务不存在")
	}

	s.mu.Lock()
	if state, ok := s.states[taskID]; ok && state.Running {
		s.mu.Unlock()
		return fmt.Errorf("该备份任务正在执行中")
	}
	s.states[taskID] = &taskState{
		Running:      true,
		Status:       "running",
		Phase:        "准备中",
		LastBackupAt: task.LastBackupAt,
		RecentLogs:   []string{},
	}
	s.mu.Unlock()

	go s.execute(*task, forceFull, triggerMode)

	return nil
}

// Status 返回备份任务当前执行情况。
func (s *Service) Status(taskID int64) model.BackupStatusSnapshot {
	task, _ := s.store.GetBackup(taskID)
	snapshot := model.BackupStatusSnapshot{TaskID: taskID, Status: "idle", Phase: "空闲"}
	if task != nil {
		snapshot.TaskName = task.Name
		snapshot.LastBackupAt = task.LastBackupAt
		snapshot.Status = task.LastStatus
		if snapshot.Status == "" {
			snapshot.Status = "idle"
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.states[taskID]
	if !ok {
		return snapshot
	}

	snapshot.Running = state.Running
	snapshot.Status = state.Status
	snapshot.Phase = state.Phase
	snapshot.Progress = state.Progress
	snapshot.Scanned = state.Scanned
	snapshot.Copied = state.Copied
	snapshot.Skipped = state.Skipped
	snapshot.Deleted = state.Deleted
	snapshot.Failed = state.Failed
	if state.LastBackupAt != "" {
		snapshot.LastBackupAt = state.LastBackupAt
	}
	snapshot.RecentLogs = append([]string{}, state.RecentLogs...)

	return snapshot
}

// RunningTaskIDs 返回当前正在执行的备份任务 ID 集合。
func (s *Service) RunningTaskIDs() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]int64, 0, len(s.states))
	for id, state := range s.states {
		if state.Running {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (s *Service) setPhase(taskID int64, phase string, progress string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, ok := s.states[taskID]; ok {
		state.Phase = phase
		state.Progress = progress
	}
}

func (s *Service) log(taskID int64, format string, args ...any) {
	line := fmt.Sprintf("%s  %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))

	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[taskID]
	if !ok {
		return
	}
	state.RecentLogs = append(state.RecentLogs, line)
	if len(state.RecentLogs) > maxRecentLogs {
		state.RecentLogs = state.RecentLogs[len(state.RecentLogs)-maxRecentLogs:]
	}
}

func (s *Service) execute(task model.BackupTask, forceFull bool, triggerMode string) {
	startedAt := time.Now().UTC()
	stats := &runStats{}

	defer func() {
		status := "success"
		if stats.Failed > 0 {
			status = "failed"
		}
		summary := fmt.Sprintf("备份完成：扫描 %d，上传 %d，跳过 %d，删除 %d，失败 %d",
			stats.Scanned, stats.Copied, stats.Skipped, stats.Deleted, stats.Failed)

		s.mu.Lock()
		if state, ok := s.states[task.ID]; ok {
			state.Running = false
			state.Status = status
			state.Phase = "已完成"
			state.Progress = ""
			state.Scanned = stats.Scanned
			state.Copied = stats.Copied
			state.Skipped = stats.Skipped
			state.Deleted = stats.Deleted
			state.Failed = stats.Failed
			state.LastBackupAt = startedAt.Format(time.RFC3339)
		}
		s.mu.Unlock()

		s.log(task.ID, "%s", summary)
		_ = s.store.UpdateBackupRunResult(task.ID, status, summary, startedAt.Format(time.RFC3339), stats.Scanned, stats.Copied, stats.Skipped, stats.Deleted)
		s.recordRunHistory(task, triggerMode, status, summary, startedAt, stats)
	}()

	if len(task.SourceDirs) == 0 {
		stats.Failed++
		s.log(task.ID, "未配置源路径，跳过执行")
		return
	}

	matcher := newFilterMatcher(task.FilterRules)
	incrementalCutoff := time.Time{}
	if !forceFull && !task.ForceFullScan && task.LastBackupAt != "" {
		if parsed, err := time.Parse(time.RFC3339, task.LastBackupAt); err == nil {
			incrementalCutoff = parsed
		}
	}

	s.log(task.ID, "开始备份「%s」", task.Name)

	// 收集源文件清单（相对路径 -> 源绝对路径），供同步删除使用。
	sourceIndex := make(map[string]string)
	totalSources := len(task.SourceDirs)

	for index, sourceDir := range task.SourceDirs {
		s.setPhase(task.ID, "扫描源目录", fmt.Sprintf("%d/%d", index+1, totalSources))

		info, err := os.Stat(sourceDir)
		if err != nil || !info.IsDir() {
			stats.Failed++
			s.log(task.ID, "源目录不可访问：%s", sourceDir)
			continue
		}

		err = filepath.WalkDir(sourceDir, func(currentPath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				stats.Failed++
				s.log(task.ID, "读取失败：%s（%v）", currentPath, walkErr)
				return nil
			}
			if currentPath == sourceDir {
				return nil
			}

			relative, relErr := filepath.Rel(sourceDir, currentPath)
			if relErr != nil {
				return nil
			}

			if entry.IsDir() {
				if matcher.excluded(relative, entry.Name(), true, 0) {
					s.log(task.ID, "按筛选规则跳过目录：%s", relative)
					return filepath.SkipDir
				}
				return nil
			}

			stats.Scanned++
			fileInfo, infoErr := entry.Info()
			if infoErr != nil {
				stats.Failed++
				return nil
			}

			if matcher.excluded(relative, entry.Name(), false, fileInfo.Size()) {
				stats.Skipped++
				return nil
			}

			sourceIndex[relative] = currentPath
			return nil
		})
		if err != nil {
			stats.Failed++
			s.log(task.ID, "扫描源目录失败：%s（%v）", sourceDir, err)
		}
	}

	s.setPhase(task.ID, "复制文件", fmt.Sprintf("共 %d 个文件", len(sourceIndex)))

	// 预解析 WebDAV 目标，避免每个文件重复解析。本地目标保持原路径。
	targets := make([]targetDesc, 0, len(task.TargetDirs))
	for _, targetDir := range task.TargetDirs {
		desc := targetDesc{raw: targetDir}
		if isWebdavPath(targetDir) {
			mountID, internal, err := parseWebdavTarget(targetDir)
			if err != nil {
				stats.Failed++
				s.log(task.ID, "目标路径无效：%s（%v）", targetDir, err)
				continue
			}
			credential, credErr := s.store.GetMountCredential(mountID)
			if credErr != nil || credential == nil {
				stats.Failed++
				s.log(task.ID, "WebDAV 挂载不可用：%s", targetDir)
				continue
			}
			if !credential.Mount.Enabled {
				stats.Failed++
				s.log(task.ID, "WebDAV 挂载「%s」已停用", credential.Mount.Name)
				continue
			}
			desc.isWebdav = true
			desc.mountID = mountID
			desc.internal = internal
			desc.client = webdav.NewClient(credential.Mount, credential.Password)
		}
		targets = append(targets, desc)
	}

	items := make([]sourceItem, 0, len(sourceIndex))
	for relative, absolute := range sourceIndex {
		items = append(items, sourceItem{relative: relative, absolute: absolute})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].relative < items[j].relative })

	for _, item := range items {
		fileInfo, statErr := os.Stat(item.absolute)
		if statErr != nil {
			continue
		}

		if !incrementalCutoff.IsZero() && !fileInfo.ModTime().After(incrementalCutoff) {
			// 增量扫描：上次备份之后未修改的文件无需重新复制，但仍保留在清单里，
			// 避免「从目标同步删除」误删目标端已有文件。
			continue
		}

		for _, target := range targets {
			if target.isWebdav {
				if err := s.copyOneWebdav(task, target, item, fileInfo, stats); err != nil {
					stats.Failed++
					s.log(task.ID, "上传失败：%s（%v）", item.relative, err)
				}
			} else {
				destination := filepath.Join(target.raw, item.relative)
				if err := s.copyOne(task, item.absolute, destination, fileInfo, stats); err != nil {
					stats.Failed++
					s.log(task.ID, "复制失败：%s（%v）", item.relative, err)
				}
			}
		}

		if (task.CompletionRule == model.BackupCompletionDeleteSource || task.CompletionRule == model.BackupCompletionDeleteSourceDirs) && len(targets) > 0 {
			if err := os.Remove(item.absolute); err == nil {
				stats.Deleted++
				delete(sourceIndex, item.relative)
			}
		}
	}

	if task.CompletionRule == model.BackupCompletionDeleteSourceDirs {
		for _, sourceDir := range task.SourceDirs {
			s.pruneEmptyDirs(sourceDir, matcher, stats)
		}
	}

	if task.SyncDeleteFromTarget && len(targets) > 0 {
		s.setPhase(task.ID, "同步删除目标", "")
		for _, target := range targets {
			if target.isWebdav {
				s.syncDeleteMissingWebdav(task, target, sourceIndex, matcher, stats)
			} else {
				s.syncDeleteMissing(target.raw, sourceIndex, matcher, stats)
			}
		}
	}

	s.log(task.ID, "备份结束：上传 %d，跳过 %d，失败 %d", stats.Copied, stats.Skipped, stats.Failed)
}

// recordRunHistory 把一次备份执行写入运行日志（run_history），
// 使用 archive_mode = "backup" 作为其专属模式标识，供日志页展示。
func (s *Service) recordRunHistory(task model.BackupTask, triggerMode, status, summary string, startedAt time.Time, stats *runStats) {
	if s.store == nil {
		return
	}

	// 运行日志的状态语义：失败优先，其次全部跳过记 skip，否则 success。
	historyStatus := status
	if historyStatus != "failed" && stats.Copied == 0 && stats.Skipped > 0 {
		historyStatus = "skip"
	}
	if strings.TrimSpace(triggerMode) == "" {
		triggerMode = model.TriggerModeManual
	}

	finishedAt := time.Now().UTC()
	ruleID := task.ID
	item := model.RunHistoryItem{
		ID:             fmt.Sprintf("backup-%d-%d", task.ID, startedAt.UnixNano()),
		RuleID:         &ruleID,
		RuleName:       task.Name,
		TriggerMode:    triggerMode,
		ArchiveMode:    "backup",
		Status:         historyStatus,
		ProcessedFiles: stats.Scanned,
		SuccessCount:   stats.Copied,
		SkipCount:      stats.Skipped,
		FailureCount:   stats.Failed,
		Summary:        summary,
		StartedAt:      startedAt,
		UpdatedAt:      finishedAt,
		FinishedAt:     &finishedAt,
	}
	_ = s.store.UpsertRunHistory(item)
}

type runStats struct {
	Scanned int
	Copied  int
	Skipped int
	Deleted int
	Failed  int
}

// sourceItem 是一个待复制的源文件条目。
type sourceItem struct {
	relative string
	absolute string
}

// targetDesc 描述一个备份目标（本地目录或 WebDAV 挂载）。
type targetDesc struct {
	raw      string
	isWebdav bool
	mountID  int64
	internal string
	client   *webdav.Client
}

func (s *Service) copyOne(task model.BackupTask, sourcePath, destination string, info os.FileInfo, stats *runStats) error {
	if _, err := os.Stat(destination); err == nil {
		if task.ReplaceRule != model.BackupReplaceOverwrite {
			stats.Skipped++
			return nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.Create(destination)
	if err != nil {
		return err
	}

	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	if err := target.Close(); err != nil {
		return err
	}

	_ = os.Chtimes(destination, time.Now(), info.ModTime())
	stats.Copied++
	s.log(task.ID, "已备份：%s", destination)
	return nil
}

func sourceDiskPath(root, relative string) string {
	return filepath.Join(root, relative)
}

// copyOneWebdav 将源文件上传到 WebDAV 目标。
// 替换规则：目标已存在且为「跳过」时跳过；「覆盖」时 PUT 覆盖。
func (s *Service) copyOneWebdav(task model.BackupTask, target targetDesc, item sourceItem, info os.FileInfo, stats *runStats) error {
	ctx := context.Background()
	internalPath := webdav.InternalPathFromParts(target.internal, filepath.ToSlash(item.relative))

	if exists, err := target.client.Exists(ctx, internalPath); err == nil && exists {
		if task.ReplaceRule != model.BackupReplaceOverwrite {
			stats.Skipped++
			return nil
		}
	} else if err != nil {
		s.log(task.ID, "检查远端文件失败：%s（%v），尝试覆盖上传", item.relative, err)
	}

	// 确保父目录存在。
	parent := path.Dir(internalPath)
	if parent != "" && parent != "/" && parent != "." {
		if err := target.client.MkdirAll(ctx, parent); err != nil {
			return err
		}
	}

	source, err := os.Open(item.absolute)
	if err != nil {
		return err
	}
	defer source.Close()

	if err := target.client.PutFile(ctx, internalPath, source, info.Size()); err != nil {
		return err
	}

	stats.Copied++
	s.log(task.ID, "已上传：%s", internalPath)
	return nil
}

func (s *Service) syncDeleteMissingWebdav(task model.BackupTask, target targetDesc, sourceIndex map[string]string, matcher *filterMatcher, stats *runStats) {
	// WebDAV 目标的「从目标同步删除」需枚举远端目录树，成本较高且易误删。
	// 为避免误删远端文件，此处不主动删除远端文件，仅记录提示。
	if len(sourceIndex) == 0 {
		return
	}
	s.log(task.ID, "WebDAV 目标暂不支持从目标同步删除，已跳过")
}

func (s *Service) syncDeleteMissing(targetDir string, sourceIndex map[string]string, matcher *filterMatcher, stats *runStats) {
	info, err := os.Stat(targetDir)
	if err != nil || !info.IsDir() {
		return
	}

	_ = filepath.WalkDir(targetDir, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			if currentPath == targetDir {
				return nil
			}
			relative, relErr := filepath.Rel(targetDir, currentPath)
			if relErr == nil && matcher.excluded(relative, entry.Name(), true, 0) {
				return filepath.SkipDir
			}
			return nil
		}

		relative, relErr := filepath.Rel(targetDir, currentPath)
		if relErr != nil {
			return nil
		}

		if _, ok := sourceIndex[relative]; ok {
			return nil
		}

		if err := os.Remove(currentPath); err != nil {
			stats.Failed++
			return nil
		}
		stats.Deleted++
		return nil
	})
}

// pruneEmptyDirs 删除源目录中已清空的文件夹（完成规则为 delete_source_dir 时）。
// 被黑名单命中的目录（及其所有子目录）不会被删除。
func (s *Service) pruneEmptyDirs(root string, matcher *filterMatcher, stats *runStats) {
	_ = filepath.WalkDir(root, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.IsDir() || currentPath == root {
			return nil
		}

		relative, relErr := filepath.Rel(root, currentPath)
		if relErr == nil && matcher.excluded(relative, entry.Name(), true, 0) {
			return filepath.SkipDir
		}

		children, err := os.ReadDir(currentPath)
		if err != nil || len(children) > 0 {
			return nil
		}
		_ = os.Remove(currentPath)
		return nil
	})
}

func fingerprintDirs(dirs []string) string {
	var builder strings.Builder
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(currentPath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			builder.WriteString(currentPath)
			builder.WriteString(fmt.Sprintf(":%d:%d;", info.Size(), info.ModTime().UnixNano()))
			return nil
		})
	}
	return builder.String()
}

// ---------- 筛选规则 ----------

type filterMatcher struct {
	rules []model.BackupFilterRule
}

func newFilterMatcher(rules []model.BackupFilterRule) *filterMatcher {
	return &filterMatcher{rules: rules}
}

func (m *filterMatcher) excluded(relativePath, name string, isDir bool, size int64) bool {
	if len(m.rules) == 0 {
		return false
	}

	hasWhitelist := false
	whitelistMatched := false

	for _, rule := range m.rules {
		if isDir && !rule.MatchDir {
			continue
		}
		if !isDir && !rule.MatchFile {
			continue
		}
		if !m.matches(rule, relativePath, name, isDir, size) {
			continue
		}
		if rule.Whitelist {
			hasWhitelist = true
			whitelistMatched = true
		}
		if rule.Blacklist {
			return true
		}
	}

	for _, rule := range m.rules {
		if !rule.Whitelist {
			continue
		}
		if isDir && !rule.MatchDir {
			continue
		}
		if !isDir && !rule.MatchFile {
			continue
		}
		hasWhitelist = true
		break
	}

	if hasWhitelist && !whitelistMatched {
		return true
	}

	return false
}

func (m *filterMatcher) matches(rule model.BackupFilterRule, relativePath, name string, isDir bool, size int64) bool {
	value := strings.TrimSpace(rule.Value)

	switch rule.Type {
	case model.BackupFilterExtension:
		if isDir {
			return false
		}
		extension := strings.ToLower(filepath.Ext(name))
		for _, candidate := range extensionCandidates(rule, value) {
			if strings.EqualFold(candidate, extension) {
				return true
			}
		}
		return false
	case model.BackupFilterRegex:
		if value == "" {
			return false
		}
		expression, err := regexp.Compile(value)
		if err != nil {
			return false
		}
		return expression.MatchString(name) || expression.MatchString(relativePath)
	case model.BackupFilterSize:
		if isDir {
			return false
		}
		unit := sizeUnitFactor(rule.SizeUnit)
		if rule.MinSize > 0 && size < rule.MinSize*unit {
			return false
		}
		if rule.MaxSize > 0 && size > rule.MaxSize*unit {
			return false
		}
		return true
	default:
		// 名称类型支持多候选：value 字段（单值）以及 extensions 数组（回车逐个添加的多值）。
		if value != "" && nameMatches(value, name, relativePath) {
			return true
		}
		for _, candidate := range rule.Extensions {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if nameMatches(candidate, name, relativePath) {
				return true
			}
		}
		return false
	}
}

func nameMatches(value, name, relativePath string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, "*?") {
		matched, err := path.Match(strings.ToLower(value), strings.ToLower(name))
		if err == nil && matched {
			return true
		}
		matchedPath, pathErr := path.Match(strings.ToLower(value), strings.ToLower(filepath.ToSlash(relativePath)))
		return pathErr == nil && matchedPath
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(value))
}

func extensionCandidates(rule model.BackupFilterRule, value string) []string {
	candidates := make([]string, 0, len(rule.Extensions)+1)
	candidates = append(candidates, rule.Extensions...)
	if value == "" {
		return candidates
	}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '|'
	}) {
		candidates = append(candidates, part)
	}
	for index, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		trimmed = strings.TrimPrefix(trimmed, "*")
		if trimmed != "" && !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}
		candidates[index] = trimmed
	}
	return candidates
}

func sizeUnitFactor(unit string) int64 {
	switch strings.ToUpper(strings.TrimSpace(unit)) {
	case "KB":
		return 1024
	case "MB":
		return 1024 * 1024
	case "GB":
		return 1024 * 1024 * 1024
	default:
		return 1
	}
}
