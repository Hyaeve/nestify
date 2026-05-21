package model

import "time"

type BrowseRoot struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type DirectoryEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	IsDir       bool   `json:"is_dir"`
	Size        int64  `json:"size"`
	ModifiedAt  string `json:"modified_at"`
	HasChildren bool   `json:"has_children"`
}

type BrowseDirectoriesResponse struct {
	CurrentPath string           `json:"current_path"`
	ParentPath  string           `json:"parent_path,omitempty"`
	Entries     []DirectoryEntry `json:"entries"`
}

type ValidatePathResponse struct {
	Path     string `json:"path"`
	Allowed  bool   `json:"allowed"`
	Exists   bool   `json:"exists"`
	IsDir    bool   `json:"is_dir"`
	Readable bool   `json:"readable"`
	Writable bool   `json:"writable"`
}

type CreateDirectoryResponse struct {
	Path string `json:"path"`
}

type FileItemsMutationResponse struct {
	Items      []string `json:"items,omitempty"`
	Total      int      `json:"total"`
	OutputPath string   `json:"output_path,omitempty"`
}

func FormatTimeRFC3339(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.UTC().Format(time.RFC3339)
}
