package executor

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type executionStats struct {
	ProcessedFiles      int
	SuccessCount        int
	SkipCount           int
	FailureCount        int
	PackedVolumes       int
	MovedFiles          int
	CleanupRemovedFiles int
	CleanupRemovedDirs  int
	HistoryEvents       int
	Summary             string
}

func (s *Service) executeRule(runID string, req ExecuteRuleRequest) (executionStats, error) {
	if strings.TrimSpace(req.ArchiveMode) == "cleanup" {
		return s.executeCleanupRule(runID, req)
	}
	if strings.TrimSpace(req.ArchiveMode) == "transform" {
		return s.executeTransformRule(runID, req)
	}
	if strings.TrimSpace(req.ArchiveMode) == "link" {
		return s.executeLinkRule(runID, req)
	}

	stats := executionStats{}

	sourceDir := filepath.Clean(strings.TrimSpace(req.SourceDir))
	targetDir := filepath.Clean(strings.TrimSpace(req.TargetDir))
	if sourceDir == "" || sourceDir == "." {
		return stats, fmt.Errorf("source dir is required")
	}
	if targetDir == "" || targetDir == "." {
		return stats, fmt.Errorf("target dir is required")
	}

	info, err := statWithMode(req.CompatibilityMode, sourceDir)
	if err != nil {
		return stats, fmt.Errorf("stat source dir: %w", err)
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("source dir must be a directory")
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return stats, fmt.Errorf("create target dir: %w", err)
	}

	entries, err := readDirWithMode(req.CompatibilityMode, sourceDir)
	if err != nil {
		return stats, fmt.Errorf("read source dir: %w", err)
	}
	entries = limitEntriesForMode(req.CompatibilityMode, entries)
	if len(entries) == 0 {
		stats.SkipCount = 1
		stats.Summary = "source directory is empty"
		return stats, nil
	}

	sortEntriesNaturally(entries)
	matchers := buildFileNameMatchers(req.Filters)
	matchArchiveEnabled := req.ArchiveMode == "package" && req.PackageOptions["match_archive"]
	matchArchiveParentRenameEnabled := req.ArchiveMode == "package" && matchArchiveEnabled && req.PackageOptions["match_archive_parent_rename"]
	directMatchers := buildArchiveDirectMatchers(req.MatchFilters)
	singleFileNestingEnabled := req.ArchiveMode == "package" && req.PackageOptions["single_file_nesting"]
	nestMatchers := buildArchiveDirectMatchers(req.NestFilters)
	packageNestedFolders := req.PackageOptions["package_nested_folders"]
	flatArchive := req.PackageOptions["flat_archive"]
	collectRecursiveEnabled := req.ArchiveMode != "collect" || req.CollectOptions["recursive_collect"]
	collectDeduplicateEnabled := req.ArchiveMode == "collect" && req.CollectOptions["deduplicate_same_name"]
	collectRemoveSourceEnabled := req.ArchiveMode != "collect" || req.CollectOptions["cleanup_source_after_archive"]
	cleanupSourceAfterArchive := false
	if req.ArchiveMode == "package" {
		cleanupSourceAfterArchive = req.PackageOptions["cleanup_source_after_archive"]
	} else if req.ArchiveMode == "collect" {
		cleanupSourceAfterArchive = collectRemoveSourceEnabled
	}
	err = processEntriesForMode(req.CompatibilityMode, entries, func(entry os.DirEntry) error {
		entryPath := filepath.Join(sourceDir, entry.Name())
		if entry.IsDir() && matchesFileName(entry.Name(), true, matchers) {
			stats.SkipCount++
			s.persistRunHistory(runID, fmt.Sprintf("skipped filtered archive directory %s", entryPath), &stats)
			s.appendLog(runID, "info", fmt.Sprintf("skipped filtered archive directory %s", entryPath))
			return nil
		}
		if !entry.IsDir() && matchesFileName(entry.Name(), false, matchers) {
			if err := s.removeFilteredArchiveFile(runID, entryPath, &stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err), &stats)
				s.appendLog(runID, "error", fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err))
			} else {
				stats.SkipCount++
			}
			return nil
		}
		if matchArchiveEnabled && !entry.IsDir() && matchesArchiveDirectly(sourceDir, entryPath, directMatchers) {
			matchedTargetDir := targetDir
			if req.ArchiveMode == "package" && !flatArchive {
				matchedTargetDir = filepath.Join(targetDir, filepath.Base(sourceDir))
			}
			if err := s.moveMatchedArchiveFile(runID, entryPath, matchedTargetDir, req.ArchiveMode, matchArchiveParentRenameEnabled, collectDeduplicateEnabled, cleanupSourceAfterArchive, &stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("move matched file %s failed: %v", entryPath, err), &stats)
				s.appendLog(runID, "error", fmt.Sprintf("move matched file %s failed: %v", entryPath, err))
			}
			return nil
		}
		if singleFileNestingEnabled && !entry.IsDir() && matchesArchiveDirectly(sourceDir, entryPath, nestMatchers) {
			if err := s.moveLooseFileToOwnDir(runID, entryPath, targetDir, &stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("nest matched file %s failed: %v", entryPath, err), &stats)
				s.appendLog(runID, "error", fmt.Sprintf("nest matched file %s failed: %v", entryPath, err))
			}
			return nil
		}
		if entry.IsDir() {
			targetSeriesDir := filepath.Join(targetDir, entry.Name())
			if req.ArchiveMode == "package" && flatArchive {
				targetSeriesDir = targetDir
			}
			if err := s.processSeriesDir(runID, sourceDir, entryPath, targetSeriesDir, req.ArchiveMode, req.CompatibilityMode, packageNestedFolders, flatArchive, cleanupSourceAfterArchive, collectRecursiveEnabled, collectDeduplicateEnabled, matchers, matchArchiveEnabled, matchArchiveParentRenameEnabled, directMatchers, &stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("process series %s failed: %v", entryPath, err), &stats)
				s.appendLog(runID, "error", fmt.Sprintf("process series %s failed: %v", entryPath, err))
			}
			return nil
		}

		if err := s.moveLooseFile(runID, entryPath, targetDir, req.ArchiveMode, collectDeduplicateEnabled, cleanupSourceAfterArchive, &stats); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("move file %s failed: %v", entryPath, err), &stats)
			s.appendLog(runID, "error", fmt.Sprintf("move file %s failed: %v", entryPath, err))
		}
		return nil
	})
	if err != nil {
		return stats, err
	}

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "no supported archive items found"
	} else {
		stats.Summary = fmt.Sprintf("packed %d volumes, moved %d files, skipped %d, failed %d", stats.PackedVolumes, stats.MovedFiles, stats.SkipCount, stats.FailureCount)
	}

	if stats.FailureCount > 0 {
		return stats, fmt.Errorf("archive finished with %d failures", stats.FailureCount)
	}

	return stats, nil
}

func (s *Service) executeLinkRule(runID string, req ExecuteRuleRequest) (executionStats, error) {
	stats := executionStats{}

	sourceDir := filepath.Clean(strings.TrimSpace(req.SourceDir))
	targetDir := filepath.Clean(strings.TrimSpace(req.TargetDir))
	if sourceDir == "" || sourceDir == "." {
		return stats, fmt.Errorf("source dir is required")
	}
	if targetDir == "" || targetDir == "." {
		return stats, fmt.Errorf("target dir is required")
	}

	info, err := statWithMode(req.CompatibilityMode, sourceDir)
	if err != nil {
		return stats, fmt.Errorf("stat source dir: %w", err)
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("source dir must be a directory")
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return stats, fmt.Errorf("create target dir: %w", err)
	}

	matchers := buildFileNameMatchers(req.Filters)
	if err := s.linkDirectory(runID, sourceDir, sourceDir, targetDir, req.LinkMode, req.CompatibilityMode, matchers, &stats); err != nil {
		return stats, err
	}

	if stats.SuccessCount == 0 && stats.SkipCount == 0 && stats.FailureCount == 0 {
		stats.SkipCount = 1
		stats.Summary = "no linkable files found"
	} else {
		modeLabel := "soft"
		if strings.EqualFold(strings.TrimSpace(req.LinkMode), "hard") {
			modeLabel = "hard"
		}
		stats.Summary = fmt.Sprintf("created %d %s links, skipped %d, failed %d", stats.SuccessCount, modeLabel, stats.SkipCount, stats.FailureCount)
	}

	if stats.FailureCount > 0 {
		return stats, fmt.Errorf("link execution finished with %d failures", stats.FailureCount)
	}

	return stats, nil
}

func (s *Service) linkDirectory(runID, rootPath, currentPath, targetRoot, linkMode, compatibilityMode string, matchers []fileNameMatcher, stats *executionStats) error {
	entries, err := readDirWithMode(compatibilityMode, currentPath)
	if err != nil {
		return fmt.Errorf("read link directory %s: %w", currentPath, err)
	}
	entries = limitEntriesForMode(compatibilityMode, entries)

	sortEntriesNaturally(entries)
	return processEntriesForMode(compatibilityMode, entries, func(entry os.DirEntry) error {
		sourcePath := filepath.Join(currentPath, entry.Name())
		relPath, relErr := filepath.Rel(rootPath, sourcePath)
		if relErr != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("resolve relative path for %s failed: %v", sourcePath, relErr))
			return nil
		}
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(targetRoot, relPath)
		if entry.IsDir() {
			if matchesFileName(entry.Name(), true, matchers) {
				stats.SkipCount++
				s.appendLog(runID, "info", fmt.Sprintf("skipped blacklisted directory %s", sourcePath))
				return nil
			}
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				stats.FailureCount++
				s.appendLog(runID, "error", fmt.Sprintf("create link target directory %s failed: %v", targetPath, err))
				return nil
			}
			if err := s.linkDirectory(runID, rootPath, sourcePath, targetRoot, linkMode, compatibilityMode, matchers, stats); err != nil {
				return err
			}
			return nil
		}

		if matchesFileName(entry.Name(), entry.IsDir(), matchers) {
			stats.SkipCount++
			s.appendLog(runID, "info", fmt.Sprintf("skipped blacklisted file %s", sourcePath))
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("create link parent directory %s failed: %v", filepath.Dir(targetPath), err))
			return nil
		}

		if _, err := os.Lstat(targetPath); err == nil {
			stats.SkipCount++
			s.appendLog(runID, "info", fmt.Sprintf("skipped existing link target %s", targetPath))
			return nil
		} else if !os.IsNotExist(err) {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("inspect link target %s failed: %v", targetPath, err))
			return nil
		}

		if err := createFileLink(sourcePath, targetPath, linkMode); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("create link %s -> %s failed: %v", targetPath, sourcePath, err))
			return nil
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		s.appendLog(runID, "info", fmt.Sprintf("created link %s -> %s", targetPath, sourcePath))
		return nil
	})
}

func createFileLink(sourcePath, targetPath, linkMode string) error {
	if strings.EqualFold(strings.TrimSpace(linkMode), "hard") {
		if err := os.Link(sourcePath, targetPath); err != nil {
			return fmt.Errorf("create hard link: %w", err)
		}
		return nil
	}

	if err := os.Symlink(sourcePath, targetPath); err != nil {
		return fmt.Errorf("create soft link: %w", err)
	}
	return nil
}

func (s *Service) processSeriesDir(runID, rootSourceDir, seriesPath, targetSeriesDir, archiveMode, compatibilityMode string, packageNestedFolders, flatArchive bool, cleanupSourceAfterArchive bool, collectRecursiveEnabled bool, collectDeduplicateEnabled bool, matchers []fileNameMatcher, matchArchiveEnabled bool, matchArchiveParentRenameEnabled bool, directMatchers []fileNameMatcher, stats *executionStats) error {
	entries, err := readDirWithMode(compatibilityMode, seriesPath)
	if err != nil {
		return fmt.Errorf("read series dir: %w", err)
	}
	entries = limitEntriesForMode(compatibilityMode, entries)
	if len(entries) == 0 {
		stats.SkipCount++
		s.persistRunHistory(runID, fmt.Sprintf("skipped empty series %s", seriesPath), stats)
		_ = os.Remove(seriesPath)
		return nil
	}

	sortEntriesNaturally(entries)
	files := make([]os.DirEntry, 0, len(entries))
	imageFiles := make([]os.DirEntry, 0, len(entries))
	nonImageFiles := make([]os.DirEntry, 0, len(entries))
	matchedFiles := make([]os.DirEntry, 0, len(entries))
	coverFiles := make([]os.DirEntry, 0, len(entries))
	hasSubdirs := false
	err = processEntriesForMode(compatibilityMode, entries, func(entry os.DirEntry) error {
		entryPath := filepath.Join(seriesPath, entry.Name())
		if entry.IsDir() {
			if matchesFileName(entry.Name(), true, matchers) {
				return nil
			}
			hasSubdirs = true
			return nil
		}
		if archiveMode == "package" && matchArchiveEnabled && matchesArchiveDirectly(rootSourceDir, entryPath, directMatchers) {
			matchedFiles = append(matchedFiles, entry)
			return nil
		}
		if archiveMode == "package" && isCoverImageFile(entry.Name()) {
			coverFiles = append(coverFiles, entry)
			return nil
		}
		files = append(files, entry)
		if isImageFile(entry.Name()) {
			imageFiles = append(imageFiles, entry)
		} else {
			nonImageFiles = append(nonImageFiles, entry)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if archiveMode == "package" && !hasSubdirs && len(imageFiles) > 0 {
		archivePath, err := createPackageCBZFromFiles(rootSourceDir, seriesPath, imageFiles, targetSeriesDir, matchArchiveParentRenameEnabled)
		if err != nil {
			return err
		}
		if cleanupSourceAfterArchive {
			if err := removePackedSourceFiles(seriesPath, imageFiles); err != nil {
				return fmt.Errorf("remove packed source files: %w", err)
			}
		}
		if err := s.moveCoverFiles(runID, seriesPath, coverFiles, targetSeriesDir, stats); err != nil {
			return err
		}
		if err := s.moveMatchedArchiveEntries(runID, seriesPath, matchedFiles, targetSeriesDir, matchArchiveParentRenameEnabled, cleanupSourceAfterArchive, stats); err != nil {
			return err
		}
		stats.ProcessedFiles += len(imageFiles)
		stats.SuccessCount++
		stats.PackedVolumes++
		s.persistRunHistory(runID, fmt.Sprintf("packed series %s -> %s", seriesPath, archivePath), stats)
		s.appendLog(runID, "info", fmt.Sprintf("packed series %s -> %s", seriesPath, archivePath))
		for _, entry := range nonImageFiles {
			sourcePath := filepath.Join(seriesPath, entry.Name())
			stats.SkipCount++
			s.persistRunHistory(runID, fmt.Sprintf("skipped non-image file %s", sourcePath), stats)
			s.appendLog(runID, "info", fmt.Sprintf("skipped non-image file %s: not included in package archive", sourcePath))
		}
		return nil
	}

	if archiveMode == "package" && !hasSubdirs && len(files) == 0 && (len(coverFiles) > 0 || len(matchedFiles) > 0) {
		if err := s.moveCoverFiles(runID, seriesPath, coverFiles, targetSeriesDir, stats); err != nil {
			return err
		}
		if err := s.moveMatchedArchiveEntries(runID, seriesPath, matchedFiles, targetSeriesDir, matchArchiveParentRenameEnabled, cleanupSourceAfterArchive, stats); err != nil {
			return err
		}
		return nil
	}

	err = processEntriesForMode(compatibilityMode, entries, func(entry os.DirEntry) error {
		entryPath := filepath.Join(seriesPath, entry.Name())
		if entry.IsDir() && matchesFileName(entry.Name(), true, matchers) {
			stats.SkipCount++
			s.persistRunHistory(runID, fmt.Sprintf("skipped filtered archive directory %s", entryPath), stats)
			s.appendLog(runID, "info", fmt.Sprintf("skipped filtered archive directory %s", entryPath))
			return nil
		}
		if !entry.IsDir() && matchesFileName(entry.Name(), false, matchers) {
			if err := s.removeFilteredArchiveFile(runID, entryPath, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err))
			} else {
				stats.SkipCount++
			}
			return nil
		}
		if archiveMode == "package" && matchArchiveEnabled && matchesArchiveDirectly(rootSourceDir, entryPath, directMatchers) {
			if err := s.moveMatchedArchiveFile(runID, entryPath, targetSeriesDir, archiveMode, matchArchiveParentRenameEnabled, false, cleanupSourceAfterArchive, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("move matched series file %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("move matched series file %s failed: %v", entryPath, err))
			}
			return nil
		}
		if entry.IsDir() {
			if archiveMode == "collect" && !collectRecursiveEnabled {
				stats.SkipCount++
				s.persistRunHistory(runID, fmt.Sprintf("skipped nested directory %s: recursive_collect disabled", entryPath), stats)
				s.appendLog(runID, "info", fmt.Sprintf("skipped nested directory %s: recursive_collect disabled", entryPath))
				return nil
			}
			if err := s.processVolumeDir(runID, rootSourceDir, entryPath, targetSeriesDir, archiveMode, compatibilityMode, packageNestedFolders, flatArchive, cleanupSourceAfterArchive, collectRecursiveEnabled, collectDeduplicateEnabled, matchers, matchArchiveEnabled, matchArchiveParentRenameEnabled, directMatchers, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("process volume %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("process volume %s failed: %v", entryPath, err))
			}
			return nil
		}

		if err := s.moveLooseFile(runID, entryPath, targetSeriesDir, archiveMode, collectDeduplicateEnabled, cleanupSourceAfterArchive, stats); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("move series file %s failed: %v", entryPath, err), stats)
			s.appendLog(runID, "error", fmt.Sprintf("move series file %s failed: %v", entryPath, err))
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) processVolumeDir(runID, rootSourceDir, volumePath, targetDir, archiveMode, compatibilityMode string, packageNestedFolders, flatArchive bool, cleanupSourceAfterArchive bool, collectRecursiveEnabled bool, collectDeduplicateEnabled bool, matchers []fileNameMatcher, matchArchiveEnabled bool, matchArchiveParentRenameEnabled bool, directMatchers []fileNameMatcher, stats *executionStats) error {
	entries, err := readDirWithMode(compatibilityMode, volumePath)
	if err != nil {
		return fmt.Errorf("read volume dir: %w", err)
	}
	entries = limitEntriesForMode(compatibilityMode, entries)
	if len(entries) == 0 {
		stats.SkipCount++
		s.persistRunHistory(runID, fmt.Sprintf("skipped empty volume %s", volumePath), stats)
		_ = os.Remove(volumePath)
		return nil
	}

	sortEntriesNaturally(entries)
	files := make([]os.DirEntry, 0, len(entries))
	imageFiles := make([]os.DirEntry, 0, len(entries))
	nonImageFiles := make([]os.DirEntry, 0, len(entries))
	matchedFiles := make([]os.DirEntry, 0, len(entries))
	coverFiles := make([]os.DirEntry, 0, len(entries))
	hasSubdirs := false
	err = processEntriesForMode(compatibilityMode, entries, func(entry os.DirEntry) error {
		entryPath := filepath.Join(volumePath, entry.Name())
		if entry.IsDir() {
			if matchesFileName(entry.Name(), true, matchers) {
				return nil
			}
			hasSubdirs = true
			return nil
		}
		if archiveMode == "package" && matchArchiveEnabled && matchesArchiveDirectly(rootSourceDir, entryPath, directMatchers) {
			matchedFiles = append(matchedFiles, entry)
			return nil
		}
		if archiveMode == "package" && isCoverImageFile(entry.Name()) {
			coverFiles = append(coverFiles, entry)
			return nil
		}
		files = append(files, entry)
		if isImageFile(entry.Name()) {
			imageFiles = append(imageFiles, entry)
		} else {
			nonImageFiles = append(nonImageFiles, entry)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if archiveMode == "package" && !hasSubdirs && len(imageFiles) > 0 {
		archivePath, err := createPackageCBZFromFiles(rootSourceDir, volumePath, imageFiles, targetDir, matchArchiveParentRenameEnabled)
		if err != nil {
			return err
		}
		if cleanupSourceAfterArchive {
			if err := removePackedSourceFiles(volumePath, imageFiles); err != nil {
				return fmt.Errorf("remove packed source files: %w", err)
			}
		}
		if err := s.moveCoverFiles(runID, volumePath, coverFiles, filepath.Join(targetDir, filepath.Base(volumePath)), stats); err != nil {
			return err
		}
		if err := s.moveMatchedArchiveEntries(runID, volumePath, matchedFiles, targetDir, matchArchiveParentRenameEnabled, cleanupSourceAfterArchive, stats); err != nil {
			return err
		}
		stats.ProcessedFiles += len(imageFiles)
		stats.SuccessCount++
		stats.PackedVolumes++
		s.persistRunHistory(runID, fmt.Sprintf("packed volume %s -> %s", volumePath, archivePath), stats)
		s.appendLog(runID, "info", fmt.Sprintf("packed volume %s -> %s", volumePath, archivePath))
		for _, entry := range nonImageFiles {
			sourcePath := filepath.Join(volumePath, entry.Name())
			stats.SkipCount++
			s.persistRunHistory(runID, fmt.Sprintf("skipped non-image file %s", sourcePath), stats)
			s.appendLog(runID, "info", fmt.Sprintf("skipped non-image file %s: not included in package archive", sourcePath))
		}
		return nil
	}

	if archiveMode == "package" && !hasSubdirs && len(files) == 0 && (len(coverFiles) > 0 || len(matchedFiles) > 0) {
		if err := s.moveCoverFiles(runID, volumePath, coverFiles, filepath.Join(targetDir, filepath.Base(volumePath)), stats); err != nil {
			return err
		}
		if err := s.moveMatchedArchiveEntries(runID, volumePath, matchedFiles, targetDir, matchArchiveParentRenameEnabled, cleanupSourceAfterArchive, stats); err != nil {
			return err
		}
		return nil
	}

	if archiveMode == "package" && hasSubdirs && !packageNestedFolders {
		stats.SkipCount++
		s.persistRunHistory(runID, fmt.Sprintf("skipped nested directory %s", volumePath), stats)
		s.appendLog(runID, "info", fmt.Sprintf("skipped nested directory %s: package_nested_folders disabled", volumePath))
		return nil
	}

	nextTargetDir := targetDir
	fileTargetDir := targetDir
	if archiveMode == "package" && hasSubdirs {
		if flatArchive {
			nextTargetDir = targetDir
			fileTargetDir = targetDir
		} else {
			nextTargetDir = filepath.Join(targetDir, filepath.Base(volumePath))
			fileTargetDir = nextTargetDir
		}
	}

	err = processEntriesForMode(compatibilityMode, entries, func(entry os.DirEntry) error {
		entryPath := filepath.Join(volumePath, entry.Name())
		if entry.IsDir() && matchesFileName(entry.Name(), true, matchers) {
			stats.SkipCount++
			s.persistRunHistory(runID, fmt.Sprintf("skipped filtered archive directory %s", entryPath), stats)
			s.appendLog(runID, "info", fmt.Sprintf("skipped filtered archive directory %s", entryPath))
			return nil
		}
		if !entry.IsDir() && matchesFileName(entry.Name(), false, matchers) {
			if err := s.removeFilteredArchiveFile(runID, entryPath, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err))
			} else {
				stats.SkipCount++
			}
			return nil
		}
		if archiveMode == "package" && matchArchiveEnabled && matchesArchiveDirectly(rootSourceDir, entryPath, directMatchers) {
			if err := s.moveMatchedArchiveFile(runID, entryPath, fileTargetDir, archiveMode, matchArchiveParentRenameEnabled, false, cleanupSourceAfterArchive, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("move matched nested file %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("move matched nested file %s failed: %v", entryPath, err))
			}
			return nil
		}
		if entry.IsDir() {
			if archiveMode == "collect" && !collectRecursiveEnabled {
				stats.SkipCount++
				s.persistRunHistory(runID, fmt.Sprintf("skipped nested directory %s: recursive_collect disabled", entryPath), stats)
				s.appendLog(runID, "info", fmt.Sprintf("skipped nested directory %s: recursive_collect disabled", entryPath))
				return nil
			}
			if err := s.processVolumeDir(runID, rootSourceDir, entryPath, nextTargetDir, archiveMode, compatibilityMode, packageNestedFolders, flatArchive, cleanupSourceAfterArchive, collectRecursiveEnabled, collectDeduplicateEnabled, matchers, matchArchiveEnabled, matchArchiveParentRenameEnabled, directMatchers, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("process nested directory %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("process nested directory %s failed: %v", entryPath, err))
			}
			return nil
		}

		if err := s.moveLooseFile(runID, entryPath, fileTargetDir, archiveMode, collectDeduplicateEnabled, cleanupSourceAfterArchive, stats); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("move nested file %s failed: %v", entryPath, err), stats)
			s.appendLog(runID, "error", fmt.Sprintf("move nested file %s failed: %v", entryPath, err))
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) moveLooseFile(runID, sourcePath, targetDir, archiveMode string, collectDeduplicateEnabled bool, cleanupSourceAfterArchive bool, stats *executionStats) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	targetPath, actionSummary, err := resolveCollectTargetPath(sourcePath, targetDir, archiveMode, collectDeduplicateEnabled)
	if err != nil {
		return err
	}
	if actionSummary == "skip-same-file" {
		stats.ProcessedFiles++
		stats.SkipCount++
		s.persistRunHistory(runID, fmt.Sprintf("skipped duplicate file %s", sourcePath), stats)
		s.appendLog(runID, "info", fmt.Sprintf("skipped duplicate file %s: same file already exists", sourcePath))
		if cleanupSourceAfterArchive {
			if err := os.Remove(sourcePath); err != nil {
				return fmt.Errorf("remove duplicate source file: %w", err)
			}
		}
		return nil
	}
	if cleanupSourceAfterArchive {
		err = moveFile(sourcePath, targetPath)
	} else {
		err = copyFile(sourcePath, targetPath)
	}
	if err != nil {
		return err
	}

	stats.ProcessedFiles++
	stats.SuccessCount++
	stats.MovedFiles++
	verb := "copied"
	if cleanupSourceAfterArchive {
		verb = "moved"
	}
	message := fmt.Sprintf("%s file %s -> %s", verb, sourcePath, targetPath)
	if actionSummary == "renamed-re" {
		message += " (renamed with -re suffix due to different file with same name)"
	}
	s.persistRunHistory(runID, message, stats)
	s.appendLog(runID, "info", message)
	return nil
}

func (s *Service) moveMatchedArchiveFile(runID, sourcePath, targetDir, archiveMode string, parentRenameEnabled bool, collectDeduplicateEnabled bool, cleanupSourceAfterArchive bool, stats *executionStats) error {
	if !parentRenameEnabled {
		return s.moveLooseFile(runID, sourcePath, targetDir, archiveMode, collectDeduplicateEnabled, cleanupSourceAfterArchive, stats)
	}

	parentName := archiveParentRenameBaseName(targetDir, sourcePath)
	if parentName == "" {
		return s.moveLooseFile(runID, sourcePath, targetDir, archiveMode, collectDeduplicateEnabled, cleanupSourceAfterArchive, stats)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	ext := filepath.Ext(sourcePath)
	targetFileName := parentName + ext
	targetPath := filepath.Join(targetDir, targetFileName)
	if _, err := os.Stat(targetPath); err == nil {
		for index := 1; ; index++ {
			candidate := filepath.Join(targetDir, fmt.Sprintf("%s-part%d%s", parentName, index, ext))
			if _, statErr := os.Stat(candidate); os.IsNotExist(statErr) {
				targetPath = candidate
				break
			}
		}
	}

	var err error
	if cleanupSourceAfterArchive {
		err = moveFile(sourcePath, targetPath)
	} else {
		err = copyFile(sourcePath, targetPath)
	}
	if err != nil {
		return err
	}

	stats.ProcessedFiles++
	stats.SuccessCount++
	stats.MovedFiles++
	verb := "copied"
	if cleanupSourceAfterArchive {
		verb = "moved"
	}
	message := fmt.Sprintf("%s matched file %s -> %s", verb, sourcePath, targetPath)
	s.persistRunHistory(runID, message, stats)
	s.appendLog(runID, "info", message)
	return nil
}

func (s *Service) moveLooseFileToOwnDir(runID, sourcePath, targetDir string, stats *executionStats) error {
	baseName := strings.TrimSpace(filepath.Base(sourcePath))
	if baseName == "" {
		return fmt.Errorf("source file name is empty")
	}

	fileName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = baseName
	}

	nestedTargetDir := filepath.Join(targetDir, fileName)
	if err := os.MkdirAll(nestedTargetDir, 0o755); err != nil {
		return fmt.Errorf("create nested target dir: %w", err)
	}

	targetPath := uniqueArchiveDestinationPath(nestedTargetDir, filepath.Base(sourcePath))
	if err := moveFile(sourcePath, targetPath); err != nil {
		return err
	}

	stats.ProcessedFiles++
	stats.SuccessCount++
	stats.MovedFiles++
	s.persistRunHistory(runID, fmt.Sprintf("nested file %s -> %s", sourcePath, targetPath), stats)
	s.appendLog(runID, "info", fmt.Sprintf("nested file %s -> %s", sourcePath, targetPath))
	return nil
}

func (s *Service) removeFilteredArchiveFile(runID, sourcePath string, stats *executionStats) error {
	if err := os.Remove(sourcePath); err != nil {
		return err
	}

	stats.ProcessedFiles++
	stats.CleanupRemovedFiles++
	s.persistRunHistory(runID, fmt.Sprintf("removed filtered archive file %s", sourcePath), stats)
	s.appendLog(runID, "info", fmt.Sprintf("removed filtered archive file %s", sourcePath))
	return nil
}

func (s *Service) moveCoverFiles(runID, basePath string, files []os.DirEntry, targetDir string, stats *executionStats) error {
	for _, entry := range files {
		sourcePath := filepath.Join(basePath, entry.Name())
		if err := s.moveLooseFile(runID, sourcePath, targetDir, "package", false, true, stats); err != nil {
			return fmt.Errorf("move cover file %s: %w", sourcePath, err)
		}
	}

	return nil
}

func (s *Service) moveMatchedArchiveEntries(runID, basePath string, files []os.DirEntry, targetDir string, parentRenameEnabled bool, cleanupSourceAfterArchive bool, stats *executionStats) error {
	for _, entry := range files {
		sourcePath := filepath.Join(basePath, entry.Name())
		if err := s.moveMatchedArchiveFile(runID, sourcePath, targetDir, "package", parentRenameEnabled, false, cleanupSourceAfterArchive, stats); err != nil {
			return fmt.Errorf("move matched archive file %s: %w", sourcePath, err)
		}
	}

	return nil
}

func archiveParentRenameBaseName(targetDir, sourcePath string) string {
	cleanTargetDir := filepath.Clean(strings.TrimSpace(targetDir))
	cleanSourcePath := filepath.Clean(strings.TrimSpace(sourcePath))
	if cleanSourcePath == "" || cleanSourcePath == "." {
		return ""
	}

	parentDir := filepath.Dir(cleanSourcePath)
	if sameCleanPath(cleanTargetDir, parentDir) {
		return ""
	}

	relDir, err := filepath.Rel(cleanTargetDir, parentDir)
	if err == nil {
		segments := splitArchivePathSegments(relDir)
		if len(segments) > 0 {
			return strings.TrimSpace(segments[0])
		}
	}

	parentName := strings.TrimSpace(filepath.Base(parentDir))
	if parentName == "" || parentName == "." || parentName == string(filepath.Separator) {
		return ""
	}
	return parentName
}

func createPackageCBZFromFiles(rootSourceDir, volumePath string, files []os.DirEntry, targetDir string, parentRenameEnabled bool) (string, error) {
	archiveName := filepath.Base(volumePath) + ".cbz"
	if parentRenameEnabled {
		if parentName := archiveParentRenameBaseName(targetDir, volumePath); parentName != "" {
			archiveName = parentName + ".cbz"
		}
	}
	return createCBZFromFiles(volumePath, files, targetDir, archiveName)
}

func buildArchiveDirectMatchers(filters []string) []fileNameMatcher {
	return buildFileNameMatchers(filters)
}

func matchesArchiveDirectly(rootPath, sourcePath string, matchers []fileNameMatcher) bool {
	rawName := strings.TrimSpace(filepath.Base(sourcePath))
	if rawName == "" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(rawName))
	extWithoutDot := strings.TrimPrefix(ext, ".")
	stem := strings.TrimSuffix(rawName, filepath.Ext(rawName))
	lowerStem := strings.ToLower(stem)
	directoryCandidates := archiveMatchDirectoryCandidates(rootPath, sourcePath)
	lowerDirectoryCandidates := make([]string, 0, len(directoryCandidates))
	for _, candidate := range directoryCandidates {
		lowerDirectoryCandidates = append(lowerDirectoryCandidates, strings.ToLower(candidate))
	}

	for _, matcher := range matchers {
		regexCandidates := make([]string, 0, len(directoryCandidates)+3)
		literalCandidates := make([]string, 0, len(lowerDirectoryCandidates)+3)
		switch matcher.target {
		case ruleMatcherDirectoryName:
			regexCandidates = append(regexCandidates, directoryCandidates...)
			literalCandidates = append(literalCandidates, lowerDirectoryCandidates...)
		case ruleMatcherExtension:
			regexCandidates = append(regexCandidates, ext, extWithoutDot)
			literalCandidates = append(literalCandidates, ext, extWithoutDot)
		case ruleMatcherGlobal:
			regexCandidates = append(regexCandidates, stem, ext, extWithoutDot)
			regexCandidates = append(regexCandidates, directoryCandidates...)
			literalCandidates = append(literalCandidates, lowerStem, ext, extWithoutDot)
			literalCandidates = append(literalCandidates, lowerDirectoryCandidates...)
		default:
			regexCandidates = append(regexCandidates, stem)
			literalCandidates = append(literalCandidates, lowerStem)
		}

		if matcher.regex != nil {
			for _, candidate := range regexCandidates {
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

func archiveMatchDirectoryCandidates(rootPath, sourcePath string) []string {
	cleanRoot := filepath.Clean(strings.TrimSpace(rootPath))
	cleanSource := filepath.Clean(strings.TrimSpace(sourcePath))
	if cleanSource == "" || cleanSource == "." {
		return nil
	}

	parentDir := filepath.Dir(cleanSource)
	if cleanRoot == "" || cleanRoot == "." {
		baseName := strings.TrimSpace(filepath.Base(parentDir))
		if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
			return nil
		}
		return []string{baseName}
	}
	if sameCleanPath(cleanRoot, parentDir) {
		return nil
	}

	relDir, err := filepath.Rel(cleanRoot, parentDir)
	if err != nil {
		baseName := strings.TrimSpace(filepath.Base(parentDir))
		if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
			return nil
		}
		return []string{baseName}
	}

	return splitArchivePathSegments(relDir)
}

func splitArchivePathSegments(path string) []string {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" || cleanPath == "." {
		return nil
	}

	parts := strings.Split(filepath.ToSlash(cleanPath), "/")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" || trimmed == "." {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}

func removePackedSourceFiles(basePath string, files []os.DirEntry) error {
	for _, entry := range files {
		if err := os.Remove(filepath.Join(basePath, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}

func createCBZFromFiles(volumePath string, files []os.DirEntry, targetDir, archiveName string) (string, error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create cbz output dir: %w", err)
	}

	archivePath := uniqueArchiveDestinationPath(targetDir, archiveName)
	tempPath := archivePath + ".tmp"
	_ = os.Remove(tempPath)
	archiveFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("create cbz file: %w", err)
	}

	zipWriter := zip.NewWriter(archiveFile)
	for _, entry := range files {
		sourcePath := filepath.Join(volumePath, entry.Name())
		if err := addFileToArchive(zipWriter, sourcePath, entry.Name()); err != nil {
			_ = zipWriter.Close()
			_ = archiveFile.Close()
			_ = os.Remove(tempPath)
			return "", err
		}
	}

	if err := zipWriter.Close(); err != nil {
		_ = archiveFile.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("finalize cbz file: %w", err)
	}
	if err := archiveFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close cbz file: %w", err)
	}
	if err := verifyAndPublishCBZ(tempPath, archivePath); err != nil {
		return "", err
	}

	return archivePath, nil
}

func verifyAndPublishCBZ(tempPath, archivePath string) error {
	reader, err := zip.OpenReader(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("verify cbz file: %w", err)
	}
	_ = reader.Close()

	if err := os.Rename(tempPath, archivePath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("publish cbz file: %w", err)
	}

	return nil
}

func addFileToArchive(zipWriter *zip.Writer, sourcePath, archivePath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat archive source: %w", err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("create archive header: %w", err)
	}
	header.Name = filepath.ToSlash(archivePath)
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create archive entry: %w", err)
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open archive source: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("write archive entry: %w", err)
	}

	return nil
}

func moveFile(sourcePath, targetPath string) error {
	if err := os.Rename(sourcePath, targetPath); err == nil {
		return nil
	}

	if err := copyFile(sourcePath, targetPath); err != nil {
		return fmt.Errorf("move file: %w", err)
	}
	if err := os.Remove(sourcePath); err != nil {
		return fmt.Errorf("remove source file after move: %w", err)
	}
	return nil
}

func copyFile(sourcePath, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer func() {
		_ = sourceFile.Close()
	}()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create target file: %w", err)
	}
	defer func() {
		_ = targetFile.Close()
	}()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("copy file contents: %w", err)
	}

	return os.Chmod(targetPath, info.Mode())
}

func removeDirIfEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(path)
}

func uniqueArchiveDestinationPath(dir, name string) string {
	baseName := filepath.Base(strings.TrimSpace(name))
	if baseName == "" || baseName == "." || baseName == string(filepath.Separator) {
		baseName = "item"
	}

	targetPath := filepath.Join(dir, baseName)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return targetPath
	}

	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	for index := 1; ; index++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, index, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func resolveCollectTargetPath(sourcePath, targetDir, archiveMode string, collectDeduplicateEnabled bool) (string, string, error) {
	baseName := filepath.Base(strings.TrimSpace(sourcePath))
	if archiveMode != "collect" || !collectDeduplicateEnabled {
		return uniqueArchiveDestinationPath(targetDir, baseName), "", nil
	}

	targetPath := filepath.Join(targetDir, baseName)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return targetPath, "", nil
	} else if err != nil {
		return "", "", fmt.Errorf("stat target file: %w", err)
	}

	sameFile, err := filesAreEquivalent(sourcePath, targetPath)
	if err != nil {
		return "", "", err
	}
	if sameFile {
		return targetPath, "skip-same-file", nil
	}

	return uniqueArchiveReSuffixPath(targetDir, baseName), "renamed-re", nil
}

func uniqueArchiveReSuffixPath(dir, name string) string {
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

	for index := 2; ; index++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-re-%d%s", stem, index, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func filesAreEquivalent(sourcePath, targetPath string) (bool, error) {
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return false, fmt.Errorf("stat source file: %w", err)
	}
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return false, fmt.Errorf("stat target file: %w", err)
	}
	if sourceInfo.Size() != targetInfo.Size() {
		return false, nil
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return false, fmt.Errorf("open source file: %w", err)
	}
	defer func() { _ = sourceFile.Close() }()

	targetFile, err := os.Open(targetPath)
	if err != nil {
		return false, fmt.Errorf("open target file: %w", err)
	}
	defer func() { _ = targetFile.Close() }()

	sourceBuffer := make([]byte, 64*1024)
	targetBuffer := make([]byte, 64*1024)
	for {
		sourceN, sourceErr := sourceFile.Read(sourceBuffer)
		targetN, targetErr := targetFile.Read(targetBuffer)
		if sourceN != targetN || !bytes.Equal(sourceBuffer[:sourceN], targetBuffer[:targetN]) {
			return false, nil
		}
		if sourceErr == io.EOF && targetErr == io.EOF {
			return true, nil
		}
		if sourceErr != nil && sourceErr != io.EOF {
			return false, fmt.Errorf("read source file: %w", sourceErr)
		}
		if targetErr != nil && targetErr != io.EOF {
			return false, fmt.Errorf("read target file: %w", targetErr)
		}
	}
}

func sortEntriesNaturally(entries []os.DirEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return naturalLess(entries[i].Name(), entries[j].Name())
	})
}

func naturalLess(a, b string) bool {
	at := tokenizeNatural(a)
	bt := tokenizeNatural(b)
	for i := 0; i < len(at) && i < len(bt); i++ {
		if at[i] == bt[i] {
			continue
		}

		an, aErr := strconv.Atoi(at[i])
		bn, bErr := strconv.Atoi(bt[i])
		if aErr == nil && bErr == nil {
			return an < bn
		}

		return strings.ToLower(at[i]) < strings.ToLower(bt[i])
	}

	return len(at) < len(bt)
}

func tokenizeNatural(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{""}
	}

	tokens := make([]string, 0)
	var current strings.Builder
	var isDigitGroup bool
	for index, r := range value {
		currentDigit := unicode.IsDigit(r)
		if index == 0 {
			isDigitGroup = currentDigit
			current.WriteRune(r)
			continue
		}

		if currentDigit != isDigitGroup {
			tokens = append(tokens, current.String())
			current.Reset()
			isDigitGroup = currentDigit
		}
		current.WriteRune(r)
	}
	tokens = append(tokens, current.String())
	return tokens
}

func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".avif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func isCoverImageFile(name string) bool {
	if !isImageFile(name) {
		return false
	}

	baseName := strings.TrimSuffix(strings.ToLower(filepath.Base(name)), filepath.Ext(name))
	return strings.Contains(baseName, "cover")
}
