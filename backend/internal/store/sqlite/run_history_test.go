package sqlite

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
)

// 锁住 run_history 的 detail_json 列：写入 / 列表读回 / 单条读回 / 不存在时的哨兵错误。
// 备份任务用它保存「本次备份了哪些文件」的明细清单。
func TestRunHistoryDetailJSONRoundTrip(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "nestify-test.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	started := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	finished := time.Now().UTC().Truncate(time.Second)
	ruleID := int64(7)
	detail := `{"kind":"backup","files":[{"path":"移动云盘/剧集/S01E01.mkv","action":"upload","size":1024,"target":"webdav://3/剧集/S01E01.mkv"}],"counts":{"upload":1},"files_total":1}`

	item := model.RunHistoryItem{
		ID:             "backup-7-1",
		RuleID:         &ruleID,
		RuleName:       "备份任务",
		TriggerMode:    model.TriggerModeManual,
		ArchiveMode:    "backup",
		Status:         "success",
		ProcessedFiles: 1,
		SuccessCount:   1,
		SkipCount:      0,
		FailureCount:   0,
		DeletedCount:   2,
		Summary:        "备份完成：扫描 1，上传 1，跳过 0，删除 2，失败 0",
		DetailJSON:     detail,
		StartedAt:      started,
		UpdatedAt:      finished,
		FinishedAt:     &finished,
	}
	if err := store.UpsertRunHistory(item); err != nil {
		t.Fatalf("upsert run history: %v", err)
	}

	items, err := store.ListRunHistory()
	if err != nil {
		t.Fatalf("list run history: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("unexpected items: %d", len(items))
	}
	if items[0].DetailJSON != detail {
		t.Fatalf("list detail_json mismatch: %q", items[0].DetailJSON)
	}
	if items[0].DeletedCount != 2 {
		t.Fatalf("deleted_count mismatch: %d", items[0].DeletedCount)
	}

	got, err := store.GetRunHistoryByID("backup-7-1")
	if err != nil {
		t.Fatalf("get run history: %v", err)
	}
	if got.DetailJSON != detail {
		t.Fatalf("detail_json mismatch: %q", got.DetailJSON)
	}
	if got.RuleID == nil || *got.RuleID != ruleID {
		t.Fatalf("rule id mismatch: %+v", got.RuleID)
	}
	if !got.StartedAt.Equal(started) {
		t.Fatalf("started_at mismatch: %s != %s", got.StartedAt, started)
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finished) {
		t.Fatalf("finished_at mismatch: %+v", got.FinishedAt)
	}

	if _, err := store.GetRunHistoryByID("missing-id"); !errors.Is(err, ErrRunHistoryNotFound) {
		t.Fatalf("expected ErrRunHistoryNotFound, got %v", err)
	}
}

// TestListRunHistoryGroupPrefersCompleteDetail 锁定「折叠组取到的是最完整的那份明细」。
//
// 一次执行会写出多行历史（每处理一个文件落一行），它们共享同一个 started_at、id 又是随机的。
// 前端「折叠任务」组取的是**组内第一条**的 detail_json，只按 id 兜底时这条是随机的，
// 明细就可能取到没带明细的逐项记录。因此这里按长度降序稳定取到最全的一份。
//
// 注意（轮 109 起）：**逐项记录不再各自带一份累积明细**，整份明细只在执行收尾落一次
// （见 executor.persistRunHistory 的说明）。所以生产库里「最长的那一份」就是收尾那条；
// 本用例用合成的长 / 短明细来验证排序机制本身，与写入策略无关。
func TestListRunHistoryGroupPrefersCompleteDetail(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "nestify-test.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	started := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	ruleID := int64(11)
	shortDetail := `{"kind":"cleanup","files":[{"path":"A.txt","action":"delete","note":"命中清理名单"}],"counts":{"delete":1},"files_total":1}`
	longDetail := `{"kind":"cleanup","files":[{"path":"A.txt","action":"delete","note":"命中清理名单"},{"path":"B.txt","action":"delete","note":"命中清理名单"},{"path":"C.txt","action":"delete","note":"命中清理名单"}],"counts":{"delete":3},"files_total":3}`

	// id 特意让「最短明细」排在最前（DESC 下 zzz 最大）：只按 id 兜底时会取错。
	base := model.RunHistoryItem{
		RuleID:         &ruleID,
		RuleName:       "净化任务",
		TriggerMode:    model.TriggerModeManual,
		ArchiveMode:    "cleanup",
		Status:         "success",
		ProcessedFiles: 3,
		SuccessCount:   3,
		StartedAt:      started,
		UpdatedAt:      started.Add(time.Minute),
	}
	rows := []model.RunHistoryItem{
		{ID: "zzz-run-a", DetailJSON: shortDetail},
		{ID: "mmm-run-b", DetailJSON: shortDetail},
		{ID: "aaa-run-c", DetailJSON: longDetail},
	}
	for index := range rows {
		rows[index].RuleID = base.RuleID
		rows[index].RuleName = base.RuleName
		rows[index].TriggerMode = base.TriggerMode
		rows[index].ArchiveMode = base.ArchiveMode
		rows[index].Status = base.Status
		rows[index].ProcessedFiles = base.ProcessedFiles
		rows[index].SuccessCount = base.SuccessCount
		rows[index].StartedAt = base.StartedAt
		rows[index].UpdatedAt = base.UpdatedAt
		if err := store.UpsertRunHistory(rows[index]); err != nil {
			t.Fatalf("upsert run history %s: %v", rows[index].ID, err)
		}
	}

	items, total, err := store.ListRunHistoryGroupPage(1, 50, "", "", "", "", "started_at", "desc")
	if err != nil {
		t.Fatalf("list grouped run history: %v", err)
	}
	if total != 1 {
		t.Fatalf("同一轮执行应折叠成一组，实际 %d 组", total)
	}
	if len(items) != 3 {
		t.Fatalf("组内应返回 3 行，实际 %d", len(items))
	}
	if items[0].DetailJSON != longDetail {
		t.Fatalf("组内第一条应是明细最完整的那份，实际拿到 %q", items[0].DetailJSON)
	}
}
