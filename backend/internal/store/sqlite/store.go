package sqlite

import (
	"database/sql"
	"fmt"
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := store.ensureDefaultAdmin(env.AdminInitialUsername, env.AdminInitialPassword); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func hashPassword(password string) (string, error) {
	return auth.HashPassword(password)
}

func (s *Store) Close() error {
	return s.db.Close()
}
