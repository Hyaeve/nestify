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
		SELECT id, name, description, enabled, monitor_enabled, compatibility_mode, archive_mode, rule_type, link_mode, run_mode,
		       source_dir, target_dir, watch_debounce_ms, cron_expression, run_on_start,
		       options_json, package_options_json, collect_options_json, filters_json, match_filters_json, transform_rules_json,
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

func (s *Store) ListRulesPage(page, pageSize int, ruleType string) ([]model.Rule, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	whereClause, args := buildRuleTypeWhereClause(ruleType)

	countQuery := `SELECT COUNT(*) FROM rules` + whereClause
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count rules: %w", err)
	}

	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(`
		SELECT id, name, description, enabled, monitor_enabled, compatibility_mode, archive_mode, rule_type, link_mode, run_mode,
		       source_dir, target_dir, watch_debounce_ms, cron_expression, run_on_start,
		       options_json, package_options_json, collect_options_json, filters_json, match_filters_json, transform_rules_json,
		       last_run_status, last_success_count, last_skip_count, last_failure_count,
		       created_at, updated_at
		FROM rules`+whereClause+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list rules page: %w", err)
	}
	defer rows.Close()

	items := make([]model.Rule, 0, pageSize)
	for rows.Next() {
		rule, scanErr := scanRule(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rules page: %w", err)
	}

	return items, total, nil
}

func buildRuleTypeWhereClause(ruleType string) (string, []any) {
	switch strings.TrimSpace(ruleType) {
	case "archive":
		return ` WHERE rule_type = ?`, []any{"archive"}
	case "cleanup":
		return ` WHERE rule_type = ?`, []any{"cleanup"}
	case "link":
		return ` WHERE rule_type = ?`, []any{"link"}
	default:
		return "", nil
	}
}

func (s *Store) GetRuleByID(id int64) (*model.Rule, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, enabled, monitor_enabled, compatibility_mode, archive_mode, rule_type, link_mode, run_mode,
		       source_dir, target_dir, watch_debounce_ms, cron_expression, run_on_start,
		       options_json, package_options_json, collect_options_json, filters_json, match_filters_json, transform_rules_json,
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
	ruleType := defaultString(strings.TrimSpace(input.RuleType), deriveRuleType(archiveMode))
	linkMode := strings.TrimSpace(input.LinkMode)
	compatibilityMode := strings.TrimSpace(input.CompatibilityMode)
	if compatibilityMode == "" {
		compatibilityMode = "local"
	}
	runMode := strings.TrimSpace(input.RunMode)

	result, err := s.db.Exec(`
		INSERT INTO rules (
			name, description, enabled, monitor_enabled, compatibility_mode, archive_mode, rule_type, link_mode, run_mode,
			source_dir, target_dir, watch_debounce_ms, cron_expression, run_on_start,
			options_json, package_options_json, collect_options_json, filters_json, match_filters_json, transform_rules_json,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		boolToInt(defaultBool(input.Enabled, true)),
		boolToInt(defaultBool(input.MonitorEnabled, true)),
		compatibilityMode,
		archiveMode,
		ruleType,
		linkMode,
		runMode,
		strings.TrimSpace(input.SourceDir),
		strings.TrimSpace(input.TargetDir),
		defaultInt(input.WatchDebounceMS, 2000),
		strings.TrimSpace(input.CronExpression),
		boolToInt(defaultBool(input.RunOnStart, true)),
		marshalBoolMap(input.Options),
		marshalBoolMap(input.PackageOptions),
		marshalBoolMap(input.CollectOptions),
		marshalStringList(input.Filters),
		marshalStringList(input.MatchFilters),
		marshalStringList(input.TransformRules),
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
		    compatibility_mode = ?,
		    archive_mode = ?,
		    rule_type = ?,
		    link_mode = ?,
		    run_mode = ?,
		    source_dir = ?,
		    target_dir = ?,
		    watch_debounce_ms = ?,
		    cron_expression = ?,
		    run_on_start = ?,
		    options_json = ?,
		    package_options_json = ?,
		    collect_options_json = ?,
		    filters_json = ?,
		    match_filters_json = ?,
		    transform_rules_json = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		strings.TrimSpace(input.Name),
		strings.TrimSpace(input.Description),
		boolToInt(defaultBool(input.Enabled, true)),
		boolToInt(defaultBool(input.MonitorEnabled, true)),
		defaultString(strings.TrimSpace(input.CompatibilityMode), "local"),
		strings.TrimSpace(input.ArchiveMode),
		defaultString(strings.TrimSpace(input.RuleType), deriveRuleType(strings.TrimSpace(input.ArchiveMode))),
		strings.TrimSpace(input.LinkMode),
		strings.TrimSpace(input.RunMode),
		strings.TrimSpace(input.SourceDir),
		strings.TrimSpace(input.TargetDir),
		defaultInt(input.WatchDebounceMS, 2000),
		strings.TrimSpace(input.CronExpression),
		boolToInt(defaultBool(input.RunOnStart, true)),
		marshalBoolMap(input.Options),
		marshalBoolMap(input.PackageOptions),
		marshalBoolMap(input.CollectOptions),
		marshalStringList(input.Filters),
		marshalStringList(input.MatchFilters),
		marshalStringList(input.TransformRules),
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

func (s *Store) DeleteRule(id int64) error {
	_, err := s.db.Exec(`DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}

	return nil
}

func (s *Store) UpdateRuleExecutionStats(id int64, status string, successCount, skipCount, failureCount int) error {
	_, err := s.db.Exec(`
		UPDATE rules
		SET last_run_status = ?,
		    last_success_count = ?,
		    last_skip_count = ?,
		    last_failure_count = ?,
		    updated_at = ?
		WHERE id = ?
	`, strings.TrimSpace(status), successCount, skipCount, failureCount, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update rule execution stats: %w", err)
	}

	return nil
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

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return value
}

func deriveRuleType(archiveMode string) string {
	switch strings.TrimSpace(archiveMode) {
	case "cleanup":
		return "cleanup"
	case "link":
		return "link"
	default:
		return "archive"
	}
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

func marshalStringList(value []string) string {
	if len(value) == 0 {
		return `[]`
	}

	items := make([]string, 0, len(value))
	for _, item := range value {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	if len(items) == 0 {
		return `[]`
	}

	encoded, err := json.Marshal(items)
	if err != nil {
		return `[]`
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
		&rule.CompatibilityMode,
		&rule.ArchiveMode,
		&rule.RuleType,
		&rule.LinkMode,
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
		&rule.MatchFiltersJSON,
		&rule.TransformRulesJSON,
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
	if strings.TrimSpace(rule.CompatibilityMode) == "" {
		rule.CompatibilityMode = "local"
	}
	if strings.TrimSpace(rule.RuleType) == "" {
		rule.RuleType = deriveRuleType(rule.ArchiveMode)
	}
	rule.RunOnStart = intToBool(runOnStart)
	rule.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	rule.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return rule, nil
}
