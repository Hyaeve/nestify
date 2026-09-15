package sqlite

import (
	"path/filepath"
	"testing"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
)

// ListMountReferences 是「删除挂载前的引用提示」的数据来源：
// 规则看源/目标路径，备份任务看源/目标目录；不相干挂载的路径不能被算进来。
func TestListMountReferences(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "nestify-test.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	first, err := store.CreateMount(model.CreateMountInput{Name: "一号挂载", Host: "10.0.0.31", Port: 5244})
	if err != nil {
		t.Fatalf("create first mount: %v", err)
	}
	second, err := store.CreateMount(model.CreateMountInput{Name: "二号挂载", Host: "10.0.0.32", Port: 5244})
	if err != nil {
		t.Fatalf("create second mount: %v", err)
	}

	if _, err := store.CreateRule(model.CreateRuleInput{
		Name:        "引用一号的链路规则",
		ArchiveMode: "link",
		RuleType:    "link",
		LinkMode:    "strm",
		RunMode:     "cron",
		SourceDir:   model.JoinMountVirtualPath(first.ID, "/移动云盘/电视剧"),
		TargetDir:   "/data/target",
		Filters:     []string{"mp4"},
	}); err != nil {
		t.Fatalf("create first rule: %v", err)
	}

	if _, err := store.CreateRule(model.CreateRuleInput{
		Name:        "目标写回二号挂载的规则",
		ArchiveMode: "link",
		RuleType:    "link",
		LinkMode:    "strm",
		RunMode:     "cron",
		SourceDir:   model.BuildMountVirtualPath(second.ID),
		TargetDir:   "/data/target",
		Filters:     []string{"mp4"},
	}); err != nil {
		t.Fatalf("create second rule: %v", err)
	}

	if _, err := store.CreateRule(model.CreateRuleInput{
		Name:        "与挂载无关的归档规则",
		ArchiveMode: "package",
		RuleType:    "archive",
		RunMode:     "cron",
		SourceDir:   "/data/media",
		TargetDir:   "/data/archive",
		Filters:     []string{"mp4"},
	}); err != nil {
		t.Fatalf("create unrelated rule: %v", err)
	}

	if _, err := store.CreateBackup(model.CreateBackupInput{
		Name:       "备份到一号挂载",
		SourceDirs: []string{"/data/media"},
		TargetDirs: []string{model.BuildMountVirtualPath(first.ID)},
	}); err != nil {
		t.Fatalf("create first backup: %v", err)
	}

	if _, err := store.CreateBackup(model.CreateBackupInput{
		Name:       "本地备份",
		SourceDirs: []string{"/data/media"},
		TargetDirs: []string{"/data/backup"},
	}); err != nil {
		t.Fatalf("create unrelated backup: %v", err)
	}

	firstUsage, err := store.ListMountReferences(first.ID)
	if err != nil {
		t.Fatalf("ListMountReferences(first): %v", err)
	}
	assertNames(t, "一号挂载规则", firstUsage.RuleNames, []string{"引用一号的链路规则"})
	assertNames(t, "一号挂载备份", firstUsage.BackupNames, []string{"备份到一号挂载"})

	secondUsage, err := store.ListMountReferences(second.ID)
	if err != nil {
		t.Fatalf("ListMountReferences(second): %v", err)
	}
	assertNames(t, "二号挂载规则", secondUsage.RuleNames, []string{"目标写回二号挂载的规则"})
	assertNames(t, "二号挂载备份", secondUsage.BackupNames, []string{})

	// 删除一号挂载后，它的引用不该再算到二号挂载头上（编号不重排的另一面：不能串号）。
	if err := store.DeleteMount(first.ID); err != nil {
		t.Fatalf("delete mount: %v", err)
	}
	afterDelete, err := store.ListMountReferences(second.ID)
	if err != nil {
		t.Fatalf("ListMountReferences(second) after delete: %v", err)
	}
	assertNames(t, "删除一号后二号挂载规则", afterDelete.RuleNames, []string{"目标写回二号挂载的规则"})
}

func assertNames(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s：得到 %v，期望 %v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s：得到 %v，期望 %v", label, got, want)
		}
	}
}
