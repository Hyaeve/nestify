package pathbrowse

import (
	"archive/zip"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var supportedArchiveExtensions = map[string]struct{}{
	".zip": {},
	".cbz": {},
}

func (s *Service) CreateDirectory(parentPath, name string) (string, error) {
	parentPath, err := s.resolveAllowedPath(parentPath)
	if err != nil {
		return "", err
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("folder name is required")
	}
	if filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("invalid folder name")
	}

	targetPath := filepath.Join(parentPath, name)
	if !s.isAllowed(targetPath) {
		return "", fmt.Errorf("target path is outside allowed browse roots")
	}
	if _, err := os.Stat(targetPath); err == nil {
		return "", fmt.Errorf("folder already exists")
	}

	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	return targetPath, nil
}

func (s *Service) UploadFiles(destinationPath string, files []*multipart.FileHeader) ([]string, error) {
	destinationPath, err := s.resolveAllowedPath(destinationPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(destinationPath)
	if err != nil {
		return nil, fmt.Errorf("stat destination: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("destination path must be a directory")
	}

	saved := make([]string, 0, len(files))
	for _, fileHeader := range files {
		src, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("open upload file: %w", err)
		}

		targetPath := uniqueDestinationPath(destinationPath, fileHeader.Filename)
		dst, err := os.Create(targetPath)
		if err != nil {
			_ = src.Close()
			return nil, fmt.Errorf("create upload destination: %w", err)
		}

		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			_ = src.Close()
			return nil, fmt.Errorf("save upload file: %w", err)
		}

		_ = dst.Close()
		_ = src.Close()
		saved = append(saved, targetPath)
	}

	return saved, nil
}

func (s *Service) CopyItems(paths []string, destinationPath string) ([]string, error) {
	destinationPath, err := s.resolveAllowedPath(destinationPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(destinationPath)
	if err != nil {
		return nil, fmt.Errorf("stat destination: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("destination path must be a directory")
	}

	copied := make([]string, 0, len(paths))
	for _, sourcePath := range paths {
		sourcePath, err = s.resolveAllowedPath(sourcePath)
		if err != nil {
			return nil, err
		}

		targetPath := uniqueDestinationPath(destinationPath, filepath.Base(sourcePath))
		if err := copyPathContents(sourcePath, targetPath); err != nil {
			return nil, err
		}
		copied = append(copied, targetPath)
	}

	return copied, nil
}

func (s *Service) MoveItems(paths []string, destinationPath string) ([]string, error) {
	destinationPath, err := s.resolveAllowedPath(destinationPath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(destinationPath)
	if err != nil {
		return nil, fmt.Errorf("stat destination: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("destination path must be a directory")
	}

	moved := make([]string, 0, len(paths))
	for _, sourcePath := range paths {
		sourcePath, err = s.resolveAllowedPath(sourcePath)
		if err != nil {
			return nil, err
		}

		targetPath := uniqueDestinationPath(destinationPath, filepath.Base(sourcePath))
		if err := os.Rename(sourcePath, targetPath); err != nil {
			if err := copyPathContents(sourcePath, targetPath); err != nil {
				return nil, fmt.Errorf("move item: %w", err)
			}
			if err := os.RemoveAll(sourcePath); err != nil {
				return nil, fmt.Errorf("remove source after move: %w", err)
			}
		}

		moved = append(moved, targetPath)
	}

	return moved, nil
}

func (s *Service) RenameItem(path string, newName string) (string, error) {
	resolvedPath, err := s.resolveAllowedPath(path)
	if err != nil {
		return "", err
	}

	newName = strings.TrimSpace(newName)
	if newName == "" {
		return "", fmt.Errorf("new name is required")
	}
	if filepath.Base(newName) != newName || newName == "." || newName == ".." {
		return "", fmt.Errorf("invalid new name")
	}

	targetPath := filepath.Join(filepath.Dir(resolvedPath), newName)
	if !s.isAllowed(targetPath) {
		return "", fmt.Errorf("target path is outside allowed browse roots")
	}
	if samePath(resolvedPath, targetPath) {
		return resolvedPath, nil
	}
	if _, err := os.Stat(targetPath); err == nil {
		return "", fmt.Errorf("target name already exists")
	}

	if err := os.Rename(resolvedPath, targetPath); err != nil {
		return "", fmt.Errorf("rename item: %w", err)
	}

	return targetPath, nil
}

func (s *Service) DeleteItems(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("paths are required")
	}

	for _, itemPath := range paths {
		resolved, err := s.resolveAllowedPath(itemPath)
		if err != nil {
			return err
		}

		if err := os.RemoveAll(resolved); err != nil {
			return fmt.Errorf("delete item: %w", err)
		}
	}

	return nil
}

func (s *Service) PackItemsAsCBZ(paths []string, outputDir, archiveName string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("paths are required")
	}

	resolvedPaths := make([]string, 0, len(paths))
	for _, itemPath := range paths {
		resolved, err := s.resolveAllowedPath(itemPath)
		if err != nil {
			return "", err
		}
		resolvedPaths = append(resolvedPaths, resolved)
	}

	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Dir(resolvedPaths[0])
	}

	outputDir, err := s.resolveAllowedPath(outputDir)
	if err != nil {
		return "", err
	}

	if archiveName = strings.TrimSpace(archiveName); archiveName == "" {
		if len(resolvedPaths) == 1 {
			archiveName = filepath.Base(resolvedPaths[0])
		} else {
			archiveName = fmt.Sprintf("selection-%s", time.Now().Format("20060102-150405"))
		}
	}
	if !strings.HasSuffix(strings.ToLower(archiveName), ".cbz") {
		archiveName += ".cbz"
	}

	archivePath := uniqueDestinationPath(outputDir, archiveName)
	tempPath := archivePath + ".tmp"
	_ = os.Remove(tempPath)
	archiveFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("create cbz file: %w", err)
	}
	defer func() {
		_ = archiveFile.Close()
	}()

	zipWriter := zip.NewWriter(archiveFile)
	for _, itemPath := range resolvedPaths {
		if err := addPathToZip(zipWriter, itemPath); err != nil {
			_ = zipWriter.Close()
			return "", err
		}
	}

	if err := zipWriter.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("finalize cbz file: %w", err)
	}

	if _, err := os.Stat(tempPath); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("verify cbz file: %w", err)
	}

	if err := os.Rename(tempPath, archivePath); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("publish cbz file: %w", err)
	}

	return archivePath, nil
}

func (s *Service) ExtractArchives(paths []string, outputDir string) ([]string, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("paths are required")
	}

	resolvedPaths := make([]string, 0, len(paths))
	for _, itemPath := range paths {
		resolved, err := s.resolveAllowedPath(itemPath)
		if err != nil {
			return nil, err
		}
		resolvedPaths = append(resolvedPaths, resolved)
	}

	if strings.TrimSpace(outputDir) == "" {
		outputDir = filepath.Dir(resolvedPaths[0])
	}

	resolvedOutputDir, err := s.resolveAllowedPath(outputDir)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(resolvedOutputDir)
	if err != nil {
		return nil, fmt.Errorf("stat output directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("output path must be a directory")
	}

	extractedPaths := make([]string, 0, len(resolvedPaths))
	for _, archivePath := range resolvedPaths {
		archiveInfo, err := os.Stat(archivePath)
		if err != nil {
			return nil, fmt.Errorf("stat archive: %w", err)
		}
		if archiveInfo.IsDir() {
			return nil, fmt.Errorf("%s 不是压缩文件", filepath.Base(archivePath))
		}
		if !isSupportedArchiveFile(archivePath) {
			return nil, fmt.Errorf("%s 不是支持的压缩包，仅支持 zip/cbz", filepath.Base(archivePath))
		}

		targetDir := uniqueDestinationPath(resolvedOutputDir, archiveBaseName(archivePath))
		if err := extractZipArchive(archivePath, targetDir); err != nil {
			_ = os.RemoveAll(targetDir)
			return nil, err
		}
		extractedPaths = append(extractedPaths, targetDir)
	}

	return extractedPaths, nil
}

func (s *Service) resolveAllowedPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	resolved, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	if !s.isAllowed(resolved) {
		return "", fmt.Errorf("path is outside allowed browse roots")
	}

	return resolved, nil
}

func uniqueDestinationPath(dir, name string) string {
	baseName := filepath.Base(strings.TrimSpace(name))
	if baseName == "." || baseName == string(filepath.Separator) || baseName == "" {
		baseName = fmt.Sprintf("item-%d", time.Now().Unix())
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

func archiveBaseName(path string) string {
	baseName := filepath.Base(path)
	ext := filepath.Ext(baseName)
	return strings.TrimSuffix(baseName, ext)
}

func isSupportedArchiveFile(path string) bool {
	_, ok := supportedArchiveExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func extractZipArchive(archivePath, outputDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	rootPath := filepath.Clean(outputDir)
	for _, file := range reader.File {
		targetPath := filepath.Join(rootPath, file.Name)
		cleanTargetPath := filepath.Clean(targetPath)
		if cleanTargetPath != rootPath && !strings.HasPrefix(cleanTargetPath, rootPath+string(filepath.Separator)) {
			return fmt.Errorf("archive contains invalid entry: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(cleanTargetPath, file.Mode()); err != nil {
				return fmt.Errorf("create extracted directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(cleanTargetPath), 0o755); err != nil {
			return fmt.Errorf("create extracted parent directory: %w", err)
		}

		sourceFile, err := file.Open()
		if err != nil {
			return fmt.Errorf("open archived file: %w", err)
		}

		targetFile, err := os.OpenFile(cleanTargetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			_ = sourceFile.Close()
			return fmt.Errorf("create extracted file: %w", err)
		}

		if _, err := io.Copy(targetFile, sourceFile); err != nil {
			_ = targetFile.Close()
			_ = sourceFile.Close()
			return fmt.Errorf("write extracted file: %w", err)
		}

		_ = targetFile.Close()
		_ = sourceFile.Close()
	}

	return nil
}

func copyPathContents(sourcePath, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if info.IsDir() {
		if err := os.MkdirAll(targetPath, info.Mode()); err != nil {
			return fmt.Errorf("create target directory: %w", err)
		}

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return fmt.Errorf("read source directory: %w", err)
		}

		for _, entry := range entries {
			if err := copyPathContents(filepath.Join(sourcePath, entry.Name()), filepath.Join(targetPath, entry.Name())); err != nil {
				return err
			}
		}

		return nil
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

func addPathToZip(zipWriter *zip.Writer, sourcePath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat archive source: %w", err)
	}

	if !info.IsDir() {
		return addFileToZip(zipWriter, sourcePath, filepath.Base(sourcePath))
	}

	return filepath.Walk(sourcePath, func(path string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(filepath.Dir(sourcePath), path)
		if err != nil {
			return err
		}

		archivePath := filepath.ToSlash(relPath)
		if walkInfo.IsDir() {
			if archivePath != "" && archivePath != "." {
				_, err := zipWriter.Create(strings.TrimSuffix(archivePath, "/") + "/")
				return err
			}
			return nil
		}

		return addFileToZip(zipWriter, path, archivePath)
	})
}

func addFileToZip(zipWriter *zip.Writer, sourcePath, archivePath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(archivePath)
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = io.Copy(writer, file)
	return err
}
