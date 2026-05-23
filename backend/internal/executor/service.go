package executor

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nestify/backend/internal/model"
)

type Service struct {
	mu      sync.RWMutex
	runs    map[string]*model.RunInstance
	logs    map[string][]model.RunLogEntry
	history []model.RunHistoryItem
}

func NewService() *Service {
	return &Service{
		runs:    make(map[string]*model.RunInstance),
		logs:    make(map[string][]model.RunLogEntry),
		history: make([]model.RunHistoryItem, 0),
	}
}

func (s *Service) ListHistory() []model.RunHistoryItem {
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
			return
		}

		entries, scanErr := s.scanSource(req)
		if scanErr != nil {
			s.finishRun(runID, model.RunStatusFailed, model.RunStageFinalizing, scanErr.Error())
			return
		}

		skipCount := 0
		successCount := len(entries)
		status := model.RunStatusSucceeded
		if len(entries) == 0 {
			skipCount = 1
			successCount = 0
			status = model.RunStatusFailed
		}

		s.mu.Lock()
		if run, ok := s.runs[runID]; ok {
			run.Stage = model.RunStageScanning
			run.ProcessedFiles = len(entries)
			run.SuccessCount = successCount
			run.SkipCount = skipCount
			run.Status = status
			run.FinishedAt = ptrTime(time.Now().UTC())
			run.UpdatedAt = time.Now().UTC()
		}
		s.mu.Unlock()
		s.appendLog(runID, "info", prepared.Summary)
		s.appendLog(runID, "info", fmt.Sprintf("processed %d files", len(entries)))
		s.appendLog(runID, "info", "execution completed")
		s.recordHistory(runID, prepared.Summary)
	}()
}

func (s *Service) scanSource(req ExecuteRuleRequest) ([]string, error) {
	if strings.TrimSpace(req.SourceDir) == "" {
		return nil, fmt.Errorf("source dir is required")
	}
	items, err := filepath.Glob(filepath.Join(req.SourceDir, "*"))
	if err != nil {
		return nil, err
	}
	return items, nil
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

func (s *Service) recordHistory(runID, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	run, ok := s.runs[runID]
	if !ok || run == nil {
		return
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
