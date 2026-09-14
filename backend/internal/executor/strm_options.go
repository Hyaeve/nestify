package executor

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Strm 规则的可调性能参数，存放于 rules.option_values_json。
const (
	// strmDefaultAPIIntervalMS 是「API 请求间隔」的缺省值（毫秒）。
	// 与 webdav 客户端内置节流保持一致，这样老规则（没有该字段）行为不变。
	strmDefaultAPIIntervalMS = 250
	// strmMinAPIIntervalMS 是允许配置的最小间隔，避免把网盘后端打到限流。
	strmMinAPIIntervalMS = 100
	// strmDefaultDownloadThreads 是「下载线程数」默认值。
	strmDefaultDownloadThreads = 3
	// strmMaxDownloadThreads 是「下载线程数」上限。
	strmMaxDownloadThreads = 16
)

// strmAPIInterval 解析「API 请求间隔」：未配置或非法时回退到内置默认值。
func strmAPIInterval(optionValues map[string]int) time.Duration {
	milliseconds := optionValues["strm_api_interval_ms"]
	if milliseconds <= 0 {
		milliseconds = strmDefaultAPIIntervalMS
	}
	if milliseconds < strmMinAPIIntervalMS {
		milliseconds = strmMinAPIIntervalMS
	}
	return time.Duration(milliseconds) * time.Millisecond
}

// strmDownloadThreads 解析「下载线程数」：默认 3，范围 1..16。
func strmDownloadThreads(optionValues map[string]int) int {
	threads := optionValues["strm_download_threads"]
	if threads <= 0 {
		threads = strmDefaultDownloadThreads
	}
	if threads > strmMaxDownloadThreads {
		threads = strmMaxDownloadThreads
	}
	return threads
}

// strmMinVideoBytes 解析「最小视频」（单位 MB）：0 表示不限制。
func strmMinVideoBytes(optionValues map[string]int) int64 {
	megabytes := optionValues["strm_min_video_mb"]
	if megabytes <= 0 {
		return 0
	}
	return int64(megabytes) * 1024 * 1024
}

// strmVideoExtensions 是视频类扩展名白名单。
// 「最小视频」只对视频生效，避免把同样命中规则的小体积字幕/图片一并过滤掉。
var strmVideoExtensions = map[string]struct{}{
	".mp4": {}, ".mkv": {}, ".avi": {}, ".mov": {}, ".wmv": {}, ".flv": {},
	".webm": {}, ".m4v": {}, ".ts": {}, ".m2ts": {}, ".rmvb": {}, ".rm": {},
	".3gp": {}, ".iso": {}, ".mpg": {}, ".mpeg": {}, ".mts": {}, ".vob": {},
}

// isStrmVideoFile 判断文件名是否属于视频类媒体文件。
func isStrmVideoFile(name string) bool {
	_, ok := strmVideoExtensions[strings.ToLower(filepath.Ext(name))]
	return ok
}

// shouldSkipByMinVideoSize 判断某个视频是否因为小于「最小视频」阈值而被跳过。
// sizeBytes <= 0（服务端未返回文件大小）时不做判断，避免误杀。
func shouldSkipByMinVideoSize(name string, sizeBytes, minVideoBytes int64) bool {
	if minVideoBytes <= 0 || sizeBytes <= 0 {
		return false
	}
	if !isStrmVideoFile(name) {
		return false
	}
	return sizeBytes < minVideoBytes
}

// describeMinVideoSize 用于日志展示「最小视频」设置。
func describeMinVideoSize(minVideoBytes int64) string {
	if minVideoBytes <= 0 {
		return "不限制"
	}
	return fmt.Sprintf("%d MB", minVideoBytes/(1024*1024))
}
