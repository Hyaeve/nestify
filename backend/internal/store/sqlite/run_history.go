package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"nestify/backend/internal/model"
)

func (s *Store) UpsertRunHistory(item model.RunHistoryItem) error {
	var ruleID any
	if item.RuleID != nil {
		ruleID = *item.RuleID
	}

	finishedAt := ""
	if item.FinishedAt != nil {
		finishedAt = item.FinishedAt.UTC().Format(time.RFC3339)
	}

	_, err := s.db.Exec(`
		INSERT INTO run_history (
			id, rule_id, rule_name, trigger_mode, archive_mode, link_mode, status,
			processed_files, success_count, skip_count, failure_count, size_bytes,
			summary, started_at, updated_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			rule_id = excluded.rule_id,
			rule_name = excluded.rule_name,
			trigger_mode = excluded.trigger_mode,
			archive_mode = excluded.archive_mode,
			link_mode = excluded.link_mode,
			status = excluded.status,
			processed_files = excluded.processed_files,
			success_count = excluded.success_count,
			skip_count = excluded.skip_count,
			failure_count = excluded.failure_count,
			size_bytes = excluded.size_bytes,
			summary = excluded.summary,
			started_at = excluded.started_at,
			updated_at = excluded.updated_at,
			finished_at = excluded.finished_at
	`,
		item.ID,
		ruleID,
		item.RuleName,
		item.TriggerMode,
		item.ArchiveMode,
		item.LinkMode,
		item.Status,
		item.ProcessedFiles,
		item.SuccessCount,
		item.SkipCount,
		item.FailureCount,
		item.SizeBytes,
		item.Summary,
		item.StartedAt.UTC().Format(time.RFC3339),
		item.UpdatedAt.UTC().Format(time.RFC3339),
		finishedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert run history: %w", err)
	}

	if err := s.applyRunHistoryRetentionPolicy(); err != nil {
		return err
	}

	return nil
}

func (s *Store) applyRunHistoryRetentionPolicy() error {
	settings, err := s.GetSettings()
	if err != nil {
		return fmt.Errorf("load settings for run history retention: %w", err)
	}
	if settings == nil {
		return nil
	}

	if settings.LogRetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -settings.LogRetentionDays).Format(time.RFC3339)
		if _, err := s.db.Exec(`DELETE FROM run_history WHERE started_at < ?`, cutoff); err != nil {
			return fmt.Errorf("delete expired run history: %w", err)
		}
	}

	if settings.LogRetentionMaxRecords > 0 {
		if _, err := s.db.Exec(`
			DELETE FROM run_history
			WHERE id NOT IN (
				SELECT id FROM run_history
				ORDER BY started_at DESC, id DESC
				LIMIT ?
			)
		`, settings.LogRetentionMaxRecords); err != nil {
			return fmt.Errorf("trim run history by max records: %w", err)
		}
	}

	return nil
}

func (s *Store) ListRunHistory() ([]model.RunHistoryItem, error) {
	rows, err := s.db.Query(`
		SELECT id, rule_id, rule_name, trigger_mode, archive_mode, link_mode, status,
		       processed_files, success_count, skip_count, failure_count, size_bytes,
		       summary, started_at, updated_at, finished_at
		FROM run_history
		ORDER BY started_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list run history: %w", err)
	}
	defer rows.Close()

	items := make([]model.RunHistoryItem, 0)
	for rows.Next() {
		item, scanErr := scanRunHistory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run history: %w", err)
	}

	return items, nil
}

func (s *Store) ListRunHistoryPage(page, pageSize int, keyword, status, archiveMode, ruleType, sortBy, sortOrder string) ([]model.RunHistoryItem, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}

	whereClause, args := buildRunHistoryWhereClause(keyword, status, archiveMode, ruleType)

	countQuery := `SELECT COUNT(*) FROM run_history` + whereClause
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count run history: %w", err)
	}

	orderClause := buildRunHistoryOrderClause(sortBy, sortOrder)
	queryArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(`
		SELECT id, rule_id, rule_name, trigger_mode, archive_mode, link_mode, status,
		       processed_files, success_count, skip_count, failure_count, size_bytes,
		       summary, started_at, updated_at, finished_at
		FROM run_history`+whereClause+`
		ORDER BY `+orderClause+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list run history page: %w", err)
	}
	defer rows.Close()

	items := make([]model.RunHistoryItem, 0, pageSize)
	for rows.Next() {
		item, scanErr := scanRunHistory(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate run history page: %w", err)
	}

	return items, total, nil
}

func buildRunHistoryOrderClause(sortBy, sortOrder string) string {
	direction := "DESC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		direction = "ASC"
	}

	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "name":
		return "LOWER(COALESCE(rule_name, '')) " + direction + ", started_at DESC, id DESC"
	case "modified_at":
		return "started_at " + direction + ", id DESC"
	default:
		return "started_at DESC, id DESC"
	}
}

func (s *Store) GetRunHistorySummary() (model.RunHistorySummary, error) {
	var summary model.RunHistorySummary
	err := s.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN date(started_at, 'localtime') = date('now', 'localtime') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'skip' THEN 1 ELSE 0 END), 0)
		FROM run_history
	`).Scan(&summary.Total, &summary.Today, &summary.Success, &summary.Failed, &summary.Skipped)
	if err != nil {
		return model.RunHistorySummary{}, fmt.Errorf("get run history summary: %w", err)
	}

	return summary, nil
}

func (s *Store) DeleteRunHistoryByID(id string) error {
	if _, err := s.db.Exec(`DELETE FROM run_history WHERE id = ?`, strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("delete run history by id: %w", err)
	}

	return nil
}

func (s *Store) DeleteRunHistoryByStatus(status string) error {
	if _, err := s.db.Exec(`DELETE FROM run_history WHERE status = ?`, strings.TrimSpace(status)); err != nil {
		return fmt.Errorf("delete run history by status: %w", err)
	}

	return nil
}

func (s *Store) ClearRunHistory() error {
	if _, err := s.db.Exec(`DELETE FROM run_history`); err != nil {
		return fmt.Errorf("clear run history: %w", err)
	}

	return nil
}

func buildRunHistoryWhereClause(keyword, status, archiveMode, ruleType string) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 8)

	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus != "" {
		clauses = append(clauses, `status = ?`)
		args = append(args, trimmedStatus)
	}

	trimmedArchiveMode := strings.TrimSpace(archiveMode)
	if trimmedArchiveMode != "" {
		clauses = append(clauses, `archive_mode = ?`)
		args = append(args, trimmedArchiveMode)
	}

	trimmedRuleType := strings.TrimSpace(ruleType)
	if trimmedRuleType != "" {
		switch trimmedRuleType {
		case "archive":
			clauses = append(clauses, `(archive_mode = 'package' OR archive_mode = 'collect')`)
		case "cleanup":
			clauses = append(clauses, `(archive_mode = 'cleanup' OR archive_mode = 'transform')`)
		case "link":
			clauses = append(clauses, `archive_mode = 'link'`)
		}
	}

	trimmedKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if trimmedKeyword != "" {
		like := "%" + trimmedKeyword + "%"
		clauses = append(clauses, `(
			LOWER(COALESCE(rule_name, '')) LIKE ? OR
			LOWER(COALESCE(summary, '')) LIKE ? OR
			LOWER(COALESCE(status, '')) LIKE ? OR
			LOWER(COALESCE(trigger_mode, '')) LIKE ? OR
			LOWER(COALESCE(archive_mode, '')) LIKE ?
		)`)
		args = append(args, like, like, like, like, like)
	}

	if len(clauses) == 0 {
		return "", nil
	}

	return " WHERE " + strings.Join(clauses, " AND "), args
}

type runHistoryScanner interface {
	Scan(dest ...any) error
}

func scanRunHistory(s runHistoryScanner) (model.RunHistoryItem, error) {
	var item model.RunHistoryItem
	var ruleID sql.NullInt64
	var startedAt string
	var updatedAt string
	var finishedAt string
	var linkMode string

	err := s.Scan(
		&item.ID,
		&ruleID,
		&item.RuleName,
		&item.TriggerMode,
		&item.ArchiveMode,
		&linkMode,
		&item.Status,
		&item.ProcessedFiles,
		&item.SuccessCount,
		&item.SkipCount,
		&item.FailureCount,
		&item.SizeBytes,
		&item.Summary,
		&startedAt,
		&updatedAt,
		&finishedAt,
	)
	if err != nil {
		return model.RunHistoryItem{}, fmt.Errorf("scan run history: %w", err)
	}

	if ruleID.Valid {
		v := ruleID.Int64
		item.RuleID = &v
	}
	item.LinkMode = strings.TrimSpace(linkMode)
	item.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if finishedAt != "" {
		if t, parseErr := time.Parse(time.RFC3339, finishedAt); parseErr == nil {
			item.FinishedAt = &t
		}
	}

	return item, nil
}
