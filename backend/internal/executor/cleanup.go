package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type fileNameMatcher struct {
	target  ruleMatcherTarget
	literal string
	regex   *regexp.Regexp
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
	if !cleanupEmptyDirs && !cleanupMatchingFiles {
		stats.SkipCount = 1
		stats.Summary = "no cleanup actions enabled"
		return stats, nil
	}

	matchers := buildFileNameMatchers(req.Filters)
	whitelist := buildDirectoryWhitelist(req.Whitelist)
	if cleanupMatchingFiles && len(matchers) == 0 {
		stats.SkipCount = 1
		stats.Summary = "cleanup_matching_files enabled but no valid matchers provided"
		return stats, nil
	}

	s.cleanupDirectory(runID, sourceDir, sourceDir, req.CompatibilityMode, cleanupEmptyDirs, cleanupMatchingFiles, matchers, whitelist, &stats)

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "no matching cleanup items found"
	} else {
		stats.Summary = fmt.Sprintf("removed %d files and %d directories, failed %d", stats.CleanupRemovedFiles, stats.CleanupRemovedDirs, stats.FailureCount)
	}

	if stats.FailureCount > 0 {
		return stats, fmt.Errorf("cleanup finished with %d failures", stats.FailureCount)
	}

	return stats, nil
}

func (s *Service) cleanupDirectory(runID, rootPath, currentPath, compatibilityMode string, cleanupEmptyDirs, cleanupMatchingFiles bool, matchers []fileNameMatcher, whitelist map[string]struct{}, stats *executionStats) {
	entries, err := readDirWithMode(compatibilityMode, currentPath)
	if err != nil {
		stats.FailureCount++
		s.persistRunHistory(runID, fmt.Sprintf("read cleanup directory %s failed: %v", currentPath, err), stats)
		s.appendLog(runID, "error", fmt.Sprintf("read cleanup directory %s failed: %v", currentPath, err))
		return
	}

	sortEntriesNaturally(entries)
	for _, entry := range entries {
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
					s.persistRunHistory(runID, fmt.Sprintf("removed matched directory %s", entryPath), stats)
					s.appendLog(runID, "info", fmt.Sprintf("removed matched directory %s", entryPath))
				}
				continue
			}
			s.cleanupDirectory(runID, rootPath, entryPath, compatibilityMode, cleanupEmptyDirs, cleanupMatchingFiles, matchers, whitelist, stats)
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
					s.persistRunHistory(runID, fmt.Sprintf("removed empty directory %s", entryPath), stats)
					s.appendLog(runID, "info", fmt.Sprintf("removed empty directory %s", entryPath))
				}
			}
			continue
		}

		if !cleanupMatchingFiles || !matchesFileName(entry.Name(), false, matchers) {
			continue
		}

		if err := os.Remove(entryPath); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("remove file %s failed: %v", entryPath, err), stats)
			s.appendLog(runID, "error", fmt.Sprintf("remove file %s failed: %v", entryPath, err))
			continue
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		stats.CleanupRemovedFiles++
		s.persistRunHistory(runID, fmt.Sprintf("removed matched file %s", entryPath), stats)
		s.appendLog(runID, "info", fmt.Sprintf("removed matched file %s", entryPath))
	}
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
	items := make([]fileNameMatcher, 0, len(filters))
	for _, filter := range filters {
		value := strings.TrimSpace(filter)
		if value == "" {
			continue
		}
		target := ruleMatcherFileName
		if len(value) >= 3 && strings.HasPrefix(value, "/") && strings.HasSuffix(value, "/") {
			target = ruleMatcherGlobal
			value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "/"), "/"))
		} else if strings.HasPrefix(value, "/") {
			target = ruleMatcherDirectoryName
			value = strings.TrimSpace(strings.TrimPrefix(value, "/"))
		} else if strings.HasPrefix(value, ".") {
			target = ruleMatcherExtension
		}
		if value == "" {
			continue
		}

		if looksLikeRegexPattern(value) {
			compiled, err := regexp.Compile(value)
			if err == nil {
				items = append(items, fileNameMatcher{target: target, regex: compiled})
				continue
			}
		}

		items = append(items, fileNameMatcher{target: target, literal: strings.ToLower(value)})
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
	normalized := strings.ToLower(strings.TrimSpace(name))
	rawName := strings.TrimSpace(name)
	ext := strings.ToLower(filepath.Ext(rawName))
	extWithoutDot := strings.TrimPrefix(ext, ".")
	stem := strings.TrimSuffix(rawName, filepath.Ext(rawName))
	lowerStem := strings.ToLower(stem)
	for _, matcher := range matchers {
		candidates := make([]string, 0, 3)
		literalCandidates := make([]string, 0, 3)
		switch matcher.target {
		case ruleMatcherDirectoryName:
			if !isDir {
				continue
			}
			candidates = append(candidates, rawName)
			literalCandidates = append(literalCandidates, normalized)
		case ruleMatcherExtension:
			if isDir {
				continue
			}
			candidates = append(candidates, ext, extWithoutDot)
			literalCandidates = append(literalCandidates, ext, extWithoutDot)
		case ruleMatcherGlobal:
			if isDir {
				candidates = append(candidates, rawName)
				literalCandidates = append(literalCandidates, normalized)
			} else {
				candidates = append(candidates, stem, ext, extWithoutDot)
				literalCandidates = append(literalCandidates, lowerStem, ext, extWithoutDot)
			}
		default:
			if isDir {
				continue
			}
			candidates = append(candidates, stem)
			literalCandidates = append(literalCandidates, lowerStem)
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
			if strings.Contains(literalCandidate, matcher.literal) {
				return true
			}
		}
	}

	return false
}

func sameCleanPath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
