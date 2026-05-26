package sqlite

import (
	"database/sql"
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
		SELECT id, timezone, log_level, log_retention_days, log_retention_max_records, created_at, updated_at
		FROM settings
		WHERE id = 1
	`)

	var item model.Settings
	var createdAt, updatedAt string
	err := row.Scan(
		&item.ID,
		&item.Timezone,
		&item.LogLevel,
		&item.LogRetentionDays,
		&item.LogRetentionMaxRecords,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get settings: %w", err)
	}

	item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &item, nil
}

func (s *Store) UpdateSettings(input model.UpdateSettingsInput) (*model.Settings, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.Exec(`
		UPDATE settings
		SET log_retention_days = ?,
		    log_retention_max_records = ?,
		    updated_at = ?
		WHERE id = 1
	`, input.LogRetentionDays, input.LogRetentionMaxRecords, now)
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
