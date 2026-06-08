package executor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"nestify/backend/internal/model"
)

func (s *Service) PrepareManualPreflight(req model.ManualPreflightRequest) (*model.RunInstance, *model.ManualPreflightResult, error) {
	sourceDir := strings.TrimSpace(req.SourceDir)
	if sourceDir == "" {
		return nil, nil, fmt.Errorf("source_dir is required")
	}

	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		outputDir = filepath.Dir(sourceDir)
	}

	run := s.newRun(model.TriggerModeManual, "manual", nil, "manual-preflight")
	run.Stage = model.RunStagePreflight
	s.appendLog(run.ID, "info", fmt.Sprintf("prepared manual preflight skeleton for %s", sourceDir))

	result := &model.ManualPreflightResult{
		SourceDir:         sourceDir,
		OutputDir:         outputDir,
		Allowed:           true,
		RejectedReasons:   []string{},
		ExecutionSkeleton: true,
	}

	return s.cloneRun(run), result, nil
}

func (s *Service) RecordManualCollectRun(sourcePaths []string, collectedPaths []string, removeSubfolders bool) {
	cleanSources := make([]string, 0, len(sourcePaths))
	for _, path := range sourcePaths {
		trimmed := strings.TrimSpace(path)
		if trimmed != "" {
			cleanSources = append(cleanSources, trimmed)
		}
	}

	run := s.newRun(model.TriggerModeManual, "collect", nil, "manual-collect")
	now := time.Now().UTC()

	s.mu.Lock()
	if currentRun, ok := s.runs[run.ID]; ok {
		currentRun.Status = model.RunStatusSucceeded
		currentRun.Stage = model.RunStageFinalizing
		currentRun.ProcessedFiles = len(cleanSources)
		currentRun.SuccessCount = len(collectedPaths)
		currentRun.SkipCount = 0
		currentRun.FailureCount = 0
		currentRun.UpdatedAt = now
		currentRun.FinishedAt = &now
	}
	s.mu.Unlock()

	s.appendLog(run.ID, "info", fmt.Sprintf("manual collect requested for %d folder(s)", len(cleanSources)))
	for _, path := range cleanSources {
		s.appendLog(run.ID, "info", fmt.Sprintf("collect root: %s", path))
	}
	s.appendLog(run.ID, "info", fmt.Sprintf("remove subfolders: %t", removeSubfolders))
	for _, path := range collectedPaths {
		s.appendLog(run.ID, "info", fmt.Sprintf("collected files into %s", path))
	}

	s.persistRunHistory(run.ID, fmt.Sprintf("manual collect completed: %d folder(s)", len(collectedPaths)), &executionStats{
		ProcessedFiles: len(cleanSources),
		SuccessCount:   len(collectedPaths),
		Summary:        fmt.Sprintf("manual collect completed: %d folder(s)", len(collectedPaths)),
	})
}
