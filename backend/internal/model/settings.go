package model

import "time"

type Settings struct {
	ID                     int64     `json:"id"`
	Timezone               string    `json:"timezone"`
	LogLevel               string    `json:"log_level"`
	LogRetentionDays       int       `json:"log_retention_days"`
	LogRetentionMaxRecords int       `json:"log_retention_max_records"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
