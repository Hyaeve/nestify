package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nestify/backend/internal/model"
)

func (s *Store) ListRules() ([]model.Rule, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, enabled, monitor_enabled, archive_mode, run_mode,
		       source_dir, target_dir, watch_debounce_ms, cron_expression, run_on_start,
		       options_json, package_options_json, collect_options_json, filters_json,
		       last_run_status, last_success_count, last_skip_count, last_failure_count,
		       created_at, updated_at
		FROM rules
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list rules: %w", err)
	}
	defer rows.Close()

	items := make([]model.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rules: %w", err)
	}

	return items, nil
}

func (s *Store) GetRuleByID(id int64) (*model.Rule, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, enabled, monitor_enabled, archive_mode, run_mode,
		       source_dir, target_dir, watch_debounce_ms, cron_expression, run_on_start,
		       options_json, package_options_json, collect_options_json, filters_json,
		       last_run_status, last_success_count, last_skip_count, last_failure_count,
		       created_at, updated_at
		FROM rules
		WHERE id = ?
	`, id)

	rule, err := scanRule(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get rule by id: %w", err)
	}

	return &rule, nil
}

func (s *Store) CreateRule(input model.CreateRuleInput) (*model.Rule, error) {
	now := time.Now().UTC()
	archiveMode := strings.TrimSpace(input.ArchiveMode)
	runMode := strings.TrimSpace(input.RunMode)

	result, err := s.db.Exec(`
		INSERT INTO rules (
			name, description, enabled, monitor_enabled, archive_mode, run_mode,
			source_dir, target_dir, watch_debounce_ms, cron_expression, run_on_start,
			options_json, package_options_json, collect_options_json, filters_json,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		boolToInt(defaultBool(input.Enabled, true)),
		boolToInt(defaultBool(input.MonitorEnabled, true)),
		archiveMode,
		runMode,
		strings.TrimSpace(input.SourceDir),
		strings.TrimSpace(input.TargetDir),
		defaultInt(input.WatchDebounceMS, 2000),
		strings.TrimSpace(input.CronExpression),
		boolToInt(defaultBool(input.RunOnStart, true)),
		`{}`,
		marshalBoolMap(input.PackageOptions),
		marshalBoolMap(input.CollectOptions),
		`{}`,
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("create rule: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get inserted rule id: %w", err)
	}

	return s.GetRuleByID(id)
}

func (s *Store) UpdateRule(id int64, input model.UpdateRuleInput) (*model.Rule, error) {
	now := time.Now().UTC()
	result, err := s.db.Exec(`
		UPDATE rules
		SET name = ?,
		    description = ?,
		    enabled = ?,
		    monitor_enabled = ?,
		    archive_mode = ?,
		    run_mode = ?,
		    source_dir = ?,
		    target_dir = ?,
		    watch_debounce_ms = ?,
		    cron_expression = ?,
		    run_on_start = ?,
		    package_options_json = ?,
		    collect_options_json = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		boolToInt(defaultBool(input.Enabled, true)),
		boolToInt(defaultBool(input.MonitorEnabled, true)),
		strings.TrimSpace(input.ArchiveMode),
		strings.TrimSpace(input.RunMode),
		strings.TrimSpace(input.SourceDir),
		strings.TrimSpace(input.TargetDir),
		defaultInt(input.WatchDebounceMS, 2000),
		strings.TrimSpace(input.CronExpression),
		boolToInt(defaultBool(input.RunOnStart, true)),
		marshalBoolMap(input.PackageOptions),
		marshalBoolMap(input.CollectOptions),
		now.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("update rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("get updated rule rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, nil
	}

	return s.GetRuleByID(id)
}

func defaultBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}

	return *value
}

func defaultInt(value int, fallback int) int {
	if value == 0 {
		return fallback
	}

	return value
}

func marshalBoolMap(value map[string]bool) string {
	if len(value) == 0 {
		return `{}`
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}

	return string(encoded)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRule(s scanner) (model.Rule, error) {
	var rule model.Rule
	var enabled, monitorEnabled, runOnStart int
	var createdAt, updatedAt string

	err := s.Scan(
		&rule.ID,
		&rule.Name,
		&rule.Description,
		&enabled,
		&monitorEnabled,
		&rule.ArchiveMode,
		&rule.RunMode,
		&rule.SourceDir,
		&rule.TargetDir,
		&rule.WatchDebounceMS,
		&rule.CronExpression,
		&runOnStart,
		&rule.OptionsJSON,
		&rule.PackageOptionsJSON,
		&rule.CollectOptionsJSON,
		&rule.FiltersJSON,
		&rule.LastRunStatus,
		&rule.LastSuccessCount,
		&rule.LastSkipCount,
		&rule.LastFailureCount,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return model.Rule{}, err
	}

	rule.Enabled = intToBool(enabled)
	rule.MonitorEnabled = intToBool(monitorEnabled)
	rule.RunOnStart = intToBool(runOnStart)
	rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rule.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return rule, nil
}
