package executor

type ExecuteRuleRequest struct {
	RuleID         int64
	RuleName       string
	ArchiveMode    string
	TriggerMode    string
	SourceDir      string
	TargetDir      string
	PackageOptions map[string]bool
}

type PreparedMode struct {
	ArchiveMode string
	SourceDir   string
	TargetDir   string
	Summary     string
}
