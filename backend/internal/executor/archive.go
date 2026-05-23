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
	ProcessedFiles int
	SuccessCount   int
	SkipCount      int
	FailureCount   int
	PackedVolumes  int
	MovedFiles     int
	Summary        string
}

func (s *Service) executeRule(runID string, req ExecuteRuleRequest) (executionStats, error) {
	stats := executionStats{}

	sourceDir := filepath.Clean(strings.TrimSpace(req.SourceDir))
	targetDir := filepath.Clean(strings.TrimSpace(req.TargetDir))
	if sourceDir == "" || sourceDir == "." {
		return stats, fmt.Errorf("source dir is required")
	}
	if targetDir == "" || targetDir == "." {
		return stats, fmt.Errorf("target dir is required")
	}

	info, err := os.Stat(sourceDir)
	if err != nil {
		return stats, fmt.Errorf("stat source dir: %w", err)
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("source dir must be a directory")
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return stats, fmt.Errorf("create target dir: %w", err)
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return stats, fmt.Errorf("read source dir: %w", err)
	}
	if len(entries) == 0 {
		stats.SkipCount = 1
		stats.Summary = "source directory is empty"
		return stats, nil
	}

	sortEntriesNaturally(entries)
	for _, entry := range entries {
		entryPath := filepath.Join(sourceDir, entry.Name())
		if entry.IsDir() {
			if err := s.processSeriesDir(runID, entryPath, filepath.Join(targetDir, entry.Name()), req.ArchiveMode, &stats); err != nil {
				stats.FailureCount++
				s.appendLog(runID, "error", fmt.Sprintf("process series %s failed: %v", entryPath, err))
			}
			continue
		}

		if err := s.moveLooseFile(runID, entryPath, targetDir, &stats); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("move file %s failed: %v", entryPath, err))
		}
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

func (s *Service) processSeriesDir(runID, seriesPath, targetSeriesDir, archiveMode string, stats *executionStats) error {
	if err := os.MkdirAll(targetSeriesDir, 0o755); err != nil {
		return fmt.Errorf("create series target dir: %w", err)
	}

	entries, err := os.ReadDir(seriesPath)
	if err != nil {
		return fmt.Errorf("read series dir: %w", err)
	}
	if len(entries) == 0 {
		stats.SkipCount++
		_ = os.Remove(seriesPath)
		return nil
	}

	sortEntriesNaturally(entries)
	for _, entry := range entries {
		entryPath := filepath.Join(seriesPath, entry.Name())
		if entry.IsDir() {
			if err := s.processVolumeDir(runID, entryPath, targetSeriesDir, archiveMode, stats); err != nil {
				stats.FailureCount++
				s.appendLog(runID, "error", fmt.Sprintf("process volume %s failed: %v", entryPath, err))
			}
			continue
		}

		if err := s.moveLooseFile(runID, entryPath, targetSeriesDir, stats); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("move series file %s failed: %v", entryPath, err))
		}
	}

	_ = removeDirIfEmpty(seriesPath)
	return nil
}

func (s *Service) processVolumeDir(runID, volumePath, targetSeriesDir, archiveMode string, stats *executionStats) error {
	entries, err := os.ReadDir(volumePath)
	if err != nil {
		return fmt.Errorf("read volume dir: %w", err)
	}
	if len(entries) == 0 {
		stats.SkipCount++
		_ = os.Remove(volumePath)
		return nil
	}

	sortEntriesNaturally(entries)
	files := make([]os.DirEntry, 0, len(entries))
	hasSubdirs := false
	allImages := true
	for _, entry := range entries {
		if entry.IsDir() {
			hasSubdirs = true
			allImages = false
			continue
		}
		files = append(files, entry)
		if !isImageFile(entry.Name()) {
			allImages = false
		}
	}

	if archiveMode == "package" && !hasSubdirs && len(files) > 0 && allImages {
		archivePath, err := createCBZFromFiles(volumePath, files, targetSeriesDir, filepath.Base(volumePath)+".cbz")
		if err != nil {
			return err
		}
		if err := os.RemoveAll(volumePath); err != nil {
			return fmt.Errorf("remove packed source directory: %w", err)
		}
		stats.ProcessedFiles += len(files)
		stats.SuccessCount++
		stats.PackedVolumes++
		s.appendLog(runID, "info", fmt.Sprintf("packed volume %s -> %s", volumePath, archivePath))
		return nil
	}

	for _, entry := range entries {
		entryPath := filepath.Join(volumePath, entry.Name())
		if entry.IsDir() {
			if err := s.processVolumeDir(runID, entryPath, targetSeriesDir, archiveMode, stats); err != nil {
				stats.FailureCount++
				s.appendLog(runID, "error", fmt.Sprintf("process nested directory %s failed: %v", entryPath, err))
			}
			continue
		}

		if err := s.moveLooseFile(runID, entryPath, targetSeriesDir, stats); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("move nested file %s failed: %v", entryPath, err))
		}
	}

	_ = removeDirIfEmpty(volumePath)
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
	s.appendLog(runID, "info", fmt.Sprintf("moved file %s -> %s", sourcePath, targetPath))
	return nil
}

func createCBZFromFiles(volumePath string, files []os.DirEntry, targetDir, archiveName string) (string, error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create cbz output dir: %w", err)
	}

	archivePath := uniqueArchiveDestinationPath(targetDir, archiveName)
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("create cbz file: %w", err)
	}
	defer func() {
		_ = archiveFile.Close()
	}()

	zipWriter := zip.NewWriter(archiveFile)
	for _, entry := range files {
		sourcePath := filepath.Join(volumePath, entry.Name())
		if err := addFileToArchive(zipWriter, sourcePath, entry.Name()); err != nil {
			_ = zipWriter.Close()
			return "", err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return "", fmt.Errorf("finalize cbz file: %w", err)
	}

	return archivePath, nil
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
