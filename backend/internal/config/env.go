package config

import (
	"os"
	"strings"
)

type Env struct {
	HTTPAddr             string
	WebDir               string
	DBPath               string
	AdminInitialUsername string
	AdminInitialPassword string
	BrowseRoots          []string
}

func LoadEnv() Env {
	return Env{
		HTTPAddr:             envOrDefault("NESTIFY_HTTP_ADDR", ":8080"),
		WebDir:               os.Getenv("NESTIFY_WEB_DIR"),
		DBPath:               envOrDefault("NESTIFY_DB_PATH", "../data/app.db"),
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
