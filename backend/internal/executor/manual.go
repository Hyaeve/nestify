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

	// 手动收集是一次性任务，这一条就是它的代表行（见 service.persistRunHistoryWithDetail）。
	s.persistRunHistoryWithDetail(run.ID, fmt.Sprintf("手动收集完成：共处理 %d 个文件夹", len(collectedPaths)), &executionStats{
		ProcessedFiles: len(cleanSources),
		SuccessCount:   len(collectedPaths),
		Summary:        fmt.Sprintf("手动收集完成：共处理 %d 个文件夹", len(collectedPaths)),
	})
}

// RecordManualPackRun 记录一次手动打包（CBZ 压缩）任务。
// 与手动解压 / 收集保持一致：写入内存运行列表 + 运行日志，供仪表盘「任务预览」
// 与运行日志页展示手动压缩的执行记录。
func (s *Service) RecordManualPackRun(sourcePaths []string, outputPaths []string, archiveName string) {
	cleanSources := make([]string, 0, len(sourcePaths))
	for _, path := range sourcePaths {
		trimmed := strings.TrimSpace(path)
		if trimmed != "" {
			cleanSources = append(cleanSources, trimmed)
		}
	}

	cleanOutputs := make([]string, 0, len(outputPaths))
	for _, path := range outputPaths {
		trimmed := strings.TrimSpace(path)
		if trimmed != "" {
			cleanOutputs = append(cleanOutputs, trimmed)
		}
	}

	run := s.newRun(model.TriggerModeManual, "package", "", nil, "手动压缩")
	now := time.Now().UTC()

	s.mu.Lock()
	if currentRun, ok := s.runs[run.ID]; ok {
		currentRun.Status = model.RunStatusSucceeded
		currentRun.Stage = model.RunStageFinalizing
		currentRun.ProcessedFiles = len(cleanSources)
		currentRun.SuccessCount = len(cleanOutputs)
		currentRun.SkipCount = 0
		currentRun.FailureCount = 0
		currentRun.UpdatedAt = now
		currentRun.FinishedAt = &now
	}
	s.mu.Unlock()

	s.appendLog(run.ID, "info", fmt.Sprintf("手动压缩任务已提交，共 %d 个来源文件夹", len(cleanSources)))
	for _, path := range cleanSources {
		s.appendLog(run.ID, "info", fmt.Sprintf("来源文件夹：%s", path))
	}
	if strings.TrimSpace(archiveName) != "" {
		s.appendLog(run.ID, "info", fmt.Sprintf("压缩包名称：%s", archiveName))
	}
	for _, path := range cleanOutputs {
		s.appendLog(run.ID, "info", fmt.Sprintf("已生成压缩包：%s", path))
	}

	summary := fmt.Sprintf("手动压缩完成：%d 个文件夹，输出 %d 个压缩包", len(cleanSources), len(cleanOutputs))
	s.persistRunHistoryWithDetail(run.ID, summary, &executionStats{
		ProcessedFiles: len(cleanSources),
		SuccessCount:   len(cleanOutputs),
		Summary:        summary,
	})
}
