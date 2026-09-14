package httpapi

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var processStartedAt = time.Now()

// userHZ 是 Linux 用户态时钟频率（CLK_TCK），/proc/self/stat 的 utime/stime 以其为单位。
// Go 标准库无法在无 cgo 场景下取 sysconf(_SC_CLK_TCK)，Linux 上该值固定为 100。
const userHZ = 100

// staleSampleWindow：两次采样间隔超过该阈值时（例如面板长时间未打开），
// 之前的基准已经失效，重新建立基准并沿用上次的值，避免算出「几分钟平均值」这种失真数据。
const staleSampleWindow = 30 * time.Second

type systemResourceSnapshot struct {
	// 整机资源（设置页等场景仍可使用）
	CPUUsagePercent    float64 `json:"cpu_usage"`
	CPUModel           string  `json:"cpu_model"`
	MemoryUsagePercent float64 `json:"memory_usage"`
	MemoryUsed         string  `json:"memory_used"`
	MemoryTotal        string  `json:"memory_total"`
	Uptime             string  `json:"uptime"`

	// 本项目（Nestify 进程）占用
	NestifyCPUPercent    float64 `json:"nestify_cpu_percent"`
	NestifyCPUCount      float64 `json:"nestify_cpu_count"`
	NestifyMemory        string  `json:"nestify_memory"`
	NestifyMemoryBytes   uint64  `json:"nestify_memory_bytes"`
	NestifyMemoryPercent float64 `json:"nestify_memory_percent"`
	NestifyMemoryLimit   string  `json:"nestify_memory_limit"`
	NestifyUploadSpeed   string  `json:"nestify_upload_speed"`
	NestifyDownloadSpeed string  `json:"nestify_download_speed"`
	NestifyUploadBps     float64 `json:"nestify_upload_bps"`
	NestifyDownloadBps   float64 `json:"nestify_download_bps"`
}

type cpuSampler struct {
	mu        sync.Mutex
	lastIdle  uint64
	lastTotal uint64
	lastValue float64
	ready     bool
}

var defaultCPUSampler cpuSampler

func collectSystemResourceSnapshot() systemResourceSnapshot {
	snapshot := systemResourceSnapshot{
		CPUUsagePercent: defaultCPUSampler.usagePercent(),
		CPUModel:        readCPUModel(),
		Uptime:          formatUptime(time.Since(processStartedAt)),
	}

	used, total, percent := readMemoryUsage()
	snapshot.MemoryUsed = formatBytesIEC(used)
	snapshot.MemoryTotal = formatBytesIEC(total)
	snapshot.MemoryUsagePercent = percent

	// —— 本项目（Nestify 进程）占用 ——
	snapshot.NestifyCPUPercent = defaultProcessCPUSampler.usagePercent()
	snapshot.NestifyCPUCount = processCPUCapacity()

	processMemory := readProcessMemoryBytes()
	memoryLimit := total
	if limit, ok := readCgroupMemoryLimitBytes(); ok && limit > 0 {
		memoryLimit = limit
	}
	snapshot.NestifyMemoryBytes = processMemory
	snapshot.NestifyMemory = formatBytesIEC(processMemory)
	snapshot.NestifyMemoryLimit = formatBytesIEC(memoryLimit)
	if memoryLimit > 0 {
		snapshot.NestifyMemoryPercent = clampPercent(float64(processMemory) / float64(memoryLimit) * 100)
	}

	upload, download := defaultNetSampler.rates()
	snapshot.NestifyUploadBps = upload
	snapshot.NestifyDownloadBps = download
	snapshot.NestifyUploadSpeed = formatSpeed(upload)
	snapshot.NestifyDownloadSpeed = formatSpeed(download)

	return snapshot
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func (s *cpuSampler) usagePercent() float64 {
	idle, total, ok := readCPUTimes()
	if !ok {
		return s.lastValue
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		s.lastIdle = idle
		s.lastTotal = total
		s.ready = true
		return s.lastValue
	}

	idleDelta := idle - s.lastIdle
	totalDelta := total - s.lastTotal
	s.lastIdle = idle
	s.lastTotal = total
	if totalDelta == 0 {
		return s.lastValue
	}

	s.lastValue = clampPercent((1 - float64(idleDelta)/float64(totalDelta)) * 100)
	return s.lastValue
}

// —————————————————————— 本项目进程占用 ——————————————————————

type processCPUSampler struct {
	mu        sync.Mutex
	lastCPU   float64
	lastAt    time.Time
	lastValue float64
	ready     bool
}

var defaultProcessCPUSampler processCPUSampler

// usagePercent 返回本项目进程的 CPU 使用率（相对整机可用算力，0~100）。
func (s *processCPUSampler) usagePercent() float64 {
	cpuSeconds, ok := readProcessCPUSeconds()
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !ok {
		return s.lastValue
	}

	elapsed := now.Sub(s.lastAt).Seconds()
	if !s.ready || elapsed <= 0 || elapsed > staleSampleWindow.Seconds() {
		s.ready = true
		s.lastCPU = cpuSeconds
		s.lastAt = now
		return s.lastValue
	}

	delta := cpuSeconds - s.lastCPU
	s.lastCPU = cpuSeconds
	s.lastAt = now
	if delta < 0 {
		return s.lastValue
	}

	// delta/elapsed 是「单核占满 = 100%」口径，除以可用核数换算成整机占比。
	capacity := processCPUCapacity()
	if capacity < 1 {
		capacity = 1
	}
	s.lastValue = clampPercent(delta / elapsed / capacity * 100)
	return s.lastValue
}

// readProcessCPUSeconds 读取本进程累计消耗的 CPU 时间（秒）。
// 优先 cgroup（容器下最精确），其次 /proc/self/stat。
func readProcessCPUSeconds() (float64, bool) {
	if content, err := os.ReadFile("/sys/fs/cgroup/cpu.stat"); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "usage_usec" {
				if usec, parseErr := strconv.ParseFloat(fields[1], 64); parseErr == nil {
					return usec / 1e6, true
				}
			}
		}
	}

	// cgroup v1
	if content, err := os.ReadFile("/sys/fs/cgroup/cpuacct/cpuacct.usage"); err == nil {
		if nanos, parseErr := strconv.ParseFloat(strings.TrimSpace(string(content)), 64); parseErr == nil {
			return nanos / 1e9, true
		}
	}

	content, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}

	// comm 字段可能含空格或右括号，从最后一个 ')' 之后开始切分。
	text := string(content)
	end := strings.LastIndex(text, ")")
	if end < 0 {
		return 0, false
	}

	fields := strings.Fields(text[end+1:])
	// 去掉 pid/comm 后从 stat 的第 3 个字段开始：utime(14) → 索引 11，stime(15) → 索引 12。
	if len(fields) < 13 {
		return 0, false
	}

	utime, errUtime := strconv.ParseFloat(fields[11], 64)
	stime, errStime := strconv.ParseFloat(fields[12], 64)
	if errUtime != nil || errStime != nil {
		return 0, false
	}

	return (utime + stime) / userHZ, true
}

// processCPUCapacity 返回本项目可用的 CPU 核数（优先容器配额，其次整机核数）。
func processCPUCapacity() float64 {
	// cgroup v2：cpu.max 形如 "200000 100000"（quota period）或 "max 100000"。
	if content, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
		fields := strings.Fields(string(content))
		if len(fields) >= 2 && fields[0] != "max" {
			quota, errQuota := strconv.ParseFloat(fields[0], 64)
			period, errPeriod := strconv.ParseFloat(fields[1], 64)
			if errQuota == nil && errPeriod == nil && quota > 0 && period > 0 {
				return quota / period
			}
		}
	}

	// cgroup v1：cpu.cfs_quota_us / cpu.cfs_period_us
	quotaContent, errQuota := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	periodContent, errPeriod := os.ReadFile("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if errQuota == nil && errPeriod == nil {
		quota, parseQuotaErr := strconv.ParseFloat(strings.TrimSpace(string(quotaContent)), 64)
		period, parsePeriodErr := strconv.ParseFloat(strings.TrimSpace(string(periodContent)), 64)
		if parseQuotaErr == nil && parsePeriodErr == nil && quota > 0 && period > 0 {
			return quota / period
		}
	}

	count := runtime.NumCPU()
	if count < 1 {
		count = 1
	}
	return float64(count)
}

// readProcessMemoryBytes 读取本进程常驻内存（RSS）。
func readProcessMemoryBytes() uint64 {
	if content, err := os.ReadFile("/proc/self/statm"); err == nil {
		fields := strings.Fields(string(content))
		// statm：size resident shared ...（单位：页）
		if len(fields) >= 2 {
			if pages, parseErr := strconv.ParseUint(fields[1], 10, 64); parseErr == nil && pages > 0 {
				return pages * uint64(os.Getpagesize())
			}
		}
	}

	// cgroup v2 兜底（含页缓存，仅在前者不可用时使用）
	if content, err := os.ReadFile("/sys/fs/cgroup/memory.current"); err == nil {
		if value, parseErr := strconv.ParseUint(strings.TrimSpace(string(content)), 10, 64); parseErr == nil && value > 0 {
			return value
		}
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return mem.Sys
}

// readCgroupMemoryLimitBytes 读取容器内存上限（无限制时返回 false）。
func readCgroupMemoryLimitBytes() (uint64, bool) {
	if content, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		text := strings.TrimSpace(string(content))
		if text != "" && text != "max" {
			if value, parseErr := strconv.ParseUint(text, 10, 64); parseErr == nil && value > 0 {
				return value, true
			}
		}
	}

	// cgroup v1；未限制时该文件是一个接近 2^63 的极大值，需要过滤掉。
	if content, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		if value, parseErr := strconv.ParseUint(strings.TrimSpace(string(content)), 10, 64); parseErr == nil {
			if value > 0 && value < 1<<60 {
				return value, true
			}
		}
	}

	return 0, false
}

type netThroughputSampler struct {
	mu         sync.Mutex
	lastRx     uint64
	lastTx     uint64
	lastAt     time.Time
	lastRxRate float64
	lastTxRate float64
	ready      bool
}

var defaultNetSampler netThroughputSampler

// rates 返回本项目的上传/下载速率（Byte/s）：上传取发送方向（tx），下载取接收方向（rx）。
// 注意：/proc/self/net/dev 是网络命名空间口径，容器内即等价于本项目自身的收发流量。
func (s *netThroughputSampler) rates() (float64, float64) {
	rx, tx, ok := readNetDevCounters()
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !ok {
		return s.lastTxRate, s.lastRxRate
	}

	elapsed := now.Sub(s.lastAt).Seconds()
	if !s.ready || elapsed <= 0 || elapsed > staleSampleWindow.Seconds() {
		s.ready = true
		s.lastRx = rx
		s.lastTx = tx
		s.lastAt = now
		return s.lastTxRate, s.lastRxRate
	}

	rxDelta := rx - s.lastRx
	txDelta := tx - s.lastTx
	s.lastRx = rx
	s.lastTx = tx
	s.lastAt = now

	// 计数器回绕或重置时不显示负值。
	if rxDelta <= uint64(1<<63) {
		s.lastRxRate = float64(rxDelta) / elapsed
	}
	if txDelta <= uint64(1<<63) {
		s.lastTxRate = float64(txDelta) / elapsed
	}

	return s.lastTxRate, s.lastRxRate
}

// readNetDevCounters 累计各网卡（跳过 lo）接收/发送字节数。
func readNetDevCounters() (uint64, uint64, bool) {
	content, err := os.ReadFile("/proc/self/net/dev")
	if err != nil {
		content, err = os.ReadFile("/proc/net/dev")
	}
	if err != nil {
		return 0, 0, false
	}

	return parseNetDevCounters(string(content))
}

// parseNetDevCounters 解析 /proc/net/dev 内容，跳过回环网卡与表头，
// 返回 (接收字节数, 发送字节数, 是否解析到有效网卡)。
func parseNetDevCounters(content string) (uint64, uint64, bool) {
	var rxTotal, txTotal uint64
	found := false

	for _, line := range strings.Split(content, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if name == "" || name == "lo" || strings.HasPrefix(name, "Inter-") || strings.HasPrefix(name, "face") {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}

		rx, errRx := strconv.ParseUint(fields[0], 10, 64)
		tx, errTx := strconv.ParseUint(fields[8], 10, 64)
		if errRx != nil || errTx != nil {
			continue
		}

		rxTotal += rx
		txTotal += tx
		found = true
	}

	return rxTotal, txTotal, found
}

func readCPUTimes() (uint64, uint64, bool) {
	content, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}

	for _, line := range strings.Split(string(content), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0, false
		}

		var total uint64
		for _, field := range fields[1:] {
			value, parseErr := strconv.ParseUint(field, 10, 64)
			if parseErr != nil {
				return 0, 0, false
			}
			total += value
		}

		idle, errIdle := strconv.ParseUint(fields[4], 10, 64)
		if errIdle != nil {
			return 0, 0, false
		}

		if len(fields) > 5 {
			ioWait, errWait := strconv.ParseUint(fields[5], 10, 64)
			if errWait == nil {
				idle += ioWait
			}
		}

		return idle, total, true
	}

	return 0, 0, false
}

func readCPUModel() string {
	content, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return runtime.GOARCH
	}

	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.ToLower(line), "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}

	return runtime.GOARCH
}

func readMemoryUsage() (uint64, uint64, float64) {
	content, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, 0
	}

	values := map[string]uint64{}
	for _, line := range strings.Split(string(content), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(fields) == 0 {
			continue
		}

		value, parseErr := strconv.ParseUint(fields[0], 10, 64)
		if parseErr != nil {
			continue
		}

		values[parts[0]] = value * 1024
	}

	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 {
		return 0, 0, 0
	}

	used := total - available
	percent := (float64(used) / float64(total)) * 100
	return used, total, percent
}

func formatBytesIEC(value uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	result := float64(value)
	unitIndex := 0
	for result >= 1024 && unitIndex < len(units)-1 {
		result /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", value, units[unitIndex])
	}

	return fmt.Sprintf("%.1f %s", result, units[unitIndex])
}

// formatSpeed 把字节速率格式化成可读文案。
func formatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond < 1 {
		return "0 B/s"
	}
	return formatBytesIEC(uint64(bytesPerSecond)) + "/s"
}

func formatUptime(duration time.Duration) string {
	if duration < time.Minute {
		seconds := int(duration.Seconds())
		return fmt.Sprintf("%d秒", seconds)
	}

	days := int(duration.Hours()) / 24
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天%d时%d分", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d时%d分", hours, minutes)
	}
	return fmt.Sprintf("%d分", minutes)
}
