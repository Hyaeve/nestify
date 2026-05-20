package executor

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"nestify/backend/internal/model"
)

type Service struct {
	mu   sync.RWMutex
	runs map[string]*model.RunInstance
	logs map[string][]model.RunLogEntry
}

func NewService() *Service {
	return &Service{
		runs: make(map[string]*model.RunInstance),
		logs: make(map[string][]model.RunLogEntry),
	}
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
