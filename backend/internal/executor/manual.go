package executor

import (
	"fmt"
	"path/filepath"
	"strings"

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
