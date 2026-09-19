package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nestify/backend/internal/model"
)

// 这三条链路（转换 / 命名 / 软链硬链）此前一次明细都没采集，任务详情窗口因此是空态。
// 现在它们各自建采集器并逐条落明细，共同的口径是：
//   - 成功改完名 / 建完链的条目落「产出型」动作（转换与命名用 move、软链硬链用 link），
//     并带上 Target（改完之后的新路径 / 目标端的链接路径）—— 前端只在成功条目上给第二行；
//   - 没改名、目标被占用、命中过滤名单的落「跳过」；
//   - 报错的落「失败」；
//   - 原地改名（转换 / 命名）没有独立目标端，根路径都取监控目录。

// TestTransformDetailRecordsRenameEntries 锁定转换链路：重命名成功落 move + 新路径。
func TestTransformDetailRecordsRenameEntries(t *testing.T) {
	sourceDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "旧名.txt"), "a")
	writeStrmFixture(t, filepath.Join(sourceDir, "保留.txt"), "b")

	service := NewService(nil)
	stats, err := service.executeTransformRule("run-transform-detail", ExecuteRuleRequest{
		ArchiveMode:    "transform",
		SourceDir:      sourceDir,
		Options:        map[string]bool{"convert_matching_text": true},
		TransformRules: []string{"旧名 => 新名"},
	})
	if err != nil {
		t.Fatalf("executeTransformRule 出错: %v", err)
	}
	if stats.SuccessCount != 1 {
		t.Fatalf("重命名成功数 = %d, want 1", stats.SuccessCount)
	}

	detail := decodeRunDetail(t, &stats)
	if detail.Kind != model.RunDetailKindTransform {
		t.Fatalf("明细类型 = %q, want %q", detail.Kind, model.RunDetailKindTransform)
	}
	if got := detail.Counts[model.RunFileActionMove]; got != 1 {
		t.Fatalf("counts.move = %d, want 1（%+v）", got, detail.Counts)
	}

	moves := detailEntriesByAction(detail, model.RunFileActionMove)
	if len(moves) != 1 {
		t.Fatalf("移动明细条数 = %d, want 1", len(moves))
	}
	oldPath := filepath.Join(sourceDir, "旧名.txt")
	newPath := filepath.Join(sourceDir, "新名.txt")
	if moves[0].Path != oldPath || moves[0].Target != newPath || moves[0].Dir {
		t.Fatalf("重命名明细 = %+v, want path %s / target %s / 文件", moves[0], oldPath, newPath)
	}
	if len(detail.SourceRoots) != 1 || detail.SourceRoots[0] != strings.TrimRight(filepath.ToSlash(sourceDir), "/") {
		t.Fatalf("source_roots = %+v, want [%s]", detail.SourceRoots, filepath.ToSlash(sourceDir))
	}

	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("重命名后的文件不存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "保留.txt")); err != nil {
		t.Fatalf("未命中的文件应保留: %v", err)
	}
}

// TestTransformDetailRecordsDirectoryRename 锁定目录重命名的明细：
// 目录条目要带 dir 标记（第二行切不切最后一段由它决定），新名字同样作为目标。
func TestTransformDetailRecordsDirectoryRename(t *testing.T) {
	sourceDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "旧目录", "A1.mkv"), "a")

	service := NewService(nil)
	stats, err := service.executeTransformRule("run-transform-dir", ExecuteRuleRequest{
		ArchiveMode: "transform",
		SourceDir:   sourceDir,
		Options:     map[string]bool{"convert_matching_text": true},
		// 目录规则必须用 /xxx/ 包裹，否则不会被应用到目录上。
		TransformRules: []string{"/旧目录/ => /新目录/"},
	})
	if err != nil {
		t.Fatalf("executeTransformRule 出错: %v", err)
	}

	detail := decodeRunDetail(t, &stats)
	moves := detailEntriesByAction(detail, model.RunFileActionMove)
	if len(moves) != 1 {
		t.Fatalf("移动明细条数 = %d, want 1（%+v）", len(moves), detail.Files)
	}
	if moves[0].Path != filepath.Join(sourceDir, "旧目录") || moves[0].Target != filepath.Join(sourceDir, "新目录") {
		t.Fatalf("目录重命名明细 = %+v", moves[0])
	}
	if !moves[0].Dir {
		t.Fatalf("目录条目必须带 dir 标记: %+v", moves[0])
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "新目录", "A1.mkv")); err != nil {
		t.Fatalf("改名后目录里的文件应还在: %v", err)
	}
}

// TestNamingDetailRecordsRenameEntries 锁定命名链路：
// 改名成功的落 move + 新名字，没改动的落跳过，新名字被占用的也落跳过（原因不同）。
func TestNamingDetailRecordsRenameEntries(t *testing.T) {
	sourceDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "老片.mkv"), "a")
	writeStrmFixture(t, filepath.Join(sourceDir, "保持.mkv"), "b")
	writeStrmFixture(t, filepath.Join(sourceDir, "老歌.mp3"), "c")
	// 预置冲突目标：老歌.mp3 想改成的 新歌.mp3 已经在了。
	writeStrmFixture(t, filepath.Join(sourceDir, "新歌.mp3"), "d")

	service := NewService(nil)
	stats, err := service.executeNamingRule("run-naming-detail", ExecuteRuleRequest{
		ArchiveMode:    "naming",
		SourceDir:      sourceDir,
		Options:        map[string]bool{"naming_include_files": true},
		TransformRules: []string{`{"category":"replace","config":{"match":"老","replacement":"新"}}`},
	})
	if err != nil {
		t.Fatalf("executeNamingRule 出错: %v", err)
	}
	if stats.SuccessCount != 1 {
		t.Fatalf("命名成功数 = %d, want 1", stats.SuccessCount)
	}
	if stats.SkipCount != 3 {
		t.Fatalf("跳过数 = %d, want 3（保持 / 新歌 无变化，老歌 撞名）", stats.SkipCount)
	}

	detail := decodeRunDetail(t, &stats)
	if detail.Kind != model.RunDetailKindNaming {
		t.Fatalf("明细类型 = %q, want %q", detail.Kind, model.RunDetailKindNaming)
	}
	if got := len(detailEntriesByAction(detail, model.RunFileActionMove)); got != 1 {
		t.Fatalf("移动明细条数 = %d, want 1", got)
	}

	moves := detailEntriesByAction(detail, model.RunFileActionMove)
	if moves[0].Path != filepath.Join(sourceDir, "老片.mkv") || moves[0].Target != filepath.Join(sourceDir, "新片.mkv") {
		t.Fatalf("命名明细 = %+v", moves[0])
	}

	notesByPath := make(map[string]string)
	for _, entry := range detailEntriesByAction(detail, model.BackupFileActionSkip) {
		notesByPath[entry.Path] = entry.Note
	}
	if got := notesByPath[filepath.Join(sourceDir, "保持.mkv")]; got != skipReasonNamingUnchanged {
		t.Fatalf("无变化文件的备注 = %q, want %q", got, skipReasonNamingUnchanged)
	}
	if got := notesByPath[filepath.Join(sourceDir, "老歌.mp3")]; got != skipReasonNamingConflict {
		t.Fatalf("撞名文件的备注 = %q, want %q", got, skipReasonNamingConflict)
	}

	if _, err := os.Stat(filepath.Join(sourceDir, "新片.mkv")); err != nil {
		t.Fatalf("命名后的文件不存在: %v", err)
	}
}

// TestLinkDetailRecordsLinkEntries 锁定软链 / 硬链链路：
// 建链成功落 link 动作 + 目标端链接路径（前端据此给第二行），
// 命中过滤名单与目标已存在的落跳过。
func TestLinkDetailRecordsLinkEntries(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "A1.mkv"), "video")
	writeStrmFixture(t, filepath.Join(sourceDir, "广告.txt"), "ad")
	writeStrmFixture(t, filepath.Join(sourceDir, "已存在.mkv"), "exists")
	// 目标端预先放好同名文件，让那条走「跳过」。
	writeStrmFixture(t, filepath.Join(targetDir, "已存在.mkv"), "target-exists")

	service := NewService(nil)
	stats, err := service.executeLinkRule("run-link-detail", ExecuteRuleRequest{
		ArchiveMode: "link",
		LinkMode:    "hard",
		SourceDir:   sourceDir,
		TargetDir:   targetDir,
		Filters:     []string{"广告"},
	})
	if err != nil {
		t.Fatalf("executeLinkRule 出错: %v", err)
	}
	if stats.SuccessCount != 1 {
		t.Fatalf("建链成功数 = %d, want 1", stats.SuccessCount)
	}
	if stats.SkipCount != 2 {
		t.Fatalf("跳过数 = %d, want 2（命中过滤名单 / 目标已存在）", stats.SkipCount)
	}

	detail := decodeRunDetail(t, &stats)
	if detail.Kind != model.RunDetailKindLink {
		t.Fatalf("明细类型 = %q, want %q", detail.Kind, model.RunDetailKindLink)
	}
	if got := detail.Counts[model.RunFileActionLink]; got != 1 {
		t.Fatalf("counts.link = %d, want 1（%+v）", got, detail.Counts)
	}

	links := detailEntriesByAction(detail, model.RunFileActionLink)
	if len(links) != 1 {
		t.Fatalf("链路明细条数 = %d, want 1", len(links))
	}
	if links[0].Path != filepath.Join(sourceDir, "剧集", "A1.mkv") || links[0].Target != filepath.Join(targetDir, "剧集", "A1.mkv") {
		t.Fatalf("链路明细 = %+v", links[0])
	}
	if links[0].Dir {
		t.Fatalf("文件条目不该标记为目录: %+v", links[0])
	}

	notes := make(map[string]string)
	for _, entry := range detailEntriesByAction(detail, model.BackupFileActionSkip) {
		notes[filepath.Base(entry.Path)] = entry.Note
	}
	if got := notes["广告.txt"]; got != skipReasonLinkFiltered {
		t.Fatalf("过滤名单跳过的备注 = %q, want %q", got, skipReasonLinkFiltered)
	}
	if got := notes["已存在.mkv"]; got != skipReasonLinkExisting {
		t.Fatalf("目标已存在跳过的备注 = %q, want %q", got, skipReasonLinkExisting)
	}

	// 源 / 目标根都要带上：前端据此裁剪第一行、第二行给完整目标路径。
	if len(detail.SourceRoots) != 1 || detail.SourceRoots[0] != strings.TrimRight(filepath.ToSlash(sourceDir), "/") {
		t.Fatalf("source_roots = %+v", detail.SourceRoots)
	}
	if len(detail.TargetRoots) != 1 || detail.TargetRoots[0] != strings.TrimRight(filepath.ToSlash(targetDir), "/") {
		t.Fatalf("target_roots = %+v", detail.TargetRoots)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "剧集", "A1.mkv")); err != nil {
		t.Fatalf("目标端的链接不存在: %v", err)
	}
}
