package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/liuzl/gocc"
)

var (
	traditionalToSimplifiedConverter *gocc.OpenCC
	traditionalToSimplifiedOnce      sync.Once
	traditionalToSimplifiedErr       error
)

type renameTransformRule struct {
	raw         string
	pattern     string
	replacement string
	regex       *regexp.Regexp
}

type transformFilterMatcher struct {
	literal       string
	regex         *regexp.Regexp
	targetDirOnly bool
}

func (s *Service) executeTransformRule(runID string, req ExecuteRuleRequest) (executionStats, error) {
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

	convertTraditional := req.Options["convert_traditional_to_simplified"]
	convertCustom := req.Options["convert_matching_text"]
	filterCustom := req.Options["filter_matching_text"]
	if !convertTraditional && !convertCustom && !filterCustom {
		stats.SkipCount = 1
		stats.Summary = "no transform actions enabled"
		return stats, nil
	}

	rules, err := parseRenameTransformRules(req.TransformRules)
	if err != nil {
		return stats, err
	}
	transformFilters, err := parseTransformFilters(req.TransformFilters)
	if err != nil {
		return stats, err
	}
	if convertCustom && len(rules) == 0 {
		stats.SkipCount = 1
		stats.Summary = "convert_matching_text enabled but no valid transform rules provided"
		return stats, nil
	}
	if filterCustom && len(transformFilters) == 0 {
		stats.SkipCount = 1
		stats.Summary = "filter_matching_text enabled but no valid transform filters provided"
		return stats, nil
	}

	s.transformDirectory(runID, sourceDir, req.CompatibilityMode, convertTraditional, convertCustom, filterCustom, rules, transformFilters, &stats)

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "no matching transform items found"
	} else {
		stats.Summary = fmt.Sprintf("renamed %d files and %d directories, failed %d", stats.CleanupRemovedFiles, stats.CleanupRemovedDirs, stats.FailureCount)
	}

	if stats.FailureCount > 0 {
		return stats, fmt.Errorf("transform finished with %d failures", stats.FailureCount)
	}

	return stats, nil
}

func (s *Service) transformDirectory(runID, currentPath, compatibilityMode string, convertTraditional, convertCustom, filterCustom bool, rules []renameTransformRule, transformFilters []transformFilterMatcher, stats *executionStats) {
	entries, err := readDirWithMode(compatibilityMode, currentPath)
	if err != nil {
		stats.FailureCount++
		s.persistRunHistory(runID, fmt.Sprintf("read transform directory %s failed: %v", currentPath, err), stats)
		s.appendLog(runID, "error", fmt.Sprintf("read transform directory %s failed: %v", currentPath, err))
		return
	}
	entries = limitEntriesForMode(compatibilityMode, entries)

	sortEntriesNaturally(entries)
	_ = processEntriesForMode(compatibilityMode, entries, func(entry os.DirEntry) error {
		entryPath := filepath.Join(currentPath, entry.Name())
		if entry.IsDir() {
			s.transformDirectory(runID, entryPath, compatibilityMode, convertTraditional, convertCustom, filterCustom, rules, transformFilters, stats)
		}
		return nil
	})

	sort.SliceStable(entries, func(i, j int) bool {
		return len(entries[i].Name()) > len(entries[j].Name())
	})

	_ = processEntriesForMode(compatibilityMode, entries, func(entry os.DirEntry) error {
		oldName := entry.Name()
		newName := applyRenameTransforms(oldName, entry.IsDir(), convertTraditional, convertCustom, filterCustom, rules, transformFilters)
		if oldName == newName || strings.TrimSpace(newName) == "" {
			return nil
		}

		oldPath := filepath.Join(currentPath, oldName)
		newPath := filepath.Join(currentPath, newName)
		if sameCleanPath(oldPath, newPath) {
			return nil
		}

		if _, statErr := os.Stat(newPath); statErr == nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("rename target already exists %s", newPath), stats)
			s.appendLog(runID, "error", fmt.Sprintf("rename target already exists %s", newPath))
			return nil
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("rename %s failed: %v", oldPath, err), stats)
			s.appendLog(runID, "error", fmt.Sprintf("rename %s failed: %v", oldPath, err))
			return nil
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		if entry.IsDir() {
			stats.CleanupRemovedDirs++
			s.persistRunHistory(runID, fmt.Sprintf("renamed directory %s -> %s", oldPath, newPath), stats)
			s.appendLog(runID, "info", fmt.Sprintf("renamed directory %s -> %s", oldPath, newPath))
		} else {
			stats.CleanupRemovedFiles++
			stats.SizeBytes += fileSizeOrZero(newPath)
			s.persistRunHistory(runID, fmt.Sprintf("renamed file %s -> %s", oldPath, newPath), stats)
			s.appendLog(runID, "info", fmt.Sprintf("renamed file %s -> %s", oldPath, newPath))
		}
		return nil
	})
}

func parseRenameTransformRules(items []string) ([]renameTransformRule, error) {
	rules := make([]renameTransformRule, 0, len(items))
	for _, item := range items {
		line := strings.TrimSpace(item)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=>", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid transform rule: %s", line)
		}
		pattern := strings.TrimSpace(parts[0])
		replacement := strings.TrimSpace(parts[1])
		if pattern == "" {
			return nil, fmt.Errorf("invalid transform rule: %s", line)
		}
		rule := renameTransformRule{raw: line, pattern: pattern, replacement: replacement}
		if looksLikeRegexPattern(pattern) {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid transform regex %q: %w", pattern, err)
			}
			rule.regex = compiled
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func parseTransformFilters(items []string) ([]transformFilterMatcher, error) {
	filters := make([]transformFilterMatcher, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		targetDirOnly := false
		pattern := trimmed
		if strings.HasPrefix(trimmed, "<-") && strings.HasSuffix(trimmed, "->") {
			targetDirOnly = true
			pattern = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<-"), "->"))
			if pattern == "" {
				continue
			}
		}

		matcher := transformFilterMatcher{literal: pattern, targetDirOnly: targetDirOnly}
		if looksLikeRegexPattern(pattern) {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid transform filter regex %q: %w", pattern, err)
			}
			matcher.regex = compiled
		}
		filters = append(filters, matcher)
	}
	return filters, nil
}

func applyRenameTransforms(name string, isDir bool, convertTraditional, convertCustom, filterCustom bool, rules []renameTransformRule, transformFilters []transformFilterMatcher) string {
	result := name
	if convertTraditional {
		if converted, err := convertTraditionalToSimplified(result); err == nil {
			result = converted
		}
	}
	if convertCustom {
		for _, rule := range rules {
			if rule.regex != nil {
				result = rule.regex.ReplaceAllString(result, rule.replacement)
				continue
			}
			result = strings.ReplaceAll(result, rule.pattern, rule.replacement)
		}
	}
	if filterCustom {
		for _, filter := range transformFilters {
			if filter.targetDirOnly && !isDir {
				continue
			}
			if !filter.targetDirOnly && isDir {
				continue
			}
			if filter.regex != nil {
				result = filter.regex.ReplaceAllString(result, "")
				continue
			}
			result = strings.ReplaceAll(result, filter.literal, "")
		}
		result = strings.TrimSpace(result)
	}
	return result
}

func convertTraditionalToSimplified(value string) (string, error) {
	traditionalToSimplifiedOnce.Do(func() {
		traditionalToSimplifiedConverter, traditionalToSimplifiedErr = gocc.New("t2s")
	})
	if traditionalToSimplifiedErr != nil {
		return value, traditionalToSimplifiedErr
	}
	if traditionalToSimplifiedConverter == nil {
		return value, fmt.Errorf("traditional to simplified converter unavailable")
	}
	converted, err := traditionalToSimplifiedConverter.Convert(value)
	if err != nil {
		return value, err
	}
	return converted, nil
}
