package config

import (
	"os"
	"strings"
)

type Env struct {
	HTTPAddr string
	WebDir   string
	// DBPath 是主数据库（规则 / 设置 / 账号 / 挂载 / 备份任务）。
	DBPath string
	// LogDBPath 是运行日志（run_history，含归巢历史与明细载荷）的独立数据库文件。
	// 单独放一个文件是为了让日志可以跟主数据分开挂载、单独清理或备份。
	// 留空时 sqlite.Open 会在 DBPath 同目录下取 logs.db。
	LogDBPath            string
	AdminInitialUsername string
	AdminInitialPassword string
	BrowseRoots          []string
}

func LoadEnv() Env {
	return Env{
		HTTPAddr:             envOrDefault("NESTIFY_HTTP_ADDR", ":8080"),
		WebDir:               os.Getenv("NESTIFY_WEB_DIR"),
		DBPath:               envOrDefault("NESTIFY_DB_PATH", "../data/app.db"),
		LogDBPath:            envOrDefault("NESTIFY_LOG_DB_PATH", "../log/logs.db"),
		AdminInitialUsername: envOrDefault("NESTIFY_ADMIN_INITIAL_USERNAME", "admin"),
		AdminInitialPassword: envOrDefault("NESTIFY_ADMIN_INITIAL_PASSWORD", "nestify123"),
		BrowseRoots:          parseBrowseRoots(os.Getenv("NESTIFY_BROWSE_ROOTS")),
	}
}

func parseBrowseRoots(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	normalized := strings.ReplaceAll(raw, ",", ";")
	parts := strings.Split(normalized, ";")
	items := make([]string, 0, len(parts))

	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			items = append(items, value)
		}
	}

	return items
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
