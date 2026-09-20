package webdav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nestify/backend/internal/pan115"
)

func (c *Client) list115(ctx context.Context, internalPath string) ([]Entry, error) {
	files, err := c.pan115.List(ctx, internalPath)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(files))
	for _, file := range files {
		entries = append(entries, Entry{Name: file.Name, Path: pan115.EntryPath(internalPath, file.Name),
			IsDir: file.IsDirectory, Size: file.Size, ModifiedAt: file.UpdateTime})
	}
	return entries, nil
}

func (c *Client) download115(ctx context.Context, internalPath, targetPath string) error {
	info, err := c.pan115.DownloadInfo(ctx, internalPath)
	if err != nil {
		return err
	}
	location, err := url.Parse(info.Url.Url)
	if err != nil || location.Scheme != "https" || location.Host == "" {
		return fmt.Errorf("115 返回了无效的下载地址")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, location.String(), nil)
	if err != nil {
		return err
	}
	request.Header = info.Header.Clone()
	request.Header.Del("Authorization")
	cookies := request.Cookies()
	request.Header.Del("Cookie")
	for _, cookie := range cookies {
		switch strings.ToUpper(cookie.Name) {
		case "UID", "CID", "SEID", "KID":
			continue
		}
		request.AddCookie(cookie)
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return pan115.APIError(ctx, "下载", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("115 下载失败（HTTP %d）", response.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(targetPath), ".nestify-115-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	written, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != int64(info.FileSize) {
		return fmt.Errorf("115 下载不完整：预期 %d 字节，实际 %d", info.FileSize, written)
	}
	return os.Rename(file.Name(), targetPath)
}
