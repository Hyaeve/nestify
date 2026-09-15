package sqlite

import (
	"fmt"
	"strings"
	"time"

	"nestify/backend/internal/model"
)

const mountColumns = `id, name, provider, auth_type, scheme, host, port, username, password, token, base_path, enabled, sort_order, created_at, updated_at`

// ListMountCredentials 返回全部挂载（含明文口令），仅限服务端内部使用。
func (s *Store) ListMountCredentials(enabledOnly bool) ([]model.MountCredential, error) {
	query := `SELECT ` + mountColumns + ` FROM webdav_mounts`
	if enabledOnly {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY sort_order ASC, id ASC;`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query mounts: %w", err)
	}
	defer rows.Close()

	items := make([]model.MountCredential, 0)
	for rows.Next() {
		credential, scanErr := scanMountWithPassword(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, credential)
	}

	return items, nil
}

// ListMounts 返回对外展示的挂载列表（不含口令）。
func (s *Store) ListMounts() ([]model.WebdavMount, error) {
	credentials, err := s.ListMountCredentials(false)
	if err != nil {
		return nil, err
	}

	items := make([]model.WebdavMount, 0, len(credentials))
	for _, credential := range credentials {
		items = append(items, credential.Mount)
	}
	return items, nil
}

// GetMountCredential 按 ID 读取挂载（含明文口令）。
func (s *Store) GetMountCredential(id int64) (*model.MountCredential, error) {
	row := s.db.QueryRow(`SELECT `+mountColumns+` FROM webdav_mounts WHERE id = ?;`, id)
	credential, err := scanMountWithPassword(row)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, nil
		}
		return nil, err
	}
	return &credential, nil
}

func (s *Store) CreateMount(input model.CreateMountInput) (*model.WebdavMount, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	result, err := s.db.Exec(`
		INSERT INTO webdav_mounts (name, provider, auth_type, scheme, host, port, username, password, token, base_path, enabled, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`,
		strings.TrimSpace(input.Name),
		model.NormalizeMountProvider(input.Provider),
		model.NormalizeMountAuthType(input.AuthType),
		model.NormalizeMountScheme(strings.TrimSpace(input.Scheme)),
		strings.TrimSpace(input.Host),
		input.Port,
		strings.TrimSpace(input.Username),
		input.Password,
		strings.TrimSpace(input.Token),
		model.NormalizeMountBasePath(input.BasePath),
		boolToInt(enabled),
		input.SortOrder,
		now,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert mount: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("mount last insert id: %w", err)
	}

	credential, err := s.GetMountCredential(id)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return nil, fmt.Errorf("mount %d not found after insert", id)
	}
	return &credential.Mount, nil
}

func (s *Store) UpdateMount(id int64, input model.UpdateMountInput) (*model.WebdavMount, error) {
	existing, err := s.GetMountCredential(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	password := input.Password
	if strings.TrimSpace(password) == "" {
		password = existing.Password
	}
	// 令牌与密码同策略：留空表示沿用已保存的值（前端编辑时不回显就不提交）。
	token := strings.TrimSpace(input.Token)
	if token == "" {
		token = existing.Token
	}

	enabled := existing.Mount.Enabled
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`
		UPDATE webdav_mounts
		SET name = ?, provider = ?, auth_type = ?, scheme = ?, host = ?, port = ?, username = ?, password = ?, token = ?, base_path = ?, enabled = ?, sort_order = ?, updated_at = ?
		WHERE id = ?;
	`,
		strings.TrimSpace(input.Name),
		model.NormalizeMountProvider(input.Provider),
		model.NormalizeMountAuthType(input.AuthType),
		model.NormalizeMountScheme(strings.TrimSpace(input.Scheme)),
		strings.TrimSpace(input.Host),
		input.Port,
		strings.TrimSpace(input.Username),
		password,
		token,
		model.NormalizeMountBasePath(input.BasePath),
		boolToInt(enabled),
		input.SortOrder,
		now,
		id,
	); err != nil {
		return nil, fmt.Errorf("update mount: %w", err)
	}

	credential, err := s.GetMountCredential(id)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return nil, nil
	}
	return &credential.Mount, nil
}

func (s *Store) SetMountEnabled(id int64, enabled bool) (*model.WebdavMount, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(`UPDATE webdav_mounts SET enabled = ?, updated_at = ? WHERE id = ?;`, boolToInt(enabled), now, id); err != nil {
		return nil, fmt.Errorf("set mount enabled: %w", err)
	}

	credential, err := s.GetMountCredential(id)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return nil, nil
	}
	return &credential.Mount, nil
}

func (s *Store) DeleteMount(id int64) error {
	if _, err := s.db.Exec(`DELETE FROM webdav_mounts WHERE id = ?;`, id); err != nil {
		return fmt.Errorf("delete mount: %w", err)
	}
	return nil
}

type mountScanner interface {
	Scan(dest ...any) error
}

func scanMountWithPassword(scanner mountScanner) (model.MountCredential, error) {
	var (
		id              int64
		name            string
		provider        string
		authType        string
		scheme          string
		host            string
		port            int
		username        string
		password        string
		token           string
		basePath        string
		enabled         int
		sortOrder       int
		createdAtSource string
		updatedAtSource string
	)

	if err := scanner.Scan(&id, &name, &provider, &authType, &scheme, &host, &port, &username, &password, &token, &basePath, &enabled, &sortOrder, &createdAtSource, &updatedAtSource); err != nil {
		return model.MountCredential{}, fmt.Errorf("scan mount: %w", err)
	}

	mount := model.WebdavMount{
		ID:          id,
		Name:        name,
		Provider:    model.NormalizeMountProvider(provider),
		AuthType:    model.NormalizeMountAuthType(authType),
		Scheme:      model.NormalizeMountScheme(scheme),
		Host:        host,
		Port:        port,
		Username:    username,
		HasPassword: strings.TrimSpace(password) != "",
		HasToken:    strings.TrimSpace(token) != "",
		BasePath:    model.NormalizeMountBasePath(basePath),
		Enabled:     intToBool(enabled),
		SortOrder:   sortOrder,
	}
	mount.BaseURL = model.BuildMountBaseURL(mount)
	mount.VirtualPath = model.BuildMountVirtualPath(id)
	mount.CreatedAt, _ = time.Parse(time.RFC3339, createdAtSource)
	mount.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtSource)

	return model.MountCredential{Mount: mount, Password: password, Token: token}, nil
}
