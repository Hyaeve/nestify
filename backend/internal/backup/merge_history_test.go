package backup

import (
	"encoding/json"
	"testing"
	"time"

	"nestify/backend/internal/model"
)

// 合并窗口：紧跟上一次监控执行的触发算同一次连续备份；超窗 / 时间异常都另起一条。
func TestWithinWatchMergeWindow(t *testing.T) {
	last := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		last   time.Time
		next   time.Time
		expect bool
	}{
		{"窗口内", last, last.Add(30 * time.Second), true},
		{"正好到窗口边界", last, last.Add(watchHistoryMergeWindow), true},
		{"超出窗口", last, last.Add(watchHistoryMergeWindow + time.Second), false},
		{"锚点缺失", time.Time{}, last, false},
		{"时间早于上次结束", last, last.Add(-time.Second), false},
	}

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := withinWatchMergeWindow(item.last, item.next); got != item.expect {
				t.Fatalf("withinWatchMergeWindow = %v, want %v", got, item.expect)
			}
		})
	}
}

// 连续两次监控触发必须收纳成一条记录：计数累加，时间滚动到最近一次触发。
func TestMergeWatchRunHistoryItemAccumulates(t *testing.T) {
	firstStartedAt := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	previous := model.RunHistoryItem{
		ID:             "backup-7-1",
		TriggerMode:    model.TriggerModeWatch,
		ArchiveMode:    "backup",
		Status:         "success",
		ProcessedFiles: 10,
		SuccessCount:   3,
		SkipCount:      7,
		StartedAt:      firstStartedAt,
		UpdatedAt:      firstStartedAt.Add(time.Second),
	}

	stats := &runStats{Scanned: 4, Copied: 2, Skipped: 2}
	stats.recordFile(model.BackupFileEntry{Path: "剧集/b.mkv", Action: model.BackupFileActionUpload})
	stats.recordFile(model.BackupFileEntry{Path: "剧集/已存在.mkv", Action: model.BackupFileActionSkip})

	startedAt := firstStartedAt.Add(5 * time.Minute)
	finishedAt := startedAt.Add(time.Second)
	merged := mergeWatchRunHistoryItem(previous, model.BackupTask{ID: 7, Name: "入库"}, "success", startedAt, finishedAt, stats)

	if merged.ID != previous.ID {
		t.Fatalf("合并必须复用同一条记录：got %s", merged.ID)
	}
	if !merged.StartedAt.Equal(startedAt) {
		t.Fatalf("started_at 应滚动到最近一次触发（否则会沉出历史列表首页）：got %v", merged.StartedAt)
	}
	if merged.FinishedAt == nil || !merged.FinishedAt.Equal(finishedAt) {
		t.Fatalf("finished_at 应更新为最新：got %v", merged.FinishedAt)
	}
	if merged.ProcessedFiles != 14 || merged.SuccessCount != 5 || merged.SkipCount != 9 {
		t.Fatalf("计数应累加：%+v", merged)
	}
	if merged.Status != "success" {
		t.Fatalf("有上传成功时状态应为 success：%s", merged.Status)
	}
	if merged.Summary != "备份完成：扫描 14，上传 5，跳过 9，删除 0，失败 0" {
		t.Fatalf("摘要应按累计值重算：%q", merged.Summary)
	}
}

// 状态按累计结果重算：累计有失败记 failed，一个都没传全是跳过记 skip。
func TestMergeWatchRunHistoryItemRecomputesStatus(t *testing.T) {
	startedAt := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)

	t.Run("累计失败记 failed", func(t *testing.T) {
		previous := model.RunHistoryItem{ID: "a", Status: "success", SuccessCount: 1}
		stats := &runStats{Failed: 1}
		merged := mergeWatchRunHistoryItem(previous, model.BackupTask{ID: 1}, "success", startedAt, startedAt, stats)
		if merged.Status != "failed" {
			t.Fatalf("want failed, got %s", merged.Status)
		}
	})

	t.Run("全程跳过记 skip", func(t *testing.T) {
		previous := model.RunHistoryItem{ID: "b", Status: "skip", SkipCount: 3}
		stats := &runStats{Skipped: 2}
		merged := mergeWatchRunHistoryItem(previous, model.BackupTask{ID: 1}, "success", startedAt, startedAt, stats)
		if merged.Status != "skip" {
			t.Fatalf("want skip, got %s", merged.Status)
		}
	})

	t.Run("手动停止保留 cancelled", func(t *testing.T) {
		previous := model.RunHistoryItem{ID: "c", Status: "success", SuccessCount: 5}
		stats := &runStats{Skipped: 1}
		merged := mergeWatchRunHistoryItem(previous, model.BackupTask{ID: 1}, "cancelled", startedAt, startedAt, stats)
		if merged.Status != "cancelled" {
			t.Fatalf("want cancelled, got %s", merged.Status)
		}
		if merged.Summary != "已手动停止：扫描 0，上传 5，跳过 1，删除 0，失败 0" {
			t.Fatalf("停止措辞应保留：%q", merged.Summary)
		}
	})
}

// 明细载荷合并：counts / files_total 累加，跳过也逐条进明细列表。
func TestMergeBackupDetailPayloadAccumulates(t *testing.T) {
	previousStats := &runStats{}
	previousStats.recordFile(model.BackupFileEntry{Path: "剧集/a.mkv", Action: model.BackupFileActionUpload})
	previousStats.recordFile(model.BackupFileEntry{Path: "剧集/x.mkv", Action: model.BackupFileActionSkip})

	nextStats := &runStats{}
	nextStats.recordFile(model.BackupFileEntry{Path: "剧集/b.mkv", Action: model.BackupFileActionUpload})
	nextStats.recordFile(model.BackupFileEntry{Path: "剧集/c.mkv", Action: model.BackupFileActionFail})
	nextStats.recordFile(model.BackupFileEntry{Path: "剧集/y.mkv", Action: model.BackupFileActionSkip})
	nextStats.recordFile(model.BackupFileEntry{Path: "剧集/z.mkv", Action: model.BackupFileActionSkip})

	payload := mergeBackupDetailPayload(previousStats.buildBackupDetailJSON(), nextStats)

	var detail model.RunDetail
	if err := json.Unmarshal([]byte(payload), &detail); err != nil {
		t.Fatalf("decode merged payload: %v", err)
	}
	if detail.Kind != model.RunDetailKindBackup {
		t.Fatalf("unexpected kind: %s", detail.Kind)
	}
	if len(detail.Files) != 6 {
		t.Fatalf("明细应含上传 / 失败 / 跳过：got %d (%+v)", len(detail.Files), detail.Files)
	}
	skips := 0
	for _, entry := range detail.Files {
		if entry.Action == model.BackupFileActionSkip {
			skips++
		}
	}
	if skips != 3 {
		t.Fatalf("合并后的跳过条目应为 3 条（上一条 x.mkv + 本次 y/z），实际 %d", skips)
	}
	if detail.Counts[model.BackupFileActionUpload] != 2 || detail.Counts[model.BackupFileActionSkip] != 3 {
		t.Fatalf("counts 应累加：%+v", detail.Counts)
	}
	if detail.FilesTotal != 6 {
		t.Fatalf("files_total 应累加且含跳过：got %d", detail.FilesTotal)
	}
	if detail.FilesTruncated {
		t.Fatalf("未触到上限不应标记截断")
	}
}

// 合并后仍要守住每个动作的明细上限：超出部分只计数、标记截断。
func TestMergeBackupDetailPayloadRespectsPerActionCap(t *testing.T) {
	previousStats := &runStats{}
	for index := 0; index < maxFileEntriesPerAction; index++ {
		previousStats.recordFile(model.BackupFileEntry{Path: "剧集/老.mkv", Action: model.BackupFileActionUpload})
	}
	if previousStats.truncated {
		t.Fatalf("上一条刚好到上限时不应标记截断")
	}

	nextStats := &runStats{}
	for index := 0; index < 5; index++ {
		nextStats.recordFile(model.BackupFileEntry{Path: "剧集/新.mkv", Action: model.BackupFileActionUpload})
	}

	var detail model.RunDetail
	if err := json.Unmarshal([]byte(mergeBackupDetailPayload(previousStats.buildBackupDetailJSON(), nextStats)), &detail); err != nil {
		t.Fatalf("decode merged payload: %v", err)
	}
	if len(detail.Files) != maxFileEntriesPerAction {
		t.Fatalf("明细应停在上限：got %d", len(detail.Files))
	}
	if !detail.FilesTruncated {
		t.Fatalf("超出上限应标记截断")
	}
	if detail.Counts[model.BackupFileActionUpload] != maxFileEntriesPerAction+5 {
		t.Fatalf("counts 仍应按真实数量累加：%+v", detail.Counts)
	}
	if detail.FilesTotal != maxFileEntriesPerAction+5 {
		t.Fatalf("files_total 也按真实数量：got %d", detail.FilesTotal)
	}
}

// 两边都没有明细时返回空串，历史记录保持干净。
func TestMergeBackupDetailPayloadEmpty(t *testing.T) {
	if payload := mergeBackupDetailPayload("", &runStats{}); payload != "" {
		t.Fatalf("empty merge should produce empty payload, got %q", payload)
	}
}
