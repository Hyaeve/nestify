package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"nestify/backend/internal/model"
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

// cleanupPlan 是净化规则的动作开关与匹配器。
//
// 本地与远程（WebDAV 挂载）两条实现共用这一份解析结果：两边对「哪些算命中」、
// 「哪些目录受白名单保护」必须是同一套口径，否则同一个规则配置在两个目录上会删出不同结果。
type cleanupPlan struct {
	emptyDirs     bool
	matchingFiles bool
	expiredFiles  bool
	retentionDays int
	matchers      []fileNameMatcher
	whitelist     map[string]struct{}
}

// parseCleanupPlan 解析净化规则的动作开关；第二个返回值非空表示这一轮没什么可做的
// （调用方按整轮跳过处理），文案与旧实现逐字一致。
func parseCleanupPlan(req ExecuteRuleRequest) (cleanupPlan, string) {
	plan := cleanupPlan{
		emptyDirs:     req.Options["cleanup_empty_dirs"],
		matchingFiles: req.Options["cleanup_matching_files"],
		expiredFiles:  req.Options["cleanup_expired_files"],
		retentionDays: req.OptionValues["cleanup_retention_days"],
	}
	if !plan.emptyDirs && !plan.matchingFiles && !plan.expiredFiles {
		return plan, "no cleanup actions enabled"
	}
	if plan.expiredFiles && plan.retentionDays < 1 {
		return plan, "cleanup_expired_files enabled but no valid retention days provided"
	}

	plan.matchers = buildCaseSensitiveFileNameMatchers(req.Filters)
	plan.whitelist = buildDirectoryWhitelist(req.Whitelist)
	if plan.matchingFiles && len(plan.matchers) == 0 {
		return plan, "cleanup_matching_files enabled but no valid matchers provided"
	}

	return plan, ""
}

func (s *Service) executeCleanupRule(runID string, req ExecuteRuleRequest) (executionStats, error) {
	stats := executionStats{}
	rawSource := strings.TrimSpace(req.SourceDir)
	if rawSource == "" {
		return stats, fmt.Errorf("source dir is required")
	}

	// 远程挂载（webdav://<id>/...）上的净化必须走 WebDAV 实现：下面那条路全是 os.* 调用，
	// 对一个虚拟路径只能得到「系统找不到指定的路径」，整轮必然失败（用户报的就是这个）。
	// 判断要排在 filepath.Clean 之前 —— Clean 会把 webdav://3/影视 拧成 webdav:\3\影视，
	// 之后再也认不出这是远程挂载。
	if isWebdavSource(rawSource) {
		return s.executeWebdavCleanupRule(runID, req, rawSource)
	}

	sourceDir := filepath.Clean(rawSource)
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

	plan, skipReason := parseCleanupPlan(req)
	if skipReason != "" {
		stats.SkipCount = 1
		stats.Summary = skipReason
		return stats, nil
	}

	// 净化链路的明细就是「这次删掉了什么」：删除动作（delete）与备份删源、strm 级联删除
	// 共用同一个统计项，所以任务详情窗口里的「删除」对净化规则同样成立（kind = cleanup）。
	// 根路径取监控目录，前端据此把绝对路径裁成相对路径显示。
	stats.detail(model.RunDetailKindCleanup)
	stats.Detail.setRoots([]string{sourceDir}, nil)

	s.cleanupDirectory(runID, sourceDir, sourceDir, req.CompatibilityMode, plan, &stats)

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

func (s *Service) cleanupDirectory(runID, rootPath, currentPath, compatibilityMode string, plan cleanupPlan, stats *executionStats) {
	// 手动停止：递归到这里直接不再往下走（返回 void，取消信号由 runExecution 统一收尾）。
	if s.runAborted(runID) {
		return
	}
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
			if plan.matchingFiles && matchesFileName(entry.Name(), true, plan.matchers) {
				if err := os.RemoveAll(entryPath); err != nil {
					stats.FailureCount++
					stats.Detail.record(model.RunFileEntry{
						Path:   entryPath,
						Action: model.BackupFileActionFail,
						Dir:    true,
						Note:   fmt.Sprintf("删除失败：%v", err),
					})
					s.persistRunHistory(runID, fmt.Sprintf("remove matched directory %s failed: %v", entryPath, err), stats)
					s.appendLog(runID, "error", fmt.Sprintf("remove matched directory %s failed: %v", entryPath, err))
				} else {
					stats.ProcessedFiles++
					stats.SuccessCount++
					stats.CleanupRemovedDirs++
					stats.SizeBytes += dirSizeOrZero(entryPath)
					// 明细先于 persistRunHistory 落：每条运行记录带的是「写它那一刻」的明细快照，
					// 顺序反了这条删除就不在快照里。
					stats.Detail.record(model.RunFileEntry{
						Path:   entryPath,
						Action: model.BackupFileActionDelete,
						Dir:    true,
						Note:   deleteNoteMatchedDir,
					})
					s.persistRunHistory(runID, fmt.Sprintf("已删除匹配目录 %s", entryPath), stats)
					s.appendLog(runID, "info", fmt.Sprintf("已删除匹配目录 %s", entryPath))
				}
				return nil
			}
			s.cleanupDirectory(runID, rootPath, entryPath, compatibilityMode, plan, stats)
			if plan.emptyDirs && !sameCleanPath(rootPath, entryPath) && !isWhitelistedDirectoryName(entry.Name(), plan.whitelist) {
				removed, removeErr := removeDirIfEmptyWithMode(compatibilityMode, entryPath)
				if removeErr != nil {
					stats.FailureCount++
					stats.Detail.record(model.RunFileEntry{
						Path:   entryPath,
						Action: model.BackupFileActionFail,
						Dir:    true,
						Note:   fmt.Sprintf("删除失败：%v", removeErr),
					})
					s.persistRunHistory(runID, fmt.Sprintf("remove empty directory %s failed: %v", entryPath, removeErr), stats)
					s.appendLog(runID, "error", fmt.Sprintf("remove empty directory %s failed: %v", entryPath, removeErr))
				} else if removed {
					stats.ProcessedFiles++
					stats.SuccessCount++
					stats.CleanupRemovedDirs++
					stats.SizeBytes += dirSizeOrZero(entryPath)
					stats.Detail.record(model.RunFileEntry{
						Path:   entryPath,
						Action: model.BackupFileActionDelete,
						Dir:    true,
						Note:   deleteNoteEmptyDir,
					})
					s.persistRunHistory(runID, fmt.Sprintf("已删除空目录 %s", entryPath), stats)
					s.appendLog(runID, "info", fmt.Sprintf("已删除空目录 %s", entryPath))
				}
			}
			return nil
		}

		if plan.matchingFiles && matchesFileName(entry.Name(), false, plan.matchers) {
			if err := os.Remove(entryPath); err != nil {
				stats.FailureCount++
				stats.Detail.record(model.RunFileEntry{
					Path:   entryPath,
					Action: model.BackupFileActionFail,
					Note:   fmt.Sprintf("删除失败：%v", err),
				})
				s.persistRunHistory(runID, fmt.Sprintf("remove file %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("remove file %s failed: %v", entryPath, err))
				return nil
			}

			stats.ProcessedFiles++
			stats.SuccessCount++
			stats.CleanupRemovedFiles++
			stats.SizeBytes += fileSizeOrZero(entryPath)
			stats.Detail.record(model.RunFileEntry{
				Path:   entryPath,
				Action: model.BackupFileActionDelete,
				Note:   deleteNoteMatchedFile,
			})
			s.persistRunHistory(runID, fmt.Sprintf("已删除匹配文件 %s", entryPath), stats)
			s.appendLog(runID, "info", fmt.Sprintf("已删除匹配文件 %s", entryPath))
			return nil
		}

		if !plan.expiredFiles || !isExpiredFile(entryPath, plan.retentionDays) {
			return nil
		}

		if err := os.Remove(entryPath); err != nil {
			stats.FailureCount++
			stats.Detail.record(model.RunFileEntry{
				Path:   entryPath,
				Action: model.BackupFileActionFail,
				Note:   fmt.Sprintf("删除失败：%v", err),
			})
			s.persistRunHistory(runID, fmt.Sprintf("remove expired file %s failed: %v", entryPath, err), stats)
			s.appendLog(runID, "error", fmt.Sprintf("remove expired file %s failed: %v", entryPath, err))
			return nil
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		stats.CleanupRemovedFiles++
		stats.SizeBytes += fileSizeOrZero(entryPath)
		stats.Detail.record(model.RunFileEntry{
			Path:   entryPath,
			Action: model.BackupFileActionDelete,
			Note:   deleteNoteExpiredFile,
		})
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
