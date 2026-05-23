package executor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
}

func NewService(store *sqlite.Store) *Service {
	return &Service{
		store:        store,
		runs:         make(map[string]*model.RunInstance),
		logs:         make(map[string][]model.RunLogEntry),
		history:      make([]model.RunHistoryItem, 0),
		watchCancels: make(map[int64]context.CancelFunc),
		activeRules:  make(map[int64]struct{}),
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

	items := make([]model.RunHistoryItem, len(s.history))
	copy(items, s.history)
	return items
}

func (s *Service) PrepareRuleRun(req ExecuteRuleRequest) (*model.RunInstance, error) {
	archiveMode := strings.TrimSpace(req.ArchiveMode)
	if archiveMode != "package" && archiveMode != "collect" {
		return nil, fmt.Errorf("unsupported archive mode: %s", archiveMode)
	}

	triggerMode := strings.TrimSpace(req.TriggerMode)
	if triggerMode == "" {
		triggerMode = model.TriggerModeOnce
	}

	ruleID := req.RuleID
	if ruleID > 0 && !s.markRuleActive(ruleID) {
		return nil, fmt.Errorf("rule is already running")
	}
	run := s.newRun(triggerMode, archiveMode, &ruleID, req.RuleName)
	s.appendLog(run.ID, "info", fmt.Sprintf("prepared %s execution skeleton for rule %q", archiveMode, req.RuleName))
	s.appendLog(run.ID, "info", fmt.Sprintf("source=%s target=%s", req.SourceDir, req.TargetDir))
	s.runExecution(run.ID, req)

	return s.cloneRun(run), nil
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

func (s *Service) ListRunLogs(runID string) []model.RunLogEntry {
	s.mu.RLock()
	entries := s.logs[runID]
	s.mu.RUnlock()

	cloned := make([]model.RunLogEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

func (s *Service) newRun(triggerMode, archiveMode string, ruleID *int64, ruleName string) *model.RunInstance {
	now := time.Now().UTC()
	run := &model.RunInstance{
		ID:          mustRandomID(),
		RuleID:      ruleID,
		RuleName:    ruleName,
		TriggerMode: triggerMode,
		ArchiveMode: archiveMode,
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
	go func() {
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
			s.finishRun(runID, model.RunStatusFailed, model.RunStageFinalizing, fmt.Sprintf("execution failed: %v", err))
			s.persistRunHistory(runID, fmt.Sprintf("execution failed: %v", err))
			return
		}

		stats, execErr := s.executeRule(runID, req)
		if execErr != nil && stats.FailureCount == 0 {
			stats.FailureCount = 1
		}

		finalStatus := model.RunStatusSucceeded
		if stats.FailureCount > 0 || execErr != nil {
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

		if execErr != nil {
			s.appendLog(runID, "error", execErr.Error())
		} else {
			s.appendLog(runID, "info", prepared.Summary)
			s.appendLog(runID, "info", stats.Summary)
			s.appendLog(runID, "info", "execution completed")
		}

		if req.RuleID > 0 && s.store != nil {
			_ = s.store.UpdateRuleExecutionStats(req.RuleID, mapRunStatusByCounts(stats.SuccessCount, stats.SkipCount, stats.FailureCount), stats.SuccessCount, stats.SkipCount, stats.FailureCount)
		}
		s.persistRunHistory(runID, stats.Summary)
		if execErr != nil {
			return
		}
	}()
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

func (s *Service) persistRunHistory(runID, summary string) {
	item := s.recordHistory(runID, summary)
	if item == nil || s.store == nil {
		return
	}
	_ = s.store.UpsertRunHistory(*item)
}

func (s *Service) recordHistory(runID, summary string) *model.RunHistoryItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[runID]
	if !ok || run == nil {
		return nil
	}

	item := model.RunHistoryItem{
		ID:             run.ID,
		RuleID:         run.RuleID,
		RuleName:       run.RuleName,
		TriggerMode:    run.TriggerMode,
		ArchiveMode:    run.ArchiveMode,
		Status:         mapRunStatus(run),
		ProcessedFiles: run.ProcessedFiles,
		SuccessCount:   run.SuccessCount,
		SkipCount:      run.SkipCount,
		FailureCount:   run.FailureCount,
		Summary:        summary,
		StartedAt:      run.StartedAt,
		UpdatedAt:      run.UpdatedAt,
		FinishedAt:     run.FinishedAt,
	}

	s.history = append([]model.RunHistoryItem{item}, s.history...)
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

func mapRunStatusByCounts(successCount, skipCount, failureCount int) string {
	if failureCount > 0 {
		return "failed"
	}
	if skipCount > 0 && successCount == 0 {
		return "skip"
	}
	if successCount > 0 {
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
