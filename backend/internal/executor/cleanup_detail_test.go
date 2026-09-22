package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nestify/backend/internal/model"
)

// TestCleanupDetailRecordsDeleteEntries 锁定净化链路的明细口径：
// 净化规则删掉的每个文件 / 文件夹都要落到「删除」动作上（与备份删源、strm 级联删除
// 共用一个统计项），任务详情窗口里的「删除」因此对净化规则同样成立。
func TestCleanupDetailRecordsDeleteEntries(t *testing.T) {
	sourceDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "广告.txt"), "ad-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "keep.mkv"), "keep-bytes")

	// 过期文件：把修改时间拨到 3 天前，保留天数 1 天时它该被删。
	expiredPath := filepath.Join(sourceDir, "old.log")
	writeStrmFixture(t, expiredPath, "old-bytes")
	oldTime := time.Now().AddDate(0, 0, -3)
	if err := os.Chtimes(expiredPath, oldTime, oldTime); err != nil {
		t.Fatalf("修改过期文件时间失败: %v", err)
	}

	emptyDir := filepath.Join(sourceDir, "空目录")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatalf("创建空目录失败: %v", err)
	}

	service := NewService(nil)
	stats, err := service.executeCleanupRule("run-cleanup-detail", ExecuteRuleRequest{
		ArchiveMode: "cleanup",
		SourceDir:   sourceDir,
		Options: map[string]bool{
			"cleanup_matching_files": true,
			"cleanup_empty_dirs":     true,
			"cleanup_expired_files":  true,
		},
		OptionValues: map[string]int{"cleanup_retention_days": 1},
		Filters:      []string{"广告"},
	})
	if err != nil {
		t.Fatalf("executeCleanupRule 出错: %v", err)
	}

	if stats.CleanupRemovedFiles != 2 || stats.CleanupRemovedDirs != 1 {
		t.Fatalf("删除统计 = %d 文件 / %d 目录, want 2 / 1", stats.CleanupRemovedFiles, stats.CleanupRemovedDirs)
	}

	detail := decodeRunDetail(t, &stats)
	if detail.Kind != model.RunDetailKindCleanup {
		t.Fatalf("明细类型 = %q, want %q", detail.Kind, model.RunDetailKindCleanup)
	}
	if got := detail.Counts[model.BackupFileActionDelete]; got != 3 {
		t.Fatalf("counts.delete = %d, want 3", got)
	}

	deletes := detailEntriesByAction(detail, model.BackupFileActionDelete)
	if len(deletes) != 3 {
		t.Fatalf("删除明细条数 = %d, want 3", len(deletes))
	}

	byPath := make(map[string]model.RunFileEntry, len(deletes))
	for _, entry := range deletes {
		byPath[entry.Path] = entry
	}

	matched, ok := byPath[filepath.Join(sourceDir, "广告.txt")]
	if !ok {
		t.Fatalf("命中清理名单的文件没有落明细: %+v", byPath)
	}
	if matched.Dir || matched.Note != deleteNoteMatchedFile {
		t.Fatalf("匹配文件明细 = dir %v / note %q", matched.Dir, matched.Note)
	}

	expired, ok := byPath[expiredPath]
	if !ok {
		t.Fatalf("过期文件没有落明细: %+v", byPath)
	}
	if expired.Note != deleteNoteExpiredFile {
		t.Fatalf("过期文件备注 = %q, want %q", expired.Note, deleteNoteExpiredFile)
	}

	removedDir, ok := byPath[emptyDir]
	if !ok {
		t.Fatalf("被删掉的空目录没有落明细: %+v", byPath)
	}
	if !removedDir.Dir || removedDir.Note != deleteNoteEmptyDir {
		t.Fatalf("空目录明细 = dir %v / note %q", removedDir.Dir, removedDir.Note)
	}

	// 源根路径要带上：前端据此把绝对路径裁成相对路径显示。
	if len(detail.SourceRoots) != 1 || detail.SourceRoots[0] != strings.TrimRight(filepath.ToSlash(sourceDir), "/") {
		t.Fatalf("source_roots = %+v, want [%s]", detail.SourceRoots, filepath.ToSlash(sourceDir))
	}
	if len(detail.TargetRoots) != 0 {
		t.Fatalf("净化链路不该有目标根路径: %+v", detail.TargetRoots)
	}

	// 保留的那份不能被误删。
	if _, err := os.Stat(filepath.Join(sourceDir, "keep.mkv")); err != nil {
		t.Fatalf("未命中的文件应保留: %v", err)
	}
	for _, path := range []string{filepath.Join(sourceDir, "广告.txt"), expiredPath, emptyDir} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s 应已被删除，实际 err=%v", path, err)
		}
	}
}

// TestCleanupDetailEmptyWhenNothingRemoved 锁定「没有删除就没有明细」：
// 一轮什么都没删（整轮跳过）时明细载荷为空，任务详情里不会多出一个空的删除统计项。
func TestCleanupDetailEmptyWhenNothingRemoved(t *testing.T) {
	sourceDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "keep.mkv"), "keep-bytes")

	service := NewService(nil)
	stats, err := service.executeCleanupRule("run-cleanup-empty", ExecuteRuleRequest{
		ArchiveMode: "cleanup",
		SourceDir:   sourceDir,
		Options:     map[string]bool{"cleanup_matching_files": true},
		Filters:     []string{"广告"},
	})
	if err != nil {
		t.Fatalf("executeCleanupRule 出错: %v", err)
	}

	if stats.SkipCount != 1 {
		t.Fatalf("没删到东西时应整轮跳过，实际 skip=%d", stats.SkipCount)
	}
	if stats.SuccessCount != 0 || stats.FailureCount != 0 {
		t.Fatalf("不该有成功 / 失败计数: %+v", stats)
	}
	if payload := stats.buildDetailJSON(); payload != "" {
		t.Fatalf("没有删除时明细载荷应为空，实际 %q", payload)
	}
}

// TestCleanupDetailSkipsUnconfiguredActions 锁定「一个动作都没开」时不写明细：
// 这种情况下整轮跳过，没有删除条目可言。
func TestCleanupDetailSkipsUnconfiguredActions(t *testing.T) {
	sourceDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "广告.txt"), "ad-bytes")

	service := NewService(nil)
	stats, err := service.executeCleanupRule("run-cleanup-off", ExecuteRuleRequest{
		ArchiveMode: "cleanup",
		SourceDir:   sourceDir,
	})
	if err != nil {
		t.Fatalf("executeCleanupRule 出错: %v", err)
	}
	if stats.SkipCount != 1 {
		t.Fatalf("未开启任何清理动作时应整轮跳过，实际 skip=%d", stats.SkipCount)
	}
	if payload := stats.buildDetailJSON(); payload != "" {
		t.Fatalf("未开启清理动作时明细载荷应为空，实际 %q", payload)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "广告.txt")); err != nil {
		t.Fatalf("未开启清理动作时不该删文件: %v", err)
	}
}

// TestCleanupDetailRecordsWhitelistSkip 锁定净化链路的「跳过」口径之一：
// 目录确实是空的、本该被「清理空目录」删掉，却因为目录名在白名单里被留下 ——
// 这是一次真实的跳过，要落一条 skip 明细；非白名单的空目录照删不误。
//
// 为什么只记「确实空了」的白名单目录：非空目录本来就不在空目录清理的范围内
// （规则没让它动），逐条记进去只会凭空多出一堆跳过条目。
func TestCleanupDetailRecordsWhitelistSkip(t *testing.T) {
	sourceDir := t.TempDir()
	guardedDir := filepath.Join(sourceDir, "受保护空目录")
	if err := os.MkdirAll(guardedDir, 0o755); err != nil {
		t.Fatalf("创建受保护空目录失败: %v", err)
	}
	removableDir := filepath.Join(sourceDir, "普通空目录")
	if err := os.MkdirAll(removableDir, 0o755); err != nil {
		t.Fatalf("创建普通空目录失败: %v", err)
	}

	service := NewService(nil)
	stats, err := service.executeCleanupRule("run-cleanup-whitelist", ExecuteRuleRequest{
		ArchiveMode: "cleanup",
		SourceDir:   sourceDir,
		Options:     map[string]bool{"cleanup_empty_dirs": true},
		Whitelist:   []string{"受保护空目录"},
	})
	if err != nil {
		t.Fatalf("executeCleanupRule 出错: %v", err)
	}

	if stats.SkipCount != 1 {
		t.Fatalf("白名单目录应算一次跳过，实际 skip=%d", stats.SkipCount)
	}
	if stats.CleanupRemovedDirs != 1 {
		t.Fatalf("非白名单空目录应被删掉，实际删除目录 %d 个", stats.CleanupRemovedDirs)
	}

	detail := decodeRunDetail(t, &stats)
	if detail.Kind != model.RunDetailKindCleanup {
		t.Fatalf("明细类型 = %q, want %q", detail.Kind, model.RunDetailKindCleanup)
	}
	skips := detailEntriesByAction(detail, model.BackupFileActionSkip)
	if len(skips) != 1 {
		t.Fatalf("跳过明细条数 = %d, want 1", len(skips))
	}
	if skips[0].Path != guardedDir || !skips[0].Dir || skips[0].Note != skipReasonCleanupWhitelist {
		t.Fatalf("白名单目录的跳过明细不对: %+v", skips[0])
	}
	// counts 与条目数恒等：点「跳过」筛出来的条数与统计数字一致。
	if got := detail.Counts[model.BackupFileActionSkip]; got != len(skips) {
		t.Fatalf("counts.skip = %d, 明细条数 = %d", got, len(skips))
	}
	if !strings.Contains(stats.Summary, "跳过 1 项") {
		t.Fatalf("摘要要带出跳过数，实际 %q", stats.Summary)
	}

	if _, err := os.Stat(guardedDir); err != nil {
		t.Fatalf("白名单目录不该被删: %v", err)
	}
	if _, err := os.Stat(removableDir); !os.IsNotExist(err) {
		t.Fatalf("普通空目录应已被删除，实际 err=%v", err)
	}
}

// TestCleanupDetailRecordsReadDirFailure 锁住「读取目录失败」也要落失败明细：
// 运行记录里写了「失败 1 项」，任务详情窗口点「失败」就得能看见是哪条目录读不出来。
func TestCleanupDetailRecordsReadDirFailure(t *testing.T) {
	service := NewService(nil)
	stats := executionStats{}
	stats.detail(model.RunDetailKindCleanup)

	missingDir := filepath.Join(t.TempDir(), "不存在的目录")
	service.cleanupDirectory("run-cleanup-readdir", missingDir, missingDir, "", cleanupPlan{}, &stats)

	if stats.FailureCount != 1 {
		t.Fatalf("读取失败应计入 FailureCount，实际 %d", stats.FailureCount)
	}

	detail := decodeRunDetail(t, &stats)
	failures := detailEntriesByAction(detail, model.BackupFileActionFail)
	if len(failures) != 1 {
		t.Fatalf("失败明细条数 = %d, want 1", len(failures))
	}
	if failures[0].Path != missingDir || !failures[0].Dir {
		t.Fatalf("失败明细不对: %+v", failures[0])
	}
	if !strings.Contains(failures[0].Note, "读取目录失败") {
		t.Fatalf("失败明细要写清原因，实际 %q", failures[0].Note)
	}
}

// TestFileExpiryReportsUnknownStatus 锁定「过期判定」的三态语义：
// 拿不到修改时间要返回 known=false（调用方据此记一条跳过明细，而不是静默放过）；
// 目录与非法保留天数都算「已知且不过期」，别把它们误记成跳过。
func TestFileExpiryReportsUnknownStatus(t *testing.T) {
	if _, known := fileExpiry(filepath.Join(t.TempDir(), "不存在.log"), 1); known {
		t.Fatal("stat 拿不到文件信息时 known 应为 false")
	}

	dir := t.TempDir()
	if expired, known := fileExpiry(dir, 1); !known || expired {
		t.Fatalf("目录 = expired %v / known %v, want false / true", expired, known)
	}

	fresh := filepath.Join(t.TempDir(), "新文件.log")
	writeStrmFixture(t, fresh, "fresh-bytes")
	if expired, known := fileExpiry(fresh, 1); !known || expired {
		t.Fatalf("刚写入的文件 = expired %v / known %v, want false / true", expired, known)
	}
	if expired, known := fileExpiry(fresh, 0); !known || expired {
		t.Fatalf("保留天数非法 = expired %v / known %v, want false / true", expired, known)
	}
}
