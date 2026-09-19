package sqlite

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
)

// 运行日志从主库拆到独立日志库（run_history_store.go）的验收。
//
// 关键约定：
//   - 升级后第一次启动把主库里的 run_history 搬到日志库，主库里那张表随即删掉；
//   - 旧库可能缺后加的列（size_bytes / link_mode / deleted_count / detail_json），
//     缺列按默认值补齐，不能因为少一列就放弃整张旧表；
//   - 只有「每一行的 id 都能在新表里找到」才允许删旧表；重复启动不重复搬（INSERT OR IGNORE）。

// createLegacyRunHistoryTable 造一个「升级前」的主库表结构：run_history 还在主库里，
// 且只有最早那批列 —— 后加的四个列一个都没有。
func createLegacyRunHistoryTable(t *testing.T, path string) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy store: %v", err)
	}
	defer func() { _ = db.Close() }()

	statements := []string{
		`CREATE TABLE run_history (
			id TEXT PRIMARY KEY,
			rule_id INTEGER,
			rule_name TEXT NOT NULL DEFAULT '',
			trigger_mode TEXT NOT NULL,
			archive_mode TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			processed_files INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			skip_count INTEGER NOT NULL DEFAULT 0,
			failure_count INTEGER NOT NULL DEFAULT 0,
			summary TEXT NOT NULL DEFAULT '',
			started_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			finished_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX idx_run_history_started ON run_history(started_at DESC, id DESC);`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create legacy run_history: %v", err)
		}
	}
}

// insertLegacyRunHistory 往旧表里塞一行（只用老列）。
func insertLegacyRunHistory(t *testing.T, path, id, ruleName string, started time.Time) {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy store: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`
		INSERT INTO run_history (
			id, rule_id, rule_name, trigger_mode, archive_mode, status,
			processed_files, success_count, skip_count, failure_count,
			summary, started_at, updated_at, finished_at
		) VALUES (?, 3, ?, 'manual', 'package', 'success', 2, 2, 0, 0, '打包完成', ?, ?, ?)
	`, id, ruleName, started.Format(time.RFC3339), started.Format(time.RFC3339), started.Format(time.RFC3339)); err != nil {
		t.Fatalf("insert legacy run history %s: %v", id, err)
	}
}

// mainStoreHasTable 直连主库文件确认某张表还在不在（迁移后旧表必须消失）。
func mainStoreHasTable(t *testing.T, path, table string) bool {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open main store: %v", err)
	}
	defer func() { _ = db.Close() }()

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
		t.Fatalf("inspect main store: %v", err)
	}

	return count > 0
}

// TestOpenMigratesLegacyRunHistoryIntoLogStore 升级路径：主库里已有的运行日志
// 搬到独立日志库，内容一条不少，主库旧表删除，且之后照常读写。
func TestOpenMigratesLegacyRunHistoryIntoLogStore(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "app.db")
	logPath := filepath.Join(dir, "log", "logs.db")

	createLegacyRunHistoryTable(t, mainPath)
	started := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	insertLegacyRunHistory(t, mainPath, "legacy-1", "归档任务", started)
	insertLegacyRunHistory(t, mainPath, "legacy-2", "归档任务", started)

	store, err := Open(config.Env{DBPath: mainPath, LogDBPath: logPath})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("运行日志库文件应独立存在 %s: %v", logPath, err)
	}
	if got := store.LogDBPath(); got != logPath {
		t.Fatalf("LogDBPath = %s, want %s", got, logPath)
	}

	items, err := store.ListRunHistory()
	if err != nil {
		t.Fatalf("list run history: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("迁移后应有 2 条运行日志，实际 %d", len(items))
	}

	byID := make(map[string]model.RunHistoryItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	migrated, ok := byID["legacy-1"]
	if !ok {
		t.Fatalf("legacy-1 没被搬过来: %+v", items)
	}
	if migrated.RuleName != "归档任务" || migrated.Status != "success" || migrated.SuccessCount != 2 {
		t.Fatalf("迁移后内容对不上: %+v", migrated)
	}
	if !migrated.StartedAt.UTC().Equal(started) {
		t.Fatalf("started_at 迁移后变化: %s != %s", migrated.StartedAt.UTC(), started)
	}
	// 旧库缺的列按默认值补，不能让整行迁移失败。
	if migrated.DetailJSON != "" || migrated.DeletedCount != 0 || migrated.LinkMode != "" || migrated.SizeBytes != 0 {
		t.Fatalf("缺列应补默认值: %+v", migrated)
	}

	if mainStoreHasTable(t, mainPath, "run_history") {
		t.Fatalf("迁移完成后主库里不应再有 run_history 表")
	}

	// 迁移之后写入 / 读取一切照常（新记录落在日志库）。
	if err := store.UpsertRunHistory(model.RunHistoryItem{
		ID:          "after-migration",
		RuleName:    "归档任务",
		TriggerMode: model.TriggerModeManual,
		ArchiveMode: "package",
		Status:      "success",
		DetailJSON:  `{"kind":"archive","files":[],"counts":{},"files_total":0}`,
		StartedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("upsert run history after migration: %v", err)
	}
	got, err := store.GetRunHistoryByID("after-migration")
	if err != nil {
		t.Fatalf("get run history after migration: %v", err)
	}
	if got.DetailJSON == "" {
		t.Fatalf("迁移后写入的明细读不回来: %+v", got)
	}
}

// TestOpenKeepsLegacyRowsWhenLogStoreAlreadyHasThem 断点重跑：日志库里已经有这些记录
// （比如上次搬到一半就退出了），再启动一次不能出现重复，且旧表仍要清掉。
func TestOpenKeepsLegacyRowsWhenLogStoreAlreadyHasThem(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "app.db")
	logPath := filepath.Join(dir, "log", "logs.db")

	createLegacyRunHistoryTable(t, mainPath)
	started := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	insertLegacyRunHistory(t, mainPath, "legacy-1", "归档任务", started)

	// 手工造一个「已经搬过一条」的日志库。
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	logDB, err := sql.Open("sqlite", logPath)
	if err != nil {
		t.Fatalf("open log store: %v", err)
	}
	if _, err := logDB.Exec(runHistorySchema); err != nil {
		t.Fatalf("create log schema: %v", err)
	}
	if _, err := logDB.Exec(`
		INSERT INTO run_history (id, rule_name, trigger_mode, archive_mode, status, started_at, updated_at)
		VALUES ('legacy-1', '归档任务', 'manual', 'package', 'success', ?, ?)
	`, started.Format(time.RFC3339), started.Format(time.RFC3339)); err != nil {
		t.Fatalf("seed log store: %v", err)
	}
	_ = logDB.Close()

	store, err := Open(config.Env{DBPath: mainPath, LogDBPath: logPath})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	items, err := store.ListRunHistory()
	if err != nil {
		t.Fatalf("list run history: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("重复启动不应产生重复记录，实际 %d 条", len(items))
	}
	if mainStoreHasTable(t, mainPath, "run_history") {
		t.Fatalf("日志库已含旧记录时，主库旧表仍应清掉")
	}
}

// TestOpenReopenKeepsRunHistoryInLogStore 正常重启：主库里已经没有被搬，日志库照常可用。
func TestOpenReopenKeepsRunHistoryInLogStore(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "app.db")
	logPath := filepath.Join(dir, "log", "logs.db")

	createLegacyRunHistoryTable(t, mainPath)
	insertLegacyRunHistory(t, mainPath, "legacy-1", "归档任务", time.Now().UTC().Add(-time.Hour).Truncate(time.Second))

	first, err := Open(config.Env{DBPath: mainPath, LogDBPath: logPath})
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	second, err := Open(config.Env{DBPath: mainPath, LogDBPath: logPath})
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer func() { _ = second.Close() }()

	items, err := second.ListRunHistory()
	if err != nil {
		t.Fatalf("list run history: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("重启后应仍是 1 条，实际 %d", len(items))
	}
}

// TestResolveRunHistoryLogPath 没配 NESTIFY_LOG_DB_PATH 时退化成主库同目录的 logs.db，
// 保证本地直接跑（不设环境变量）也能写日志。
func TestResolveRunHistoryLogPath(t *testing.T) {
	cases := []struct {
		name    string
		logPath string
		dbPath  string
		want    string
	}{
		{name: "显式配置优先", logPath: filepath.Join("custom", "logs.db"), dbPath: filepath.Join("data", "app.db"), want: filepath.Join("custom", "logs.db")},
		{name: "未配置时落在主库同目录", logPath: "  ", dbPath: filepath.Join("data", "app.db"), want: filepath.Join("data", "logs.db")},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := resolveRunHistoryLogPath(testCase.logPath, testCase.dbPath); got != testCase.want {
				t.Fatalf("resolveRunHistoryLogPath(%q, %q) = %q, want %q", testCase.logPath, testCase.dbPath, got, testCase.want)
			}
		})
	}
}
