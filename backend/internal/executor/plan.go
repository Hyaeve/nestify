package executor

import (
	"fmt"
	"strings"
)

func PrepareMode(req ExecuteRuleRequest) (*PreparedMode, error) {
	switch req.ArchiveMode {
	case "package":
		return &PreparedMode{
			ArchiveMode: req.ArchiveMode,
			RuleType:    req.RuleType,
			SourceDir:   req.SourceDir,
			TargetDir:   req.TargetDir,
			Summary:     "package mode skeleton prepared",
		}, nil
	case "collect":
		return &PreparedMode{
			ArchiveMode: req.ArchiveMode,
			RuleType:    req.RuleType,
			SourceDir:   req.SourceDir,
			TargetDir:   req.TargetDir,
			Summary:     "collect mode skeleton prepared",
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
			Summary:     summary,
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
			Summary:     fmt.Sprintf("%s模式链路规则已准备", modeLabel),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported archive mode: %s", req.ArchiveMode)
	}
}
