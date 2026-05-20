package model

type BrowseRoot struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type DirectoryEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
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
