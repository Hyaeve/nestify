package backup

import (
	"encoding/json"
	"testing"

	"nestify/backend/internal/model"
)

// 每个动作的采集上限为 maxFileEntriesPerAction，超出后不再记录明细，
// 但计数仍按真实数量累加 —— 详情面板依赖这个语义显示「共 N 项（仅记录前 X 项）」。
func TestRunStatsRecordFileCapsPerAction(t *testing.T) {
	stats := &runStats{}

	total := maxFileEntriesPerAction + 25
	for index := 0; index < total; index++ {
		stats.recordFile(model.BackupFileEntry{
			Path:   "移动云盘/剧集/" + string(rune('a'+index%26)) + ".mkv",
			Action: model.BackupFileActionUpload,
		})
	}
	for index := 0; index < 3; index++ {
		stats.recordFile(model.BackupFileEntry{Path: "失败.txt", Action: model.BackupFileActionFail})
	}

	if len(stats.Files) != maxFileEntriesPerAction+3 {
		t.Fatalf("unexpected recorded entries: got %d, want %d", len(stats.Files), maxFileEntriesPerAction+3)
	}
	if !stats.truncated {
		t.Fatalf("expected truncated flag to be set")
	}
	if stats.fileCounts[model.BackupFileActionUpload] != total {
		t.Fatalf("upload count should keep real total: got %d, want %d", stats.fileCounts[model.BackupFileActionUpload], total)
	}

	encoded := stats.buildBackupDetailJSON()
	if encoded == "" {
		t.Fatalf("expected non-empty detail json")
	}

	var detail model.BackupDetail
	if err := json.Unmarshal([]byte(encoded), &detail); err != nil {
		t.Fatalf("decode detail json: %v", err)
	}
	if detail.Kind != "backup" {
		t.Fatalf("unexpected kind: %s", detail.Kind)
	}
	if detail.FilesTotal != total+3 {
		t.Fatalf("unexpected files_total: got %d, want %d", detail.FilesTotal, total+3)
	}
	if !detail.FilesTruncated {
		t.Fatalf("expected files_truncated in payload")
	}
	if detail.Counts[model.BackupFileActionUpload] != total {
		t.Fatalf("unexpected counts payload: %+v", detail.Counts)
	}
}

// 空路径不入库；未采集到任何文件时不写 detail_json（保持历史记录干净）。
func TestRunStatsSkipsEmptyPathAndEmptyPayload(t *testing.T) {
	stats := &runStats{}
	stats.recordFile(model.BackupFileEntry{Path: "   ", Action: model.BackupFileActionUpload})
	if stats.fileTotal != 0 || len(stats.Files) != 0 {
		t.Fatalf("blank path should be ignored: %+v", stats)
	}
	if payload := stats.buildBackupDetailJSON(); payload != "" {
		t.Fatalf("empty stats should produce empty payload, got %q", payload)
	}
}

func TestBackupFileActionsAreStable(t *testing.T) {
	// 前端按这些字符串做筛选与配色，改动等于破坏兼容。
	expected := map[string]string{
		"upload": model.BackupFileActionUpload,
		"skip":   model.BackupFileActionSkip,
		"fail":   model.BackupFileActionFail,
		"delete": model.BackupFileActionDelete,
	}
	for want, got := range expected {
		if got != want {
			t.Fatalf("action constant changed: %s != %s", got, want)
		}
	}
}
