package executor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"nestify/backend/internal/model"
)

// decodeRunDetail 解析明细载荷，供各断言复用。
func decodeRunDetail(t *testing.T, stats *executionStats) model.RunDetail {
	t.Helper()
	payload := stats.buildDetailJSON()
	if payload == "" {
		t.Fatal("明细载荷为空，应当写入 detail_json")
	}
	var detail model.RunDetail
	if err := json.Unmarshal([]byte(payload), &detail); err != nil {
		t.Fatalf("明细载荷不是合法 JSON: %v", err)
	}
	return detail
}

// detailEntriesByAction 取出某个动作下的明细条目。
func detailEntriesByAction(detail model.RunDetail, action string) []model.RunFileEntry {
	entries := make([]model.RunFileEntry, 0, len(detail.Files))
	for _, entry := range detail.Files {
		if entry.Action == action {
			entries = append(entries, entry)
		}
	}
	return entries
}

// TestRunDetailCollectorCapsPerAction 锁定「每个动作最多 200 条明细」的约定：
// 超出部分只体现在 counts 里，避免明细载荷把日志接口撑爆。
func TestRunDetailCollectorCapsPerAction(t *testing.T) {
	collector := newRunDetailCollector(model.RunDetailKindStrm)
	for index := 0; index < maxDetailEntriesPerAction+30; index++ {
		collector.record(model.RunFileEntry{
			Path:   "媒体/" + string(rune('a'+index%26)) + ".mkv",
			Action: model.RunFileActionStrm,
		})
	}

	var detail model.RunDetail
	if err := json.Unmarshal([]byte(collector.buildJSON()), &detail); err != nil {
		t.Fatalf("解析明细失败: %v", err)
	}

	if got := len(detail.Files); got != maxDetailEntriesPerAction {
		t.Fatalf("明细条数 = %d, want %d", got, maxDetailEntriesPerAction)
	}
	if got := detail.Counts[model.RunFileActionStrm]; got != maxDetailEntriesPerAction+30 {
		t.Fatalf("counts 应保留真实数量 %d，实际 %d", maxDetailEntriesPerAction+30, got)
	}
	if !detail.FilesTruncated {
		t.Fatal("超出上限时应标记 files_truncated")
	}
}

// TestRunDetailCollectorSkipIsCountOnly 锁定「跳过只计数不列明细」：
// 跳过数量计入 counts，但不参与 files_total，也不产生逐条明细。
func TestRunDetailCollectorSkipIsCountOnly(t *testing.T) {
	collector := newRunDetailCollector(model.RunDetailKindStrm)
	collector.record(model.RunFileEntry{Path: "A.mkv", Action: model.RunFileActionStrm})
	collector.recordSkip(9)

	var detail model.RunDetail
	if err := json.Unmarshal([]byte(collector.buildJSON()), &detail); err != nil {
		t.Fatalf("解析明细失败: %v", err)
	}

	if detail.Counts[model.BackupFileActionSkip] != 9 {
		t.Fatalf("counts.skip = %d, want 9", detail.Counts[model.BackupFileActionSkip])
	}
	if detail.FilesTotal != 1 {
		t.Fatalf("files_total = %d, want 1（跳过不计入）", detail.FilesTotal)
	}
	if len(detail.Files) != 1 {
		t.Fatalf("明细条数 = %d, want 1", len(detail.Files))
	}
}

// TestRunDetailCollectorConcurrentRecord 确认采集器在多线程写入下不丢条数
// （strm 并发列举会多线程记录明细）。
func TestRunDetailCollectorConcurrentRecord(t *testing.T) {
	collector := newRunDetailCollector(model.RunDetailKindStrm)

	var group sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		group.Add(1)
		go func(worker int) {
			defer group.Done()
			for index := 0; index < 25; index++ {
				collector.record(model.RunFileEntry{
					Path:   "媒体/" + strings.Repeat("x", worker) + ".mkv",
					Action: model.RunFileActionStrm,
				})
			}
		}(worker)
	}
	group.Wait()

	var detail model.RunDetail
	if err := json.Unmarshal([]byte(collector.buildJSON()), &detail); err != nil {
		t.Fatalf("解析明细失败: %v", err)
	}
	if got := detail.Counts[model.RunFileActionStrm]; got != 200 {
		t.Fatalf("并发记录后 counts = %d, want 200", got)
	}
	if got := len(detail.Files); got != 200 {
		t.Fatalf("并发记录后明细条数 = %d, want 200", got)
	}
}

// TestExecuteStrmRuleRecordsRunDetail 锁定 strm 链路的运行详情：
// 必须能看到「哪些文件生成了 strm」「哪些元数据被复制成实体文件」，
// 且记录的是源路径与落地产物路径，而不只是统计数目。
func TestExecuteStrmRuleRecordsRunDetail(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "S01E01.mkv"), "video-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "S01E01.srt"), "subtitle-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "notes.txt"), "ignored-bytes")

	service := NewService(nil)
	stats, err := service.executeStrmRule("run-detail-strm", ExecuteRuleRequest{
		Filters:         []string{"mkv"},
		MetadataFilters: []string{"srt"},
	}, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("executeStrmRule 出错: %v", err)
	}

	detail := decodeRunDetail(t, &stats)
	if detail.Kind != model.RunDetailKindStrm {
		t.Fatalf("kind = %q, want %q", detail.Kind, model.RunDetailKindStrm)
	}

	strmEntries := detailEntriesByAction(detail, model.RunFileActionStrm)
	if len(strmEntries) != 1 {
		t.Fatalf("strm 明细条数 = %d, want 1", len(strmEntries))
	}
	if want := filepath.Join(sourceDir, "剧集", "S01E01.mkv"); strmEntries[0].Path != want {
		t.Fatalf("strm 明细 Path = %q, want %q", strmEntries[0].Path, want)
	}
	if want := filepath.Join(targetDir, "剧集", "S01E01.strm"); strmEntries[0].Target != want {
		t.Fatalf("strm 明细 Target = %q, want %q", strmEntries[0].Target, want)
	}
	if strmEntries[0].Note != "新建" {
		t.Fatalf("首次生成的备注应为「新建」，实际 %q", strmEntries[0].Note)
	}

	// 元数据（封面 / 字幕 / nfo）一律合并成一条汇总明细：逐个文件列一条会把
	// 归巢历史 / 任务日志的详情面板撑满。数量仍按真实值记在 counts 里。
	metadataEntries := detailEntriesByAction(detail, model.RunFileActionMetadata)
	if len(metadataEntries) != 1 {
		t.Fatalf("元数据明细应合并为 1 条汇总，实际 %d 条", len(metadataEntries))
	}
	if got := detail.Counts[model.RunFileActionMetadata]; got != 1 {
		t.Fatalf("元数据 counts = %d, want 1（真实数量）", got)
	}
	if want := targetDir; metadataEntries[0].Target != want {
		t.Fatalf("元数据汇总 Target = %q, want %q", metadataEntries[0].Target, want)
	}
	if want := "同步 1 个元数据文件（封面 / 字幕 / nfo）"; metadataEntries[0].Note != want {
		t.Fatalf("元数据汇总 Note = %q, want %q", metadataEntries[0].Note, want)
	}

	// 未命中后缀的文件只计入跳过，不产生明细。
	if detail.Counts[model.BackupFileActionSkip] == 0 {
		t.Fatal("跳过数量应计入 counts")
	}
	if len(detailEntriesByAction(detail, model.BackupFileActionSkip)) != 0 {
		t.Fatal("跳过不应产生逐条明细")
	}
}

// TestExecuteStrmRuleAggregatesMetadataEntries 锁定「元数据不逐条列」：
// 一次同步多个封面 / 字幕 / nfo 时，明细里只允许出现一条汇总（数量写进 counts 与备注），
// 运行日志同样只留一行汇总 —— 原来是「下载一个元数据文件就一条目」，太占地方。
func TestExecuteStrmRuleAggregatesMetadataEntries(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeStrmFixture(t, filepath.Join(sourceDir, "S01E01.mkv"), "video-bytes")
	for _, name := range []string{"S01E01.srt", "S01E02.srt", "poster.jpg", "fanart.jpg", "tvshow.nfo"} {
		writeStrmFixture(t, filepath.Join(sourceDir, name), name+"-bytes")
	}

	service := NewService(nil)
	runID := "run-merge-metadata"
	stats, err := service.executeStrmRule(runID, ExecuteRuleRequest{
		Filters:         []string{"mkv"},
		MetadataFilters: []string{"srt", "jpg", "nfo"},
	}, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("executeStrmRule 出错: %v", err)
	}
	if stats.MetadataCount != 5 {
		t.Fatalf("MetadataCount = %d, want 5", stats.MetadataCount)
	}

	detail := decodeRunDetail(t, &stats)
	metadataEntries := detailEntriesByAction(detail, model.RunFileActionMetadata)
	if len(metadataEntries) != 1 {
		t.Fatalf("元数据明细条数 = %d, want 1（多个元数据文件应合并为一条汇总）", len(metadataEntries))
	}
	if got := detail.Counts[model.RunFileActionMetadata]; got != 5 {
		t.Fatalf("元数据 counts = %d, want 5（页签仍显示真实数量）", got)
	}
	if want := "同步 5 个元数据文件（封面 / 字幕 / nfo）"; metadataEntries[0].Note != want {
		t.Fatalf("元数据汇总 Note = %q, want %q", metadataEntries[0].Note, want)
	}
	if metadataEntries[0].Target != targetDir {
		t.Fatalf("元数据汇总 Target = %q, want %q", metadataEntries[0].Target, targetDir)
	}

	// 运行日志同样不能每个元数据文件一行。
	summaryLines := 0
	for _, entry := range service.ListRunLogs(runID) {
		if strings.Contains(entry.Message, "metadata ") && strings.Contains(entry.Message, "->") {
			t.Fatalf("不应逐条打印元数据搬运日志：%q", entry.Message)
		}
		if strings.HasPrefix(entry.Message, "同步 ") && strings.Contains(entry.Message, "元数据文件") {
			summaryLines++
		}
	}
	if summaryLines != 1 {
		t.Fatalf("元数据汇总日志行数 = %d, want 1", summaryLines)
	}
}

// TestExecutePackageRuleRecordsPackedDirectories 锁定打包规则的运行详情：
// 必须能看到「打包了哪个文件夹、产出了哪个压缩包」。
func TestExecutePackageRuleRecordsPackedDirectories(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeStrmFixture(t, filepath.Join(sourceDir, "剧集A", "001.jpg"), "img-1")
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集A", "002.jpg"), "img-2")

	service := NewService(nil)
	stats, err := service.executeRule("run-detail-pack", ExecuteRuleRequest{
		ArchiveMode: "package",
		SourceDir:   sourceDir,
		TargetDir:   targetDir,
	})
	if err != nil {
		t.Fatalf("executeRule(package) 出错: %v", err)
	}

	detail := decodeRunDetail(t, &stats)
	if detail.Kind != model.RunDetailKindPackage {
		t.Fatalf("kind = %q, want %q", detail.Kind, model.RunDetailKindPackage)
	}

	packEntries := detailEntriesByAction(detail, model.RunFileActionPack)
	if len(packEntries) != 1 {
		t.Fatalf("打包明细条数 = %d, want 1", len(packEntries))
	}
	entry := packEntries[0]
	if !entry.Dir {
		t.Fatal("被打包的是文件夹，明细必须带 Dir 标记")
	}
	if want := filepath.Join(sourceDir, "剧集A"); entry.Path != want {
		t.Fatalf("打包明细 Path = %q, want %q", entry.Path, want)
	}
	if !strings.HasSuffix(entry.Target, ".cbz") {
		t.Fatalf("打包明细 Target = %q，应指向产出的压缩包", entry.Target)
	}
	if _, err := os.Stat(entry.Target); err != nil {
		t.Fatalf("明细里的压缩包应真实存在: %v", err)
	}
	if entry.Size <= 0 {
		t.Fatalf("打包明细应带压缩包大小，实际 %d", entry.Size)
	}
}
