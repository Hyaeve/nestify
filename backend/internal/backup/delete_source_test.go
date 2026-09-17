package backup

import (
	"os"
	"path/filepath"
	"testing"

	"nestify/backend/internal/model"
)

// 「删除源文件」类完成规则必须满足「所有配置目标都可用」才允许执行：
// 目标解析失败 / 挂载停用会被剔除出 targets，此时源文件并没有落到每个目标。
func TestCanDeleteSourceAfterCopy(t *testing.T) {
	cases := []struct {
		name          string
		rule          string
		usable        int
		configured    int
		wantCanDelete bool
		note          string
	}{
		{"无操作不删源", model.BackupCompletionNone, 1, 1, false, "完成规则为 none 时绝不删源"},
		{"单源单目标可删", model.BackupCompletionDeleteSource, 1, 1, true, "标准场景"},
		{"多目标全部可用才可删", model.BackupCompletionDeleteSourceDirs, 3, 3, true, "多目标全部就绪"},
		{"多目标缺一个不可删", model.BackupCompletionDeleteSource, 2, 3, false, "有一个目标不可用，源文件不能删"},
		{"目标全部不可用不可删", model.BackupCompletionDeleteSource, 0, 2, false, "全部目标不可用"},
		{"未配目标不可删", model.BackupCompletionDeleteSource, 0, 0, false, "没有目标就谈不上备份完成"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := canDeleteSourceAfterCopy(tc.rule, tc.usable, tc.configured)
			if got != tc.wantCanDelete {
				t.Fatalf("%s: got %v want %v", tc.note, got, tc.wantCanDelete)
			}
		})
	}
}

// 待删清单里的源文件在整轮复制结束后被统一删除，且逐条记入运行明细。
func TestDeleteBackedUpSourcesRemovesPendingFiles(t *testing.T) {
	root := t.TempDir()
	srcA := filepath.Join(root, "srcA")
	srcB := filepath.Join(root, "srcB")
	for _, dir := range []string{srcA, srcB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("建目录失败：%v", err)
		}
	}

	// 模拟两个源各自贡献一个文件（多源多目标场景下，它们的源文件都要等全部写完再删）。
	fileA := filepath.Join(srcA, "电影", "a.mkv")
	fileB := filepath.Join(srcB, "剧集", "b.mkv")
	for _, f := range []string{fileA, fileB} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatalf("建目录失败：%v", err)
		}
		if err := os.WriteFile(f, []byte("payload"), 0o644); err != nil {
			t.Fatalf("写测试文件失败：%v", err)
		}
	}

	// 留在源目录里、不在待删清单中的文件必须原样保留（只有备份成功的才允许删）。
	keep := filepath.Join(srcA, "电影", "keep.mkv")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatalf("写测试文件失败：%v", err)
	}

	stats := &runStats{}
	svc := &Service{}
	svc.deleteBackedUpSources(model.BackupTask{ID: 1}, []pendingSourceDelete{
		{relative: "srcA/电影/a.mkv", absolute: fileA, size: 7},
		{relative: "srcB/剧集/b.mkv", absolute: fileB, size: 7},
	}, stats)

	for _, f := range []string{fileA, fileB} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("源文件应已删除：%s（err=%v）", f, err)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("不在待删清单里的文件必须保留：%v", err)
	}
	if stats.Deleted != 2 {
		t.Fatalf("删除计数应为 2：got %d", stats.Deleted)
	}
	if stats.Failed != 0 {
		t.Fatalf("不应有失败：got %d", stats.Failed)
	}
	if got := stats.fileCounts[model.BackupFileActionDelete]; got != 2 {
		t.Fatalf("明细应记 2 条删除：got %d", got)
	}
	for _, entry := range stats.Files {
		if entry.Action != model.BackupFileActionDelete {
			t.Fatalf("明细动作应为 delete：%+v", entry)
		}
		if entry.Note != "按完成规则删除源文件" {
			t.Fatalf("明细说明不符：%q", entry.Note)
		}
	}
}

// 删除失败（文件已被移走 / 被占用）不能静默吞掉：要计入失败并在明细里可见。
func TestDeleteBackedUpSourcesReportsFailure(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.mkv")

	stats := &runStats{}
	svc := &Service{}
	svc.deleteBackedUpSources(model.BackupTask{ID: 2}, []pendingSourceDelete{
		{relative: "missing.mkv", absolute: missing, size: 1},
	}, stats)

	if stats.Deleted != 0 {
		t.Fatalf("没有文件被删掉：got deleted=%d", stats.Deleted)
	}
	if stats.Failed != 1 {
		t.Fatalf("删除失败应计入失败：got %d", stats.Failed)
	}
	if got := stats.fileCounts[model.BackupFileActionFail]; got != 1 {
		t.Fatalf("明细应记 1 条失败：got %d", got)
	}
}

// 回归：按完成规则删掉源文件后，目标端那份**本轮刚写好的备份不能被「从目标同步删除」清掉**。
// 源端条目必须留在 sourceIndex 里，否则紧随其后的同步删除会把它当成陈旧文件。
func TestSyncDeleteMissingKeepsFilesBackedUpThisRun(t *testing.T) {
	target := t.TempDir()
	backedUp := filepath.Join(target, "电影", "a.mkv")
	stale := filepath.Join(target, "电影", "stale.mkv")
	for _, f := range []string{backedUp, stale} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatalf("建目录失败：%v", err)
		}
		if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
			t.Fatalf("写测试文件失败：%v", err)
		}
	}

	// 源文件已被完成规则删除，但清单里仍然保留条目（源端删了 ≠ 目标端不要了）。
	sourceIndex := map[string]string{
		filepath.Join("电影", "a.mkv"): "/source/电影/a.mkv",
	}

	stats := &runStats{}
	svc := &Service{}
	svc.syncDeleteMissing(target, sourceIndex, newFilterMatcher(nil), stats)

	if _, err := os.Stat(backedUp); err != nil {
		t.Fatalf("本轮刚备份的文件必须保留在目标端：%v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("源中已不存在的文件应从目标删除（err=%v）", err)
	}
	if stats.Deleted != 1 {
		t.Fatalf("同步删除只应删掉陈旧文件：got %d", stats.Deleted)
	}
}
