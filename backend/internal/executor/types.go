package executor

type ExecuteRuleRequest struct {
	RuleID      int64
	RuleName    string
	ArchiveMode string
	TriggerMode string
	SourceDir   string
	TargetDir   string
}

type PreparedMode struct {
	ArchiveMode string
	SourceDir   string
	TargetDir   string
	Summary     string
}
