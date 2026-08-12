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

type systemResourceSnapshot struct {
	CPUUsagePercent    float64 `json:"cpu_usage"`
	CPUModel           string  `json:"cpu_model"`
	MemoryUsagePercent float64 `json:"memory_usage"`
	MemoryUsed         string  `json:"memory_used"`
	MemoryTotal        string  `json:"memory_total"`
	NestifyMemory      string  `json:"nestify_memory"`
	Uptime             string  `json:"uptime"`
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
		CPUUsagePercent:    defaultCPUSampler.usagePercent(),
		CPUModel:           readCPUModel(),
		MemoryUsagePercent: 0,
		MemoryUsed:         "0 B",
		MemoryTotal:        "0 B",
		NestifyMemory:      readNestifyMemory(),
		Uptime:             formatUptime(time.Since(processStartedAt)),
	}

	used, total, percent := readMemoryUsage()
	snapshot.MemoryUsed = formatBytesIEC(used)
	snapshot.MemoryTotal = formatBytesIEC(total)
	snapshot.MemoryUsagePercent = percent

	return snapshot
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

	usage := (1 - float64(idleDelta)/float64(totalDelta)) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}
	s.lastValue = usage
	return usage
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

func readNestifyMemory() string {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return formatBytesIEC(mem.Alloc)
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
