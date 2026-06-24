package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"nestify/backend/internal/auth"
	"nestify/backend/internal/config"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
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

	store := &Store{db: db}
	log.Printf("sqlite: starting migration")
	if err := store.migrate(); err != nil {
		log.Printf("sqlite: migration failed: %v", err)
		_ = db.Close()
		return nil, err
	}
	log.Printf("sqlite: migration complete")

	log.Printf("sqlite: ensuring default admin")
	if err := store.ensureDefaultAdmin(env.AdminInitialUsername, env.AdminInitialPassword); err != nil {
		log.Printf("sqlite: ensure default admin failed: %v", err)
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
	return s.db.Close()
}
