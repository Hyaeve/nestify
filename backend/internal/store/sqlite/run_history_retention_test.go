package sqlite

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
)

// 锁定「运行日志保留策略按时间节流」。
//
// run_history 是一次执行写很多行（每处理一项一行），而保留策略里那条
// `DELETE ... WHERE id NOT IN (SELECT id ... ORDER BY started_at DESC LIMIT N)`
// 每次都要扫一遍全表、并把最多 N 个 id 物化成临时 B 树；逐行执行的话代价是
// O(项数 × 表行数) —— 用户看到的「容器内存 / CPU 爆掉」就是这么来的。
//
// 用「限额 3 行 + 一次性插 8 行」来观察：只有第一次插入会触发裁剪，所以表里会短暂
// 多于 3 行（这就是节流的证据）；随后显式跑一次策略必须裁到 3 行（正确性一条不少）。
func TestRunHistoryRetentionIsThrottledPerBurst(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "nestify-test.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	// 只留「最大条数」，不启用「按天数过期」，免得两条规则互相干扰。
	if _, err := store.UpdateSettings(model.UpdateSettingsInput{
		LogRetentionDays:       0,
		LogRetentionMaxRecords: 3,
	}); err != nil {
		t.Fatalf("更新设置失败: %v", err)
	}

	started := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	for index := 0; index < 8; index++ {
		at := started.Add(time.Duration(index) * time.Second)
		item := model.RunHistoryItem{
			ID:        fmt.Sprintf("row-%02d", index),
			RuleName:  "节流用例",
			Status:    "success",
			StartedAt: at,
			UpdatedAt: at,
		}
		if err := store.UpsertRunHistory(item); err != nil {
			t.Fatalf("upsert %s: %v", item.ID, err)
		}
	}

	summary, err := store.GetRunHistorySummary()
	if err != nil {
		t.Fatalf("读取运行日志汇总失败: %v", err)
	}
	if summary.Total <= 3 {
		t.Fatalf("保留策略被逐行执行了：连插 8 行后表里只剩 %d 行（期望 > 3，说明发生了节流）", summary.Total)
	}

	// 节流只影响「什么时候裁」，不影响「裁不裁」：显式跑一次必须裁到限额。
	if err := store.applyRunHistoryRetentionPolicy(); err != nil {
		t.Fatalf("显式执行保留策略失败: %v", err)
	}
	summary, err = store.GetRunHistorySummary()
	if err != nil {
		t.Fatalf("读取运行日志汇总失败: %v", err)
	}
	if summary.Total != 3 {
		t.Fatalf("显式执行保留策略后应裁到 3 行，实际 %d", summary.Total)
	}
}
