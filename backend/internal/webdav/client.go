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

// minRequestInterval 是同一个挂载点两次请求之间的最小间隔，
// 避免对网盘后端造成过高的请求频繁度。
const minRequestInterval = 250 * time.Millisecond

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
	}
}

// BuildFileURL 生成 http strm 需要写入的完整访问地址：
// 挂载的 http 根地址 + WebDAV 内部文件路径。
func (c *Client) BuildFileURL(internalPath string) string {
	normalized := normalizeInternalPath(internalPath)
	return c.baseURL + escapePath(joinRemotePath(c.basePath, normalized))
}

// Wait 在必要时阻塞，保证两次请求之间满足最小间隔。
func (c *Client) Wait(ctx context.Context) error {
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	wait := minRequestInterval - elapsed
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
