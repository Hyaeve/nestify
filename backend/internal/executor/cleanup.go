package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type fileNameMatcher struct {
	target        ruleMatcherTarget
	literal       string
	regex         *regexp.Regexp
	fuzzy         bool
	caseSensitive bool
}

type ruleMatcherTarget int

const (
	ruleMatcherFileName ruleMatcherTarget = iota
	ruleMatcherExtension
	ruleMatcherDirectoryName
	ruleMatcherGlobal
)

func (s *Service) executeCleanupRule(runID string, req ExecuteRuleRequest) (executionStats, error) {
	stats := executionStats{}
	sourceDir := filepath.Clean(strings.TrimSpace(req.SourceDir))
	if sourceDir == "" || sourceDir == "." {
		return stats, fmt.Errorf("source dir is required")
	}

	info, err := statWithMode(req.CompatibilityMode, sourceDir)
	if err != nil {
		return stats, fmt.Errorf("stat source dir: %w", err)
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("source dir must be a directory")
	}

	cleanupEmptyDirs := req.Options["cleanup_empty_dirs"]
	cleanupMatchingFiles := req.Options["cleanup_matching_files"]
	cleanupExpiredFiles := req.Options["cleanup_expired_files"]
	cleanupRetentionDays := req.OptionValues["cleanup_retention_days"]
	if !cleanupEmptyDirs && !cleanupMatchingFiles && !cleanupExpiredFiles {
		stats.SkipCount = 1
		stats.Summary = "no cleanup actions enabled"
		return stats, nil
	}
	if cleanupExpiredFiles && cleanupRetentionDays < 1 {
		stats.SkipCount = 1
		stats.Summary = "cleanup_expired_files enabled but no valid retention days provided"
		return stats, nil
	}

	matchers := buildCaseSensitiveFileNameMatchers(req.Filters)
	whitelist := buildDirectoryWhitelist(req.Whitelist)
	if cleanupMatchingFiles && len(matchers) == 0 {
		stats.SkipCount = 1
		stats.Summary = "cleanup_matching_files enabled but no valid matchers provided"
		return stats, nil
	}

	s.cleanupDirectory(runID, sourceDir, sourceDir, req.CompatibilityMode, cleanupEmptyDirs, cleanupMatchingFiles, cleanupExpiredFiles, cleanupRetentionDays, matchers, whitelist, &stats)

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "未发现可清理项目"
	} else {
		stats.Summary = fmt.Sprintf("清理完成：删除 %d 个文件、%d 个文件夹，失败 %d 项", stats.CleanupRemovedFiles, stats.CleanupRemovedDirs, stats.FailureCount)
	}

	if stats.FailureCount > 0 {
		return stats, fmt.Errorf("cleanup finished with %d failures", stats.FailureCount)
	}

	return stats, nil
}

func (s *Service) cleanupDirectory(runID, rootPath, currentPath, compatibilityMode string, cleanupEmptyDirs, cleanupMatchingFiles, cleanupExpiredFiles bool, cleanupRetentionDays int, matchers []fileNameMatcher, whitelist map[string]struct{}, stats *executionStats) {
	entries, err := readDirWithMode(compatibilityMode, currentPath)
	if err != nil {
		stats.FailureCount++
		s.persistRunHistory(runID, fmt.Sprintf("read cleanup directory %s failed: %v", currentPath, err), stats)
		s.appendLog(runID, "error", fmt.Sprintf("read cleanup directory %s failed: %v", currentPath, err))
		return
	}
	entries = limitEntriesForMode(compatibilityMode, entries)

	sortEntriesNaturally(entries)
	_ = processEntriesForMode(compatibilityMode, entries, func(entry os.DirEntry) error {
		entryPath := filepath.Join(currentPath, entry.Name())
		if entry.IsDir() {
			if cleanupMatchingFiles && matchesFileName(entry.Name(), true, matchers) {
				if err := os.RemoveAll(entryPath); err != nil {
					stats.FailureCount++
					s.persistRunHistory(runID, fmt.Sprintf("remove matched directory %s failed: %v", entryPath, err), stats)
					s.appendLog(runID, "error", fmt.Sprintf("remove matched directory %s failed: %v", entryPath, err))
				} else {
					stats.ProcessedFiles++
					stats.SuccessCount++
					stats.CleanupRemovedDirs++
					stats.SizeBytes += dirSizeOrZero(entryPath)
					s.persistRunHistory(runID, fmt.Sprintf("已删除匹配目录 %s", entryPath), stats)
					s.appendLog(runID, "info", fmt.Sprintf("已删除匹配目录 %s", entryPath))
				}
				return nil
			}
			s.cleanupDirectory(runID, rootPath, entryPath, compatibilityMode, cleanupEmptyDirs, cleanupMatchingFiles, cleanupExpiredFiles, cleanupRetentionDays, matchers, whitelist, stats)
			if cleanupEmptyDirs && !sameCleanPath(rootPath, entryPath) && !isWhitelistedDirectoryName(entry.Name(), whitelist) {
				removed, removeErr := removeDirIfEmptyWithMode(compatibilityMode, entryPath)
				if removeErr != nil {
					stats.FailureCount++
					s.persistRunHistory(runID, fmt.Sprintf("remove empty directory %s failed: %v", entryPath, removeErr), stats)
					s.appendLog(runID, "error", fmt.Sprintf("remove empty directory %s failed: %v", entryPath, removeErr))
				} else if removed {
					stats.ProcessedFiles++
					stats.SuccessCount++
					stats.CleanupRemovedDirs++
					stats.SizeBytes += dirSizeOrZero(entryPath)
					s.persistRunHistory(runID, fmt.Sprintf("已删除空目录 %s", entryPath), stats)
					s.appendLog(runID, "info", fmt.Sprintf("已删除空目录 %s", entryPath))
				}
			}
			return nil
		}

		if cleanupMatchingFiles && matchesFileName(entry.Name(), false, matchers) {
			if err := os.Remove(entryPath); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("remove file %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("remove file %s failed: %v", entryPath, err))
				return nil
			}

			stats.ProcessedFiles++
			stats.SuccessCount++
			stats.CleanupRemovedFiles++
			stats.SizeBytes += fileSizeOrZero(entryPath)
			s.persistRunHistory(runID, fmt.Sprintf("已删除匹配文件 %s", entryPath), stats)
			s.appendLog(runID, "info", fmt.Sprintf("已删除匹配文件 %s", entryPath))
			return nil
		}

		if !cleanupExpiredFiles || !isExpiredFile(entryPath, cleanupRetentionDays) {
			return nil
		}

		if err := os.Remove(entryPath); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("remove expired file %s failed: %v", entryPath, err), stats)
			s.appendLog(runID, "error", fmt.Sprintf("remove expired file %s failed: %v", entryPath, err))
			return nil
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		stats.CleanupRemovedFiles++
		stats.SizeBytes += fileSizeOrZero(entryPath)
		s.persistRunHistory(runID, fmt.Sprintf("已删除过期文件 %s", entryPath), stats)
		s.appendLog(runID, "info", fmt.Sprintf("已删除过期文件 %s", entryPath))
		return nil
	})
}

func isExpiredFile(path string, retentionDays int) bool {
	if retentionDays < 1 {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	return info.ModTime().Before(cutoff)
}

func dirSizeOrZero(path string) int64 {
	var total int64
	if err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	}); err != nil {
		return 0
	}
	return total
}

func removeDirIfEmptyWithMode(compatibilityMode, path string) (bool, error) {
	entries, err := readDirWithMode(compatibilityMode, path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if len(entries) > 0 {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func buildFileNameMatchers(filters []string) []fileNameMatcher {
	return buildFileNameMatchersWithCaseSensitivity(filters, false)
}

func buildCaseSensitiveFileNameMatchers(filters []string) []fileNameMatcher {
	return buildFileNameMatchersWithCaseSensitivity(filters, true)
}

func buildFileNameMatchersWithCaseSensitivity(filters []string, caseSensitive bool) []fileNameMatcher {
	items := make([]fileNameMatcher, 0, len(filters))
	for _, filter := range filters {
		value := strings.TrimSpace(filter)
		if value == "" {
			continue
		}
		target := ruleMatcherFileName
		fuzzy := false
		if len(value) >= 3 && strings.HasPrefix(value, "/") && strings.HasSuffix(value, "/") {
			target = ruleMatcherGlobal
			value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "/"), "/"))
		} else if strings.HasPrefix(value, "/") {
			target = ruleMatcherDirectoryName
			value = strings.TrimSpace(strings.TrimPrefix(value, "/"))
		} else if strings.HasPrefix(value, ".") {
			target = ruleMatcherExtension
		}
		if target != ruleMatcherExtension && strings.HasPrefix(value, "*") {
			fuzzy = true
			value = strings.TrimSpace(strings.TrimPrefix(value, "*"))
		}
		if value == "" {
			continue
		}

		if looksLikeRegexPattern(value) {
			compiled, err := regexp.Compile(value)
			if err == nil {
				items = append(items, fileNameMatcher{target: target, regex: compiled, fuzzy: fuzzy, caseSensitive: caseSensitive})
				continue
			}
		}

		literal := value
		if !caseSensitive {
			literal = strings.ToLower(value)
		}
		items = append(items, fileNameMatcher{target: target, literal: literal, fuzzy: fuzzy, caseSensitive: caseSensitive})
	}

	return items
}

func buildDirectoryWhitelist(filters []string) map[string]struct{} {
	items := make(map[string]struct{}, len(filters))
	for _, filter := range filters {
		value := strings.ToLower(strings.TrimSpace(filter))
		if value == "" {
			continue
		}
		items[value] = struct{}{}
	}

	return items
}

func isWhitelistedDirectoryName(name string, whitelist map[string]struct{}) bool {
	if len(whitelist) == 0 {
		return false
	}
	_, ok := whitelist[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func looksLikeRegexPattern(value string) bool {
	return strings.ContainsAny(value, `\^$[](){}|*+?`)
}

func matchesFileName(name string, isDir bool, matchers []fileNameMatcher) bool {
	rawName := strings.TrimSpace(name)
	normalized := strings.ToLower(rawName)
	rawExt := filepath.Ext(rawName)
	ext := strings.ToLower(rawExt)
	rawExtWithoutDot := strings.TrimPrefix(rawExt, ".")
	extWithoutDot := strings.TrimPrefix(ext, ".")
	stem := strings.TrimSuffix(rawName, rawExt)
	lowerStem := strings.ToLower(stem)
	for _, matcher := range matchers {
		candidates := make([]string, 0, 3)
		literalCandidates := make([]string, 0, 3)
		directoryLiteral := normalized
		extLiteral := ext
		extWithoutDotLiteral := extWithoutDot
		stemLiteral := lowerStem
		if matcher.caseSensitive {
			directoryLiteral = rawName
			extLiteral = rawExt
			extWithoutDotLiteral = rawExtWithoutDot
			stemLiteral = stem
		}
		switch matcher.target {
		case ruleMatcherDirectoryName:
			if !isDir {
				continue
			}
			candidates = append(candidates, rawName)
			literalCandidates = append(literalCandidates, directoryLiteral)
		case ruleMatcherExtension:
			if isDir {
				continue
			}
			candidates = append(candidates, rawExt, rawExtWithoutDot)
			literalCandidates = append(literalCandidates, extLiteral, extWithoutDotLiteral)
		case ruleMatcherGlobal:
			if isDir {
				candidates = append(candidates, rawName)
				literalCandidates = append(literalCandidates, directoryLiteral)
			} else {
				candidates = append(candidates, stem, rawExt, rawExtWithoutDot)
				literalCandidates = append(literalCandidates, stemLiteral, extLiteral, extWithoutDotLiteral)
			}
		default:
			if isDir {
				continue
			}
			candidates = append(candidates, stem)
			literalCandidates = append(literalCandidates, stemLiteral)
		}
		if matcher.regex != nil {
			for _, candidate := range candidates {
				if matcher.regex.MatchString(candidate) {
					return true
				}
			}
			continue
		}
		if matcher.literal == "" {
			continue
		}
		if matcher.target == ruleMatcherExtension {
			for _, literalCandidate := range literalCandidates {
				if literalCandidate == matcher.literal {
					return true
				}
			}
			continue
		}
		for _, literalCandidate := range literalCandidates {
			if matcher.fuzzy {
				if strings.Contains(literalCandidate, matcher.literal) {
					return true
				}
				continue
			}
			if literalCandidate == matcher.literal {
				return true
			}
		}
	}

	return false
}

func sameCleanPath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
