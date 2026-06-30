package pathbrowse

import (
	"archive/zip"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var supportedArchiveExtensions = map[string]struct{}{
	".zip": {},
	".cbz": {},
}

var supportedImageExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".gif":  {},
	".webp": {},
	".bmp":  {},
	".avif": {},
	".tif":  {},
	".tiff": {},
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

func (s *Service) UploadFiles(destinationPath string, files []*multipart.FileHeader, relativePaths []string) ([]string, error) {
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
	for index, fileHeader := range files {
		src, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("open upload file: %w", err)
		}

		relativePath := ""
		if index < len(relativePaths) {
			relativePath = relativePaths[index]
		}

		normalizedRelativePath, err := normalizeUploadRelativePath(relativePath, fileHeader.Filename)
		if err != nil {
			_ = src.Close()
			return nil, err
		}

		targetPath := filepath.Join(destinationPath, normalizedRelativePath)
		if !s.isAllowed(targetPath) {
			_ = src.Close()
			return nil, fmt.Errorf("upload target path is outside allowed browse roots")
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			_ = src.Close()
			return nil, fmt.Errorf("create upload parent directory: %w", err)
		}
		targetPath = uniqueDestinationPath(filepath.Dir(targetPath), filepath.Base(targetPath))
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

func normalizeUploadRelativePath(relativePath, fallbackName string) (string, error) {
	fallbackName = strings.TrimSpace(filepath.Base(fallbackName))
	if fallbackName == "" || fallbackName == "." {
		return "", fmt.Errorf("invalid upload file name")
	}

	trimmed := strings.TrimSpace(relativePath)
	if trimmed == "" {
		return fallbackName, nil
	}

	trimmed = strings.ReplaceAll(trimmed, `\\`, "/")
	cleanRelativePath := filepath.Clean(filepath.FromSlash(trimmed))
	if cleanRelativePath == "." || cleanRelativePath == "" {
		return fallbackName, nil
	}
	if filepath.IsAbs(cleanRelativePath) || cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid upload relative path")
	}

	baseName := strings.TrimSpace(filepath.Base(cleanRelativePath))
	if baseName == "" || baseName == "." || baseName == ".." {
		return "", fmt.Errorf("invalid upload relative path")
	}

	return cleanRelativePath, nil
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

func (s *Service) PackFoldersAsCBZ(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("paths are required")
	}

	outputPaths := make([]string, 0, len(paths))
	for _, itemPath := range paths {
		rootPath, err := s.resolveAllowedPath(itemPath)
		if err != nil {
			return nil, err
		}

		info, err := os.Stat(rootPath)
		if err != nil {
			return nil, fmt.Errorf("stat folder: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("pack folders requires directory paths: %s", rootPath)
		}

		archivePath := uniqueDestinationPath(filepath.Dir(rootPath), filepath.Base(rootPath)+".cbz")
		createdPath, err := packFolderImagesAsCBZ(rootPath, archivePath)
		if err != nil {
			return nil, err
		}
		outputPaths = append(outputPaths, createdPath)
	}

	return outputPaths, nil
}

func (s *Service) PackItemsAsCBZ(paths []string, outputDir, archiveName string, nestSourceFolder bool) (string, error) {
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
		if err := addPathToZip(zipWriter, itemPath, nestSourceFolder); err != nil {
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

func (s *Service) CollectItems(paths []string, removeSubfolders bool) ([]string, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("paths are required")
	}

	collectedRoots := make([]string, 0, len(paths))
	for _, itemPath := range paths {
		rootPath, err := s.resolveAllowedPath(itemPath)
		if err != nil {
			return nil, err
		}

		info, err := os.Stat(rootPath)
		if err != nil {
			return nil, fmt.Errorf("stat collect root: %w", err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%s 不是文件夹", filepath.Base(rootPath))
		}

		files, dirs, err := scanNestedFiles(rootPath)
		if err != nil {
			return nil, err
		}

		for _, filePath := range files {
			targetPath := uniqueDestinationPath(rootPath, filepath.Base(filePath))
			if err := os.Rename(filePath, targetPath); err != nil {
				if err := copyPathContents(filePath, targetPath); err != nil {
					return nil, fmt.Errorf("collect file: %w", err)
				}
				if err := os.Remove(filePath); err != nil {
					return nil, fmt.Errorf("remove source file after collect: %w", err)
				}
			}
		}

		if removeSubfolders {
			sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
			for _, dirPath := range dirs {
				if samePath(dirPath, rootPath) {
					continue
				}
				if err := os.Remove(dirPath); err != nil && !os.IsNotExist(err) {
					return nil, fmt.Errorf("remove subfolder: %w", err)
				}
			}
		}

		collectedRoots = append(collectedRoots, rootPath)
	}

	return collectedRoots, nil
}

func scanNestedFiles(rootPath string) ([]string, []string, error) {
	files := make([]string, 0)
	dirs := make([]string, 0)
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if samePath(path, rootPath) {
			return nil
		}
		if info.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		if samePath(filepath.Dir(path), rootPath) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("scan nested files: %w", err)
	}
	return files, dirs, nil
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

func packFolderImagesAsCBZ(rootPath, archivePath string) (string, error) {
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
	usedNames := make(map[string]struct{})
	addedCount := 0

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		if isSupportedArchiveFile(path) {
			count, err := addArchiveImagesToZip(zipWriter, path, rootPath, usedNames)
			if err != nil {
				return err
			}
			addedCount += count
			return nil
		}

		if !isImageFile(path) {
			return nil
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return err
		}
		archiveEntryPath := uniqueZipEntryName(filepath.ToSlash(relPath), usedNames)
		if err := addFileToZip(zipWriter, path, archiveEntryPath); err != nil {
			return err
		}
		addedCount++
		return nil
	})
	if err != nil {
		_ = zipWriter.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("pack folder images: %w", err)
	}

	if addedCount == 0 {
		_ = zipWriter.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("folder contains no supported image files")
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

func addArchiveImagesToZip(zipWriter *zip.Writer, archivePath, rootPath string, usedNames map[string]struct{}) (int, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, fmt.Errorf("open nested archive: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	relArchivePath, err := filepath.Rel(rootPath, archivePath)
	if err != nil {
		return 0, err
	}
	archiveStem := strings.TrimSuffix(filepath.ToSlash(relArchivePath), filepath.Ext(relArchivePath))
	addedCount := 0

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !isImageFile(file.Name) {
			continue
		}

		sourceFile, err := file.Open()
		if err != nil {
			return addedCount, fmt.Errorf("open nested archive entry: %w", err)
		}

		header := file.FileHeader
		header.Name = uniqueZipEntryName(filepath.ToSlash(filepath.Join(archiveStem, file.Name)), usedNames)
		header.Method = zip.Deflate
		writer, err := zipWriter.CreateHeader(&header)
		if err != nil {
			_ = sourceFile.Close()
			return addedCount, err
		}
		if _, err := io.Copy(writer, sourceFile); err != nil {
			_ = sourceFile.Close()
			return addedCount, err
		}
		_ = sourceFile.Close()
		addedCount++
	}

	return addedCount, nil
}

func isImageFile(path string) bool {
	_, ok := supportedImageExtensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

func uniqueZipEntryName(name string, usedNames map[string]struct{}) string {
	name = strings.TrimLeft(filepath.ToSlash(filepath.Clean(name)), "/")
	if name == "." || name == "" {
		name = fmt.Sprintf("image-%d", time.Now().UnixNano())
	}

	if _, exists := usedNames[name]; !exists {
		usedNames[name] = struct{}{}
		return name
	}

	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for index := 1; ; index++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, index, ext)
		if _, exists := usedNames[candidate]; !exists {
			usedNames[candidate] = struct{}{}
			return candidate
		}
	}
}

func addPathToZip(zipWriter *zip.Writer, sourcePath string, nestSourceFolder bool) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat archive source: %w", err)
	}

	if !info.IsDir() {
		return addFileToZip(zipWriter, sourcePath, filepath.Base(sourcePath))
	}

	baseDir := filepath.Dir(sourcePath)
	if !nestSourceFolder {
		baseDir = sourcePath
	}

	return filepath.Walk(sourcePath, func(path string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(baseDir, path)
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
