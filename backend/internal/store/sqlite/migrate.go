package sqlite

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"nestify/backend/internal/model"
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
			sort_order INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			monitor_enabled INTEGER NOT NULL DEFAULT 1,
			compatibility_mode TEXT NOT NULL DEFAULT 'local',
			archive_mode TEXT NOT NULL,
			rule_type TEXT NOT NULL DEFAULT 'archive',
			link_mode TEXT NOT NULL DEFAULT '',
			run_mode TEXT NOT NULL,
			source_dir TEXT NOT NULL,
			target_dir TEXT NOT NULL,
			watch_debounce_ms INTEGER NOT NULL DEFAULT 2000,
			cron_expression TEXT NOT NULL DEFAULT '',
			run_on_start INTEGER NOT NULL DEFAULT 1,
			options_json TEXT NOT NULL DEFAULT '{}',
			option_values_json TEXT NOT NULL DEFAULT '{}',
			package_options_json TEXT NOT NULL DEFAULT '{}',
			collect_options_json TEXT NOT NULL DEFAULT '{}',
			filters_json TEXT NOT NULL DEFAULT '[]',
			metadata_filters_json TEXT NOT NULL DEFAULT '[]',
			whitelist_json TEXT NOT NULL DEFAULT '[]',
			match_filters_json TEXT NOT NULL DEFAULT '[]',
			nest_filters_json TEXT NOT NULL DEFAULT '[]',
			transform_rules_json TEXT NOT NULL DEFAULT '[]',
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
			history_view_mode TEXT NOT NULL DEFAULT 'flat',
			default_page TEXT NOT NULL DEFAULT 'dashboard',
			page_size INTEGER NOT NULL DEFAULT 50,
			cache_dir TEXT NOT NULL DEFAULT '/tmp',
			cache_persist_enabled INTEGER NOT NULL DEFAULT 1,
			ignored_extensions_json TEXT NOT NULL DEFAULT '[]',
			upload_queue_upper_limit INTEGER NOT NULL DEFAULT 10000,
			upload_queue_lower_limit INTEGER NOT NULL DEFAULT 0,
			max_concurrent_scans INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS webdav_mounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			provider TEXT NOT NULL DEFAULT 'webdav',
			auth_type TEXT NOT NULL DEFAULT 'password',
			scheme TEXT NOT NULL DEFAULT 'http',
			host TEXT NOT NULL DEFAULT '',
			port INTEGER NOT NULL DEFAULT 0,
			username TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			token TEXT NOT NULL DEFAULT '',
			base_path TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS backup_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sort_order INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			source_dirs_json TEXT NOT NULL DEFAULT '[]',
			target_dirs_json TEXT NOT NULL DEFAULT '[]',
			monitor_enabled INTEGER NOT NULL DEFAULT 0,
			completion_rule TEXT NOT NULL DEFAULT 'none',
			replace_rule TEXT NOT NULL DEFAULT 'skip',
			sync_delete_from_target INTEGER NOT NULL DEFAULT 0,
			force_full_scan INTEGER NOT NULL DEFAULT 0,
			scan_interval_seconds INTEGER NOT NULL DEFAULT 0,
			cron_expression TEXT NOT NULL DEFAULT '',
			filter_rules_json TEXT NOT NULL DEFAULT '[]',
			last_backup_at TEXT NOT NULL DEFAULT '',
			last_status TEXT NOT NULL DEFAULT '',
			last_summary TEXT NOT NULL DEFAULT '',
			last_scanned_files INTEGER NOT NULL DEFAULT 0,
			last_copied_files INTEGER NOT NULL DEFAULT 0,
			last_skipped_files INTEGER NOT NULL DEFAULT 0,
			last_deleted_files INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
	}

	for index, statement := range statements {
		log.Printf("sqlite:migrate: exec base statement %d/%d", index+1, len(statements))
		if _, err := s.db.Exec(statement); err != nil {
			log.Printf("sqlite:migrate: base statement %d failed: %v", index+1, err)
			return fmt.Errorf("migrate sqlite schema: %w", err)
		}
	}
	log.Printf("sqlite:migrate: base schema statements complete")

	log.Printf("sqlite:migrate: ensure default settings")
	if err := s.ensureDefaultSettings(); err != nil {
		log.Printf("sqlite:migrate: ensure default settings failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure compatibility_mode column")
	if err := s.ensureRuleCompatibilityModeColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure compatibility_mode column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure rule_type column")
	if err := s.ensureRuleTypeColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure rule_type column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure link_mode column")
	if err := s.ensureRuleLinkModeColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure link_mode column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure option_values_json column")
	if err := s.ensureRuleOptionValuesColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure option_values_json column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure transform_rules column")
	if err := s.ensureRuleTransformRulesColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure transform_rules column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure transform_filters column")
	if err := s.ensureRuleTransformFiltersColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure transform_filters column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure match_filters column")
	if err := s.ensureRuleMatchFiltersColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure match_filters column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure whitelist column")
	if err := s.ensureRuleWhitelistColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure whitelist column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure nest_filters column")
	if err := s.ensureRuleNestFiltersColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure nest_filters column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure sort_order column")
	if err := s.ensureRuleSortOrderColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure sort_order column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure backup_tasks sort_order column")
	if err := s.ensureBackupSortOrderColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure backup_tasks sort_order column failed: %v", err)
		return err
	}

	// 运行日志表（run_history）不在这里建 —— 它已经搬到独立的日志库，
	// 见 run_history_store.go 的 migrateRunHistoryStore。
	log.Printf("sqlite:migrate: ensure settings history_view_mode column")
	if err := s.ensureSettingsHistoryViewModeColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure settings history_view_mode column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure settings extended columns")
	if err := s.ensureSettingsExtendedColumns(); err != nil {
		log.Printf("sqlite:migrate: ensure settings extended columns failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure webdav_mounts provider columns")
	if err := s.ensureMountProviderColumns(); err != nil {
		log.Printf("sqlite:migrate: ensure webdav_mounts provider columns failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure rules metadata filters column")
	if err := s.ensureRuleMetadataFiltersColumn(); err != nil {
		log.Printf("sqlite:migrate: ensure rules metadata filters column failed: %v", err)
		return err
	}

	log.Printf("sqlite:migrate: ensure performance indexes")
	if err := s.ensurePerformanceIndexes(); err != nil {
		log.Printf("sqlite:migrate: ensure performance indexes failed: %v", err)
		return err
	}
	log.Printf("sqlite:migrate: migration pipeline complete")

	return nil
}

func (s *Store) ensureSettingsHistoryViewModeColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(settings);`)
	if err != nil {
		return fmt.Errorf("query settings schema: %w", err)
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
			return fmt.Errorf("scan settings schema: %w", err)
		}
		if strings.EqualFold(name, "history_view_mode") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE settings ADD COLUMN history_view_mode TEXT NOT NULL DEFAULT 'flat';`); err != nil {
		return fmt.Errorf("add settings history_view_mode column: %w", err)
	}

	return nil
}

// ensureSettingsExtendedColumns 为 settings 表补齐缓存/持久化/忽略扩展名/资源限制等新列。
func (s *Store) ensureSettingsExtendedColumns() error {
	existing := map[string]bool{}
	rows, err := s.db.Query(`PRAGMA table_info(settings);`)
	if err != nil {
		return fmt.Errorf("query settings schema: %w", err)
	}
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("scan settings schema: %w", err)
		}
		existing[strings.ToLower(name)] = true
	}
	rows.Close()

	columns := []struct {
		name string
		dcl  string
	}{
		{"default_page", `default_page TEXT NOT NULL DEFAULT 'dashboard'`},
		{"page_size", `page_size INTEGER NOT NULL DEFAULT 50`},
		{"cache_dir", `cache_dir TEXT NOT NULL DEFAULT '/tmp'`},
		{"cache_persist_enabled", `cache_persist_enabled INTEGER NOT NULL DEFAULT 1`},
		{"ignored_extensions_json", `ignored_extensions_json TEXT NOT NULL DEFAULT '[]'`},
		{"upload_queue_upper_limit", `upload_queue_upper_limit INTEGER NOT NULL DEFAULT 10000`},
		{"upload_queue_lower_limit", `upload_queue_lower_limit INTEGER NOT NULL DEFAULT 0`},
		{"max_concurrent_scans", `max_concurrent_scans INTEGER NOT NULL DEFAULT 1`},
	}

	for _, col := range columns {
		if existing[strings.ToLower(col.name)] {
			continue
		}
		if _, err := s.db.Exec(`ALTER TABLE settings ADD COLUMN ` + col.dcl + `;`); err != nil {
			return fmt.Errorf("add settings %s column: %w", col.name, err)
		}
	}

	return nil
}

// ensureMountProviderColumns 为 webdav_mounts 补齐挂载类型与令牌认证相关的新列。
// 老库里的历史挂载没有这些列，补上后统一回退为 provider=webdav / auth_type=password。
func (s *Store) ensureMountProviderColumns() error {
	existing := map[string]bool{}
	rows, err := s.db.Query(`PRAGMA table_info(webdav_mounts);`)
	if err != nil {
		return fmt.Errorf("query webdav_mounts schema: %w", err)
	}
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("scan webdav_mounts schema: %w", err)
		}
		existing[strings.ToLower(name)] = true
	}
	rows.Close()

	columns := []struct {
		name string
		dcl  string
	}{
		{"provider", `provider TEXT NOT NULL DEFAULT 'webdav'`},
		{"auth_type", `auth_type TEXT NOT NULL DEFAULT 'password'`},
		{"token", `token TEXT NOT NULL DEFAULT ''`},
		{"cookie", `cookie TEXT NOT NULL DEFAULT ''`},
		{"device", `device TEXT NOT NULL DEFAULT 'web'`},
		{"request_interval_ms", `request_interval_ms INTEGER NOT NULL DEFAULT 1000`},
	}

	for _, col := range columns {
		if existing[strings.ToLower(col.name)] {
			continue
		}
		if _, err := s.db.Exec(`ALTER TABLE webdav_mounts ADD COLUMN ` + col.dcl + `;`); err != nil {
			return fmt.Errorf("add webdav_mounts %s column: %w", col.name, err)
		}
	}

	return nil
}

// ensureRuleMetadataFiltersColumn 为 rules 表补齐「元数据后缀」列。
//
// strm 链路里两类命中后缀的处理方式不同：
//   - 媒体后缀（filters_json）生成 .strm（内容为可播放的直链/本地路径）；
//   - 元数据后缀（本列）必须以实体文件落到目标目录，生成 .strm 对媒体服务器毫无意义。
//
// 老库补列成功后，顺手把历史规则里混在 filters_json 中的元数据后缀拆到新列，
// 让已存在的规则立刻符合「元数据不生成 strm」的行为。
func (s *Store) ensureRuleMetadataFiltersColumn() error {
	existing := map[string]bool{}
	rows, err := s.db.Query(`PRAGMA table_info(rules);`)
	if err != nil {
		return fmt.Errorf("query rules schema: %w", err)
	}
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("scan rules schema: %w", err)
		}
		existing[strings.ToLower(name)] = true
	}
	rows.Close()

	if existing["metadata_filters_json"] {
		return nil
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN metadata_filters_json TEXT NOT NULL DEFAULT '[]';`); err != nil {
		return fmt.Errorf("add rules metadata_filters_json column: %w", err)
	}

	return s.splitLegacyStrmMetadataFilters()
}

// splitLegacyStrmMetadataFilters 把历史 strm 规则 filters_json 中的元数据后缀
// （图片 / 字幕 / nfo，见 model.DefaultStrmMetadataExtensions）挪到 metadata_filters_json。
// 只处理 link_mode = 'strm' 且新列为空的规则，重复执行无副作用。
func (s *Store) splitLegacyStrmMetadataFilters() error {
	metadataPreset := make(map[string]bool)
	for _, extension := range model.DefaultStrmMetadataExtensions {
		metadataPreset[strings.ToLower(extension)] = true
	}

	rows, err := s.db.Query(
		`SELECT id, filters_json FROM rules WHERE link_mode = 'strm' AND (metadata_filters_json IS NULL OR metadata_filters_json IN ('', '[]'));`)
	if err != nil {
		return fmt.Errorf("query legacy strm rules: %w", err)
	}

	type legacyRule struct {
		id      int64
		filters []string
	}
	pending := make([]legacyRule, 0)
	for rows.Next() {
		var id int64
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return fmt.Errorf("scan legacy strm rule: %w", err)
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			raw = "[]"
		}
		var filters []string
		if err := json.Unmarshal([]byte(raw), &filters); err != nil {
			continue
		}
		pending = append(pending, legacyRule{id: id, filters: filters})
	}
	rows.Close()

	for _, item := range pending {
		media := make([]string, 0, len(item.filters))
		metadata := make([]string, 0, len(item.filters))
		for _, filter := range item.filters {
			normalized := normalizeStrmFilterExtension(filter)
			if normalized == "" {
				continue
			}
			if metadataPreset[normalized] {
				metadata = append(metadata, normalized)
				continue
			}
			media = append(media, normalized)
		}
		if len(metadata) == 0 {
			continue
		}

		mediaJSON, err := json.Marshal(media)
		if err != nil {
			return fmt.Errorf("marshal rule %d filters: %w", item.id, err)
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal rule %d metadata filters: %w", item.id, err)
		}
		if _, err := s.db.Exec(`UPDATE rules SET filters_json = ?, metadata_filters_json = ? WHERE id = ?`,
			string(mediaJSON), string(metadataJSON), item.id); err != nil {
			return fmt.Errorf("split rule %d strm metadata filters: %w", item.id, err)
		}
		log.Printf("sqlite:migrate: rule %d 元数据后缀已拆分到 metadata_filters_json: %v", item.id, metadata)
	}

	return nil
}

// normalizeStrmFilterExtension 把各种写法的后缀统一成「带点 + 小写」：mp4 / .MP4 / *.mp4 -> .mp4。
func normalizeStrmFilterExtension(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, `"'`)
	trimmed = strings.TrimPrefix(trimmed, "*")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, ".") {
		trimmed = "." + trimmed
	}
	return strings.ToLower(trimmed)
}

func (s *Store) ensurePerformanceIndexes() error {
	// run_history 的三组索引不在这里 —— 表在独立的日志库里（见 run_history_store.go）。
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_rules_sort_order ON rules(sort_order, id);`,
		`CREATE INDEX IF NOT EXISTS idx_rules_type_order ON rules(rule_type, sort_order, id);`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("create performance index: %w", err)
		}
	}

	return nil
}

func (s *Store) ensureRuleSortOrderColumn() error {
	log.Printf("sqlite:migrate: ensure sort_order column: query rules schema")
	rows, err := s.db.Query(`PRAGMA table_info(rules);`)
	if err != nil {
		log.Printf("sqlite:migrate: ensure sort_order column: query rules schema failed: %v", err)
		return fmt.Errorf("query rules schema: %w", err)
	}
	defer rows.Close()

	hasSortOrder := false
	log.Printf("sqlite:migrate: ensure sort_order column: scanning schema rows")
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
		if strings.EqualFold(name, "sort_order") {
			hasSortOrder = true
			break
		}
	}
	log.Printf("sqlite:migrate: ensure sort_order column: has_sort_order=%t", hasSortOrder)

	if !hasSortOrder {
		log.Printf("sqlite:migrate: ensure sort_order column: adding column")
		if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;`); err != nil {
			log.Printf("sqlite:migrate: ensure sort_order column: add column failed: %v", err)
			return fmt.Errorf("add sort_order column: %w", err)
		}
	}

	log.Printf("sqlite:migrate: ensure sort_order column: closing schema rows before backfill")
	if err := rows.Close(); err != nil {
		log.Printf("sqlite:migrate: ensure sort_order column: close rows failed: %v", err)
		return fmt.Errorf("close rules schema rows: %w", err)
	}
	rows = nil

	log.Printf("sqlite:migrate: ensure sort_order column: backfilling sort_order")
	result, err := s.db.Exec(`
		UPDATE rules
		SET sort_order = id
		WHERE sort_order = 0;
	`)
	if err != nil {
		log.Printf("sqlite:migrate: ensure sort_order column: backfill failed: %v", err)
		return fmt.Errorf("backfill sort_order column: %w", err)
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr == nil {
		log.Printf("sqlite:migrate: ensure sort_order column: backfill affected_rows=%d", affected)
	} else {
		log.Printf("sqlite:migrate: ensure sort_order column: backfill rows affected unavailable: %v", affectedErr)
	}
	log.Printf("sqlite:migrate: ensure sort_order column: complete")

	return nil
}

func (s *Store) ensureBackupSortOrderColumn() error {
	rows, err := s.db.Query(`PRAGMA table_info(backup_tasks);`)
	if err != nil {
		return fmt.Errorf("query backup_tasks schema: %w", err)
	}
	defer rows.Close()

	hasSortOrder := false
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan backup_tasks schema: %w", err)
		}
		if strings.EqualFold(name, "sort_order") {
			hasSortOrder = true
			break
		}
	}

	if !hasSortOrder {
		if _, err := s.db.Exec(`ALTER TABLE backup_tasks ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;`); err != nil {
			return fmt.Errorf("add backup_tasks sort_order column: %w", err)
		}
	}

	if err := rows.Close(); err != nil {
		return fmt.Errorf("close backup_tasks schema rows: %w", err)
	}

	if _, err := s.db.Exec(`
		UPDATE backup_tasks
		SET sort_order = id
		WHERE sort_order = 0;
	`); err != nil {
		return fmt.Errorf("backfill backup_tasks sort_order column: %w", err)
	}

	return nil
}

func (s *Store) ensureRuleMatchFiltersColumn() error {
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
		if strings.EqualFold(name, "match_filters_json") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN match_filters_json TEXT NOT NULL DEFAULT '[]';`); err != nil {
		return fmt.Errorf("add match_filters_json column: %w", err)
	}

	return nil
}

func (s *Store) ensureRuleWhitelistColumn() error {
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
		if strings.EqualFold(name, "whitelist_json") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN whitelist_json TEXT NOT NULL DEFAULT '[]';`); err != nil {
		return fmt.Errorf("add whitelist_json column: %w", err)
	}

	return nil
}

func (s *Store) ensureRuleNestFiltersColumn() error {
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
		if strings.EqualFold(name, "nest_filters_json") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN nest_filters_json TEXT NOT NULL DEFAULT '[]';`); err != nil {
		return fmt.Errorf("add nest_filters_json column: %w", err)
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

func (s *Store) ensureRuleTypeColumn() error {
	exists := false
	rows, err := s.db.Query(`PRAGMA table_info(rules);`)
	if err != nil {
		return fmt.Errorf("query rules schema: %w", err)
	}
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("scan rules schema: %w", err)
		}
		if strings.EqualFold(name, "rule_type") {
			exists = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate rules schema: %w", err)
	}
	rows.Close()

	if !exists {
		if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN rule_type TEXT NOT NULL DEFAULT 'archive';`); err != nil {
			return fmt.Errorf("add rule_type column: %w", err)
		}
	}

	// rule_type 是由 archive_mode 派生的冗余字段。历史上该列以默认值 'archive'
	// 新增，会把既有的净化/链路/命名规则误标为归档，分页筛选后导致这些规则
	// 在其栏目里“消失”。这里按 archive_mode 回填纠正，保证分类始终一致。
	return s.normalizeRuleType()
}

// normalizeRuleType 依据 archive_mode 回填并纠正 rule_type。
func (s *Store) normalizeRuleType() error {
	const derived = `CASE
			WHEN archive_mode IN ('cleanup','transform') THEN 'cleanup'
			WHEN archive_mode = 'link' THEN 'link'
			WHEN archive_mode = 'naming' THEN 'naming'
			ELSE 'archive'
		END`
	query := `UPDATE rules SET rule_type = ` + derived + `
		WHERE rule_type IS NULL OR rule_type = '' OR rule_type <> ` + derived
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("normalize rule_type: %w", err)
	}
	return nil
}

func (s *Store) ensureRuleLinkModeColumn() error {
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
		if strings.EqualFold(name, "link_mode") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN link_mode TEXT NOT NULL DEFAULT '';`); err != nil {
		return fmt.Errorf("add link_mode column: %w", err)
	}

	return nil
}

func (s *Store) ensureRuleOptionValuesColumn() error {
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
		if strings.EqualFold(name, "option_values_json") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN option_values_json TEXT NOT NULL DEFAULT '{}';`); err != nil {
		return fmt.Errorf("add option_values_json column: %w", err)
	}

	return nil
}

func (s *Store) ensureRuleTransformRulesColumn() error {
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
		if strings.EqualFold(name, "transform_rules_json") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN transform_rules_json TEXT NOT NULL DEFAULT '[]';`); err != nil {
		return fmt.Errorf("add transform_rules_json column: %w", err)
	}

	return nil
}

func (s *Store) ensureRuleTransformFiltersColumn() error {
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
		if strings.EqualFold(name, "transform_filters_json") {
			return nil
		}
	}

	if _, err := s.db.Exec(`ALTER TABLE rules ADD COLUMN transform_filters_json TEXT NOT NULL DEFAULT '[]';`); err != nil {
		return fmt.Errorf("add transform_filters_json column: %w", err)
	}

	return nil
}
