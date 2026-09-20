package pan115

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/SheltonZhu/115driver/pkg/driver"
	"github.com/go-resty/resty/v2"
)

var ErrNotFound = errors.New("115 路径不存在")

type Device struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

var Devices = []Device{
	{string(driver.LoginAppWeb), "网页端"},
	{string(driver.LoginAppAndroid), "Android"},
	{string(driver.LoginAppIOS), "iOS"},
	{string(driver.LoginAppTV), "电视端"},
	{string(driver.LoginAppAlipayMini), "支付宝小程序"},
	{string(driver.LoginAppWechatMini), "微信小程序"},
	{string(driver.LoginQAppAndroid), "115管理 Android"},
}

func ValidDevice(value string) bool {
	for _, device := range Devices {
		if device.Value == value {
			return true
		}
	}
	return false
}

func ParseCookie(value string) (*driver.Credential, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, errors.New("CK 不能包含换行")
	}
	request := &http.Request{Header: http.Header{"Cookie": []string{strings.TrimSpace(value)}}}
	values := map[string]string{}
	for _, cookie := range request.Cookies() {
		values[strings.ToUpper(cookie.Name)] = cookie.Value
	}
	credential := &driver.Credential{UID: values["UID"], CID: values["CID"], SEID: values["SEID"], KID: values["KID"]}
	if credential.UID == "" || credential.CID == "" || credential.SEID == "" {
		return nil, errors.New("CK 必须包含 UID、CID 和 SEID")
	}
	return credential, nil
}

type requestGate struct {
	token chan struct{}
	last  time.Time
}

func newGate() *requestGate {
	gate := &requestGate{token: make(chan struct{}, 1)}
	gate.token <- struct{}{}
	return gate
}

func (gate *requestGate) wait(ctx context.Context, interval time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-gate.token:
	}
	defer func() { gate.token <- struct{}{} }()
	if delay := time.Until(gate.last.Add(interval)); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	gate.last = time.Now()
	return nil
}

// 固定分片避免长期保留账号/CK；同账号的不同挂载与并发 worker 共用节流状态。
var accountGates = func() [256]*requestGate {
	var gates [256]*requestGate
	for index := range gates {
		gates[index] = newGate()
	}
	return gates
}()

type throttledTransport struct {
	base     http.RoundTripper
	gate     *requestGate
	interval time.Duration
}

func (transport throttledTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := transport.gate.wait(request.Context(), transport.interval); err != nil {
		return nil, err
	}
	return transport.base.RoundTrip(request)
}

func NewDriver(ctx context.Context, cookie, device string, interval time.Duration) (*driver.Pan115Client, error) {
	if !ValidDevice(device) {
		return nil, errors.New("不支持的 115 设备类型")
	}
	var credential *driver.Credential
	key := "qr-login"
	if cookie != "" {
		var err error
		credential, err = ParseCookie(cookie)
		if err != nil {
			return nil, err
		}
		key, _, _ = strings.Cut(credential.UID, "_")
	}
	hash := sha256.Sum256([]byte(key))
	transport := throttledTransport{base: http.DefaultTransport, gate: accountGates[hash[0]], interval: interval}
	userAgent := driver.UADefault
	if device == string(driver.LoginAppIOS) {
		userAgent = driver.UAIosApp
	}
	client := driver.New(driver.WithClient(&http.Client{Transport: transport, Timeout: 30 * time.Second}), driver.UA(userAgent))
	client.Client.SetRetryCount(0)
	client.Client.OnBeforeRequest(func(_ *resty.Client, request *resty.Request) error {
		request.SetContext(ctx)
		return ctx.Err()
	})
	if credential != nil {
		client.ImportCredential(credential)
	}
	return client, nil
}

func APIError(ctx context.Context, operation string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil {
		return nil
	}
	// 上游错误可能包含响应体或登录会话参数，不能直接回传给浏览器或运行日志。
	return fmt.Errorf("115 %s失败，请检查 CK 是否有效、设备类型及请求间隔", operation)
}

type Client struct {
	Cookie   string
	Device   string
	Root     string
	Interval time.Duration
}

func (client *Client) Driver(ctx context.Context) (*driver.Pan115Client, error) {
	if client.Cookie == "" {
		return nil, errors.New("115 CK 尚未设置")
	}
	return NewDriver(ctx, client.Cookie, client.Device, client.Interval)
}

func cleanParts(value string) ([]string, error) {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == ".." || part == "." || strings.Contains(part, "\\") {
			return nil, errors.New("115 路径不允许包含 .、.. 或反斜杠")
		}
		if part != "" {
			result = append(result, part)
		}
	}
	return result, nil
}

func list(ctx context.Context, client *driver.Pan115Client, id string) ([]driver.File, error) {
	var files []driver.File
	const limit = 500
	for offset := int64(0); ; {
		response, err := driver.GetFiles(client.NewRequest().SetContext(ctx).ForceContentType("application/json"),
			id, driver.WithLimit(limit), driver.WithOffset(offset))
		if err != nil {
			return nil, APIError(ctx, "列目录", err)
		}
		if string(response.CategoryID) != id || int64(response.Offset) != offset {
			return nil, errors.New("115 返回的目录或分页位置不匹配")
		}
		for _, info := range response.Files {
			file := (&driver.File{}).From(&info)
			if file.Name == "" || strings.ContainsAny(file.Name, "/\\") || file.Name == "." || file.Name == ".." {
				return nil, errors.New("115 返回了无效的文件名")
			}
			files = append(files, *file)
		}
		offset += int64(len(response.Files))
		if offset >= int64(response.Count) {
			return files, nil
		}
		if len(response.Files) == 0 {
			return nil, errors.New("115 目录分页提前结束")
		}
	}
}

func (client *Client) resolve(ctx context.Context, sdk *driver.Pan115Client, internal string) (driver.File, error) {
	rootParts, err := cleanParts(client.Root)
	if err != nil {
		return driver.File{}, err
	}
	parts, err := cleanParts(internal)
	if err != nil {
		return driver.File{}, err
	}
	current := driver.File{FileID: "0", IsDirectory: true, Name: "/"}
	for _, name := range append(rootParts, parts...) {
		if !current.IsDirectory {
			return driver.File{}, ErrNotFound
		}
		files, err := list(ctx, sdk, current.FileID)
		if err != nil {
			return driver.File{}, err
		}
		matches := 0
		for _, file := range files {
			if file.Name == name {
				current = file
				matches++
			}
		}
		if matches == 0 {
			return driver.File{}, ErrNotFound
		}
		if matches > 1 {
			return driver.File{}, errors.New("115 同目录存在重名条目，无法安全定位路径")
		}
	}
	return current, nil
}

func (client *Client) Stat(ctx context.Context, internal string) (driver.File, error) {
	sdk, err := client.Driver(ctx)
	if err != nil {
		return driver.File{}, err
	}
	file, err := client.resolve(ctx, sdk, internal)
	if err == nil && strings.Trim(internal, "/") == "" && file.FileID == "0" {
		_, err = list(ctx, sdk, "0")
	}
	return file, err
}

func (client *Client) List(ctx context.Context, internal string) ([]driver.File, error) {
	sdk, err := client.Driver(ctx)
	if err != nil {
		return nil, err
	}
	directory, err := client.resolve(ctx, sdk, internal)
	if err != nil {
		return nil, err
	}
	if !directory.IsDirectory {
		return nil, errors.New("115 路径不是目录")
	}
	return list(ctx, sdk, directory.FileID)
}

func (client *Client) Delete(ctx context.Context, internal string, directory bool) error {
	if strings.Trim(internal, "/") == "" {
		return errors.New("禁止删除 115 挂载根目录")
	}
	sdk, err := client.Driver(ctx)
	if err != nil {
		return err
	}
	file, err := client.resolve(ctx, sdk, internal)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if file.IsDirectory != directory {
		return errors.New("115 删除目标类型已改变")
	}
	if directory {
		children, err := list(ctx, sdk, file.FileID)
		if err != nil {
			return err
		}
		if len(children) != 0 {
			return errors.New("115 目录尚未清空，拒绝递归删除")
		}
	}
	return APIError(ctx, "删除", sdk.Delete(file.FileID))
}

func (client *Client) DownloadInfo(ctx context.Context, internal string) (*driver.DownloadInfo, error) {
	sdk, err := client.Driver(ctx)
	if err != nil {
		return nil, err
	}
	file, err := client.resolve(ctx, sdk, internal)
	if err != nil {
		return nil, err
	}
	if file.IsDirectory || file.PickCode == "" {
		return nil, errors.New("115 下载目标不是有效文件")
	}
	info, err := sdk.DownloadWithUA(file.PickCode, driver.UADefault)
	if err != nil {
		return nil, APIError(ctx, "获取下载地址", err)
	}
	if info == nil || !info.Url.Valid || info.Url.Url == "" {
		return nil, errors.New("115 未返回可用下载地址")
	}
	return info, nil
}

func EntryPath(internal, name string) string {
	return path.Join("/", internal, name)
}
