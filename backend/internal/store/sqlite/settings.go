package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"nestify/backend/internal/model"
)

func (s *Store) ensureDefaultSettings() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM settings WHERE id = 1`).Scan(&count); err != nil {
		return fmt.Errorf("count default settings: %w", err)
	}

	if count > 0 {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		INSERT INTO settings (
			id, timezone, log_level, log_retention_days, log_retention_max_records, created_at, updated_at
		) VALUES (1, ?, ?, ?, ?, ?, ?)
	`, "Asia/Shanghai", "info", 5, 10000, now, now); err != nil {
		return fmt.Errorf("insert default settings: %w", err)
	}

	return nil
}

func (s *Store) GetSettings() (*model.Settings, error) {
	row := s.db.QueryRow(`
		SELECT id, timezone, log_level, log_retention_days, log_retention_max_records, history_view_mode,
		       default_page, page_size,
		       cache_dir, cache_persist_enabled, ignored_extensions_json,
		       upload_queue_upper_limit, upload_queue_lower_limit, max_concurrent_scans,
		       created_at, updated_at
		FROM settings
		WHERE id = 1
	`)

	var item model.Settings
	var createdAt, updatedAt string
	var cachePersist int
	var ignoredJSON string
	err := row.Scan(
		&item.ID,
		&item.Timezone,
		&item.LogLevel,
		&item.LogRetentionDays,
		&item.LogRetentionMaxRecords,
		&item.HistoryViewMode,
		&item.DefaultPage,
		&item.PageSize,
		&item.CacheDir,
		&cachePersist,
		&ignoredJSON,
		&item.UploadQueueUpperLimit,
		&item.UploadQueueLowerLimit,
		&item.MaxConcurrentScans,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get settings: %w", err)
	}

	item.CachePersistEnabled = cachePersist != 0
	if err := json.Unmarshal([]byte(ignoredJSON), &item.IgnoredExtensions); err != nil {
		item.IgnoredExtensions = []string{}
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &item, nil
}

func (s *Store) UpdateSettings(input model.UpdateSettingsInput) (*model.Settings, error) {
	// 合并现有设置，避免未传字段被覆盖为空值。
	current, err := s.GetSettings()
	if err != nil {
		return nil, err
	}
	if current == nil {
		current = &model.Settings{
			LogRetentionDays:       5,
			LogRetentionMaxRecords: 10000,
			HistoryViewMode:        "flat",
			DefaultPage:            "dashboard",
			PageSize:               50,
			CacheDir:               "/tmp",
			CachePersistEnabled:    true,
			UploadQueueUpperLimit:  10000,
			MaxConcurrentScans:     1,
		}
	}

	defaultPage := input.DefaultPage
	if defaultPage == "" {
		defaultPage = current.DefaultPage
	}
	if defaultPage == "" {
		defaultPage = "dashboard"
	}

	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = current.PageSize
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	cacheDir := input.CacheDir
	if cacheDir == "" {
		cacheDir = current.CacheDir
	}
	if cacheDir == "" {
		cacheDir = "/tmp"
	}

	cachePersist := current.CachePersistEnabled
	if input.CachePersistEnabled != nil {
		cachePersist = *input.CachePersistEnabled
	}

	ignored := current.IgnoredExtensions
	if input.IgnoredExtensions != nil {
		ignored = input.IgnoredExtensions
	}

	upperLimit := input.UploadQueueUpperLimit
	if upperLimit <= 0 {
		upperLimit = current.UploadQueueUpperLimit
	}
	lowerLimit := input.UploadQueueLowerLimit
	if lowerLimit <= 0 {
		lowerLimit = current.UploadQueueLowerLimit
	}
	concurrent := input.MaxConcurrentScans
	if concurrent <= 0 {
		concurrent = current.MaxConcurrentScans
	}

	ignoredJSON, err := json.Marshal(ignored)
	if err != nil {
		return nil, fmt.Errorf("marshal ignored extensions: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		UPDATE settings
		SET log_retention_days = ?,
		    log_retention_max_records = ?,
		    history_view_mode = ?,
		    default_page = ?,
		    page_size = ?,
		    cache_dir = ?,
		    cache_persist_enabled = ?,
		    ignored_extensions_json = ?,
		    upload_queue_upper_limit = ?,
		    upload_queue_lower_limit = ?,
		    max_concurrent_scans = ?,
		    updated_at = ?
		WHERE id = 1
	`,
		input.LogRetentionDays,
		input.LogRetentionMaxRecords,
		input.HistoryViewMode,
		defaultPage,
		pageSize,
		cacheDir,
		boolToInt(cachePersist),
		string(ignoredJSON),
		upperLimit,
		lowerLimit,
		concurrent,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("update settings: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("settings rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return nil, nil
	}

	return s.GetSettings()
}
