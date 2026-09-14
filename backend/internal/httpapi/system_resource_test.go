package httpapi

import (
	"testing"
)

// 上传取 tx（发送方向）、下载取 rx（接收方向），这里用固定的网卡计数器验证方向映射与 lo 过滤。
func TestParseNetDevCounters(t *testing.T) {
	content := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1000       10    0    0    0     0          0         0     1000       10    0    0    0     0       0          0
  eth0: 2048       20    0    0    0     0          0         0     4096       30    0    0    0     0       0          0
  eth1: 1024       11    0    0    0     0          0         0     1024       11    0    0    0     0       0          0
`

	rx, tx, ok := parseNetDevCounters(content)
	if !ok {
		t.Fatalf("期望解析出有效网卡")
	}
	if rx != 3072 {
		t.Fatalf("接收字节数期望 3072，实际 %d", rx)
	}
	if tx != 5120 {
		t.Fatalf("发送字节数期望 5120，实际 %d", tx)
	}
}

func TestParseNetDevCountersSkipsLoopbackOnly(t *testing.T) {
	content := "    lo: 1000       10    0    0    0     0          0         0     1000       10    0    0    0     0       0          0\n"

	rx, tx, ok := parseNetDevCounters(content)
	if ok || rx != 0 || tx != 0 {
		t.Fatalf("仅有 lo 时应视为无有效网卡，实际 ok=%v rx=%d tx=%d", ok, rx, tx)
	}
}

func TestClampPercent(t *testing.T) {
	cases := []struct {
		input    float64
		expected float64
	}{
		{-5, 0},
		{0, 0},
		{42.5, 42.5},
		{100, 100},
		{180, 100},
	}

	for _, item := range cases {
		if actual := clampPercent(item.input); actual != item.expected {
			t.Fatalf("clampPercent(%v) 期望 %v，实际 %v", item.input, item.expected, actual)
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	cases := []struct {
		input    float64
		expected string
	}{
		{0, "0 B/s"},
		{0.4, "0 B/s"},
		{512, "512 B/s"},
		{2048, "2.0 KB/s"},
		{3 * 1024 * 1024, "3.0 MB/s"},
	}

	for _, item := range cases {
		if actual := formatSpeed(item.input); actual != item.expected {
			t.Fatalf("formatSpeed(%v) 期望 %q，实际 %q", item.input, item.expected, actual)
		}
	}
}
