package model

import "time"

type Rule struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Enabled            bool      `json:"enabled"`
	MonitorEnabled     bool      `json:"monitor_enabled"`
	ArchiveMode        string    `json:"archive_mode"`
	RunMode            string    `json:"run_mode"`
	SourceDir          string    `json:"source_dir"`
	TargetDir          string    `json:"target_dir"`
	WatchDebounceMS    int       `json:"watch_debounce_ms"`
	CronExpression     string    `json:"cron_expression"`
	RunOnStart         bool      `json:"run_on_start"`
	OptionsJSON        string    `json:"options_json"`
	PackageOptionsJSON string    `json:"package_options_json"`
	CollectOptionsJSON string    `json:"collect_options_json"`
	FiltersJSON        string    `json:"filters_json"`
	LastRunStatus      string    `json:"last_run_status"`
	LastSuccessCount   int       `json:"last_success_count"`
	LastSkipCount      int       `json:"last_skip_count"`
	LastFailureCount   int       `json:"last_failure_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateRuleInput struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Enabled         *bool           `json:"enabled"`
	MonitorEnabled  *bool           `json:"monitor_enabled"`
	ArchiveMode     string          `json:"archive_mode"`
	RunMode         string          `json:"run_mode"`
	SourceDir       string          `json:"source_dir"`
	TargetDir       string          `json:"target_dir"`
	WatchDebounceMS int             `json:"watch_debounce_ms"`
	CronExpression  string          `json:"cron_expression"`
	RunOnStart      *bool           `json:"run_on_start"`
	PackageOptions  map[string]bool `json:"package_options"`
	CollectOptions  map[string]bool `json:"collect_options"`
}

type UpdateRuleInput struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Enabled         *bool           `json:"enabled"`
	MonitorEnabled  *bool           `json:"monitor_enabled"`
	ArchiveMode     string          `json:"archive_mode"`
	RunMode         string          `json:"run_mode"`
	SourceDir       string          `json:"source_dir"`
	TargetDir       string          `json:"target_dir"`
	WatchDebounceMS int             `json:"watch_debounce_ms"`
	CronExpression  string          `json:"cron_expression"`
	RunOnStart      *bool           `json:"run_on_start"`
	PackageOptions  map[string]bool `json:"package_options"`
	CollectOptions  map[string]bool `json:"collect_options"`
}
