package executor

import "fmt"

func PrepareMode(req ExecuteRuleRequest) (*PreparedMode, error) {
	switch req.ArchiveMode {
	case "package":
		return &PreparedMode{
			ArchiveMode: req.ArchiveMode,
			SourceDir:   req.SourceDir,
			TargetDir:   req.TargetDir,
			Summary:     "package mode skeleton prepared",
		}, nil
	case "collect":
		return &PreparedMode{
			ArchiveMode: req.ArchiveMode,
			SourceDir:   req.SourceDir,
			TargetDir:   req.TargetDir,
			Summary:     "collect mode skeleton prepared",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported archive mode: %s", req.ArchiveMode)
	}
}
