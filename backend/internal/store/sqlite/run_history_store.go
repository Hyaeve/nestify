package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// 运行日志（run_history）单独一个 sqlite 文件，不再塞在主库 app.db 里。
//
// 为什么拆：规则 / 设置 / 账号 / 挂载 / 备份任务这些是「配置类」数据，体积稳定、
// 值得天天备份；而运行日志是一次执行落一批、还带着明细 JSON，是唯一会无上限增长的
// 表。混在同一个文件里，主库会随着日志一起膨胀，备份和迁移都得连日志一起搬。
//
// 拆开之后：
//   - 文件位置由 NESTIFY_LOG_DB_PATH 决定（compose 里挂到 /log/logs.db）；
//   - 「运行日志」页与「归巢历史」页的数据都在这个文件里，主库里只剩配置；
//   - 升级时主库遗留的 run_history 会被整体搬过来，然后删掉旧表（见
//     migrateLegacyRunHistory）。

// 日志表结构：与主库里原先的 run_history 完全一致，列名 / 类型都没变，
// 只是换了所在的文件。原先靠 ensureXxxColumn 逐个补的列（size_bytes / link_mode /
// deleted_count / detail_json）这里一次性写全。
const runHistorySchema = `CREATE TABLE IF NOT EXISTS run_history (
	id TEXT PRIMARY KEY,
	rule_id INTEGER,
	rule_name TEXT NOT NULL DEFAULT '',
	trigger_mode TEXT NOT NULL DEFAULT '',
	archive_mode TEXT NOT NULL DEFAULT '',
	link_mode TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	processed_files INTEGER NOT NULL DEFAULT 0,
	success_count INTEGER NOT NULL DEFAULT 0,
	skip_count INTEGER NOT NULL DEFAULT 0,
	failure_count INTEGER NOT NULL DEFAULT 0,
	deleted_count INTEGER NOT NULL DEFAULT 0,
	size_bytes INTEGER NOT NULL DEFAULT 0,
	summary TEXT NOT NULL DEFAULT '',
	detail_json TEXT NOT NULL DEFAULT '',
	detail_size INTEGER NOT NULL DEFAULT 0,
	started_at TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT '',
	finished_at TEXT NOT NULL DEFAULT ''
);`

// 日志表的索引，跟主库时代保持同样的三组（列表默认排序、状态筛选、模式筛选）。
var runHistoryIndexStatements = []string{
	`CREATE INDEX IF NOT EXISTS idx_run_history_started ON run_history(started_at DESC, id DESC);`,
	`CREATE INDEX IF NOT EXISTS idx_run_history_status ON run_history(status);`,
	`CREATE INDEX IF NOT EXISTS idx_run_history_archive_mode ON run_history(archive_mode);`,
}

// runHistoryColumns 是日志表的全部列，顺序与 run_historySchema / 各条 SELECT 一致。
//
// defaultValue 只在「迁移主库旧表」时用到：升级路径上老版本的主库可能还没有
// detail_json / deleted_count 这些列，缺列就填同类型的默认值 —— 不能因为少一列就
// 把整张旧表放弃掉。
var runHistoryColumns = []struct {
	name         string
	defaultValue string
}{
	{"id", "''"},
	{"rule_id", "NULL"},
	{"rule_name", "''"},
	{"trigger_mode", "''"},
	{"archive_mode", "''"},
	{"link_mode", "''"},
	{"status", "''"},
	{"processed_files", "0"},
	{"success_count", "0"},
	{"skip_count", "0"},
	{"failure_count", "0"},
	{"deleted_count", "0"},
	{"size_bytes", "0"},
	{"summary", "''"},
	{"detail_json", "''"},
	{"detail_size", "0"},
	{"started_at", "''"},
	{"updated_at", "''"},
	{"finished_at", "''"},
}

// resolveRunHistoryLogPath 决定日志库文件放哪。
// 没配 NESTIFY_LOG_DB_PATH 时退化成「主库同目录下的 logs.db」——
// 本地直接跑（不进容器、不设环境变量）也不会把日志写丢。
func resolveRunHistoryLogPath(logPath, dbPath string) string {
	if trimmed := strings.TrimSpace(logPath); trimmed != "" {
		return trimmed
	}

	return filepath.Join(filepath.Dir(dbPath), "logs.db")
}

// openRunHistoryLogStore 打开日志库连接：建目录、开库、设 pragma。
// 参数与主库保持一致（单连接 + WAL），两边读写特性一样，行为可预期。
func openRunHistoryLogStore(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create run history log directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open run history log sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA journal_mode = WAL; PRAGMA synchronous = NORMAL; PRAGMA temp_store = MEMORY;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure run history log pragmas: %w", err)
	}

	return db, nil
}

// migrateRunHistoryStore 准备日志库：建表、补列、建索引，最后搬走主库里的旧记录。
//
// 顺序不能反：先把表建好，迁移才插得进去；旧表只在「校验搬齐了」之后才删。
func (s *Store) migrateRunHistoryStore() error {
	log.Printf("sqlite:runhistory: creating log store schema")
	if _, err := s.logDB.Exec(runHistorySchema); err != nil {
		return fmt.Errorf("create run history table in log store: %w", err)
	}

	addedDetailSize, err := s.ensureRunHistoryColumns()
	if err != nil {
		return err
	}

	for _, statement := range runHistoryIndexStatements {
		if _, err := s.logDB.Exec(statement); err != nil {
			return fmt.Errorf("create run history index in log store: %w", err)
		}
	}
	log.Printf("sqlite:runhistory: log store schema ready")

	reclaimed, err := s.migrateLegacyRunHistory()
	if err != nil {
		return err
	}
	if reclaimed {
		// 旧表删掉只是把页还给 freelist，文件本身不会变小；跑一次 VACUUM
		// 才真的把主库文件缩回去（用户要的就是「日志从数据文件里分离出去」）。
		s.vacuumMainStore()
	}

	// 回填排在旧表迁移之后：搬过来的行同样带着明细，也要一起算长度。
	if addedDetailSize {
		s.backfillRunHistoryDetailSize()
	}

	return nil
}

// LogDBPath 返回运行日志库实际使用的文件路径（已经解析过默认值）。
// 供系统信息接口展示 —— 排查「日志写到哪去了」时不用猜。
func (s *Store) LogDBPath() string {
	if s == nil {
		return ""
	}

	return s.logPath
}

// ensureRunHistoryColumns 兜住「日志库是更早的版本建的、还缺列」的情况。
// 新库建表时列就齐了，这里是幂等的保险。
//
// 返回值表示 detail_size 是否**本次才补上**：只有这一次需要回填老数据
// （存量行没有这个值，不回填的话折叠组会挑不到最完整的那份明细）。
func (s *Store) ensureRunHistoryColumns() (bool, error) {
	additions := []struct {
		name string
		ddl  string
	}{
		{"size_bytes", `ALTER TABLE run_history ADD COLUMN size_bytes INTEGER NOT NULL DEFAULT 0;`},
		{"link_mode", `ALTER TABLE run_history ADD COLUMN link_mode TEXT NOT NULL DEFAULT '';`},
		{"deleted_count", `ALTER TABLE run_history ADD COLUMN deleted_count INTEGER NOT NULL DEFAULT 0;`},
		{"detail_json", `ALTER TABLE run_history ADD COLUMN detail_json TEXT NOT NULL DEFAULT '';`},
		{"detail_size", `ALTER TABLE run_history ADD COLUMN detail_size INTEGER NOT NULL DEFAULT 0;`},
	}

	columns, err := s.logRunHistoryColumns()
	if err != nil {
		return false, err
	}

	addedDetailSize := false
	for _, addition := range additions {
		if columns[addition.name] {
			continue
		}
		log.Printf("sqlite:runhistory: add missing column %s", addition.name)
		if _, err := s.logDB.Exec(addition.ddl); err != nil {
			return false, fmt.Errorf("add run history log column %s: %w", addition.name, err)
		}
		if addition.name == "detail_size" {
			addedDetailSize = true
		}
	}

	return addedDetailSize, nil
}

// backfillRunHistoryDetailSize 给存量行补上明细长度。
//
// 为什么要这个值：折叠组要挑出「明细最完整的那一份」当代表行，原来靠
// `ORDER BY LENGTH(detail_json)` 排序 —— 那会让**每一次列表查询**都把所有行的
// detail_json 读出来（一次执行上万行、每行累积明细可达几十 KB 就是几百 MB，
// 仪表盘还每 5 秒拉一次）。改成先把长度落成一列，排序只读这个整数列。
//
// 按 rowid 游标分小批推进，单批只读 500 行：既不会把整个大字段表读进内存，
// 中断了下次也只是少回填一部分（那些行的 detail_size 保持 0，排序里排在最后，
// 折叠组仍会优先选中真正带明细的那条），不会把库写坏。
func (s *Store) backfillRunHistoryDetailSize() {
	const batchSize = 500

	cursor := int64(0)
	processed := 0
	for {
		rows, err := s.logDB.Query(`SELECT rowid FROM run_history WHERE rowid > ? AND detail_size = 0 ORDER BY rowid LIMIT ?`, cursor, batchSize)
		if err != nil {
			log.Printf("sqlite:runhistory: backfill detail_size aborted: %v", err)
			return
		}

		ids := make([]int64, 0, batchSize)
		for rows.Next() {
			var rowID int64
			if scanErr := rows.Scan(&rowID); scanErr != nil {
				rows.Close()
				log.Printf("sqlite:runhistory: backfill detail_size aborted: %v", scanErr)
				return
			}
			ids = append(ids, rowID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			log.Printf("sqlite:runhistory: backfill detail_size aborted: %v", err)
			return
		}
		if len(ids) == 0 {
			break
		}

		placeholders := make([]string, len(ids))
		args := make([]any, 0, len(ids))
		for index, rowID := range ids {
			placeholders[index] = "?"
			args = append(args, rowID)
		}
		if _, err := s.logDB.Exec(`UPDATE run_history SET detail_size = LENGTH(CAST(COALESCE(detail_json, '') AS BLOB)) WHERE rowid IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
			log.Printf("sqlite:runhistory: backfill detail_size aborted: %v", err)
			return
		}

		cursor = ids[len(ids)-1]
		processed += len(ids)
	}

	if processed > 0 {
		log.Printf("sqlite:runhistory: backfilled detail_size for %d legacy rows", processed)
	}
}

// logRunHistoryColumns 读出日志表现有的列名（小写）。
func (s *Store) logRunHistoryColumns() (map[string]bool, error) {
	rows, err := s.logDB.Query(`PRAGMA table_info(run_history);`)
	if err != nil {
		return nil, fmt.Errorf("query run history log schema: %w", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("scan run history log schema: %w", err)
		}
		columns[strings.ToLower(name)] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run history log schema: %w", err)
	}

	return columns, nil
}

// migrateLegacyRunHistory 把主库里遗留的 run_history 整体搬到日志库，成功就删掉旧表。
//
// 只在主库里还存在 run_history 表时才有事可做 —— 搬完即删，所以正常情况下只在
// 升级后的第一次启动跑一次。约定：
//   - 半途失败（附加不上、插入报错）→ 保留旧表、留日志，下次启动重试，
//     日志库本身照常可用，不影响服务启动；
//   - 校验「旧表每一行的 id 都能在新表里找到」通过后才 DROP，绝不带着可能丢日志的
//     状态删表；
//   - 用 INSERT OR IGNORE，重复启动不会因为主键冲突整批失败。
//
// 返回值表示是否可以回收主库空间（旧表已删）。
func (s *Store) migrateLegacyRunHistory() (bool, error) {
	if strings.TrimSpace(s.dbPath) == "" {
		return false, nil
	}
	// ATTACH 只认文件路径，不认 DSN：主库路径写成 URI（file:...）时不去猜真实文件名，
	// 免得在别处凭空造出一个同名空文件。这种情况直接跳过迁移。
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(s.dbPath)), "file:") {
		log.Printf("sqlite:runhistory: main store path uses a sqlite URI, skipping legacy run history migration")
		return false, nil
	}

	if _, err := s.logDB.Exec(`ATTACH DATABASE '` + escapeSQLiteLiteral(s.dbPath) + `' AS legacy`); err != nil {
		log.Printf("sqlite:runhistory: attach legacy store failed (skip migration): %v", err)
		return false, nil
	}
	defer func() {
		if _, err := s.logDB.Exec(`DETACH DATABASE legacy`); err != nil {
			log.Printf("sqlite:runhistory: detach legacy store failed: %v", err)
		}
	}()

	exists, err := s.legacyRunHistoryExists()
	if err != nil {
		log.Printf("sqlite:runhistory: inspect legacy store failed (skip migration): %v", err)
		return false, nil
	}
	if !exists {
		return false, nil
	}

	// 附加进来的必须确实是主库本身：用 settings 表做指纹。
	// 路径形态万一没被 sqlite 当成同一个文件（相对路径 / URI 写法），附加出来的会是个空库，
	// 那样「迁移」会搬出零行、还顺手把真实旧表留在主库，用户看到的是「日志全没了」。
	// 指纹对不上就什么都不做 —— 旧表原样保留，日志库照常可用。
	marker, err := s.tableExistsIn("legacy", "settings")
	if err != nil {
		log.Printf("sqlite:runhistory: inspect legacy store fingerprint failed (skip migration): %v", err)
		return false, nil
	}
	if !marker {
		log.Printf("sqlite:runhistory: attached store has no settings table, not the main store, keep legacy table untouched")
		return false, nil
	}

	legacyColumns, err := s.legacyRunHistoryColumns()
	if err != nil {
		log.Printf("sqlite:runhistory: read legacy schema failed (skip migration): %v", err)
		return false, nil
	}
	if !legacyColumns["id"] {
		log.Printf("sqlite:runhistory: legacy run_history has no id column, keep it untouched")
		return false, nil
	}

	var legacyRows int
	if err := s.logDB.QueryRow(`SELECT COUNT(*) FROM legacy.run_history`).Scan(&legacyRows); err != nil {
		log.Printf("sqlite:runhistory: count legacy rows failed (skip migration): %v", err)
		return false, nil
	}
	if legacyRows == 0 {
		log.Printf("sqlite:runhistory: legacy run_history is empty, dropping it")
		return true, s.dropLegacyRunHistory()
	}

	log.Printf("sqlite:runhistory: migrating %d legacy rows into log store", legacyRows)
	if _, err := s.logDB.Exec(buildLegacyRunHistoryInsert(legacyColumns)); err != nil {
		log.Printf("sqlite:runhistory: copy legacy rows failed (keep legacy table): %v", err)
		return false, nil
	}

	var migrated int
	if err := s.logDB.QueryRow(`SELECT COUNT(*) FROM legacy.run_history WHERE id IN (SELECT id FROM run_history)`).Scan(&migrated); err != nil {
		log.Printf("sqlite:runhistory: verify legacy rows failed (keep legacy table): %v", err)
		return false, nil
	}
	if migrated != legacyRows {
		log.Printf("sqlite:runhistory: only %d/%d legacy rows present in log store, keep legacy table for retry", migrated, legacyRows)
		return false, nil
	}

	log.Printf("sqlite:runhistory: migrated %d rows, dropping legacy table", migrated)
	return true, s.dropLegacyRunHistory()
}

// tableExistsIn 判断某个（附加的）库里有没有这张表。
func (s *Store) tableExistsIn(schema, table string) (bool, error) {
	var count int
	if err := s.logDB.QueryRow(`SELECT COUNT(*) FROM `+schema+`.sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

// legacyRunHistoryExists 判断主库里是否还留着旧的 run_history 表。
func (s *Store) legacyRunHistoryExists() (bool, error) {
	return s.tableExistsIn("legacy", "run_history")
}

// legacyRunHistoryColumns 读出主库旧表的列名（小写），用来拼迁移用的 SELECT。
func (s *Store) legacyRunHistoryColumns() (map[string]bool, error) {
	rows, err := s.logDB.Query(`PRAGMA legacy.table_info(run_history);`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[strings.ToLower(name)] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return columns, nil
}

// dropLegacyRunHistory 删掉主库里的旧日志表（索引随表一起消失）。
func (s *Store) dropLegacyRunHistory() error {
	if _, err := s.logDB.Exec(`DROP TABLE IF EXISTS legacy.run_history`); err != nil {
		return fmt.Errorf("drop legacy run history table: %w", err)
	}

	return nil
}

// vacuumMainStore 回收主库删表后留下的空闲页。失败只记日志：
// 空间回收是「顺手做」的事，不该让服务启动不了。
func (s *Store) vacuumMainStore() {
	log.Printf("sqlite:runhistory: vacuuming main store to reclaim space")
	if _, err := s.db.Exec(`VACUUM;`); err != nil {
		log.Printf("sqlite:runhistory: vacuum main store skipped: %v", err)
		return
	}
	log.Printf("sqlite:runhistory: main store vacuum complete")
}

// buildLegacyRunHistoryInsert 生成「旧表 → 新表」的搬运语句。
// 旧表缺的列用默认值顶替，保证列数永远对得上。
func buildLegacyRunHistoryInsert(legacyColumns map[string]bool) string {
	names := make([]string, 0, len(runHistoryColumns))
	exprs := make([]string, 0, len(runHistoryColumns))

	for _, column := range runHistoryColumns {
		names = append(names, column.name)
		if legacyColumns[strings.ToLower(column.name)] {
			exprs = append(exprs, column.name)
			continue
		}
		exprs = append(exprs, column.defaultValue)
	}

	return `INSERT OR IGNORE INTO run_history (` + strings.Join(names, ", ") + `) SELECT ` +
		strings.Join(exprs, ", ") + ` FROM legacy.run_history`
}

// escapeSQLiteLiteral 把路径塞进 SQL 字符串字面量（ATTACH 的文件名不能参数化，
// 只能自己转义单引号）。
func escapeSQLiteLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
