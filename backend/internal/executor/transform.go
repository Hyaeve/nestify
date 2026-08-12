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
	targetDir   bool
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
	mergeSameNameDirs := req.Options["merge_same_name_dirs"]
	if !convertTraditional && !convertCustom && !filterCustom && !mergeSameNameDirs {
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

	s.transformDirectory(runID, sourceDir, req.CompatibilityMode, convertTraditional, convertCustom, filterCustom, mergeSameNameDirs, rules, transformFilters, &stats)

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "未发现可转换项目"
	} else {
		stats.Summary = fmt.Sprintf("转换完成：重命名 %d 个文件、%d 个文件夹，失败 %d 项", stats.CleanupRemovedFiles, stats.CleanupRemovedDirs, stats.FailureCount)
	}

	if stats.FailureCount > 0 {
		return stats, fmt.Errorf("transform finished with %d failures", stats.FailureCount)
	}

	return stats, nil
}

func (s *Service) transformDirectory(runID, currentPath, compatibilityMode string, convertTraditional, convertCustom, filterCustom, mergeSameNameDirs bool, rules []renameTransformRule, transformFilters []transformFilterMatcher, stats *executionStats) {
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
			s.transformDirectory(runID, entryPath, compatibilityMode, convertTraditional, convertCustom, filterCustom, mergeSameNameDirs, rules, transformFilters, stats)
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
			if entry.IsDir() && mergeSameNameDirs {
				if err := mergeDirectories(oldPath, newPath); err != nil {
					stats.FailureCount++
					s.persistRunHistory(runID, fmt.Sprintf("merge directory %s into %s failed: %v", oldPath, newPath, err), stats)
					s.appendLog(runID, "error", fmt.Sprintf("merge directory %s into %s failed: %v", oldPath, newPath, err))
					return nil
				}
				stats.ProcessedFiles++
				stats.SuccessCount++
				stats.CleanupRemovedDirs++
				s.persistRunHistory(runID, fmt.Sprintf("merged directory %s -> %s", oldPath, newPath), stats)
				s.appendLog(runID, "info", fmt.Sprintf("merged directory %s -> %s", oldPath, newPath))
				return nil
			}

			fallbackPath := uniqueTransformRenameSuffixPath(currentPath, newName)
			if fallbackPath == "" || sameCleanPath(oldPath, fallbackPath) {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("rename target already exists %s", newPath), stats)
				s.appendLog(runID, "error", fmt.Sprintf("rename target already exists %s", newPath))
				return nil
			}
			newPath = fallbackPath
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
		targetDir := false
		if unwrappedPattern, ok := unwrapDirectoryTransformToken(pattern); ok {
			unwrappedReplacement, replacementWrapped := unwrapDirectoryTransformToken(replacement)
			if !replacementWrapped {
				return nil, fmt.Errorf("directory transform rule must wrap both pattern and replacement with /: %s", line)
			}
			pattern = unwrappedPattern
			replacement = unwrappedReplacement
			targetDir = true
		}
		rule := renameTransformRule{raw: line, pattern: pattern, replacement: replacement, targetDir: targetDir}
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
		if len(pattern) >= 2 && strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") {
			regexPattern := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/"))
			if regexPattern == "" {
				continue
			}
			matcher.literal = regexPattern
			compiled, err := regexp.Compile(regexPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid transform filter regex %q: %w", pattern, err)
			}
			matcher.regex = compiled
		}
		filters = append(filters, matcher)
	}
	return filters, nil
}

func uniqueTransformRenameSuffixPath(dir, name string) string {
	baseName := filepath.Base(strings.TrimSpace(name))
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		baseName = "item"
	}

	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	firstCandidate := filepath.Join(dir, fmt.Sprintf("%s-re%s", stem, ext))
	if _, err := os.Stat(firstCandidate); os.IsNotExist(err) {
		return firstCandidate
	}

	for index := 1; ; index++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-re%d%s", stem, index, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func mergeDirectories(sourceDir, targetDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		sourcePath := filepath.Join(sourceDir, entry.Name())
		targetPath := filepath.Join(targetDir, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			if err := mergeDirectories(sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}

		if _, err := os.Stat(targetPath); err == nil {
			targetPath = uniqueTransformRenameSuffixPath(targetDir, entry.Name())
		} else if !os.IsNotExist(err) {
			return err
		}

		if err := moveFile(sourcePath, targetPath); err != nil {
			return err
		}
	}

	return os.Remove(sourceDir)
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
			if rule.targetDir != isDir {
				continue
			}
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

func unwrapDirectoryTransformToken(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 2 || !strings.HasPrefix(trimmed, "/") || !strings.HasSuffix(trimmed, "/") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "/"), "/")), true
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
