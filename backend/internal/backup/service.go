// Package backup 提供「备份规则」的执行引擎：
// 把源目录同步到目标目录，支持完成规则、替换规则、筛选规则、
// 完整扫描 / 扫描间隔 / Cron 计划扫描，以及实时监控。
package backup

import (
	"context"
	"encoding/json"
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
	// maxFileEntriesPerAction 限制运行日志详情里每个动作（上传/失败/删除）
	// 最多记录多少条文件明细，避免日志记录体积失控。
	// 「跳过」不记明细，只累计数量（见 recordFile）。
	maxFileEntriesPerAction = 200
	// watchHistoryMergeWindow 是实时监控「连续执行合并」的时间窗：
	// 上一次监控执行结束后，在这个时间内再次触发，视为同一次连续备份，
	// 累加进同一条运行历史（见 recordRunHistory）。
	// 监控每 5 秒轮询一次，源目录持续写入时若每次都新建记录，
	// 归巢历史 / 任务日志会被几十条「上传 0，跳过 N」刷满。
	watchHistoryMergeWindow = 10 * time.Minute
)

type taskState struct {
	Running bool
	// CancelRequested 由「扫描」按钮再次点击时置上；execute 的主循环在每个文件前检查它。
	// 每次 runTask 都会换成全新的 taskState，所以上一轮的标记不会残留。
	CancelRequested bool
	Status          string
	Phase           string
	Progress        string
	Scanned         int
	Copied          int
	Skipped         int
	Deleted         int
	Failed          int
	LastBackupAt    string
	RecentLogs      []string
}

// watchMergeRecord 指向一条仍处于合并窗口内的「实时监控」运行历史。
// 窗口内的下一次监控触发会累加进它，而不是新建记录。
type watchMergeRecord struct {
	historyID  string
	finishedAt time.Time
}

type Service struct {
	store *sqlite.Store

	mu      sync.RWMutex
	states  map[int64]*taskState
	cancels map[int64]context.CancelFunc

	// watchMerge 记录每个备份任务最近一条实时监控历史（taskID -> 记录）。
	// 仅在内存里跟踪：进程重启后第一次监控触发会新建记录，之后继续合并。
	watchMerge map[int64]watchMergeRecord

	cronRunner *cron.Cron
	cronIDs    map[int64]cron.EntryID

	ctx    context.Context
	cancel context.CancelFunc
}

func NewService(store *sqlite.Store) *Service {
	return &Service{
		store:      store,
		states:     make(map[int64]*taskState),
		cancels:    make(map[int64]context.CancelFunc),
		watchMerge: make(map[int64]watchMergeRecord),
		cronIDs:    make(map[int64]cron.EntryID),
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

// CancelTask 请求停止正在执行的备份任务。
// 停止是协作式的：已经在传的那个文件会传完，之后立刻收尾并记为「已停止」，
// 不会执行「从目标同步删除」——中途停下的 sourceIndex 不完整，继续同步删除会误删目标端文件。
func (s *Service) CancelTask(taskID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.states[taskID]
	if !ok || state == nil || !state.Running {
		return false, fmt.Errorf("该备份任务未在执行中")
	}
	state.CancelRequested = true
	state.Phase = "正在停止"
	return true, nil
}

// cancelled 读取停止标记（由 execute 的循环检查点调用）。
func (s *Service) cancelled(taskID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.states[taskID]
	return ok && state != nil && state.CancelRequested
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

		s.mu.Lock()
		if state, ok := s.states[task.ID]; ok {
			// 手动停止优先于「成功 / 失败」判定：不算失败，措辞也换成停止。
			if state.CancelRequested {
				status = "cancelled"
			}
			state.Running = false
			state.CancelRequested = false
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

		summary := describeBackupRunSummary(status, stats.Scanned, stats.Copied, stats.Skipped, stats.Deleted, stats.Failed)
		s.log(task.ID, "%s", summary)

		// 实时监控的连续触发会合并进同一条历史（见 recordRunHistory）。
		// 卡片上的「上次备份」结果跟着用写回后的累计值，与归巢历史 / 任务日志保持一致。
		if item, ok := s.recordRunHistory(task, triggerMode, status, startedAt, stats); ok {
			_ = s.store.UpdateBackupRunResult(task.ID, item.Status, item.Summary,
				startedAt.Format(time.RFC3339), item.ProcessedFiles, item.SuccessCount, item.SkipCount, item.DeletedCount)
		}
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

	// 收集源文件清单（「目标端相对路径」-> 源绝对路径），供同步删除使用。
	//
	// 键是**目标端**的相对路径：单源任务就是「相对源目录的路径」（保持既有目标结构不变）；
	// 多源任务会多一层「源目录名」前缀（见 sourceDirPrefixes）—— 否则两个源里同名相对
	// 路径（例如都有 电影/流浪地球.mkv）会互相覆盖，后者静默盖掉前者的键，
	// 目标端只剩一个文件，等于悄悄丢了数据。
	sourceIndex := make(map[string]string)
	sourcePrefixes := sourceDirPrefixes(task.SourceDirs)
	totalSources := len(task.SourceDirs)
	// 筛选规则命中的目录 / 文件只累计数目：逐条打日志会把「最近日志」整屏刷成
	// 「按筛选规则跳过…」，用户只需要知道跳过了多少（明细里也只有一个 skip 计数）。
	skippedDirs := 0
	skippedFiles := 0

	for index, sourceDir := range task.SourceDirs {
		s.setPhase(task.ID, "扫描源目录", fmt.Sprintf("%d/%d", index+1, totalSources))

		// 多源任务：说明该源落到目标下的哪一层子目录，方便对照目标端结构。
		if prefix := sourcePrefixes[index]; prefix != "" {
			s.log(task.ID, "多源备份：源目录「%s」备份到目标下的「%s」子目录",
				filepath.Base(filepath.Clean(sourceDir)), prefix)
		}

		info, err := os.Stat(sourceDir)
		if err != nil || !info.IsDir() {
			stats.Failed++
			s.log(task.ID, "源目录不可访问：%s", sourceDir)
			continue
		}

		err = filepath.WalkDir(sourceDir, func(currentPath string, entry os.DirEntry, walkErr error) error {
			// 停止请求优先于一切：SkipAll 让 WalkDir 干净地结束，不再往下扫。
			if s.cancelled(task.ID) {
				return filepath.SkipAll
			}
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
					skippedDirs++
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

			// 筛选规则一律按「相对源目录的路径」判定，与目标端前缀无关。
			if matcher.excluded(relative, entry.Name(), false, fileInfo.Size()) {
				stats.Skipped++
				skippedFiles++
				stats.recordFile(model.BackupFileEntry{
					Path:   filepath.ToSlash(relative),
					Action: model.BackupFileActionSkip,
					Size:   fileInfo.Size(),
					Note:   "未通过筛选规则",
				})
				return nil
			}

			sourceIndex[sourceKey(sourcePrefixes[index], relative)] = currentPath
			return nil
		})
		if err != nil {
			stats.Failed++
			s.log(task.ID, "扫描源目录失败：%s（%v）", sourceDir, err)
		}
	}

	// 筛选规则命中的目录 / 文件不再逐条列日志，这里给一行汇总（数目为准）。
	if skippedDirs > 0 || skippedFiles > 0 {
		s.log(task.ID, "按筛选规则跳过 %d 个目录、%d 个文件", skippedDirs, skippedFiles)
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
			desc.client = webdav.NewClient(*credential)
		}
		targets = append(targets, desc)
	}

	// 「删除源文件」类完成规则只在**所有配置目标都可用**时才执行：
	// 目标解析失败 / 挂载停用的会被剔除出 targets，此时源文件并没有真正落到每个目标，
	// 删掉就是数据丢失。
	canDeleteSources := canDeleteSourceAfterCopy(task.CompletionRule, len(targets), len(task.TargetDirs))
	if isSourceDeletingRule(task.CompletionRule) && !canDeleteSources {
		s.log(task.ID, "完成规则「删除源文件」已跳过：目标路径不完整或不可用，避免误删源文件")
	}

	items := make([]sourceItem, 0, len(sourceIndex))
	for relative, absolute := range sourceIndex {
		items = append(items, sourceItem{relative: relative, absolute: absolute})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].relative < items[j].relative })

	// 待删除的源文件：整轮复制结束后统一处理，不在复制循环里「复制一个删一个」。
	pendingDeletes := make([]pendingSourceDelete, 0, len(items))

	for _, item := range items {
		if s.cancelled(task.ID) {
			s.log(task.ID, "已收到停止请求，停止上传剩余文件")
			break
		}

		fileInfo, statErr := os.Stat(item.absolute)
		if statErr != nil {
			continue
		}

		if !incrementalCutoff.IsZero() && !fileInfo.ModTime().After(incrementalCutoff) {
			// 增量扫描：上次备份之后未修改的文件无需重新复制，但仍保留在清单里，
			// 避免「从目标同步删除」误删目标端已有文件。
			continue
		}

		// 逐个目标复制；任一目标失败，这个源文件都不允许按完成规则删除。
		allTargetsOK := true
		for _, target := range targets {
			if target.isWebdav {
				if err := s.copyOneWebdav(task, target, item, fileInfo, stats); err != nil {
					stats.Failed++
					allTargetsOK = false
					s.log(task.ID, "上传失败：%s（%v）", item.relative, err)
					stats.recordFile(model.BackupFileEntry{
						Path:   filepath.ToSlash(item.relative),
						Action: model.BackupFileActionFail,
						Size:   fileInfo.Size(),
						Target: target.raw + webdav.InternalPathFromParts(target.internal, filepath.ToSlash(item.relative)),
						Note:   err.Error(),
					})
				}
			} else {
				destination := filepath.Join(target.raw, item.relative)
				if err := s.copyOne(task, item.relative, item.absolute, destination, fileInfo, stats); err != nil {
					stats.Failed++
					allTargetsOK = false
					s.log(task.ID, "复制失败：%s（%v）", item.relative, err)
					stats.recordFile(model.BackupFileEntry{
						Path:   filepath.ToSlash(item.relative),
						Action: model.BackupFileActionFail,
						Size:   fileInfo.Size(),
						Target: destination,
						Note:   err.Error(),
					})
				}
			}
		}

		// 完成规则（删除源文件）**不在循环里立即删**：多源 / 多目标时，
		// 「复制一个删一个」会让尚未写完的目标永远拿不到这个文件，而源文件删掉就补不回来。
		// 这里只把「本轮确实写到了所有目标」的文件登记起来，等整轮复制结束后统一删。
		if canDeleteSources && allTargetsOK {
			pendingDeletes = append(pendingDeletes, pendingSourceDelete{
				relative: item.relative,
				absolute: item.absolute,
				size:     fileInfo.Size(),
			})
		}
	}

	// 停止执行时不做「删除源文件 / 清理空目录 / 从目标同步删除」：
	// 中途停下的结果并不完整（sourceIndex 可能只扫了一部分），
	// 继续收尾动作会误删源文件 / 目标端已有文件。
	stopped := s.cancelled(task.ID)

	// 收尾动作严格按「复制 → 删除源文件 → 清理空目录 → 从目标同步删除」的顺序。
	if !stopped && len(pendingDeletes) > 0 {
		s.setPhase(task.ID, "删除源文件", fmt.Sprintf("共 %d 个文件", len(pendingDeletes)))
		s.deleteBackedUpSources(task, pendingDeletes, stats)
	}

	if !stopped && task.CompletionRule == model.BackupCompletionDeleteSourceDirs {
		for _, sourceDir := range task.SourceDirs {
			s.pruneEmptyDirs(sourceDir, matcher, stats)
		}
	}

	if !stopped && task.SyncDeleteFromTarget && len(targets) > 0 {
		s.setPhase(task.ID, "同步删除目标", "")
		for _, target := range targets {
			if target.isWebdav {
				s.syncDeleteMissingWebdav(task, target, sourceIndex, matcher, stats)
			} else {
				s.syncDeleteMissing(target.raw, sourceIndex, matcher, stats)
			}
		}
	}

	// 收尾的统计行由 execute 的 defer 统一输出（describeBackupRunSummary），
	// 这里不再重复打一遍，避免「最近日志」里同一件事出现两行。
}

// recordRunHistory 把一次备份执行写入运行日志（run_history），
// 使用 archive_mode = "backup" 作为其专属模式标识，供日志页展示。
//
// 实时监控触发的执行会被「收纳」：距上一条监控记录结束不超过
// watchHistoryMergeWindow 时累加进那条记录，而不是新建 —— 监控每 5 秒轮询一次，
// 源目录持续写入时若每次触发都新建，归巢历史 / 任务日志会刷出几十条「上传 0，跳过 N」。
//
// 返回写入后的记录（written=false 表示存储不可用，什么都没写）。
func (s *Service) recordRunHistory(task model.BackupTask, triggerMode, status string, startedAt time.Time, stats *runStats) (model.RunHistoryItem, bool) {
	if s.store == nil {
		return model.RunHistoryItem{}, false
	}

	if strings.TrimSpace(triggerMode) == "" {
		triggerMode = model.TriggerModeManual
	}

	finishedAt := time.Now().UTC()

	if triggerMode == model.TriggerModeWatch {
		if merged, ok := s.mergeWatchRunHistory(task, status, startedAt, finishedAt, stats); ok {
			return merged, true
		}
	} else {
		// 手动 / 定时执行会打断「一次连续监控」的语义：丢弃合并锚点，
		// 免得后续监控触发被并进一条时间上更早、中间却夹着别的执行的记录里。
		s.forgetWatchRunHistory(task.ID)
	}

	ruleID := task.ID
	item := model.RunHistoryItem{
		ID:             fmt.Sprintf("backup-%d-%d", task.ID, startedAt.UnixNano()),
		RuleID:         &ruleID,
		RuleName:       task.Name,
		TriggerMode:    triggerMode,
		ArchiveMode:    "backup",
		Status:         resolveBackupHistoryStatus(status, stats.Copied, stats.Skipped, stats.Failed),
		ProcessedFiles: stats.Scanned,
		SuccessCount:   stats.Copied,
		SkipCount:      stats.Skipped,
		FailureCount:   stats.Failed,
		DeletedCount:   stats.Deleted,
		Summary:        describeBackupRunSummary(status, stats.Scanned, stats.Copied, stats.Skipped, stats.Deleted, stats.Failed),
		DetailJSON:     stats.buildBackupDetailJSON(),
		StartedAt:      startedAt,
		UpdatedAt:      finishedAt,
		FinishedAt:     &finishedAt,
	}
	if err := s.store.UpsertRunHistory(item); err != nil {
		return model.RunHistoryItem{}, false
	}
	if triggerMode == model.TriggerModeWatch {
		s.rememberWatchRunHistory(task.ID, item)
	}
	return item, true
}

// mergeWatchRunHistory 尝试把本次实时监控执行并入上一条监控历史。
// 命中条件：存在合并锚点，且距那次执行结束不超过 watchHistoryMergeWindow。
// ok=false 表示未合并，调用方改为新建记录。
func (s *Service) mergeWatchRunHistory(task model.BackupTask, status string, startedAt, finishedAt time.Time, stats *runStats) (model.RunHistoryItem, bool) {
	s.mu.RLock()
	anchor, ok := s.watchMerge[task.ID]
	s.mu.RUnlock()
	if !ok || !withinWatchMergeWindow(anchor.finishedAt, startedAt) {
		return model.RunHistoryItem{}, false
	}

	previous, err := s.store.GetRunHistoryByID(anchor.historyID)
	if err != nil {
		// 记录已被清理（例如用户清空日志）：丢掉锚点，下次新建。
		s.forgetWatchRunHistory(task.ID)
		return model.RunHistoryItem{}, false
	}

	merged := mergeWatchRunHistoryItem(previous, task, status, startedAt, finishedAt, stats)
	if err := s.store.UpsertRunHistory(merged); err != nil {
		return model.RunHistoryItem{}, false
	}
	s.rememberWatchRunHistory(task.ID, merged)
	return merged, true
}

// rememberWatchRunHistory 记下最新的监控历史，供窗口内的下一次触发累加。
func (s *Service) rememberWatchRunHistory(taskID int64, item model.RunHistoryItem) {
	finishedAt := item.UpdatedAt
	if item.FinishedAt != nil {
		finishedAt = *item.FinishedAt
	}
	s.mu.Lock()
	s.watchMerge[taskID] = watchMergeRecord{historyID: item.ID, finishedAt: finishedAt}
	s.mu.Unlock()
}

// forgetWatchRunHistory 丢弃合并锚点，使下一次监控触发新建记录。
func (s *Service) forgetWatchRunHistory(taskID int64) {
	s.mu.Lock()
	delete(s.watchMerge, taskID)
	s.mu.Unlock()
}

// withinWatchMergeWindow 判断本次触发是否紧跟上一次监控执行 —— 是则视为同一次连续备份。
func withinWatchMergeWindow(lastFinishedAt, startedAt time.Time) bool {
	if lastFinishedAt.IsZero() {
		return false
	}
	gap := startedAt.Sub(lastFinishedAt)
	if gap < 0 {
		// 时钟回拨或记录时间异常：不合并，免得把两次无关执行粘在一起。
		return false
	}
	return gap <= watchHistoryMergeWindow
}

// mergeWatchRunHistoryItem 把本次监控执行的统计与明细累加进上一条运行历史。
//
// 纯计算（便于单测）：计数按真实数量累加，状态与摘要按累计结果重算。
// started_at / finished_at 都滚动到本次触发 —— 记录代表「这个任务最近的连续备份」，
// 时间停留在最早一次会让它沉到历史列表下方（甚至掉出分页），用户反而找不到。
func mergeWatchRunHistoryItem(previous model.RunHistoryItem, task model.BackupTask, status string, startedAt, finishedAt time.Time, stats *runStats) model.RunHistoryItem {
	ruleID := task.ID
	scanned := previous.ProcessedFiles + stats.Scanned
	copied := previous.SuccessCount + stats.Copied
	skipped := previous.SkipCount + stats.Skipped
	deleted := previous.DeletedCount + stats.Deleted
	failed := previous.FailureCount + stats.Failed

	merged := previous
	merged.RuleID = &ruleID
	merged.RuleName = task.Name
	merged.TriggerMode = model.TriggerModeWatch
	merged.ArchiveMode = "backup"
	merged.Status = resolveBackupHistoryStatus(status, copied, skipped, failed)
	merged.ProcessedFiles = scanned
	merged.SuccessCount = copied
	merged.SkipCount = skipped
	merged.FailureCount = failed
	merged.DeletedCount = deleted
	merged.Summary = describeBackupRunSummary(status, scanned, copied, skipped, deleted, failed)
	merged.DetailJSON = mergeBackupDetailPayload(previous.DetailJSON, stats)
	merged.StartedAt = startedAt
	merged.UpdatedAt = finishedAt
	merged.FinishedAt = &finishedAt
	return merged
}

// resolveBackupHistoryStatus 归一化运行日志的状态：
// 失败优先，其次「一个都没传、全是跳过」记 skip，其余保持传入状态（success / cancelled）。
func resolveBackupHistoryStatus(status string, copied, skipped, failed int) string {
	if failed > 0 || status == "failed" {
		return "failed"
	}
	if status != "failed" && copied == 0 && skipped > 0 {
		return "skip"
	}
	return status
}

// describeBackupRunSummary 生成运行历史的摘要，新建与合并共用同一套措辞，
// 保证「一次连续备份」被合并后读起来仍是完整的一句话。
func describeBackupRunSummary(status string, scanned, copied, skipped, deleted, failed int) string {
	verb := "备份完成"
	if status == "cancelled" {
		verb = "已手动停止"
	}
	return fmt.Sprintf("%s：扫描 %d，上传 %d，跳过 %d，删除 %d，失败 %d",
		verb, scanned, copied, skipped, deleted, failed)
}

// mergeBackupDetailPayload 把本次执行的明细并入上一条历史的载荷（detail_json）。
//
// counts 与 files_total 按真实数量累加；files 追加时仍遵守每个动作
// maxFileEntriesPerAction 的上限，「跳过」照旧只体现在 counts 里、不落明细。
func mergeBackupDetailPayload(previousJSON string, stats *runStats) string {
	merged := model.RunDetail{Kind: model.RunDetailKindBackup}
	if trimmed := strings.TrimSpace(previousJSON); trimmed != "" {
		var stored model.RunDetail
		if err := json.Unmarshal([]byte(trimmed), &stored); err == nil {
			merged = stored
		}
	}
	if strings.TrimSpace(merged.Kind) == "" {
		merged.Kind = model.RunDetailKindBackup
	}
	if merged.Counts == nil {
		merged.Counts = make(map[string]int, 4)
	}

	listed := make(map[string]int, 4)
	for _, entry := range merged.Files {
		listed[entry.Action]++
	}

	truncated := merged.FilesTruncated
	for _, entry := range stats.Files {
		listed[entry.Action]++
		if listed[entry.Action] > maxFileEntriesPerAction {
			truncated = true
			continue
		}
		merged.Files = append(merged.Files, entry)
	}
	for action, count := range stats.fileCounts {
		merged.Counts[action] += count
	}

	// files_total 不含跳过：只累加本次会列出来的明细（上传 / 失败 / 删除）。
	listable := stats.fileTotal - stats.fileCounts[model.BackupFileActionSkip]
	if listable < 0 {
		listable = 0
	}
	merged.FilesTotal += listable
	merged.FilesTruncated = truncated

	total := 0
	for _, count := range merged.Counts {
		total += count
	}
	if len(merged.Files) == 0 && total == 0 {
		return ""
	}

	encoded, err := json.Marshal(merged)
	if err != nil {
		return ""
	}
	return string(encoded)
}

type runStats struct {
	Scanned int
	Copied  int
	Skipped int
	Deleted int
	Failed  int

	// 文件明细：写入 run_history.detail_json，供日志详情查看「备份了什么文件」。
	Files      []model.BackupFileEntry
	fileCounts map[string]int
	fileTotal  int
	truncated  bool
}

// recordFile 采集一条文件明细。每个动作最多保留 maxFileEntriesPerAction 条，
// 但 fileCounts / fileTotal 始终按真实数量累加（详情面板用它显示准确条数）。
//
// 「跳过」只计数、不落明细：归巢历史 / 运行日志只需要知道跳过了多少
// （筛选规则排除、目标同名文件都可能产生海量条目，逐条列出没有意义）。
func (s *runStats) recordFile(entry model.BackupFileEntry) {
	if s == nil || strings.TrimSpace(entry.Path) == "" {
		return
	}

	if s.fileCounts == nil {
		s.fileCounts = make(map[string]int, 4)
	}
	s.fileCounts[entry.Action]++
	s.fileTotal++

	if entry.Action == model.BackupFileActionSkip {
		return
	}

	if s.fileCounts[entry.Action] > maxFileEntriesPerAction {
		s.truncated = true
		return
	}
	s.Files = append(s.Files, entry)
}

// buildBackupDetailJSON 把采集到的文件明细序列化成 run_history.detail_json。
// files_total 只统计会被列出的明细（上传/失败/删除）；跳过是纯计数，只出现在 counts 里。
func (s *runStats) buildBackupDetailJSON() string {
	if s == nil || (len(s.Files) == 0 && s.fileTotal == 0) {
		return ""
	}

	skipCount := s.fileCounts[model.BackupFileActionSkip]
	filesTotal := s.fileTotal - skipCount
	if filesTotal < 0 {
		filesTotal = 0
	}

	encoded, err := json.Marshal(model.BackupDetail{
		Kind:           "backup",
		Files:          s.Files,
		Counts:         s.fileCounts,
		FilesTotal:     filesTotal,
		FilesTruncated: s.truncated,
	})
	if err != nil {
		return ""
	}

	return string(encoded)
}

// sourceDirPrefixes 为每个源目录计算它在目标端的前缀目录名。
//
// 单源返回全空串 —— 目标结构保持原样（源里的相对路径直接落在目标根下），
// 不动既有任务的布局。多源才启用前缀：每个源在目标端各占一层以「源目录名」命名的
// 子目录，这样「多个源 -> 一个目标」「多个源 -> 多个目标」都不会因为跨源同名相对
// 路径互相覆盖（每个目标仍然会收到全部源，只是分目录存放）。
//
// 源目录名重复时（例如 /mnt/a/电影 与 /mnt/b/电影）按出现顺序追加 -2、-3… 后缀，
// 去重按大小写不敏感处理（Windows / macOS 目标端不区分大小写）。
func sourceDirPrefixes(sourceDirs []string) []string {
	prefixes := make([]string, len(sourceDirs))
	if len(sourceDirs) <= 1 {
		return prefixes
	}

	taken := make(map[string]struct{}, len(sourceDirs))
	for index, dir := range sourceDirs {
		base := filepath.Base(filepath.Clean(strings.TrimSpace(dir)))
		if base == "" || base == "." || base == string(filepath.Separator) {
			base = fmt.Sprintf("源%d", index+1)
		}

		name := base
		for suffix := 2; ; suffix++ {
			if _, exists := taken[strings.ToLower(name)]; !exists {
				break
			}
			name = fmt.Sprintf("%s-%d", base, suffix)
		}
		taken[strings.ToLower(name)] = struct{}{}
		prefixes[index] = name
	}
	return prefixes
}

// sourceKey 拼出源文件在目标端的相对路径：单源（prefix 为空）保持原样，多源加一层前缀。
func sourceKey(prefix, relative string) string {
	if prefix == "" {
		return relative
	}
	return filepath.Join(prefix, relative)
}

// sourceItem 是一个待复制的源文件条目。
type sourceItem struct {
	relative string
	absolute string
}

// isSourceDeletingRule 判断完成规则是否会删除源文件。
func isSourceDeletingRule(completionRule string) bool {
	return completionRule == model.BackupCompletionDeleteSource ||
		completionRule == model.BackupCompletionDeleteSourceDirs
}

// canDeleteSourceAfterCopy 判断是否具备「按完成规则删除源文件」的前提条件。
//
// 必须**所有配置的目标都可用**才允许删除：目标解析失败 / 挂载停用的会被剔除出
// usableTargets，此时源文件并没有真正落到每个目标，删掉就是数据丢失。
func canDeleteSourceAfterCopy(completionRule string, usableTargets, configuredTargets int) bool {
	if !isSourceDeletingRule(completionRule) {
		return false
	}
	return usableTargets > 0 && usableTargets == configuredTargets
}

// pendingSourceDelete 是一个「已成功写到所有目标、等待按完成规则删除」的源文件。
type pendingSourceDelete struct {
	// relative 是目标端相对路径（多源任务带源目录名前缀），仅用于运行明细展示。
	relative string
	absolute string
	size     int64
}

// deleteBackedUpSources 在整轮复制（所有源 × 所有目标）结束后统一删除源文件。
//
// 为什么必须等整轮结束：多源 / 多目标时每个文件要写到 N 个目标，循环里
// 「复制一个删一个」会让还没写完的目标拿不到文件，而源文件删掉就补不回来。
// 传进来的清单已经保证了「每个目标都写成功」，任一目标失败的文件不会出现在这里。
//
// 这里**不把条目从 sourceIndex 移除**：删源只是「本机副本不要了」，
// 目标端刚写好的那份仍是有效备份，必须留在清单里，
// 否则紧随其后的「从目标同步删除」会把它当成陈旧文件删掉。
func (s *Service) deleteBackedUpSources(task model.BackupTask, pending []pendingSourceDelete, stats *runStats) {
	for _, item := range pending {
		if err := os.Remove(item.absolute); err != nil {
			stats.Failed++
			stats.recordFile(model.BackupFileEntry{
				Path:   filepath.ToSlash(item.relative),
				Action: model.BackupFileActionFail,
				Size:   item.size,
				Note:   "按完成规则删除源文件失败：" + err.Error(),
			})
			continue
		}

		stats.Deleted++
		stats.recordFile(model.BackupFileEntry{
			Path:   filepath.ToSlash(item.relative),
			Action: model.BackupFileActionDelete,
			Size:   item.size,
			Note:   "按完成规则删除源文件",
		})
	}
}

// targetDesc 描述一个备份目标（本地目录或 WebDAV 挂载）。
type targetDesc struct {
	raw      string
	isWebdav bool
	mountID  int64
	internal string
	client   *webdav.Client
}

func (s *Service) copyOne(task model.BackupTask, relative, sourcePath, destination string, info os.FileInfo, stats *runStats) error {
	if _, err := os.Stat(destination); err == nil {
		if task.ReplaceRule != model.BackupReplaceOverwrite {
			stats.Skipped++
			stats.recordFile(model.BackupFileEntry{
				Path:   filepath.ToSlash(relative),
				Action: model.BackupFileActionSkip,
				Size:   info.Size(),
				Target: destination,
				Note:   "目标已存在同名文件，按「同名跳过」处理",
			})
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
	stats.recordFile(model.BackupFileEntry{
		Path:   filepath.ToSlash(relative),
		Action: model.BackupFileActionUpload,
		Size:   info.Size(),
		Target: destination,
	})
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
			stats.recordFile(model.BackupFileEntry{
				Path:   filepath.ToSlash(item.relative),
				Action: model.BackupFileActionSkip,
				Size:   info.Size(),
				Target: target.raw + internalPath,
				Note:   "远端已存在同名文件，按「同名跳过」处理",
			})
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
	stats.recordFile(model.BackupFileEntry{
		Path:   filepath.ToSlash(item.relative),
		Action: model.BackupFileActionUpload,
		Size:   info.Size(),
		Target: target.raw + internalPath,
	})
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
			stats.recordFile(model.BackupFileEntry{
				Path:   filepath.ToSlash(relative),
				Action: model.BackupFileActionFail,
				Target: currentPath,
				Note:   "从目标同步删除失败：" + err.Error(),
			})
			return nil
		}
		stats.Deleted++
		stats.recordFile(model.BackupFileEntry{
			Path:   filepath.ToSlash(relative),
			Action: model.BackupFileActionDelete,
			Target: currentPath,
			Note:   "源中已不存在，从目标同步删除",
		})
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
		if !ruleAppliesTo(rule, isDir) {
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
		if !ruleAppliesTo(rule, isDir) {
			continue
		}
		hasWhitelist = true
		break
	}

	// 存在适用于本条目的白名单规则却一条都没命中 → 排除。
	// 扩展名白名单因此就是「允许清单」：只有命中的扩展名才会进入备份清单，其余文件直接被无视。
	if hasWhitelist && !whitelistMatched {
		return true
	}

	return false
}

// ruleAppliesTo 判断一条规则是否会作用于该条目：既看 match_dir / match_file 开关，
// 也看规则类型本身。扩展名与体积只对文件有意义，所以即便勾了「文件夹」也不该剪目录，
// 否则一条扩展名白名单会连带剪掉整棵目录树，最终一个文件都匹配不到。
func ruleAppliesTo(rule model.BackupFilterRule, isDir bool) bool {
	if !isDir {
		return rule.MatchFile
	}
	if !rule.MatchDir {
		return false
	}
	return rule.Type != model.BackupFilterExtension && rule.Type != model.BackupFilterSize
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
	// 兼容旧的单值写法（value 里用逗号/分号/空格/竖线分隔多个扩展名）。
	if value != "" {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '|'
		}) {
			candidates = append(candidates, part)
		}
	}
	// 归一必须在取值之后统一做：前端存的是「无点小写」（如 mp4），
	// 这里补上前导点后配合 EqualFold 才能匹配到 .mp4 / .MP4。
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
