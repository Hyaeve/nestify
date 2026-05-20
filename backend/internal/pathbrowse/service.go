package pathbrowse

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"nestify/backend/internal/model"
)

type Service struct {
	roots []string
}

func New(roots []string) *Service {
	normalized := normalizeRoots(roots)
	if len(normalized) == 0 {
		normalized = inferDefaultRoots()
	}

	return &Service{roots: normalized}
}

func (s *Service) Roots() []model.BrowseRoot {
	items := make([]model.BrowseRoot, 0, len(s.roots))
	for _, root := range s.roots {
		items = append(items, model.BrowseRoot{
			Name: root,
			Path: root,
		})
	}

	return items
}

func (s *Service) Browse(path string) (*model.BrowseDirectoriesResponse, error) {
	resolved, root, err := s.resolvePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	items := make([]model.DirectoryEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		childPath := filepath.Join(resolved, entry.Name())
		items = append(items, model.DirectoryEntry{
			Name:        entry.Name(),
			Path:        childPath,
			HasChildren: directoryHasChildren(childPath),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	response := &model.BrowseDirectoriesResponse{
		CurrentPath: resolved,
		Entries:     items,
	}

	if !samePath(resolved, root) {
		parent := filepath.Dir(resolved)
		if s.isAllowed(parent) {
			response.ParentPath = parent
		}
	}

	return response, nil
}

func (s *Service) Validate(path string) (*model.ValidatePathResponse, error) {
	resolved := path
	if strings.TrimSpace(resolved) != "" {
		abs, err := filepath.Abs(filepath.Clean(resolved))
		if err == nil {
			resolved = abs
		}
	}

	allowed := s.isAllowed(resolved)
	result := &model.ValidatePathResponse{
		Path:    resolved,
		Allowed: allowed,
	}

	if !allowed {
		return result, nil
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("stat path: %w", err)
	}

	result.Exists = true
	result.IsDir = info.IsDir()
	result.Readable = true
	if info.IsDir() {
		result.Writable = isDirectoryWritable(resolved)
	}

	return result, nil
}

func (s *Service) resolvePath(path string) (string, string, error) {
	if strings.TrimSpace(path) == "" {
		if len(s.roots) == 0 {
			return "", "", fmt.Errorf("no browse roots configured")
		}
		return s.roots[0], s.roots[0], nil
	}

	resolved, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}

	for _, root := range s.roots {
		if isPathWithin(root, resolved) {
			return resolved, root, nil
		}
	}

	return "", "", fmt.Errorf("path is outside allowed browse roots")
}

func (s *Service) isAllowed(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}

	resolved, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}

	for _, root := range s.roots {
		if isPathWithin(root, resolved) {
			return true
		}
	}

	return false
}

func normalizeRoots(roots []string) []string {
	items := make([]string, 0, len(roots))
	seen := make(map[string]struct{})

	for _, root := range roots {
		value := strings.TrimSpace(root)
		if value == "" {
			continue
		}

		abs, err := filepath.Abs(filepath.Clean(value))
		if err != nil {
			continue
		}

		key := strings.ToLower(abs)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, abs)
	}

	sort.Strings(items)
	return items
}

func inferDefaultRoots() []string {
	if runtime.GOOS == "windows" {
		items := make([]string, 0)
		for drive := 'C'; drive <= 'Z'; drive++ {
			root := fmt.Sprintf("%c:\\", drive)
			if _, err := os.Stat(root); err == nil {
				items = append(items, root)
			}
		}
		if len(items) > 0 {
			return items
		}
	}

	return []string{"/"}
}

func directoryHasChildren(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return true
		}
	}

	return false
}

func isDirectoryWritable(path string) bool {
	file, err := os.CreateTemp(path, ".nestify-write-check-*")
	if err != nil {
		return false
	}
	_ = file.Close()
	_ = os.Remove(file.Name())
	return true
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func isPathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}

	if rel == "." {
		return true
	}

	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}
