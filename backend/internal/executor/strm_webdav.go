package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nestify/backend/internal/model"
	"nestify/backend/internal/webdav"
)

// isWebdavSource 判断源路径是否指向 WebDAV 挂载目录。
func isWebdavSource(sourceDir string) bool {
	return strings.HasPrefix(strings.TrimSpace(sourceDir), model.MountPathScheme)
}

func parseWebdavSource(sourceDir string) (int64, string, error) {
	trimmed := strings.TrimSpace(sourceDir)
	rest := strings.TrimPrefix(trimmed, model.MountPathScheme)
	parts := strings.SplitN(rest, "/", 2)

	id, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || id <= 0 {
		return 0, "", fmt.Errorf("无效的 WebDAV 挂载源路径：%s", sourceDir)
	}

	internal := ""
	if len(parts) == 2 {
		internal = "/" + strings.TrimLeft(parts[1], "/")
	}
	if internal == "/" {
		internal = ""
	}
	return id, internal, nil
}

// executeWebdavStrmRule 针对 WebDAV 挂载源生成 http strm：
// strm 内容 = 挂载的 http 根地址 + 直链端点（/d）+ WebDAV 内部文件路径。
// 注意使用直链端点而非 WebDAV 端点（/dav），否则媒体服务器无法直接播放。
// 所有远端请求都经由客户端节流，避免对网盘后端造成过高的请求频繁度。
func (s *Service) executeWebdavStrmRule(runID string, req ExecuteRuleRequest, sourceDir, targetDir string, stats *executionStats) (executionStats, error) {
	extensions := normalizeStrmExtensions(req.Filters)
	if len(extensions) == 0 {
		return *stats, fmt.Errorf("strm extensions are required")
	}
	matchers := buildFileNameMatchers(req.Whitelist)

	mountID, internalPath, err := parseWebdavSource(sourceDir)
	if err != nil {
		return *stats, err
	}

	credential, err := s.store.GetMountCredential(mountID)
	if err != nil {
		return *stats, fmt.Errorf("读取 WebDAV 挂载失败: %w", err)
	}
	if credential == nil {
		return *stats, fmt.Errorf("WebDAV 挂载不存在或已被删除")
	}
	if !credential.Mount.Enabled {
		return *stats, fmt.Errorf("WebDAV 挂载「%s」已停用", credential.Mount.Name)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return *stats, fmt.Errorf("create target dir: %w", err)
	}

	client := webdav.NewClient(credential.Mount, credential.Password)

	overwrite := req.Options["strm_overwrite"]

	if req.Options["strm_full_sync"] {
		if err := s.removeExistingStrmFiles(runID, targetDir, req.CompatibilityMode, stats); err != nil {
			return *stats, err
		}
	}

	s.appendLog(runID, "info", fmt.Sprintf("WebDAV 源：%s（%s）", credential.Mount.Name, credential.Mount.BaseURL))

	if err := s.walkWebdavStrm(context.Background(), runID, client, internalPath, internalPath, targetDir, extensions, matchers, overwrite, stats); err != nil {
		return *stats, err
	}

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "未发现可生成 Strm 的媒体文件"
	} else {
		syncLabel := "增量同步"
		if req.Options["strm_full_sync"] {
			syncLabel = "全量同步"
		}
		if overwrite {
			syncLabel += "·覆盖生成"
			stats.Summary = fmt.Sprintf("Strm%s完成（http strm）：%s -> %s；生成/覆盖 %d 个 Strm，跳过 %d 项，失败 %d 项",
				syncLabel, sourceDir, targetDir, stats.SuccessCount, stats.SkipCount, stats.FailureCount)
		} else {
			stats.Summary = fmt.Sprintf("Strm%s完成（http strm）：%s -> %s；生成 %d 个 Strm，跳过 %d 项，失败 %d 项",
				syncLabel, sourceDir, targetDir, stats.SuccessCount, stats.SkipCount, stats.FailureCount)
		}
	}

	if stats.FailureCount > 0 {
		return *stats, fmt.Errorf("strm execution finished with %d failures", stats.FailureCount)
	}

	return *stats, nil
}

func (s *Service) walkWebdavStrm(
	ctx context.Context,
	runID string,
	client *webdav.Client,
	rootInternal string,
	currentInternal string,
	targetRoot string,
	extensions map[string]struct{},
	matchers []fileNameMatcher,
	overwrite bool,
	stats *executionStats,
) error {
	entries, err := client.List(ctx, currentInternal)
	if err != nil {
		stats.FailureCount++
		s.appendLog(runID, "error", fmt.Sprintf("列出 WebDAV 目录 %s 失败：%v", currentInternal, err))
		return nil
	}

	for _, entry := range entries {
		if matchesFileName(entry.Name, entry.IsDir, matchers) {
			stats.SkipCount++
			s.appendLog(runID, "info", fmt.Sprintf("skipped blacklisted entry %s", entry.Path))
			continue
		}

		if entry.IsDir {
			if err := s.walkWebdavStrm(ctx, runID, client, rootInternal, entry.Path, targetRoot, extensions, matchers, overwrite, stats); err != nil {
				return err
			}
			continue
		}

		if !matchesStrmExtension(entry.Name, extensions) {
			stats.SkipCount++
			continue
		}

		relative := strings.TrimPrefix(entry.Path, rootInternal)
		relative = strings.TrimLeft(relative, "/")
		if relative == "" {
			continue
		}

		targetPath := filepath.Join(targetRoot, strmRelativePath(relative))
		existed := false
		if _, err := os.Lstat(targetPath); err == nil {
			existed = true
			if !overwrite {
				stats.SkipCount++
				continue
			}
		}

		if err := writeStrmContent(targetPath, client.BuildStrmURL(entry.Path)); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("create strm %s failed: %v", targetPath, err))
			continue
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		if existed {
			s.appendLog(runID, "info", fmt.Sprintf("overwrote http strm %s", targetPath))
		} else {
			s.appendLog(runID, "info", fmt.Sprintf("created http strm %s", targetPath))
		}
	}

	return nil
}

func strmRelativePath(relative string) string {
	ext := filepath.Ext(relative)
	if ext == "" {
		return relative + ".strm"
	}
	return strings.TrimSuffix(relative, ext) + ".strm"
}

func writeStrmContent(targetPath, content string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create strm parent: %w", err)
	}
	if err := os.WriteFile(targetPath, []byte(content+"\n"), 0o644); err != nil {
		return fmt.Errorf("write strm file: %w", err)
	}
	return nil
}
