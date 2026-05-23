package model

import "time"

const (
	TriggerModeWatch  = "watch"
	TriggerModeCron   = "cron"
	TriggerModeOnce   = "once"
	TriggerModeManual = "manual"
)

const (
	RunStatusPending   = "pending"
	RunStatusRunning   = "running"
	RunStatusSucceeded = "succeeded"
	RunStatusFailed    = "failed"
)

const (
	RunStageQueued     = "queued"
	RunStagePreflight  = "preflight"
	RunStageDispatch   = "dispatch"
	RunStageScanning   = "scanning"
	RunStageProcessing = "processing"
	RunStageFinalizing = "finalizing"
)

type RunInstance struct {
	ID                 string     `json:"id"`
	RuleID             *int64     `json:"rule_id,omitempty"`
	RuleName           string     `json:"rule_name,omitempty"`
	TriggerMode        string     `json:"trigger_mode"`
	ArchiveMode        string     `json:"archive_mode,omitempty"`
	Status             string     `json:"status"`
	Stage              string     `json:"stage"`
	CurrentSeries      string     `json:"current_series,omitempty"`
	CurrentVolumeOrDir string     `json:"current_volume_or_dir,omitempty"`
	ProcessedFiles     int        `json:"processed_files"`
	SuccessCount       int        `json:"success_count"`
	SkipCount          int        `json:"skip_count"`
	FailureCount       int        `json:"failure_count"`
	StartedAt          time.Time  `json:"started_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	FinishedAt         *time.Time `json:"finished_at,omitempty"`
}

type RunLogEntry struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type RunHistoryItem struct {
	ID             string     `json:"id"`
	RuleID         *int64     `json:"rule_id,omitempty"`
	RuleName       string     `json:"rule_name,omitempty"`
	TriggerMode    string     `json:"trigger_mode"`
	ArchiveMode    string     `json:"archive_mode,omitempty"`
	Status         string     `json:"status"`
	ProcessedFiles int        `json:"processed_files"`
	SuccessCount   int        `json:"success_count"`
	SkipCount      int        `json:"skip_count"`
	FailureCount   int        `json:"failure_count"`
	Summary        string     `json:"summary"`
	StartedAt      time.Time  `json:"started_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

type ManualPreflightRequest struct {
	SourceDir string `json:"source_dir"`
	OutputDir string `json:"output_dir,omitempty"`
}

type ManualPreflightResult struct {
	SourceDir         string   `json:"source_dir"`
	OutputDir         string   `json:"output_dir"`
	Allowed           bool     `json:"allowed"`
	ImageCount        int      `json:"image_count"`
	HasNestedDirs     bool     `json:"has_nested_dirs"`
	HasNonImageFiles  bool     `json:"has_non_image_files"`
	RejectedReasons   []string `json:"rejected_reasons"`
	ExecutionSkeleton bool     `json:"execution_skeleton"`
}
