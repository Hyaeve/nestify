package executor

type ExecuteRuleRequest struct {
	RuleID            int64
	RuleName          string
	ArchiveMode       string
	RuleType          string
	LinkMode          string
	TriggerMode       string
	CompatibilityMode string
	SourceDir         string
	SourceDirs        []string
	TargetDir         string
	Options           map[string]bool
	OptionValues      map[string]int
	PackageOptions    map[string]bool
	CollectOptions    map[string]bool
	Filters           []string
	Whitelist         []string
	MatchFilters      []string
	NestFilters       []string
	TransformRules    []string
	TransformFilters  []string
}

type PreparedMode struct {
	ArchiveMode string
	RuleType    string
	SourceDir   string
	TargetDir   string
	Summary     string
}
