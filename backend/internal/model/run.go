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
	// RunStatusCancelled 表示任务被用户手动停止（卡片上的执行 / 扫描按钮再次点击）。
	// 停止是「协作式」的：执行器在目录递归与文件循环的检查点发现取消信号后自行退出，
	// 已经在处理的那个文件会写完，不会留下半截产物。
	RunStatusCancelled = "cancelled"
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

// 明细载荷的动作标识。
// 备份链路：上传 / 跳过 / 失败 / 删除；其它链路：生成或覆盖 strm / 同步元数据实体文件 /
// 打包产出压缩包 / 移动文件或目录。前端按动作渲染中文标签与配色。
const (
	BackupFileActionUpload = "upload"
	BackupFileActionSkip   = "skip"
	BackupFileActionFail   = "fail"
	BackupFileActionDelete = "delete"

	RunFileActionStrm     = "strm"
	RunFileActionMetadata = "metadata"
	RunFileActionPack     = "pack"
	RunFileActionMove     = "move"
)

// 明细载荷的类型标识（detail_json.kind），前端据此决定面板标题与筛选页签。
const (
	RunDetailKindBackup  = "backup"
	RunDetailKindStrm    = "strm"
	RunDetailKindPackage = "package"
	RunDetailKindCollect = "collect"
	RunDetailKindArchive = "archive"
)

// RunFileEntry 记录一次执行中单个文件（或文件夹）的处理结果。
// Path 是源路径，Target 是落地产物（.strm / 元数据实体文件 / 压缩包）。
// 写入 run_history.detail_json，供运行日志 / 归巢历史的详情展开查看「到底动了哪些文件」。
// 设计上只收录真的动了文件的结果（上传 / 生成 / 下载 / 打包 / 失败 / 删除）；
// 「跳过」只计数、不逐条列明细，否则筛选规则与同名跳过会产生海量条目。
type RunFileEntry struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Size   int64  `json:"size,omitempty"`
	Target string `json:"target,omitempty"`
	Note   string `json:"note,omitempty"`
	Dir    bool   `json:"dir,omitempty"`
}

// BackupFileEntry 是 RunFileEntry 的历史名称（备份链路沿用），二者完全等价。
type BackupFileEntry = RunFileEntry

// RunDetail 是运行详情的载荷（run_history.detail_json），备份 / strm / 打包等链路共用。
// Files 为明细列表（每个动作只保留有限条数，见 backup.maxFileEntriesPerAction 与
// executor.maxDetailEntriesPerAction）；Counts 是采集到的真实数量（含只计数的 skip），
// FilesTotal 是明细总数（不含跳过），均不受截断影响。
type RunDetail struct {
	Kind           string         `json:"kind"`
	Files          []RunFileEntry `json:"files"`
	Counts         map[string]int `json:"counts,omitempty"`
	FilesTotal     int            `json:"files_total"`
	FilesTruncated bool           `json:"files_truncated,omitempty"`
}

// BackupDetail 是 RunDetail 的历史名称（备份链路沿用），二者完全等价。
type BackupDetail = RunDetail

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
