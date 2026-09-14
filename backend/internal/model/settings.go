package model

import "time"

type Settings struct {
	ID                     int64     `json:"id"`
	Timezone               string    `json:"timezone"`
	LogLevel               string    `json:"log_level"`
	LogRetentionDays       int       `json:"log_retention_days"`
	LogRetentionMaxRecords int       `json:"log_retention_max_records"`
	HistoryViewMode        string    `json:"history_view_mode"`
	DefaultPage            string    `json:"default_page"`
	PageSize               int       `json:"page_size"`
	CacheDir               string    `json:"cache_dir"`
	CachePersistEnabled    bool      `json:"cache_persist_enabled"`
	IgnoredExtensions      []string  `json:"ignored_extensions"`
	UploadQueueUpperLimit  int       `json:"upload_queue_upper_limit"`
	UploadQueueLowerLimit  int       `json:"upload_queue_lower_limit"`
	MaxConcurrentScans     int       `json:"max_concurrent_scans"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type UpdateSettingsInput struct {
	LogRetentionDays       int      `json:"log_retention_days"`
	LogRetentionMaxRecords int      `json:"log_retention_max_records"`
	HistoryViewMode        string   `json:"history_view_mode"`
	DefaultPage            string   `json:"default_page"`
	PageSize               int      `json:"page_size"`
	CacheDir               string   `json:"cache_dir"`
	CachePersistEnabled    *bool    `json:"cache_persist_enabled"`
	IgnoredExtensions      []string `json:"ignored_extensions"`
	UploadQueueUpperLimit  int      `json:"upload_queue_upper_limit"`
	UploadQueueLowerLimit  int      `json:"upload_queue_lower_limit"`
	MaxConcurrentScans     int      `json:"max_concurrent_scans"`
}
