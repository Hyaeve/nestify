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
