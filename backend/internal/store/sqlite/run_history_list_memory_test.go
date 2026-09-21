package sqlite

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
)

// 锁住「列表查询的成本与明细体积无关」。
//
// 背景（用户报的「打开网页内存就暴涨」，读数是 docker stats / NAS 面板）：
//
//	run_history 是「每处理一项写一行」，每行的 detail_json 是一次执行的明细快照，
//	单行可达几十 KB。而列表类查询原来把 detail_json 一起 SELECT 出来，还按
//	`LENGTH(detail_json)` 排序 —— 于是单次请求要读完整表所有明细（上万行 × 几十 KB
//	就是几百 MB），而仪表盘每 5 秒就拉一次不带分页参数的 /run-history。
//
// 修法：列表让 detail_json 列返回空字符串，排序改用写入时算好的 detail_size 列。
// 这个用例塞进带大明细的行，断言列表查询的**分配量**远小于明细总量 —— 一旦有人把
// detail_json 加回列表 SQL（或改回 LENGTH 排序），这里会立刻炸。
func TestRunHistoryListIgnoresDetailVolume(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "nestify-test.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	const rowCount = 120
	// 64 KB 的明细 × 120 行 ≈ 7.7 MB：读与不读差一个数量级，断言留足余量也不会误报。
	const detailBytes = 64 * 1024
	detail := `{"kind":"cleanup","blob":"` + strings.Repeat("x", detailBytes) + `"}`

	started := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	for index := 0; index < rowCount; index++ {
		item := model.RunHistoryItem{
			ID:          fmt.Sprintf("run-%03d", index),
			RuleName:    "净化任务",
			TriggerMode: model.TriggerModeManual,
			ArchiveMode: "cleanup",
			Status:      "success",
			Summary:     "已删除匹配文件",
			DetailJSON:  detail,
			StartedAt:   started,
			UpdatedAt:   started.Add(time.Duration(index) * time.Second),
		}
		if err := store.UpsertRunHistory(item); err != nil {
			t.Fatalf("upsert run history %d: %v", index, err)
		}
	}

	totalDetailBytes := rowCount * len(detail)

	measure := func(name string, run func()) {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		run()
		runtime.ReadMemStats(&after)

		allocated := after.TotalAlloc - before.TotalAlloc
		t.Logf("%s: 分配 %.2f MB（明细总量 %.2f MB）", name, float64(allocated)/1024/1024, float64(totalDetailBytes)/1024/1024)

		// 读了明细的话，分配量必然≥明细总量；不读的话连零头都不到。
		if allocated > uint64(totalDetailBytes/4) {
			t.Fatalf("%s 的分配量 %.2f MB 与明细总量同量级 —— 列表查询又把 detail_json 读出来了",
				name, float64(allocated)/1024/1024)
		}
	}

	var listed []model.RunHistoryItem
	measure("ListRunHistory", func() {
		items, listErr := store.ListRunHistory()
		if listErr != nil {
			t.Fatalf("list run history: %v", listErr)
		}
		listed = items
	})
	if len(listed) != rowCount {
		t.Fatalf("列表应返回 %d 行（未触及上限），实际 %d", rowCount, len(listed))
	}
	for _, item := range listed {
		if item.DetailJSON != "" {
			t.Fatalf("列表不应返回明细，实际 %q", item.DetailJSON[:min(40, len(item.DetailJSON))])
		}
	}

	measure("ListRunHistoryGroupPage", func() {
		items, _, groupErr := store.ListRunHistoryGroupPage(1, 50, "", "", "", "", "started_at", "desc")
		if groupErr != nil {
			t.Fatalf("list grouped run history: %v", groupErr)
		}
		if len(items) == 0 {
			t.Fatalf("分组列表不应为空")
		}
	})

	// 明细本身当然要还在：详情接口按 id 单条读回，仍然是完整的。
	loaded, err := store.GetRunHistoryByID("run-000")
	if err != nil {
		t.Fatalf("get run history: %v", err)
	}
	if loaded.DetailJSON != detail {
		t.Fatalf("单条详情应带完整明细，实际长度 %d，期望 %d", len(loaded.DetailJSON), len(detail))
	}
}
