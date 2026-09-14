// Package webdav 提供 Nestify 所需的极简 WebDAV 客户端能力：
// 只做「列目录」与「拼接访问地址」两件事，并对请求做节流，
// 以适配网盘（如 OpenList / Alist）对请求频率较敏感的特性。
package webdav

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"nestify/backend/internal/model"
)

// defaultMinRequestInterval 是同一个挂载点两次请求之间的默认最小间隔，
// 避免对网盘后端造成过高的请求频繁度。
// Strm 规则里的「API 请求间隔」会通过 SetRequestInterval 覆盖它。
const defaultMinRequestInterval = 250 * time.Millisecond

type Entry struct {
	Name       string
	Path       string // 相对于挂载根的内部路径，始终以 / 开头
	IsDir      bool
	Size       int64
	ModifiedAt time.Time
}

type Client struct {
	baseURL    string
	basePath   string
	username   string
	password   string
	httpClient *http.Client

	// minInterval 为 0 表示不节流；每个 Client 实例各自维护节流状态。
	minInterval time.Duration

	mu          sync.Mutex
	lastRequest time.Time
}

func NewClient(mount model.WebdavMount, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(model.BuildMountBaseURL(mount), "/"),
		basePath: model.NormalizeMountBasePath(mount.BasePath),
		username: mount.Username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConns:        4,
				MaxIdleConnsPerHost: 2,
			},
		},
		minInterval: defaultMinRequestInterval,
	}
}

// SetRequestInterval 覆盖两次请求之间的最小间隔（<=0 表示不节流）。
func (c *Client) SetRequestInterval(interval time.Duration) {
	if interval < 0 {
		interval = 0
	}
	c.minInterval = interval
}

// RequestInterval 返回当前的请求最小间隔。
func (c *Client) RequestInterval() time.Duration {
	return c.minInterval
}

// Fork 返回一个共享同一份 http 连接池、但拥有独立节流状态的副本。
// 并发工作线程各持有一份，可让「API 请求间隔」按线程生效，而不是全局串行等待。
// 这里显式逐字段构造，避免复制内含 sync.Mutex 的结构体。
func (c *Client) Fork() *Client {
	return &Client{
		baseURL:     c.baseURL,
		basePath:    c.basePath,
		username:    c.username,
		password:    c.password,
		httpClient:  c.httpClient,
		minInterval: c.minInterval,
	}
}

// davEndpointName 是 OpenList / Alist 默认的 WebDAV 协议端点末段名称（/dav）。
// directLinkName 是同一服务直链（可直接播放 / 下载）端点的末段名称（/d）。
const (
	davEndpointName    = "dav"
	directLinkName     = "d"
	davEndpointSegment = "/dav"
	directLinkSegment  = "/d"
)

// StrmBaseURL 返回生成 strm 时使用的直链根地址，例如 http://10.0.0.31:5244/d。
// 运行日志会打印它，便于直接核对写入 strm 的地址前缀。
func (c *Client) StrmBaseURL() string {
	return c.directLinkBaseURL() + escapePath(directLinkBasePath(c.basePath))
}

// BuildStrmURL 生成 http strm 需要写入的完整访问地址：直链根地址 + WebDAV 内部文件路径。
//
// strm 会被交给媒体服务器直接播放，必须使用直链端点（OpenList / Alist 的 /d）；
// WebDAV 端点（/dav）需要 Basic 认证且不是媒体直链，写进去会导致播放失败。
// 因此这里把各种形态的 WebDAV 端点统统改写为直链端点：
//
//	/dav、/DAV、/dav/   -> /d
//	/dav/xxx           -> /d/xxx
//	/xxx/dav           -> /xxx/d
//
// 协议请求（列目录 / 上传 / 建目录 / 删除）依旧按挂载的 base_path 原样发送，两者互不影响。
func (c *Client) BuildStrmURL(internalPath string) string {
	normalized := normalizeInternalPath(internalPath)
	return c.StrmBaseURL() + escapePath(joinRemotePath("", normalized))
}

// directLinkBaseURL 去掉根地址尾部可能残留的 WebDAV 端点半段。
// 少数用户会把端点连同路径一起填进「域名或 IP」栏（例如 10.0.0.31/dav），
// 这里统一剔除，避免生成 /dav/d 这种畸形直链。
func (c *Client) directLinkBaseURL() string {
	root := strings.TrimRight(strings.TrimSpace(c.baseURL), "/")
	if hasEndpointSuffix(root, davEndpointName) {
		return root[:len(root)-len(davEndpointSegment)]
	}
	return root
}

// directLinkBasePath 把 WebDAV 挂载的「指定路径」映射为直链根路径：
// 留空或 /dav -> /d、/dav/xxx -> /d/xxx、/xxx/dav -> /xxx/d、/d 保持不动，
// 其它自定义路径 -> <base>/d。端点段名大小写不敏感。
func directLinkBasePath(basePath string) string {
	base := model.NormalizeMountBasePath(basePath)
	if base == "" {
		return directLinkSegment
	}

	segments := strings.Split(strings.Trim(base, "/"), "/")
	for index, segment := range segments {
		if !strings.EqualFold(segment, davEndpointName) {
			continue
		}
		// WebDAV 端点只可能出现在路径开头（/dav/xxx）或结尾（/xxx/dav）。
		if index == 0 || index == len(segments)-1 {
			segments[index] = directLinkName
			return "/" + strings.Join(segments, "/")
		}
	}

	if strings.EqualFold(segments[0], directLinkName) {
		return "/" + strings.Join(segments, "/")
	}
	return base + directLinkSegment
}

// hasEndpointSuffix 判断 value 是否以 "/<segment>" 结尾（大小写不敏感）。
func hasEndpointSuffix(value, segment string) bool {
	suffix := "/" + segment
	if len(value) <= len(suffix) {
		return false
	}
	return strings.EqualFold(value[len(value)-len(suffix):], suffix)
}

// requestURL 拼接某内部路径对应的完整请求地址（不含尾斜杠处理）。
func (c *Client) requestURL(internalPath string) string {
	normalized := normalizeInternalPath(internalPath)
	return c.baseURL + escapePath(joinRemotePath(c.basePath, normalized))
}

// PutFile 通过 PUT 上传文件内容到 WebDAV 内部路径（自动创建父目录可选项由上层处理）。
func (c *Client) PutFile(ctx context.Context, internalPath string, content io.Reader, size int64) error {
	if err := c.Wait(ctx); err != nil {
		return err
	}

	requestURL := c.requestURL(internalPath)
	request, err := http.NewRequestWithContext(ctx, "PUT", requestURL, content)
	if err != nil {
		return fmt.Errorf("build put request: %w", err)
	}
	if size >= 0 {
		request.ContentLength = size
	}
	request.Header.Set("Accept", "*/*")
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("webdav put failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("WebDAV 认证失败，请检查用户名与密码")
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	return fmt.Errorf("WebDAV PUT 返回状态 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
}

// MkdirAll 在 WebDAV 上逐级创建目录（MKCOL）。已存在则忽略。
func (c *Client) MkdirAll(ctx context.Context, internalPath string) error {
	normalized := normalizeInternalPath(internalPath)
	if normalized == "" {
		return nil
	}

	segments := strings.Split(strings.Trim(normalized, "/"), "/")
	current := ""
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if current == "" {
			current = segment
		} else {
			current = current + "/" + segment
		}
		if err := c.mkcol(ctx, current); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) mkcol(ctx context.Context, internalPath string) error {
	if err := c.Wait(ctx); err != nil {
		return err
	}

	requestURL := c.requestURL(internalPath)
	request, err := http.NewRequestWithContext(ctx, "MKCOL", requestURL, nil)
	if err != nil {
		return fmt.Errorf("build mkcol request: %w", err)
	}
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("webdav mkcol failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	// 405 Method Not Allowed 通常表示目录已存在（部分实现如此）。
	if response.StatusCode == http.StatusMethodNotAllowed {
		return nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("WebDAV 认证失败，请检查用户名与密码")
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	return fmt.Errorf("WebDAV MKCOL 返回状态 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
}

// Exists 判断某内部路径是否存在（文件或目录）。
func (c *Client) Exists(ctx context.Context, internalPath string) (bool, error) {
	if err := c.Wait(ctx); err != nil {
		return false, err
	}

	requestURL := c.requestURL(internalPath)
	request, err := http.NewRequestWithContext(ctx, "PROPFIND", requestURL, strings.NewReader(propfindBody))
	if err != nil {
		return false, fmt.Errorf("build propfind request: %w", err)
	}
	request.Header.Set("Depth", "0")
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("webdav propfind failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return true, nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return false, fmt.Errorf("WebDAV 认证失败，请检查用户名与密码")
	}
	return false, fmt.Errorf("WebDAV PROPFIND 返回状态 %d", response.StatusCode)
}

// Delete 删除某个内部路径（文件或目录，目录用 DELETE 可能递归取决于服务端实现）。
func (c *Client) Delete(ctx context.Context, internalPath string) error {
	if err := c.Wait(ctx); err != nil {
		return err
	}

	requestURL := c.requestURL(internalPath)
	request, err := http.NewRequestWithContext(ctx, "DELETE", requestURL, nil)
	if err != nil {
		return fmt.Errorf("build delete request: %w", err)
	}
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("webdav delete failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 || response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("WebDAV 认证失败，请检查用户名与密码")
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	return fmt.Errorf("WebDAV DELETE 返回状态 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
}

// InternalPath 返回挂载下某相对路径（以 / 开头的内部路径）对应的内部路径字符串。
func InternalPathFromParts(parts ...string) string {
	joined := path.Join(parts...)
	return normalizeInternalPath(joined)
}

// Wait 在必要时阻塞，保证两次请求之间满足最小间隔。
func (c *Client) Wait(ctx context.Context) error {
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	wait := c.minInterval - elapsed
	if wait < 0 {
		wait = 0
	}
	if wait > 0 {
		c.lastRequest = time.Now().Add(wait)
	} else {
		c.lastRequest = time.Now()
	}
	c.mu.Unlock()

	if wait <= 0 {
		return nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// List 列出 internalPath 下的直接子项。internalPath 为空表示挂载根目录。
func (c *Client) List(ctx context.Context, internalPath string) ([]Entry, error) {
	if err := c.Wait(ctx); err != nil {
		return nil, err
	}

	remotePath := joinRemotePath(c.basePath, normalizeInternalPath(internalPath))
	// 目录级 PROPFIND 必须带尾斜杠，否则 OpenList / Alist 等会返回 301 重定向，
	// 而 Go http.Client 对 301/302 会把 PROPFIND 降级为 GET，导致拿不到 multistatus、目录显示为空。
	if !strings.HasSuffix(remotePath, "/") {
		remotePath += "/"
	}
	requestURL := c.baseURL + escapePath(remotePath)

	request, err := http.NewRequestWithContext(ctx, "PROPFIND", requestURL, strings.NewReader(propfindBody))
	if err != nil {
		return nil, fmt.Errorf("build propfind request: %w", err)
	}
	request.Header.Set("Depth", "1")
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	request.Header.Set("Accept", "application/xml")
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("webdav request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("WebDAV 认证失败，请检查用户名与密码")
	}
	if response.StatusCode != http.StatusMultiStatus && response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return nil, fmt.Errorf("WebDAV 返回状态 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var multistatus multiStatus
	if err := xml.NewDecoder(response.Body).Decode(&multistatus); err != nil {
		return nil, fmt.Errorf("解析 WebDAV 响应失败: %w", err)
	}

	basePrefix := normalizeInternalPath(c.basePath)
	// 请求目录在「内部路径」坐标系下的值（即去掉 base_path 前缀后的相对挂载根路径）。
	// 用于与响应 href 对齐并过滤自身及非直接子项。
	requestDir := normalizeInternalPath(internalPath)
	entries := make([]Entry, 0, len(multistatus.Responses))
	for _, item := range multistatus.Responses {
		decoded, decodeErr := decodeHref(item.Href)
		if decodeErr != nil {
			continue
		}

		trimmed := strings.TrimRight(decoded, "/")
		if trimmed == "" {
			continue
		}

		relative := trimmed
		if basePrefix != "" && strings.HasPrefix(trimmed, basePrefix) {
			relative = strings.TrimPrefix(trimmed, basePrefix)
		}
		relative = normalizeInternalPath(relative)

		// 过滤掉请求目录本身（其 href 通常与请求路径一致）。
		if relative == "" || relative == requestDir {
			continue
		}
		// 只保留当前目录的直接子项。父目录需归一化后再比较，
		// 否则根目录（""）与 path.Dir 返回的 "/" 永远不相等，导致根目录子项被全部过滤。
		if normalizeInternalPath(path.Dir(relative)) != requestDir {
			continue
		}

		name := path.Base(relative)
		if name == "" || name == "." || name == "/" {
			continue
		}

		entry := Entry{
			Name:       name,
			Path:       relative,
			IsDir:      item.Propstat.Prop.ResourceType.Collection != nil,
			Size:       item.Propstat.Prop.ContentLength,
			ModifiedAt: item.Propstat.Prop.LastModified.Time,
		}
		if entry.IsDir {
			entry.Size = 0
			entry.Name = strings.TrimRight(name, "/")
		}
		if entry.Name == "" {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func joinRemotePath(basePath, internalPath string) string {
	base := strings.TrimRight(basePath, "/")
	rest := strings.TrimLeft(internalPath, "/")
	if base == "" && rest == "" {
		return "/"
	}
	if base == "" {
		return "/" + rest
	}
	if rest == "" {
		return base
	}
	return base + "/" + rest
}

func normalizeInternalPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "/" {
		return ""
	}
	cleaned := path.Clean("/" + strings.TrimLeft(trimmed, "/"))
	if cleaned == "/" {
		return ""
	}
	return cleaned
}

func decodeHref(href string) (string, error) {
	trimmed := strings.TrimSpace(href)
	if trimmed == "" {
		return "", fmt.Errorf("empty href")
	}

	rawPath := trimmed
	parsed, err := url.Parse(trimmed)
	if err == nil {
		if parsed.Path != "" {
			rawPath = parsed.Path
		}
	}

	// 部分实现返回的是相对路径（不带前导 /），统一补上以便后续处理。
	if !strings.HasPrefix(rawPath, "/") {
		rawPath = "/" + rawPath
	}

	decoded, err := url.PathUnescape(rawPath)
	if err != nil {
		return rawPath, nil
	}
	return decoded, nil
}

// escapePath 只转义会破坏 URL 语义的字符，保留中文与空格，
// 使生成的 strm 内容与网盘里看到的路径一致。
func escapePath(value string) string {
	var builder strings.Builder
	for _, char := range value {
		switch char {
		case '%', '#', '?', '\\', '\n', '\r', '\t':
			builder.WriteString(fmt.Sprintf("%%%02X", char))
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

const propfindBody = `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:resourcetype/>
    <d:getcontentlength/>
    <d:getlastmodified/>
  </d:prop>
</d:propfind>`

type multiStatus struct {
	Responses []propfindResponse `xml:"response"`
}

type propfindResponse struct {
	Href     string        `xml:"href"`
	Propstat propfindStats `xml:"propstat"`
}

type propfindStats struct {
	Prop propfindProp `xml:"prop"`
}

type propfindProp struct {
	ResourceType  resourceType `xml:"resourcetype"`
	ContentLength int64        `xml:"getcontentlength"`
	LastModified  dateTime     `xml:"getlastmodified"`
}

type resourceType struct {
	Collection *struct{} `xml:"collection"`
}

type dateTime struct {
	time.Time
}

func (d *dateTime) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var raw string
	if err := decoder.DecodeElement(&raw, &start); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	layouts := []string{
		http.TimeFormat,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		"Mon, 02 Jan 2006 15:04:05 GMT",
		"2006-01-02T15:04:05Z",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			d.Time = parsed
			return nil
		}
	}
	return nil
}
