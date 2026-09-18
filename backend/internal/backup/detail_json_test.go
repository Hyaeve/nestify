package backup

import (
	"encoding/json"
	"strings"
	"testing"

	"nestify/backend/internal/model"
)

// 明细不再按动作封顶（轮 96）：记多少条就有多少条，counts / files / files_total 三者恒等，
// 任务详情窗口靠分页查看（一页条数跟随系统设置）。
func TestRunStatsRecordFileKeepsAllEntries(t *testing.T) {
	stats := &runStats{}

	total := 225
	for index := 0; index < total; index++ {
		stats.recordFile(model.BackupFileEntry{
			Path:   "移动云盘/剧集/" + string(rune('a'+index%26)) + ".mkv",
			Action: model.BackupFileActionUpload,
		})
	}
	for index := 0; index < 3; index++ {
		stats.recordFile(model.BackupFileEntry{Path: "失败.txt", Action: model.BackupFileActionFail})
	}

	if len(stats.Files) != total+3 {
		t.Fatalf("unexpected recorded entries: got %d, want %d", len(stats.Files), total+3)
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
	if len(detail.Files) != total+3 {
		t.Fatalf("明细条数 = %d, want %d（不再截断）", len(detail.Files), total+3)
	}
	if detail.FilesTotal != total+3 {
		t.Fatalf("unexpected files_total: got %d, want %d", detail.FilesTotal, total+3)
	}
	if detail.FilesTruncated {
		t.Fatalf("已无每动作上限，不应再标记 files_truncated")
	}
	if detail.Counts[model.BackupFileActionUpload] != total {
		t.Fatalf("unexpected counts payload: %+v", detail.Counts)
	}
}

// 「跳过」也逐条写进明细列表：任务详情要能看出「到底是哪些文件被跳过了」，
// 只有数目不够；files_total 同样把跳过算进去。
func TestRunStatsRecordsSkipEntries(t *testing.T) {
	stats := &runStats{}

	stats.recordFile(model.BackupFileEntry{Path: "剧集/a.mkv", Action: model.BackupFileActionUpload})
	stats.recordFile(model.BackupFileEntry{Path: "剧集/b.mkv", Action: model.BackupFileActionFail})
	for index := 0; index < 5; index++ {
		stats.recordFile(model.BackupFileEntry{
			Path:   "剧集/已存在.mkv",
			Action: model.BackupFileActionSkip,
			Note:   "目标已存在同名文件，按「同名跳过」处理",
		})
	}

	if len(stats.Files) != 7 {
		t.Fatalf("skip entries should be listed: got %d, want 7", len(stats.Files))
	}

	var detail model.BackupDetail
	if err := json.Unmarshal([]byte(stats.buildBackupDetailJSON()), &detail); err != nil {
		t.Fatalf("decode detail json: %v", err)
	}
	if detail.Counts[model.BackupFileActionSkip] != 5 {
		t.Fatalf("skip count must stay accurate: %+v", detail.Counts)
	}
	if detail.FilesTotal != 7 {
		t.Fatalf("files_total should include skipped entries: got %d, want 7", detail.FilesTotal)
	}
	if detail.FilesTruncated {
		t.Fatalf("five skip entries must not mark the payload as truncated")
	}
	skipEntries := 0
	for _, entry := range detail.Files {
		if entry.Action == model.BackupFileActionSkip {
			skipEntries++
			if entry.Note == "" {
				t.Fatalf("跳过条目应带原因备注: %+v", entry)
			}
		}
	}
	if skipEntries != 5 {
		t.Fatalf("明细里的跳过条数 = %d, want 5", skipEntries)
	}
}

// 海量跳过同样逐条保留（不再截断），也不会挤掉上传 / 失败条目；
// counts.skip 与明细里的跳过条数恒等。
func TestRunStatsKeepsAllSkipEntries(t *testing.T) {
	const skips = 210
	stats := &runStats{}
	stats.recordFile(model.BackupFileEntry{Path: "剧集/a.mkv", Action: model.BackupFileActionUpload})
	for index := 0; index < skips; index++ {
		stats.recordFile(model.BackupFileEntry{Path: "剧集/已存在.mkv", Action: model.BackupFileActionSkip})
	}

	var detail model.BackupDetail
	if err := json.Unmarshal([]byte(stats.buildBackupDetailJSON()), &detail); err != nil {
		t.Fatalf("decode detail json: %v", err)
	}
	if detail.Counts[model.BackupFileActionSkip] != skips {
		t.Fatalf("counts.skip = %d, want %d", detail.Counts[model.BackupFileActionSkip], skips)
	}
	gotSkips := 0
	gotUploads := 0
	for _, entry := range detail.Files {
		switch entry.Action {
		case model.BackupFileActionSkip:
			gotSkips++
		case model.BackupFileActionUpload:
			gotUploads++
		}
	}
	if gotSkips != skips {
		t.Fatalf("跳过条目 = %d, want %d（不再截断）", gotSkips, skips)
	}
	if gotUploads != 1 {
		t.Fatalf("上传条目不应被跳过条目影响，实际 %d 条", gotUploads)
	}
	if detail.FilesTruncated {
		t.Fatal("已无每动作上限，不应再标记 files_truncated")
	}
}

// 一次执行全部跳过时仍然要写 detail_json（前端靠 counts.skip 展示跳过数目）。
func TestRunStatsWritesPayloadWhenEverythingSkipped(t *testing.T) {
	stats := &runStats{}
	stats.recordFile(model.BackupFileEntry{Path: "剧集/a.mkv", Action: model.BackupFileActionSkip})

	payload := stats.buildBackupDetailJSON()
	if payload == "" {
		t.Fatalf("all-skip run should still persist counts")
	}

	var detail model.BackupDetail
	if err := json.Unmarshal([]byte(payload), &detail); err != nil {
		t.Fatalf("decode detail json: %v", err)
	}
	if len(detail.Files) != 1 || detail.FilesTotal != 1 {
		t.Fatalf("all-skip run should list the skipped file: %+v", detail)
	}
	if detail.Counts[model.BackupFileActionSkip] != 1 {
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

// 源 / 目标根路径随明细入库：前端据此把目标端绝对路径裁成「根路径下一级」显示。
// 归一化要求与 executor 侧一致：去尾部斜杠、去重复、丢掉空值与「/」（裁不出内容）。
func TestRunStatsSerializesRoots(t *testing.T) {
	stats := &runStats{
		SourceRoots: []string{"/mnt/a/电影/", "/mnt/a/电影", "   "},
		TargetRoots: []string{"webdav://2/备份/电影", "/"},
	}
	stats.recordFile(model.BackupFileEntry{
		Path:   "电影/a.mkv",
		Action: model.BackupFileActionUpload,
		Target: "webdav://2/备份/电影/电影/a.mkv",
	})

	var detail model.BackupDetail
	if err := json.Unmarshal([]byte(stats.buildBackupDetailJSON()), &detail); err != nil {
		t.Fatalf("decode detail json: %v", err)
	}
	if got := strings.Join(detail.SourceRoots, "|"); got != "/mnt/a/电影" {
		t.Fatalf("source_roots = %q, want %q", got, "/mnt/a/电影")
	}
	if got := strings.Join(detail.TargetRoots, "|"); got != "webdav://2/备份/电影" {
		t.Fatalf("target_roots = %q, want %q", got, "webdav://2/备份/电影")
	}
}

// 实时监控的连续触发走合并路径：根路径必须以本次任务配置为准带上，
// 否则合并后的历史记录丢掉裁剪依据，前端只能显示整条绝对路径。
func TestMergeBackupDetailPayloadKeepsRoots(t *testing.T) {
	previous := `{"kind":"backup","files":[{"path":"电影/a.mkv","action":"upload"}],"counts":{"upload":1},"files_total":1}`
	stats := &runStats{TargetRoots: []string{"/mnt/b/电影/"}}
	stats.recordFile(model.BackupFileEntry{Path: "电影/b.mkv", Action: model.BackupFileActionUpload})

	var detail model.BackupDetail
	if err := json.Unmarshal([]byte(mergeBackupDetailPayload(previous, stats)), &detail); err != nil {
		t.Fatalf("decode merged detail json: %v", err)
	}
	if got := strings.Join(detail.TargetRoots, "|"); got != "/mnt/b/电影" {
		t.Fatalf("merged target_roots = %q, want %q", got, "/mnt/b/电影")
	}
	if len(detail.Files) != 2 {
		t.Fatalf("merged files = %d, want 2", len(detail.Files))
	}
}

// WebDAV 目标端的展示路径 = 配置串 + 源相对路径。
// 曾经的 target.raw + internalPath 会把配置里的内部路径重复一遍
// （webdav://2/备份/电影/备份/电影/a.mkv），前端裁掉根路径后仍会多出「备份/电影」一段。
func TestWebdavDisplayTargetAvoidsDuplicatedInternalPath(t *testing.T) {
	got := webdavDisplayTarget("webdav://2/备份/电影", "电影/a.mkv")
	if want := "webdav://2/备份/电影/电影/a.mkv"; got != want {
		t.Fatalf("webdavDisplayTarget = %q, want %q", got, want)
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
