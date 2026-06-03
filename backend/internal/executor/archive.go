package executor

import (
	"archive/zip"
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
	if len(entries) == 0 {
		stats.SkipCount = 1
		stats.Summary = "source directory is empty"
		return stats, nil
	}

	sortEntriesNaturally(entries)
	matchers := buildFileNameMatchers(req.Filters)
	matchArchiveEnabled := req.ArchiveMode == "package" && req.PackageOptions["match_archive"]
	directMatchers := buildArchiveDirectMatchers(req.MatchFilters)
	singleFileNestingEnabled := req.ArchiveMode == "package" && req.PackageOptions["single_file_nesting"]
	nestMatchers := buildArchiveDirectMatchers(req.NestFilters)
	packageNestedFolders := req.PackageOptions["package_nested_folders"]
	flatArchive := req.PackageOptions["flat_archive"]
	cleanupSourceAfterArchive := false
	if req.ArchiveMode == "package" {
		cleanupSourceAfterArchive = req.PackageOptions["cleanup_source_after_archive"]
	} else if req.ArchiveMode == "collect" {
		cleanupSourceAfterArchive = req.CollectOptions["cleanup_source_after_archive"]
	}
	for _, entry := range entries {
		entryPath := filepath.Join(sourceDir, entry.Name())
		if !entry.IsDir() && matchesFileName(entry.Name(), matchers) {
			if err := s.removeFilteredArchiveFile(runID, entryPath, &stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err), &stats)
				s.appendLog(runID, "error", fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err))
			} else {
				stats.SkipCount++
			}
			continue
		}
		if matchArchiveEnabled && !entry.IsDir() && matchesArchiveDirectly(entry.Name(), directMatchers) {
			if err := s.moveLooseFile(runID, entryPath, targetDir, &stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("move matched file %s failed: %v", entryPath, err), &stats)
				s.appendLog(runID, "error", fmt.Sprintf("move matched file %s failed: %v", entryPath, err))
			}
			continue
		}
		if singleFileNestingEnabled && !entry.IsDir() && matchesArchiveDirectly(entry.Name(), nestMatchers) {
			if err := s.moveLooseFileToOwnDir(runID, entryPath, targetDir, &stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("nest matched file %s failed: %v", entryPath, err), &stats)
				s.appendLog(runID, "error", fmt.Sprintf("nest matched file %s failed: %v", entryPath, err))
			}
			continue
		}
		if entry.IsDir() {
			targetSeriesDir := filepath.Join(targetDir, entry.Name())
			if req.ArchiveMode == "package" && flatArchive {
				targetSeriesDir = targetDir
			}
			if err := s.processSeriesDir(runID, entryPath, targetSeriesDir, req.ArchiveMode, req.CompatibilityMode, packageNestedFolders, cleanupSourceAfterArchive, matchers, matchArchiveEnabled, directMatchers, &stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("process series %s failed: %v", entryPath, err), &stats)
				s.appendLog(runID, "error", fmt.Sprintf("process series %s failed: %v", entryPath, err))
			}
			continue
		}

		if err := s.moveLooseFile(runID, entryPath, targetDir, &stats); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("move file %s failed: %v", entryPath, err), &stats)
			s.appendLog(runID, "error", fmt.Sprintf("move file %s failed: %v", entryPath, err))
		}
	}

	if cleanupSourceAfterArchive {
		_ = removeDirIfEmpty(sourceDir)
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

	sortEntriesNaturally(entries)
	for _, entry := range entries {
		sourcePath := filepath.Join(currentPath, entry.Name())
		relPath, relErr := filepath.Rel(rootPath, sourcePath)
		if relErr != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("resolve relative path for %s failed: %v", sourcePath, relErr))
			continue
		}
		if relPath == "." {
			continue
		}

		targetPath := filepath.Join(targetRoot, relPath)
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				stats.FailureCount++
				s.appendLog(runID, "error", fmt.Sprintf("create link target directory %s failed: %v", targetPath, err))
				continue
			}
			if err := s.linkDirectory(runID, rootPath, sourcePath, targetRoot, linkMode, compatibilityMode, matchers, stats); err != nil {
				return err
			}
			continue
		}

		if matchesFileName(entry.Name(), matchers) {
			stats.SkipCount++
			s.appendLog(runID, "info", fmt.Sprintf("skipped blacklisted file %s", sourcePath))
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("create link parent directory %s failed: %v", filepath.Dir(targetPath), err))
			continue
		}

		if _, err := os.Lstat(targetPath); err == nil {
			stats.SkipCount++
			s.appendLog(runID, "info", fmt.Sprintf("skipped existing link target %s", targetPath))
			continue
		} else if !os.IsNotExist(err) {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("inspect link target %s failed: %v", targetPath, err))
			continue
		}

		if err := createFileLink(sourcePath, targetPath, linkMode); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("create link %s -> %s failed: %v", targetPath, sourcePath, err))
			continue
		}

		stats.ProcessedFiles++
		stats.SuccessCount++
		s.appendLog(runID, "info", fmt.Sprintf("created link %s -> %s", targetPath, sourcePath))
	}

	return nil
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

func (s *Service) processSeriesDir(runID, seriesPath, targetSeriesDir, archiveMode, compatibilityMode string, packageNestedFolders bool, cleanupSourceAfterArchive bool, matchers []fileNameMatcher, matchArchiveEnabled bool, directMatchers []archiveDirectMatcher, stats *executionStats) error {
	entries, err := readDirWithMode(compatibilityMode, seriesPath)
	if err != nil {
		return fmt.Errorf("read series dir: %w", err)
	}
	if len(entries) == 0 {
		stats.SkipCount++
		s.persistRunHistory(runID, fmt.Sprintf("skipped empty series %s", seriesPath), stats)
		_ = os.Remove(seriesPath)
		return nil
	}

	sortEntriesNaturally(entries)
	files := make([]os.DirEntry, 0, len(entries))
	coverFiles := make([]os.DirEntry, 0, len(entries))
	hasSubdirs := false
	allImages := true
	for _, entry := range entries {
		if entry.IsDir() {
			hasSubdirs = true
			allImages = false
			continue
		}
		if archiveMode == "package" && matchArchiveEnabled && matchesArchiveDirectly(entry.Name(), directMatchers) {
			coverFiles = append(coverFiles, entry)
			continue
		}
		if archiveMode == "package" && isCoverImageFile(entry.Name()) {
			coverFiles = append(coverFiles, entry)
			continue
		}
		files = append(files, entry)
		if !isImageFile(entry.Name()) {
			allImages = false
		}
	}

	if archiveMode == "package" && !hasSubdirs && len(files) > 0 && allImages {
		archivePath, err := createCBZFromFiles(seriesPath, files, targetSeriesDir, filepath.Base(seriesPath)+".cbz")
		if err != nil {
			return err
		}
		if cleanupSourceAfterArchive {
			if err := removePackedSourceFiles(seriesPath, files); err != nil {
				return fmt.Errorf("remove packed source files: %w", err)
			}
		}
		if err := s.moveCoverFiles(runID, seriesPath, coverFiles, targetSeriesDir, stats); err != nil {
			return err
		}
		stats.ProcessedFiles += len(files)
		stats.SuccessCount++
		stats.PackedVolumes++
		s.persistRunHistory(runID, fmt.Sprintf("packed series %s -> %s", seriesPath, archivePath), stats)
		s.appendLog(runID, "info", fmt.Sprintf("packed series %s -> %s", seriesPath, archivePath))
		if cleanupSourceAfterArchive {
			_ = removeDirIfEmpty(seriesPath)
		}
		return nil
	}

	if archiveMode == "package" && !hasSubdirs && len(files) == 0 && len(coverFiles) > 0 {
		if err := s.moveCoverFiles(runID, seriesPath, coverFiles, targetSeriesDir, stats); err != nil {
			return err
		}
		if cleanupSourceAfterArchive {
			_ = removeDirIfEmpty(seriesPath)
		}
		return nil
	}

	for _, entry := range entries {
		entryPath := filepath.Join(seriesPath, entry.Name())
		if !entry.IsDir() && matchesFileName(entry.Name(), matchers) {
			if err := s.removeFilteredArchiveFile(runID, entryPath, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err))
			} else {
				stats.SkipCount++
			}
			continue
		}
		if entry.IsDir() {
			if err := s.processVolumeDir(runID, entryPath, targetSeriesDir, archiveMode, compatibilityMode, packageNestedFolders, cleanupSourceAfterArchive, matchers, matchArchiveEnabled, directMatchers, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("process volume %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("process volume %s failed: %v", entryPath, err))
			}
			continue
		}

		if err := s.moveLooseFile(runID, entryPath, targetSeriesDir, stats); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("move series file %s failed: %v", entryPath, err), stats)
			s.appendLog(runID, "error", fmt.Sprintf("move series file %s failed: %v", entryPath, err))
		}
	}

	if cleanupSourceAfterArchive {
		_ = removeDirIfEmpty(seriesPath)
	}
	return nil
}

func (s *Service) processVolumeDir(runID, volumePath, targetDir, archiveMode, compatibilityMode string, packageNestedFolders bool, cleanupSourceAfterArchive bool, matchers []fileNameMatcher, matchArchiveEnabled bool, directMatchers []archiveDirectMatcher, stats *executionStats) error {
	entries, err := readDirWithMode(compatibilityMode, volumePath)
	if err != nil {
		return fmt.Errorf("read volume dir: %w", err)
	}
	if len(entries) == 0 {
		stats.SkipCount++
		s.persistRunHistory(runID, fmt.Sprintf("skipped empty volume %s", volumePath), stats)
		_ = os.Remove(volumePath)
		return nil
	}

	sortEntriesNaturally(entries)
	files := make([]os.DirEntry, 0, len(entries))
	coverFiles := make([]os.DirEntry, 0, len(entries))
	hasSubdirs := false
	allImages := true
	for _, entry := range entries {
		if entry.IsDir() {
			hasSubdirs = true
			allImages = false
			continue
		}
		if archiveMode == "package" && matchArchiveEnabled && matchesArchiveDirectly(entry.Name(), directMatchers) {
			coverFiles = append(coverFiles, entry)
			continue
		}
		if archiveMode == "package" && isCoverImageFile(entry.Name()) {
			coverFiles = append(coverFiles, entry)
			continue
		}
		files = append(files, entry)
		if !isImageFile(entry.Name()) {
			allImages = false
		}
	}

	if archiveMode == "package" && !hasSubdirs && len(files) > 0 && allImages {
		archivePath, err := createCBZFromFiles(volumePath, files, targetDir, filepath.Base(volumePath)+".cbz")
		if err != nil {
			return err
		}
		if cleanupSourceAfterArchive {
			if err := removePackedSourceFiles(volumePath, files); err != nil {
				return fmt.Errorf("remove packed source files: %w", err)
			}
		}
		if err := s.moveCoverFiles(runID, volumePath, coverFiles, filepath.Join(targetDir, filepath.Base(volumePath)), stats); err != nil {
			return err
		}
		stats.ProcessedFiles += len(files)
		stats.SuccessCount++
		stats.PackedVolumes++
		s.persistRunHistory(runID, fmt.Sprintf("packed volume %s -> %s", volumePath, archivePath), stats)
		s.appendLog(runID, "info", fmt.Sprintf("packed volume %s -> %s", volumePath, archivePath))
		if cleanupSourceAfterArchive {
			_ = removeDirIfEmpty(volumePath)
		}
		return nil
	}

	if archiveMode == "package" && !hasSubdirs && len(files) == 0 && len(coverFiles) > 0 {
		if err := s.moveCoverFiles(runID, volumePath, coverFiles, filepath.Join(targetDir, filepath.Base(volumePath)), stats); err != nil {
			return err
		}
		if cleanupSourceAfterArchive {
			_ = removeDirIfEmpty(volumePath)
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
		nextTargetDir = filepath.Join(targetDir, filepath.Base(volumePath))
		fileTargetDir = nextTargetDir
	}

	for _, entry := range entries {
		entryPath := filepath.Join(volumePath, entry.Name())
		if !entry.IsDir() && matchesFileName(entry.Name(), matchers) {
			if err := s.removeFilteredArchiveFile(runID, entryPath, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("remove filtered archive file %s failed: %v", entryPath, err))
			} else {
				stats.SkipCount++
			}
			continue
		}
		if entry.IsDir() {
			if err := s.processVolumeDir(runID, entryPath, nextTargetDir, archiveMode, compatibilityMode, packageNestedFolders, cleanupSourceAfterArchive, matchers, matchArchiveEnabled, directMatchers, stats); err != nil {
				stats.FailureCount++
				s.persistRunHistory(runID, fmt.Sprintf("process nested directory %s failed: %v", entryPath, err), stats)
				s.appendLog(runID, "error", fmt.Sprintf("process nested directory %s failed: %v", entryPath, err))
			}
			continue
		}

		if err := s.moveLooseFile(runID, entryPath, fileTargetDir, stats); err != nil {
			stats.FailureCount++
			s.persistRunHistory(runID, fmt.Sprintf("move nested file %s failed: %v", entryPath, err), stats)
			s.appendLog(runID, "error", fmt.Sprintf("move nested file %s failed: %v", entryPath, err))
		}
	}

	if cleanupSourceAfterArchive {
		_ = removeDirIfEmpty(volumePath)
	}
	return nil
}

func (s *Service) moveLooseFile(runID, sourcePath, targetDir string, stats *executionStats) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	targetPath := uniqueArchiveDestinationPath(targetDir, filepath.Base(sourcePath))
	if err := moveFile(sourcePath, targetPath); err != nil {
		return err
	}

	stats.ProcessedFiles++
	stats.SuccessCount++
	stats.MovedFiles++
	s.persistRunHistory(runID, fmt.Sprintf("moved file %s -> %s", sourcePath, targetPath), stats)
	s.appendLog(runID, "info", fmt.Sprintf("moved file %s -> %s", sourcePath, targetPath))
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
		if err := s.moveLooseFile(runID, sourcePath, targetDir, stats); err != nil {
			return fmt.Errorf("move cover file %s: %w", sourcePath, err)
		}
	}

	return nil
}

type archiveDirectMatcher struct {
	value string
}

func buildArchiveDirectMatchers(filters []string) []archiveDirectMatcher {
	items := make([]archiveDirectMatcher, 0, len(filters))
	for _, filter := range filters {
		trimmed := strings.ToLower(strings.TrimSpace(filter))
		if trimmed == "" {
			continue
		}
		items = append(items, archiveDirectMatcher{value: strings.TrimPrefix(trimmed, ".")})
	}
	return items
}

func matchesArchiveDirectly(name string, matchers []archiveDirectMatcher) bool {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(lowerName)), ".")
	for _, matcher := range matchers {
		if matcher.value == "" {
			continue
		}
		if ext != "" && matcher.value == ext {
			return true
		}
		if strings.Contains(lowerName, matcher.value) {
			return true
		}
	}
	return false
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
