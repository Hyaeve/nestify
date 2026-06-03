package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func PrepareMode(req ExecuteRuleRequest) (*PreparedMode, error) {
	targetDirSummary := buildTargetDirSummary(req)

	switch req.ArchiveMode {
	case "package":
		return &PreparedMode{
			ArchiveMode: req.ArchiveMode,
			RuleType:    req.RuleType,
			SourceDir:   req.SourceDir,
			TargetDir:   req.TargetDir,
			Summary:     joinPreparedSummary("package mode skeleton prepared", targetDirSummary),
		}, nil
	case "collect":
		return &PreparedMode{
			ArchiveMode: req.ArchiveMode,
			RuleType:    req.RuleType,
			SourceDir:   req.SourceDir,
			TargetDir:   req.TargetDir,
			Summary:     joinPreparedSummary("collect mode skeleton prepared", targetDirSummary),
		}, nil
	case "cleanup":
		actions := make([]string, 0, 2)
		if req.Options["cleanup_empty_dirs"] {
			actions = append(actions, "empty directories")
		}
		if req.Options["cleanup_matching_files"] {
			actions = append(actions, fmt.Sprintf("matched files (%d filters)", len(req.Filters)))
		}
		summary := "cleanup mode skeleton prepared"
		if len(actions) > 0 {
			summary = fmt.Sprintf("cleanup mode skeleton prepared: %s", strings.Join(actions, ", "))
		}
		return &PreparedMode{
			ArchiveMode: req.ArchiveMode,
			RuleType:    req.RuleType,
			SourceDir:   req.SourceDir,
			TargetDir:   req.TargetDir,
			Summary:     joinPreparedSummary(summary, targetDirSummary),
		}, nil
	case "link":
		modeLabel := "软链"
		if strings.TrimSpace(req.LinkMode) == "hard" {
			modeLabel = "硬链"
		}
		return &PreparedMode{
			ArchiveMode: req.ArchiveMode,
			RuleType:    req.RuleType,
			SourceDir:   req.SourceDir,
			TargetDir:   req.TargetDir,
			Summary:     joinPreparedSummary(fmt.Sprintf("%s模式链路规则已准备", modeLabel), targetDirSummary),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported archive mode: %s", req.ArchiveMode)
	}
}

func buildTargetDirSummary(req ExecuteRuleRequest) string {
	archiveMode := strings.TrimSpace(req.ArchiveMode)
	if archiveMode == "cleanup" {
		return ""
	}

	targetDir := strings.TrimSpace(req.TargetDir)
	if targetDir == "" || targetDir == "." {
		return ""
	}

	cleanTargetDir := filepath.Clean(targetDir)
	info, err := os.Stat(cleanTargetDir)
	if err == nil {
		if info.IsDir() {
			return "目标目录已存在"
		}
		return "目标路径已存在但不是文件夹"
	}

	if os.IsNotExist(err) {
		return "目标目录不存在，执行时将自动创建"
	}

	return ""
}

func joinPreparedSummary(base, suffix string) string {
	if strings.TrimSpace(suffix) == "" {
		return base
	}
	return fmt.Sprintf("%s；%s", base, suffix)
}
