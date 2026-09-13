package model

import "time"

// 备份完成规则。
const (
	BackupCompletionNone             = "none"              // 备份完成后无操作
	BackupCompletionDeleteSource     = "delete_source"     // 删除源文件
	BackupCompletionDeleteSourceDirs = "delete_source_dir" // 删除源文件和空文件夹
)

// 目标同名文件替换规则。
const (
	BackupReplaceSkip      = "skip"      // 跳过
	BackupReplaceOverwrite = "overwrite" // 覆盖
)

// 筛选规则类型。
const (
	BackupFilterName      = "name"
	BackupFilterExtension = "extension"
	BackupFilterRegex     = "regex"
	BackupFilterSize      = "size"
)

// BackupFilterRule 描述一条筛选规则。
type BackupFilterRule struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Value      string   `json:"value"`
	Blacklist  bool     `json:"blacklist"`
	Whitelist  bool     `json:"whitelist"`
	MatchDir   bool     `json:"match_dir"`
	MatchFile  bool     `json:"match_file"`
	MinSize    int64    `json:"min_size"`
	MaxSize    int64    `json:"max_size"`
	SizeUnit   string   `json:"size_unit"`
	Extensions []string `json:"extensions,omitempty"`
}

// BackupTask 描述一个备份任务。
type BackupTask struct {
	ID                   int64              `json:"id"`
	Name                 string             `json:"name"`
	Enabled              bool               `json:"enabled"`
	SourceDirs           []string           `json:"source_dirs"`
	TargetDirs           []string           `json:"target_dirs"`
	MonitorEnabled       bool               `json:"monitor_enabled"`
	CompletionRule       string             `json:"completion_rule"`
	ReplaceRule          string             `json:"replace_rule"`
	SyncDeleteFromTarget bool               `json:"sync_delete_from_target"`
	ForceFullScan        bool               `json:"force_full_scan"`
	ScanIntervalSeconds  int                `json:"scan_interval_seconds"`
	CronExpression       string             `json:"cron_expression"`
	FilterRules          []BackupFilterRule `json:"filter_rules"`
	LastBackupAt         string             `json:"last_backup_at"`
	LastStatus           string             `json:"last_status"`
	LastSummary          string             `json:"last_summary"`
	LastScannedFiles     int                `json:"last_scanned_files"`
	LastCopiedFiles      int                `json:"last_copied_files"`
	LastSkippedFiles     int                `json:"last_skipped_files"`
	LastDeletedFiles     int                `json:"last_deleted_files"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
}

type CreateBackupInput struct {
	Name                 string             `json:"name"`
	Enabled              *bool              `json:"enabled"`
	SourceDirs           []string           `json:"source_dirs"`
	TargetDirs           []string           `json:"target_dirs"`
	MonitorEnabled       *bool              `json:"monitor_enabled"`
	CompletionRule       string             `json:"completion_rule"`
	ReplaceRule          string             `json:"replace_rule"`
	SyncDeleteFromTarget *bool              `json:"sync_delete_from_target"`
	ForceFullScan        *bool              `json:"force_full_scan"`
	ScanIntervalSeconds  int                `json:"scan_interval_seconds"`
	CronExpression       string             `json:"cron_expression"`
	FilterRules          []BackupFilterRule `json:"filter_rules"`
}

type UpdateBackupInput = CreateBackupInput

// BackupStatusSnapshot 是备份任务当前执行情况的快照，用于「状态详情」窗口。
type BackupStatusSnapshot struct {
	TaskID       int64    `json:"task_id"`
	TaskName     string   `json:"task_name"`
	Running      bool     `json:"running"`
	Status       string   `json:"status"`
	Phase        string   `json:"phase"`
	Progress     string   `json:"progress"`
	Scanned      int      `json:"scanned"`
	Copied       int      `json:"copied"`
	Skipped      int      `json:"skipped"`
	Deleted      int      `json:"deleted"`
	Failed       int      `json:"failed"`
	LastBackupAt string   `json:"last_backup_at"`
	RecentLogs   []string `json:"recent_logs"`
}
