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
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nestify/backend/internal/model"
)

// defaultMinRequestInterval 是同一个挂载点两次请求之间的默认最小间隔，
// 避免对网盘后端造成过高的请求频繁度。
// Strm 规则里的「API 请求间隔」会通过 SetRequestInterval 覆盖它。
const defaultMinRequestInterval = 250 * time.Millisecond

// defaultRequestTimeout 用于列目录等协议请求。
const defaultRequestTimeout = 30 * time.Second

// downloadRequestTimeout 用于元数据文件下载：海报/字幕等可能比协议请求慢得多，
// 沿用协议请求的 30s 会把正常下载误判成失败。
const downloadRequestTimeout = 10 * time.Minute

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
	provider   string
	authType   string
	username   string
	password   string
	token      string
	httpClient *http.Client
	// downloadClient 用于把元数据文件拉到本地，超时比协议请求宽松，
	// 与 httpClient 共用同一个 Transport，从而复用连接池。
	downloadClient *http.Client

	// minInterval 为 0 表示不节流；每个 Client 实例各自维护节流状态。
	minInterval time.Duration

	mu          sync.Mutex
	lastRequest time.Time
}

func NewClient(credential model.MountCredential) *Client {
	mount := credential.Mount
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        8,
		MaxIdleConnsPerHost: 4,
	}
	return &Client{
		baseURL:  strings.TrimRight(model.BuildMountBaseURL(mount), "/"),
		basePath: model.NormalizeMountBasePath(mount.BasePath),
		provider: model.NormalizeMountProvider(mount.Provider),
		authType: model.NormalizeMountAuthType(mount.AuthType),
		username: mount.Username,
		password: credential.Password,
		token:    strings.TrimSpace(credential.Token),
		httpClient: &http.Client{
			Timeout:   defaultRequestTimeout,
			Transport: transport,
		},
		downloadClient: &http.Client{
			Timeout:   downloadRequestTimeout,
			Transport: transport,
		},
		minInterval: defaultMinRequestInterval,
	}
}

// applyAuth 按认证方式给请求挂上凭证。
// token 模式使用 OpenList 的 `Authorization: Bearer <永久令牌>`（见其 WebDAVAuth 中间件，
// 令牌取自 OpenList 后台「设置 → 令牌」）；password 模式回落到 HTTP Basic。
func (c *Client) applyAuth(request *http.Request) {
	if c.usesTokenAuth() {
		request.Header.Set("Authorization", "Bearer "+c.token)
		return
	}
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}
}

func (c *Client) usesTokenAuth() bool {
	return model.NormalizeMountAuthType(c.authType) == model.MountAuthToken
}

// authFailedError 按认证方式给出可操作的错误提示，避免令牌用户被误导去翻密码。
func (c *Client) authFailedError() error {
	if c.usesTokenAuth() {
		return fmt.Errorf("OpenList 令牌认证失败，请核对后台「设置 → 令牌」中的永久令牌")
	}
	return fmt.Errorf("WebDAV 认证失败，请核对挂载的用户名与密码")
}

// Provider 返回挂载类型（webdav / openlist）。
func (c *Client) Provider() string {
	return c.provider
}

// SupportsRecursiveList 判断该挂载是否应走 OpenList 原生递归列举
// （一次 PROPFIND 取回整棵子树，而不是每个目录各来一次）。
func (c *Client) SupportsRecursiveList() bool {
	return c.provider == model.MountProviderOpenList
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
		baseURL:        c.baseURL,
		basePath:       c.basePath,
		provider:       c.provider,
		authType:       c.authType,
		username:       c.username,
		password:       c.password,
		token:          c.token,
		httpClient:     c.httpClient,
		downloadClient: c.downloadClient,
		minInterval:    c.minInterval,
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
	c.applyAuth(request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("webdav put failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return c.authFailedError()
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	return fmt.Errorf("WebDAV PUT 返回状态 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
}

// Download 用 GET 把远端文件内容下载到本地目标路径。
//
// 元数据（海报 / 字幕 / nfo）必须作为实体文件出现在目标目录，
// 因此这里走真正的数据传输（而不是生成 strm 的地址）。
// 先写 `<target>.download` 再改名，避免半截文件被媒体服务器读到；
// 与列目录一样遵守「API 请求间隔」并复用同一份连接池。
func (c *Client) Download(ctx context.Context, internalPath, targetPath string) error {
	if err := c.Wait(ctx); err != nil {
		return err
	}

	requestURL := c.requestURL(internalPath)
	request, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return fmt.Errorf("build get request: %w", err)
	}
	request.Header.Set("Accept", "*/*")
	c.applyAuth(request)

	response, err := c.downloadClient.Do(request)
	if err != nil {
		return fmt.Errorf("webdav get failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized {
		return c.authFailedError()
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("WebDAV GET 返回状态 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create download parent: %w", err)
	}

	tempPath := targetPath + ".download"
	target, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}

	written, copyErr := io.Copy(target, response.Body)
	closeErr := target.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("write download file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close download file: %w", closeErr)
	}
	if response.ContentLength >= 0 && written != response.ContentLength {
		_ = os.Remove(tempPath)
		return fmt.Errorf("下载不完整：期望 %d 字节，实际 %d 字节", response.ContentLength, written)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("move download into place: %w", err)
	}

	return nil
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
	c.applyAuth(request)

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
		return c.authFailedError()
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
	request, err := http.NewRequestWithContext(ctx, "PROPFIND", requestURL, strings.NewReader(propfindBody(true)))
	if err != nil {
		return false, fmt.Errorf("build propfind request: %w", err)
	}
	request.Header.Set("Depth", "0")
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	c.applyAuth(request)

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
		return false, c.authFailedError()
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
	c.applyAuth(request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("webdav delete failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 || response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode == http.StatusUnauthorized {
		return c.authFailedError()
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

// Depth 请求头取值：1 只取直接子项，infinity 递归整棵子树。
const (
	depthOne      = "1"
	depthInfinity = "infinity"
)

// List 列出 internalPath 下的直接子项（PROPFIND Depth: 1）。internalPath 为空表示挂载根目录。
func (c *Client) List(ctx context.Context, internalPath string) ([]Entry, error) {
	entries, err := c.propfind(ctx, internalPath, depthOne, true)
	if err != nil {
		return nil, err
	}

	// 只保留当前目录的直接子项。父目录需归一化后再比较，
	// 否则根目录（""）与 path.Dir 返回的 "/" 永远不相等，导致根目录子项被全部过滤。
	requestDir := normalizeInternalPath(internalPath)
	direct := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if normalizeInternalPath(path.Dir(entry.Path)) != requestDir {
			continue
		}
		direct = append(direct, entry)
	}
	return direct, nil
}

// ListRecursive 用一次请求取回 internalPath 下整棵子树的条目（PROPFIND Depth: infinity）。
//
// 这是 OpenList / Alist 的原生能力：其 handlePropfind 在未指定 Depth 时默认按无限深度走
// walkFS，服务端在同一个响应里递归遍历整棵目录树，于是「每个目录一次请求」被压缩成一次。
// 以两千个目录的媒体库为例，请求数从 2000 次降到 1 次，
// 挂载级节流带来的等待也从「目录数 × 请求间隔」降为一次往返。
//
// withSize 为 false 时不请求 getcontentlength，可省掉服务端为每个文件取大小的开销；
// 只有启用「最小视频」过滤时才真正需要文件大小。
//
// 仅对支持无限深度的服务端（OpenList / Alist / SabreDAV 等）调用；
// 受限实现可能返回 403/400，调用方需回落到逐目录列举。
func (c *Client) ListRecursive(ctx context.Context, internalPath string, withSize bool) ([]Entry, error) {
	return c.propfind(ctx, internalPath, depthInfinity, withSize)
}

// propfind 发起一次 PROPFIND 并流式解析 multistatus。
// 递归响应可能非常大（整棵目录树），所以按 <d:response> 逐个解码，不把整份 XML 读进内存。
func (c *Client) propfind(ctx context.Context, internalPath, depth string, withSize bool) ([]Entry, error) {
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

	request, err := http.NewRequestWithContext(ctx, "PROPFIND", requestURL, strings.NewReader(propfindBody(withSize)))
	if err != nil {
		return nil, fmt.Errorf("build propfind request: %w", err)
	}
	request.Header.Set("Depth", depth)
	request.Header.Set("Content-Type", "application/xml; charset=utf-8")
	request.Header.Set("Accept", "application/xml")
	c.applyAuth(request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("webdav request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized {
		return nil, c.authFailedError()
	}
	if response.StatusCode != http.StatusMultiStatus && response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return nil, fmt.Errorf("WebDAV 返回状态 %d：%s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	basePrefix := normalizeInternalPath(c.basePath)
	// 请求目录在「内部路径」坐标系下的值（即去掉 base_path 前缀后的相对挂载根路径）。
	// 用于与响应 href 对齐并过滤自身。
	requestDir := normalizeInternalPath(internalPath)

	entries := make([]Entry, 0, 64)
	decoder := xml.NewDecoder(response.Body)
	for {
		token, tokenErr := decoder.Token()
		if tokenErr == io.EOF {
			break
		}
		if tokenErr != nil {
			return nil, fmt.Errorf("解析 WebDAV 响应失败: %w", tokenErr)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "response" {
			continue
		}
		var item propfindResponse
		if err := decoder.DecodeElement(&item, &start); err != nil {
			return nil, fmt.Errorf("解析 WebDAV 响应失败: %w", err)
		}
		entry, ok := decodePropfindEntry(item, basePrefix, requestDir)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// decodePropfindEntry 把一条 <d:response> 转成挂载内的相对条目（ok=false 表示该条应丢弃）。
func decodePropfindEntry(item propfindResponse, basePrefix, requestDir string) (Entry, bool) {
	decoded, err := decodeHref(item.Href)
	if err != nil {
		return Entry{}, false
	}

	trimmed := strings.TrimRight(decoded, "/")
	if trimmed == "" {
		return Entry{}, false
	}

	relative := trimmed
	if basePrefix != "" && strings.HasPrefix(trimmed, basePrefix) {
		relative = strings.TrimPrefix(trimmed, basePrefix)
	}
	relative = normalizeInternalPath(relative)

	// 过滤掉请求目录自身（其 href 通常与请求路径一致）。
	if relative == "" || relative == requestDir {
		return Entry{}, false
	}

	name := path.Base(relative)
	if name == "" || name == "." || name == "/" {
		return Entry{}, false
	}

	// 目录判定优先看 resourcetype/collection；部分实现会省略 collection，
	// 此时回退到 href 的尾斜杠（OpenList 的 PROPFIND 会给目录 href 补 "/"）。
	// 少了这条回退，递归结果里的目录会被整体误判成文件。
	isDir := item.Propstat.Prop.ResourceType.Collection != nil || strings.HasSuffix(decoded, "/")

	entry := Entry{
		Name:       name,
		Path:       relative,
		IsDir:      isDir,
		Size:       item.Propstat.Prop.ContentLength,
		ModifiedAt: item.Propstat.Prop.LastModified.Time,
	}
	if entry.IsDir {
		entry.Size = 0
		entry.Name = strings.TrimRight(name, "/")
	}
	if entry.Name == "" {
		return Entry{}, false
	}

	return entry, true
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

// propfindBody 生成 PROPFIND 请求体。
// withSize 为 false 时不索取 getcontentlength：Strm 生成只需要路径与目录标记，
// 只有「最小视频」过滤才用到文件大小，少要一个属性即可让部分服务端省掉
// 为每个文件取大小的额外上游请求。
func propfindBody(withSize bool) string {
	sizeProp := ""
	if withSize {
		sizeProp = "\n    <d:getcontentlength/>"
	}
	return `<?xml version="1.0" encoding="utf-8"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:resourcetype/>` + sizeProp + `
    <d:getlastmodified/>
  </d:prop>
</d:propfind>`
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
