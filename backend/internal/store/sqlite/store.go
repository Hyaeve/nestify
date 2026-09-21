package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nestify/backend/internal/auth"
	"nestify/backend/internal/config"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
	// logDB 是运行日志（run_history）的独立数据库连接。
	// 日志的写入频率远高于规则 / 设置这类配置数据，单独一个文件后：
	// 主库不会因为明细 JSON 膨胀，运维上也能只备份配置而不备份日志。
	logDB *sql.DB
	// logPath 与 dbPath 是各自的文件路径 —— 首次启动时要把主库里遗留的
	// run_history 记录搬到日志库里（见 run_history_store.go）。
	logPath string
	dbPath  string

	// 运行日志保留策略的时间节流（见 maybeApplyRunHistoryRetention）：
	// run_history 是「每处理一项写一行」，一次大执行会插上万行；如果每插一行都跑一遍
	// 全表清理，代价就是 O(项数 × 表行数)，容器内存与 CPU 会一起爆掉。
	retentionMu sync.Mutex
	retentionAt time.Time
}

func Open(env config.Env) (*Store, error) {
	path := env.DBPath
	log.Printf("sqlite: preparing db path=%s dir=%s", path, filepath.Dir(path))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("sqlite: create db directory failed: %v", err)
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	log.Printf("sqlite: db directory ready")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Printf("sqlite: sql open failed: %v", err)
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	log.Printf("sqlite: sql open succeeded")

	db.SetMaxOpenConns(1)
	log.Printf("sqlite: max open conns set to 1")

	if _, err := db.Exec(`PRAGMA journal_mode = WAL; PRAGMA synchronous = NORMAL; PRAGMA temp_store = MEMORY;`); err != nil {
		log.Printf("sqlite: configure pragmas failed: %v", err)
		_ = db.Close()
		return nil, fmt.Errorf("configure sqlite pragmas: %w", err)
	}
	log.Printf("sqlite: performance pragmas configured")

	store := &Store{db: db, dbPath: path}
	log.Printf("sqlite: starting migration")
	if err := store.migrate(); err != nil {
		log.Printf("sqlite: migration failed: %v", err)
		_ = db.Close()
		return nil, err
	}
	log.Printf("sqlite: migration complete")

	// 运行日志库：与主库分开的第二个 sqlite 文件。
	logPath := resolveRunHistoryLogPath(env.LogDBPath, path)
	log.Printf("sqlite: opening run history log store path=%s", logPath)
	logDB, err := openRunHistoryLogStore(logPath)
	if err != nil {
		log.Printf("sqlite: open run history log store failed: %v", err)
		_ = db.Close()
		return nil, err
	}
	store.logDB = logDB
	store.logPath = logPath

	log.Printf("sqlite: migrating run history log store")
	if err := store.migrateRunHistoryStore(); err != nil {
		log.Printf("sqlite: run history log store migration failed: %v", err)
		_ = logDB.Close()
		_ = db.Close()
		return nil, err
	}
	log.Printf("sqlite: run history log store ready")

	log.Printf("sqlite: ensuring default admin")
	if err := store.ensureDefaultAdmin(env.AdminInitialUsername, env.AdminInitialPassword); err != nil {
		log.Printf("sqlite: ensure default admin failed: %v", err)
		_ = logDB.Close()
		_ = db.Close()
		return nil, err
	}
	log.Printf("sqlite: default admin ready")

	return store, nil
}

func hashPassword(password string) (string, error) {
	return auth.HashPassword(password)
}

func (s *Store) Close() error {
	var logErr error
	if s.logDB != nil {
		logErr = s.logDB.Close()
	}
	if err := s.db.Close(); err != nil {
		return err
	}
	return logErr
}
