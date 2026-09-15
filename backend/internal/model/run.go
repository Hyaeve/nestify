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
	LinkMode           string     `json:"link_mode,omitempty"`
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
	LinkMode       string     `json:"link_mode,omitempty"`
	Status         string     `json:"status"`
	ProcessedFiles int        `json:"processed_files"`
	SuccessCount   int        `json:"success_count"`
	SkipCount      int        `json:"skip_count"`
	FailureCount   int        `json:"failure_count"`
	DeletedCount   int        `json:"deleted_count"`
	SizeBytes      int64      `json:"size_bytes"`
	Summary        string     `json:"summary"`
	DetailJSON     string     `json:"detail_json,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}

// 备份文件明细的动作标识：上传成功 / 跳过 / 失败 / 删除。
const (
	BackupFileActionUpload = "upload"
	BackupFileActionSkip   = "skip"
	BackupFileActionFail   = "fail"
	BackupFileActionDelete = "delete"
)

// BackupFileEntry 记录备份执行中单个文件（或文件夹）的处理结果。
// 写入 run_history.detail_json，供运行日志 / 归巢历史的详情展开查看「备份了什么」。
// 策划上只收录真的动了文件的结果（上传/失败/删除）；「跳过」只计数、不逐条列明细。
type BackupFileEntry struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Size   int64  `json:"size,omitempty"`
	Target string `json:"target,omitempty"`
	Note   string `json:"note,omitempty"`
	Dir    bool   `json:"dir,omitempty"`
}

// BackupDetail 是备份任务的详情载荷（run_history.detail_json）。
// Files 为明细列表（上传/失败/删除，每个动作只保留有限条数，见 backup.maxFileEntriesPerAction）；
// Counts 是采集到的真实数量（含只计数的 skip），FilesTotal 是明细总数（不含跳过），均不受截断影响。
type BackupDetail struct {
	Kind           string            `json:"kind"`
	Files          []BackupFileEntry `json:"files"`
	Counts         map[string]int    `json:"counts,omitempty"`
	FilesTotal     int               `json:"files_total"`
	FilesTruncated bool              `json:"files_truncated,omitempty"`
}

type RunHistorySummary struct {
	Total   int `json:"total"`
	Today   int `json:"today"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
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
