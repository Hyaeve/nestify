package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"nestify/backend/internal/model"
)

func (s *Store) ensureDefaultAdmin(username, password string) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&count); err != nil {
		return fmt.Errorf("count admins: %w", err)
	}

	if count > 0 {
		return nil
	}

	hash, err := hashPassword(password)
	if err != nil {
		return fmt.Errorf("hash default admin password: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		INSERT INTO admins (username, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, strings.TrimSpace(username), hash, now, now); err != nil {
		return fmt.Errorf("insert default admin: %w", err)
	}

	return nil
}

func (s *Store) GetAdminByUsername(username string) (*model.Admin, error) {
	row := s.db.QueryRow(`
		SELECT id, username, password_hash, created_at, updated_at
		FROM admins
		WHERE username = ?
	`, strings.TrimSpace(username))

	var admin model.Admin
	var createdAt, updatedAt string
	err := row.Scan(&admin.ID, &admin.Username, &admin.PasswordHash, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get admin by username: %w", err)
	}

	admin.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	admin.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &admin, nil
}
