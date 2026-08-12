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

	run := s.newRun(model.TriggerModeManual, "manual", "", nil, "manual-preflight")
	run.Stage = model.RunStagePreflight
	s.appendLog(run.ID, "info", fmt.Sprintf("已生成手动预检任务：%s", sourceDir))

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

	run := s.newRun(model.TriggerModeManual, "collect", "", nil, "manual-collect")
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

	s.appendLog(run.ID, "info", fmt.Sprintf("手动收集任务已提交，共 %d 个文件夹", len(cleanSources)))
	for _, path := range cleanSources {
		s.appendLog(run.ID, "info", fmt.Sprintf("收集根目录：%s", path))
	}
	s.appendLog(run.ID, "info", fmt.Sprintf("收集后清理子文件夹：%t", removeSubfolders))
	for _, path := range collectedPaths {
		s.appendLog(run.ID, "info", fmt.Sprintf("文件已收集至：%s", path))
	}

	s.persistRunHistory(run.ID, fmt.Sprintf("手动收集完成：共处理 %d 个文件夹", len(collectedPaths)), &executionStats{
		ProcessedFiles: len(cleanSources),
		SuccessCount:   len(collectedPaths),
		Summary:        fmt.Sprintf("手动收集完成：共处理 %d 个文件夹", len(collectedPaths)),
	})
}
