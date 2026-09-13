package sqlite

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nestify/backend/internal/model"
)

const backupColumns = `id, sort_order, name, enabled, source_dirs_json, target_dirs_json, monitor_enabled,
	completion_rule, replace_rule, sync_delete_from_target, force_full_scan, scan_interval_seconds,
	cron_expression, filter_rules_json, last_backup_at, last_status, last_summary,
	last_scanned_files, last_copied_files, last_skipped_files, last_deleted_files, created_at, updated_at`

func (s *Store) ListBackups() ([]model.BackupTask, error) {
	rows, err := s.db.Query(`SELECT ` + backupColumns + ` FROM backup_tasks ORDER BY sort_order ASC, id ASC;`)
	if err != nil {
		return nil, fmt.Errorf("query backup tasks: %w", err)
	}
	defer rows.Close()

	items := make([]model.BackupTask, 0)
	for rows.Next() {
		item, scanErr := scanBackupTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *Store) GetBackup(id int64) (*model.BackupTask, error) {
	row := s.db.QueryRow(`SELECT `+backupColumns+` FROM backup_tasks WHERE id = ?;`, id)
	item, err := scanBackupTask(row)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (s *Store) CreateBackup(input model.CreateBackupInput) (*model.BackupTask, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	sourceJSON, _ := json.Marshal(normalizeStringList(input.SourceDirs))
	targetJSON, _ := json.Marshal(normalizeStringList(input.TargetDirs))
	filterJSON, _ := json.Marshal(normalizeBackupFilters(input.FilterRules))

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	monitor := false
	if input.MonitorEnabled != nil {
		monitor = *input.MonitorEnabled
	}
	syncDelete := false
	if input.SyncDeleteFromTarget != nil {
		syncDelete = *input.SyncDeleteFromTarget
	}
	forceFull := false
	if input.ForceFullScan != nil {
		forceFull = *input.ForceFullScan
	}

	var maxSortOrder int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) FROM backup_tasks`).Scan(&maxSortOrder); err != nil {
		return nil, fmt.Errorf("query max backup sort_order: %w", err)
	}

	result, err := s.db.Exec(`
		INSERT INTO backup_tasks (
			sort_order, name, enabled, source_dirs_json, target_dirs_json, monitor_enabled,
			completion_rule, replace_rule, sync_delete_from_target, force_full_scan, scan_interval_seconds,
			cron_expression, filter_rules_json, last_backup_at, last_status, last_summary,
			last_scanned_files, last_copied_files, last_skipped_files, last_deleted_files, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '', 0, 0, 0, 0, ?, ?);
	`,
		maxSortOrder+1,
		strings.TrimSpace(input.Name),
		boolToInt(enabled),
		string(sourceJSON),
		string(targetJSON),
		boolToInt(monitor),
		normalizeCompletionRule(input.CompletionRule),
		normalizeReplaceRule(input.ReplaceRule),
		boolToInt(syncDelete),
		boolToInt(forceFull),
		normalizeScanInterval(input.ScanIntervalSeconds),
		strings.TrimSpace(input.CronExpression),
		string(filterJSON),
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert backup task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("backup task last insert id: %w", err)
	}

	return s.GetBackup(id)
}

func (s *Store) UpdateBackup(id int64, input model.UpdateBackupInput) (*model.BackupTask, error) {
	existing, err := s.GetBackup(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	sourceJSON, _ := json.Marshal(normalizeStringList(input.SourceDirs))
	targetJSON, _ := json.Marshal(normalizeStringList(input.TargetDirs))
	filterJSON, _ := json.Marshal(normalizeBackupFilters(input.FilterRules))

	enabled := existing.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	monitor := existing.MonitorEnabled
	if input.MonitorEnabled != nil {
		monitor = *input.MonitorEnabled
	}
	syncDelete := existing.SyncDeleteFromTarget
	if input.SyncDeleteFromTarget != nil {
		syncDelete = *input.SyncDeleteFromTarget
	}
	forceFull := existing.ForceFullScan
	if input.ForceFullScan != nil {
		forceFull = *input.ForceFullScan
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		UPDATE backup_tasks
		SET name = ?, enabled = ?, source_dirs_json = ?, target_dirs_json = ?, monitor_enabled = ?,
			completion_rule = ?, replace_rule = ?, sync_delete_from_target = ?, force_full_scan = ?, scan_interval_seconds = ?,
			cron_expression = ?, filter_rules_json = ?, updated_at = ?
		WHERE id = ?;
	`,
		strings.TrimSpace(input.Name),
		boolToInt(enabled),
		string(sourceJSON),
		string(targetJSON),
		boolToInt(monitor),
		normalizeCompletionRule(input.CompletionRule),
		normalizeReplaceRule(input.ReplaceRule),
		boolToInt(syncDelete),
		boolToInt(forceFull),
		normalizeScanInterval(input.ScanIntervalSeconds),
		strings.TrimSpace(input.CronExpression),
		string(filterJSON),
		now,
		id,
	); err != nil {
		return nil, fmt.Errorf("update backup task: %w", err)
	}

	return s.GetBackup(id)
}

func (s *Store) SetBackupEnabled(id int64, enabled bool) (*model.BackupTask, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE backup_tasks SET enabled = ?, updated_at = ? WHERE id = ?;`, boolToInt(enabled), now, id); err != nil {
		return nil, fmt.Errorf("set backup enabled: %w", err)
	}
	return s.GetBackup(id)
}

func (s *Store) UpdateBackupRunResult(id int64, status, summary, lastBackupAt string, scanned, copied, skipped, deleted int) error {
	if _, err := s.db.Exec(`
		UPDATE backup_tasks
		SET last_status = ?, last_summary = ?, last_backup_at = ?,
			last_scanned_files = ?, last_copied_files = ?, last_skipped_files = ?, last_deleted_files = ?, updated_at = ?
		WHERE id = ?;
	`,
		strings.TrimSpace(status),
		summary,
		lastBackupAt,
		scanned,
		copied,
		skipped,
		deleted,
		time.Now().UTC().Format(time.RFC3339),
		id,
	); err != nil {
		return fmt.Errorf("update backup run result: %w", err)
	}
	return nil
}

func (s *Store) DeleteBackup(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM backup_tasks WHERE id = ?;`, id); err != nil {
		return fmt.Errorf("delete backup task: %w", err)
	}
	return nil
}

func (s *Store) ReorderBackups(items []model.BackupReorderItem) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin reorder backups transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE backup_tasks SET sort_order = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("prepare reorder backups statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, item := range items {
		if _, err := stmt.Exec(item.SortOrder, now, item.ID); err != nil {
			return fmt.Errorf("update backup sort_order: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder backups transaction: %w", err)
	}

	return nil
}

type backupScanner interface {
	Scan(dest ...any) error
}

func scanBackupTask(scanner backupScanner) (model.BackupTask, error) {
	var (
		id              int64
		sortOrder       int
		name            string
		enabled         int
		sourceJSON      string
		targetJSON      string
		monitorEnabled  int
		completionRule  string
		replaceRule     string
		syncDelete      int
		forceFull       int
		scanInterval    int
		cronExpression  string
		filterJSON      string
		lastBackupAt    string
		lastStatus      string
		lastSummary     string
		lastScanned     int
		lastCopied      int
		lastSkipped     int
		lastDeleted     int
		createdAtSource string
		updatedAtSource string
	)

	if err := scanner.Scan(
		&id, &sortOrder, &name, &enabled, &sourceJSON, &targetJSON, &monitorEnabled,
		&completionRule, &replaceRule, &syncDelete, &forceFull, &scanInterval,
		&cronExpression, &filterJSON, &lastBackupAt, &lastStatus, &lastSummary,
		&lastScanned, &lastCopied, &lastSkipped, &lastDeleted, &createdAtSource, &updatedAtSource,
	); err != nil {
		return model.BackupTask{}, fmt.Errorf("scan backup task: %w", err)
	}

	item := model.BackupTask{
		ID:                   id,
		SortOrder:            sortOrder,
		Name:                 name,
		Enabled:              intToBool(enabled),
		SourceDirs:           decodeStringList(sourceJSON),
		TargetDirs:           decodeStringList(targetJSON),
		MonitorEnabled:       intToBool(monitorEnabled),
		CompletionRule:       normalizeCompletionRule(completionRule),
		ReplaceRule:          normalizeReplaceRule(replaceRule),
		SyncDeleteFromTarget: intToBool(syncDelete),
		ForceFullScan:        intToBool(forceFull),
		ScanIntervalSeconds:  scanInterval,
		CronExpression:       cronExpression,
		FilterRules:          decodeBackupFilters(filterJSON),
		LastBackupAt:         lastBackupAt,
		LastStatus:           lastStatus,
		LastSummary:          lastSummary,
		LastScannedFiles:     lastScanned,
		LastCopiedFiles:      lastCopied,
		LastSkippedFiles:     lastSkipped,
		LastDeletedFiles:     lastDeleted,
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339, createdAtSource)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtSource)

	return item, nil
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func normalizeBackupFilters(filters []model.BackupFilterRule) []model.BackupFilterRule {
	result := make([]model.BackupFilterRule, 0, len(filters))
	for index, filter := range filters {
		filter.Type = strings.TrimSpace(filter.Type)
		if filter.Type == "" {
			filter.Type = model.BackupFilterName
		}
		if strings.TrimSpace(filter.ID) == "" {
			filter.ID = fmt.Sprintf("rule-%d-%d", time.Now().UnixNano(), index)
		}
		if filter.SizeUnit == "" {
			filter.SizeUnit = "MB"
		}
		result = append(result, filter)
	}
	return result
}

func normalizeCompletionRule(value string) string {
	switch strings.TrimSpace(value) {
	case model.BackupCompletionDeleteSource:
		return model.BackupCompletionDeleteSource
	case model.BackupCompletionDeleteSourceDirs:
		return model.BackupCompletionDeleteSourceDirs
	default:
		return model.BackupCompletionNone
	}
}

func normalizeReplaceRule(value string) string {
	if strings.TrimSpace(value) == model.BackupReplaceOverwrite {
		return model.BackupReplaceOverwrite
	}
	return model.BackupReplaceSkip
}

func normalizeScanInterval(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func decodeStringList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return values
}

func decodeBackupFilters(raw string) []model.BackupFilterRule {
	if strings.TrimSpace(raw) == "" {
		return []model.BackupFilterRule{}
	}
	var values []model.BackupFilterRule
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []model.BackupFilterRule{}
	}
	return values
}
