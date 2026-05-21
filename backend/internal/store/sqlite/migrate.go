package sqlite

import (
	"fmt"
	"strings"
)

func (s *Store) migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS admins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			monitor_enabled INTEGER NOT NULL DEFAULT 1,
			compatibility_mode TEXT NOT NULL DEFAULT 'local',
			archive_mode TEXT NOT NULL,
			run_mode TEXT NOT NULL,
			source_dir TEXT NOT NULL,
			target_dir TEXT NOT NULL,
			watch_debounce_ms INTEGER NOT NULL DEFAULT 2000,
			cron_expression TEXT NOT NULL DEFAULT '',
			run_on_start INTEGER NOT NULL DEFAULT 1,
			options_json TEXT NOT NULL DEFAULT '{}',
			package_options_json TEXT NOT NULL DEFAULT '{}',
			collect_options_json TEXT NOT NULL DEFAULT '{}',
			filters_json TEXT NOT NULL DEFAULT '{}',
			last_run_status TEXT NOT NULL DEFAULT '',
			last_success_count INTEGER NOT NULL DEFAULT 0,
			last_skip_count INTEGER NOT NULL DEFAULT 0,
			last_failure_count INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			timezone TEXT NOT NULL,
			log_level TEXT NOT NULL,
			log_retention_days INTEGER NOT NULL,
			log_retention_max_records INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate sqlite schema: %w", err)
		}
	}

	if err := s.ensureDefaultSettings(); err != nil {
		return err
	}

	if err := s.ensureRuleCompatibilityModeColumn(); err != nil {
		return err
	}

	return nil
}

func (s *Store) ensureRuleCompatibilityModeColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(rules);`)
	if err != nil {
		return fmt.Errorf("query rules schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan rules schema: %w", err)
		}
		if strings.EqualFold(name, "compatibility_mode") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN compatibility_mode TEXT NOT NULL DEFAULT 'local';`); err != nil {
		return fmt.Errorf("add compatibility_mode column: %w", err)
	}

	return nil
}
