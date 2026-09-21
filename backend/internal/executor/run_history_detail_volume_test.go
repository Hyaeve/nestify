package executor

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"nestify/backend/internal/model"
)

// 轮 109 起 run_history 的明细写入量必须是 O(项数)，不再是 O(项数²)。
//
// 背景：一次执行是「每处理一项写一行」，而每一行原来都带一份**累积**明细
// （第 k 行装前 k 项的明细、轮 96 起又没有 200 条上限）。于是一轮上万项的执行会写出
// O(n²) 的 JSON：序列化开销、sqlite 写入量、以及内存镜像（recordHistory 会顺手塞一份到
// service.history）会一起把容器顶到几个 G —— 用户报的「内存暴增」就是这个。
//
// 现在：逐项记录一律不带明细，整份明细只在收尾落一次；前端折叠组按
// 「组内 detail_json 最长的一份」取代表行，取到的仍是那一份，展示口径不变。
func TestRunHistoryDetailWrittenOncePerRun(t *testing.T) {
	store := openRunHistoryTestStore(t)

	sourceDir := t.TempDir()
	const matched = 6
	for index := 0; index < matched; index++ {
		writeStrmFixture(t, filepath.Join(sourceDir, fmt.Sprintf("广告%d.txt", index)), "ad")
		writeStrmFixture(t, filepath.Join(sourceDir, fmt.Sprintf("保留%d.mkv", index)), "keep")
	}

	rule, err := store.CreateRule(model.CreateRuleInput{
		Name:        "清理模式-明细写入量",
		ArchiveMode: "cleanup",
		RuleType:    "cleanup",
		SourceDir:   sourceDir,
		SourceDirs:  []string{sourceDir},
		// 只开「匹配清理」：不让空目录参与，逐项记录的条数才可预期。
		Options: map[string]bool{"cleanup_matching_files": true},
		Filters: []string{"*广告"},
	})
	if err != nil {
		t.Fatalf("创建清理模式规则失败: %v", err)
	}

	service := NewService(store)
	run, err := service.PrepareRuleRun(buildRuleExecuteRequest(*rule, model.TriggerModeManual))
	if err != nil {
		t.Fatalf("启动清理模式执行失败: %v", err)
	}
	waitRunSettled(t, service, run.ID)
	assertRunSucceeded(t, service, run.ID)

	items, total, err := store.ListRunHistoryGroupPage(1, 100, "", "", "", "cleanup", "", "")
	if err != nil {
		t.Fatalf("读取分组运行历史失败: %v", err)
	}
	if total != 1 {
		t.Fatalf("一次执行应折叠成一组，实际 %d 组", total)
	}
	// 逐项记录必须还在，否则下面「只有一条带明细」就是假通过。
	if len(items) < matched {
		t.Fatalf("逐项记录缺失：组内只有 %d 行，期望至少 %d 行", len(items), matched)
	}

	bearing := make([]string, 0, 1)
	for _, item := range items {
		if strings.TrimSpace(item.DetailJSON) != "" {
			bearing = append(bearing, item.ID)
		}
	}
	if len(bearing) != 1 {
		t.Fatalf("带明细的记录应恰好 1 条（收尾那条），实际 %d 条 / 组内共 %d 行", len(bearing), len(items))
	}
	// 折叠组代表行 = 组内明细最长的一份，必须就是那一条。
	if items[0].ID != bearing[0] {
		t.Fatalf("折叠组第一条应是带明细的收尾记录：第一条=%s，带明细=%s", items[0].ID, bearing[0])
	}

	var detail model.RunDetail
	if err := json.Unmarshal([]byte(items[0].DetailJSON), &detail); err != nil {
		t.Fatalf("收尾记录的明细不是合法 JSON: %v", err)
	}
	if got := detail.Counts[model.BackupFileActionDelete]; got != matched {
		t.Fatalf("收尾记录的 counts.delete = %d, want %d（收尾那条必须是完整明细）", got, matched)
	}
	if detail.FilesTotal != matched {
		t.Fatalf("收尾记录的 files_total = %d, want %d", detail.FilesTotal, matched)
	}
	if len(detail.SourceRoots) == 0 {
		t.Fatalf("收尾记录的 source_roots 为空（前端要靠它裁成相对路径）")
	}
}

// 内存镜像（service.history）不驻留明细，且内部按写入顺序存放、读取时翻回「最新在最前」。
//
// 它只是 store 不可用时的兜底。原来每写一行都带一份累积明细、还要「往前插」
// （append([]T{item}, s.history...)），一轮上万项就是 O(n²) 的堆占用 + O(n²) 的切片拷贝。
func TestRunHistoryMemoryMirrorDropsDetail(t *testing.T) {
	service := NewService(nil)
	run := service.newRun(model.TriggerModeManual, "cleanup", "", nil, "内存镜像")

	stats := executionStats{}
	stats.detail(model.RunDetailKindCleanup)
	stats.Detail.record(model.RunFileEntry{Path: "A.txt", Action: model.BackupFileActionDelete})

	service.persistRunHistoryWithDetail(run.ID, "第一条", &stats)
	service.persistRunHistory(run.ID, "第二条", &stats)

	service.mu.RLock()
	mirror := append([]model.RunHistoryItem(nil), service.history...)
	service.mu.RUnlock()

	if len(mirror) != 2 {
		t.Fatalf("内存镜像应有 2 条，实际 %d", len(mirror))
	}
	if mirror[0].Summary != "第一条" {
		t.Fatalf("镜像内部应按写入顺序存放（最新的在最后），实际第一条摘要 = %q", mirror[0].Summary)
	}
	for _, item := range mirror {
		if item.DetailJSON != "" {
			t.Fatalf("内存镜像不该驻留明细，实际 %q", item.DetailJSON)
		}
	}

	// 对外读取时翻回「最新在最前」，与 store.ListRunHistory 的顺序一致。
	listed := service.ListHistory()
	if len(listed) != 2 || listed[0].Summary != "第二条" {
		t.Fatalf("ListHistory 应最新在最前，实际 %+v", listed)
	}
}
