package model

import "time"

type Rule struct {
	ID                   int64     `json:"id"`
	SortOrder            int       `json:"sort_order"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	Enabled              bool      `json:"enabled"`
	MonitorEnabled       bool      `json:"monitor_enabled"`
	CompatibilityMode    string    `json:"compatibility_mode"`
	ArchiveMode          string    `json:"archive_mode"`
	RuleType             string    `json:"rule_type"`
	LinkMode             string    `json:"link_mode"`
	RunMode              string    `json:"run_mode"`
	SourceDir            string    `json:"source_dir"`
	SourceDirs           []string  `json:"source_dirs,omitempty"`
	TargetDir            string    `json:"target_dir"`
	WatchDebounceMS      int       `json:"watch_debounce_ms"`
	CronExpression       string    `json:"cron_expression"`
	RunOnStart           bool      `json:"run_on_start"`
	OptionsJSON          string    `json:"options_json"`
	OptionValuesJSON     string    `json:"option_values_json"`
	PackageOptionsJSON   string    `json:"package_options_json"`
	CollectOptionsJSON   string    `json:"collect_options_json"`
	FiltersJSON          string    `json:"filters_json"`
	WhitelistJSON        string    `json:"whitelist_json"`
	MatchFiltersJSON     string    `json:"match_filters_json"`
	NestFiltersJSON      string    `json:"nest_filters_json"`
	TransformRulesJSON   string    `json:"transform_rules_json"`
	TransformFiltersJSON string    `json:"transform_filters_json"`
	LastRunStatus        string    `json:"last_run_status"`
	LastSuccessCount     int       `json:"last_success_count"`
	LastSkipCount        int       `json:"last_skip_count"`
	LastFailureCount     int       `json:"last_failure_count"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type RuleReorderItem struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sort_order"`
}

type CreateRuleInput struct {
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	Enabled           *bool           `json:"enabled"`
	MonitorEnabled    *bool           `json:"monitor_enabled"`
	CompatibilityMode string          `json:"compatibility_mode"`
	ArchiveMode       string          `json:"archive_mode"`
	RuleType          string          `json:"rule_type"`
	LinkMode          string          `json:"link_mode"`
	RunMode           string          `json:"run_mode"`
	SourceDir         string          `json:"source_dir"`
	SourceDirs        []string        `json:"source_dirs"`
	TargetDir         string          `json:"target_dir"`
	WatchDebounceMS   int             `json:"watch_debounce_ms"`
	CronExpression    string          `json:"cron_expression"`
	RunOnStart        *bool           `json:"run_on_start"`
	Options           map[string]bool `json:"options"`
	OptionValues      map[string]int  `json:"option_values"`
	PackageOptions    map[string]bool `json:"package_options"`
	CollectOptions    map[string]bool `json:"collect_options"`
	Filters           []string        `json:"filters"`
	Whitelist         []string        `json:"whitelist"`
	MatchFilters      []string        `json:"match_filters"`
	NestFilters       []string        `json:"nest_filters"`
	TransformRules    []string        `json:"transform_rules"`
	TransformFilters  []string        `json:"transform_filters"`
}

type UpdateRuleInput struct {
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	Enabled           *bool           `json:"enabled"`
	MonitorEnabled    *bool           `json:"monitor_enabled"`
	CompatibilityMode string          `json:"compatibility_mode"`
	ArchiveMode       string          `json:"archive_mode"`
	RuleType          string          `json:"rule_type"`
	LinkMode          string          `json:"link_mode"`
	RunMode           string          `json:"run_mode"`
	SourceDir         string          `json:"source_dir"`
	SourceDirs        []string        `json:"source_dirs"`
	TargetDir         string          `json:"target_dir"`
	WatchDebounceMS   int             `json:"watch_debounce_ms"`
	CronExpression    string          `json:"cron_expression"`
	RunOnStart        *bool           `json:"run_on_start"`
	Options           map[string]bool `json:"options"`
	OptionValues      map[string]int  `json:"option_values"`
	PackageOptions    map[string]bool `json:"package_options"`
	CollectOptions    map[string]bool `json:"collect_options"`
	Filters           []string        `json:"filters"`
	Whitelist         []string        `json:"whitelist"`
	MatchFilters      []string        `json:"match_filters"`
	NestFilters       []string        `json:"nest_filters"`
	TransformRules    []string        `json:"transform_rules"`
	TransformFilters  []string        `json:"transform_filters"`
}

type RuleBackup struct {
	Version    string `json:"version"`
	ExportedAt string `json:"exported_at"`
	Rules      []Rule `json:"rules"`
}
