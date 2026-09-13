package model

import "time"

// MountPathScheme 是虚拟 WebDAV 挂载路径的协议前缀，形如 webdav://12 或 webdav://12/移动云盘/电视剧。
const MountPathScheme = "webdav://"

// WebdavMount 描述一个 OpenList / WebDAV 挂载点。
type WebdavMount struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Scheme      string `json:"scheme"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	HasPassword bool   `json:"has_password"`
	// Password 仅在选择单个挂载（编辑）时回填，供前端回显；列表接口始终为空。
	Password    string    `json:"password,omitempty"`
	BasePath    string    `json:"base_path"`
	Enabled     bool      `json:"enabled"`
	SortOrder   int       `json:"sort_order"`
	BaseURL     string    `json:"base_url"`
	VirtualPath string    `json:"virtual_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateMountInput struct {
	Name      string `json:"name"`
	Scheme    string `json:"scheme"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	BasePath  string `json:"base_path"`
	Enabled   *bool  `json:"enabled"`
	SortOrder int    `json:"sort_order"`
}

type UpdateMountInput struct {
	Name      string `json:"name"`
	Scheme    string `json:"scheme"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	BasePath  string `json:"base_path"`
	Enabled   *bool  `json:"enabled"`
	SortOrder int    `json:"sort_order"`
}

// MountCredential 携带明文口令，仅供服务端内部建立连接使用，绝不出现在 HTTP 响应里。
type MountCredential struct {
	Mount    WebdavMount
	Password string
}

// NormalizeMountScheme 把用户输入统一成 http / https。
func NormalizeMountScheme(value string) string {
	if value == "https" {
		return "https"
	}
	return "http"
}

// NormalizeMountBasePath 规整「指定路径」，保证以 / 开头且不带结尾斜杠。
func NormalizeMountBasePath(value string) string {
	trimmed := value
	for len(trimmed) > 0 && (trimmed[len(trimmed)-1] == '/' || trimmed[len(trimmed)-1] == '\\') {
		trimmed = trimmed[:len(trimmed)-1]
	}
	if trimmed == "" || trimmed == "/" {
		return ""
	}
	if trimmed[0] != '/' {
		return "/" + trimmed
	}
	return trimmed
}

// BuildMountBaseURL 拼接挂载的 http 根地址，例如 http://10.0.0.31:25244。
func BuildMountBaseURL(mount WebdavMount) string {
	host := mount.Host
	port := mount.Port
	scheme := NormalizeMountScheme(mount.Scheme)

	defaultPort := (scheme == "http" && port == 80) || (scheme == "https" && port == 443)
	if port <= 0 || defaultPort {
		return scheme + "://" + host
	}
	return scheme + "://" + host + ":" + itoa(port)
}

// BuildMountVirtualPath 生成挂载在文件系统视图中的虚拟根路径。
func BuildMountVirtualPath(id int64) string {
	return MountPathScheme + itoa64(id)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	buf := make([]byte, 0, 12)
	for value > 0 {
		buf = append([]byte{byte('0' + value%10)}, buf...)
		value /= 10
	}
	if negative {
		return "-" + string(buf)
	}
	return string(buf)
}

func itoa64(value int64) string {
	if value == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for value > 0 {
		buf = append([]byte{byte('0' + value%10)}, buf...)
		value /= 10
	}
	return string(buf)
}
