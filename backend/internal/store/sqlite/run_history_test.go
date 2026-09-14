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
