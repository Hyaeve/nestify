package sqlite

import (
	"database/sql"
	"fmt"
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
			id, rule_id, rule_name, trigger_mode, archive_mode, status,
			processed_files, success_count, skip_count, failure_count,
			summary, started_at, updated_at, finished_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			rule_id = excluded.rule_id,
			rule_name = excluded.rule_name,
			trigger_mode = excluded.trigger_mode,
			archive_mode = excluded.archive_mode,
			status = excluded.status,
			processed_files = excluded.processed_files,
			success_count = excluded.success_count,
			skip_count = excluded.skip_count,
			failure_count = excluded.failure_count,
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
		item.Status,
		item.ProcessedFiles,
		item.SuccessCount,
		item.SkipCount,
		item.FailureCount,
		item.Summary,
		item.StartedAt.UTC().Format(time.RFC3339),
		item.UpdatedAt.UTC().Format(time.RFC3339),
		finishedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert run history: %w", err)
	}

	return nil
}

func (s *Store) ListRunHistory() ([]model.RunHistoryItem, error) {
	rows, err := s.db.Query(`
		SELECT id, rule_id, rule_name, trigger_mode, archive_mode, status,
		       processed_files, success_count, skip_count, failure_count,
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

func (s *Store) ClearRunHistory() error {
	if _, err := s.db.Exec(`DELETE FROM run_history`); err != nil {
		return fmt.Errorf("clear run history: %w", err)
	}

	return nil
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

	err := s.Scan(
		&item.ID,
		&ruleID,
		&item.RuleName,
		&item.TriggerMode,
		&item.ArchiveMode,
		&item.Status,
		&item.ProcessedFiles,
		&item.SuccessCount,
		&item.SkipCount,
		&item.FailureCount,
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
	item.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if finishedAt != "" {
		if t, parseErr := time.Parse(time.RFC3339, finishedAt); parseErr == nil {
			item.FinishedAt = &t
		}
	}

	return item, nil
}
