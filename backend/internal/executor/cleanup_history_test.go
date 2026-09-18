package executor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
	"nestify/backend/internal/store/sqlite"
)

// 这组用例走「真实落库」的完整链路：建一条清理模式规则 → 用自动触发同款映射执行 →
// 按前端折叠表的取法（view_mode=tree，组内取明细最完整的那一份）回读明细。
//
// 任务详情窗口里的「删除」统计项完全依赖这条链路：只有 detail_json 里真的有 delete 动作，
// 净化规则的详情窗口才会多出「删除 N」这一项（净化链路与 strm 级联删除、备份删源共用一个口）。

func openRunHistoryTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.Open(config.Env{
		DBPath:               filepath.Join(t.TempDir(), "app.db"),
		AdminInitialUsername: "admin",
		AdminInitialPassword: "nestify123",
	})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// waitRunSettled 等自动/手动触发的那次执行跑完（PrepareRuleRun 是异步的）。
func waitRunSettled(t *testing.T, service *Service, runID string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		run, ok := service.GetRun(runID)
		if ok && run != nil && run.Status != model.RunStatusRunning && run.Status != model.RunStatusPending {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等待执行结束超时: %s", runID)
}

// assertRunSucceeded 断言这次执行整体成功。
//
// 多监控目录的规则曾经把 source_dir 里的 JSON 数组串当成一个路径，于是每次执行都多跑
// 一个必然失败的「监控目录」、记录里凭空多一条失败；这条断言把它钉住。
func assertRunSucceeded(t *testing.T, service *Service, runID string) {
	t.Helper()
	run, ok := service.GetRun(runID)
	if !ok || run == nil {
		t.Fatalf("运行实例不存在: %s", runID)
	}
	if run.Status != model.RunStatusSucceeded || run.FailureCount != 0 {
		t.Fatalf("执行状态 = %s / 失败 %d，want succeeded / 0", run.Status, run.FailureCount)
	}
}

// representativedetailFromHistory 取前端折叠表真正会用的那一份明细：
// 列表走 `view_mode=tree`（组内按明细长度降序），第一条即该次执行的代表行；
// 详情弹窗再按它的 id 去拉 detail_json。
func representativeDetailFromHistory(t *testing.T, store *sqlite.Store, ruleType string) model.RunDetail {
	t.Helper()
	items, total, err := store.ListRunHistoryGroupPage(1, 25, "", "", "", ruleType, "", "")
	if err != nil {
		t.Fatalf("读取分组运行历史失败: %v", err)
	}
	if total == 0 || len(items) == 0 {
		t.Fatal("运行历史为空：清理模式执行没有落任何记录")
	}

	// 组内第一条即代表行（列表接口在 HTTP 层统一清空明细，这边直接按 id 重新拉一次，
	// 与详情弹窗取数一致）。
	representative := items[0]
	item, err := store.GetRunHistoryByID(representative.ID)
	if err != nil {
		t.Fatalf("按 id 拉取明细失败: %v", err)
	}
	if item.DetailJSON == "" {
		t.Fatalf("代表行的 detail_json 为空：详情窗口不会出现任何明细与「删除」统计项（summary=%q）", item.Summary)
	}

	var detail model.RunDetail
	if err := json.Unmarshal([]byte(item.DetailJSON), &detail); err != nil {
		t.Fatalf("明细载荷不是合法 JSON: %v", err)
	}
	return detail
}

func createCleanupRule(t *testing.T, store *sqlite.Store, name string, sourceDirs []string) model.Rule {
	t.Helper()
	rule, err := store.CreateRule(model.CreateRuleInput{
		Name:        name,
		ArchiveMode: "cleanup",
		RuleType:    "cleanup",
		SourceDir:   sourceDirs[0],
		SourceDirs:  sourceDirs,
		Options: map[string]bool{
			"cleanup_matching_files": true,
			"cleanup_empty_dirs":     true,
		},
		// 「*」前缀才是模糊匹配：不带星号是「精确等于文件名主干」，两者别混。
		Filters: []string{"*广告"},
	})
	if err != nil {
		t.Fatalf("创建清理模式规则失败: %v", err)
	}
	return *rule
}

// TestCleanupDeleteDetailLandsInRunHistory 单个监控目录：清理模式的删除明细要真的落进
// run_history.detail_json，且前端折叠组取到的那一份就带着它。
func TestCleanupDeleteDetailLandsInRunHistory(t *testing.T) {
	store := openRunHistoryTestStore(t)

	sourceDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "广告.txt"), "ad")
	writeStrmFixture(t, filepath.Join(sourceDir, "保留.mkv"), "keep")
	if err := os.MkdirAll(filepath.Join(sourceDir, "空目录"), 0o755); err != nil {
		t.Fatalf("创建空目录失败: %v", err)
	}

	rule := createCleanupRule(t, store, "清理模式-单目录", []string{sourceDir})
	service := NewService(store)
	run, err := service.PrepareRuleRun(buildRuleExecuteRequest(rule, model.TriggerModeManual))
	if err != nil {
		t.Fatalf("启动清理模式执行失败: %v", err)
	}
	waitRunSettled(t, service, run.ID)
	assertRunSucceeded(t, service, run.ID)

	detail := representativeDetailFromHistory(t, store, "cleanup")
	if detail.Kind != model.RunDetailKindCleanup {
		t.Fatalf("明细类型 = %q, want %q", detail.Kind, model.RunDetailKindCleanup)
	}
	if got := detail.Counts[model.BackupFileActionDelete]; got != 2 {
		t.Fatalf("counts.delete = %d, want 2（命中清理名单的文件 + 空目录）", got)
	}

	deletes := detailEntriesByAction(detail, model.BackupFileActionDelete)
	if len(deletes) != 2 {
		t.Fatalf("删除明细条数 = %d, want 2", len(deletes))
	}
	paths := make(map[string]model.RunFileEntry, len(deletes))
	for _, entry := range deletes {
		paths[entry.Path] = entry
	}
	if _, ok := paths[filepath.Join(sourceDir, "广告.txt")]; !ok {
		t.Fatalf("命中清理名单的文件没落明细: %+v", paths)
	}
	emptyDir, ok := paths[filepath.Join(sourceDir, "空目录")]
	if !ok || !emptyDir.Dir {
		t.Fatalf("空目录明细缺失或没标成目录: %+v", emptyDir)
	}
	if len(detail.SourceRoots) == 0 || detail.SourceRoots[0] != filepath.ToSlash(sourceDir) {
		t.Fatalf("source_roots 应带监控目录（前端据此裁成相对路径），实际 %+v", detail.SourceRoots)
	}
}

// TestCleanupDeleteDetailCoversAllSourceDirs 多个监控目录：详情窗口取的是「组内最完整的那一份」，
// 所以跨目录的删除必须并进同一份明细 —— 否则用户点开详情只看到条目最多的那个目录，
// 其它目录的删除条目凭空消失。
func TestCleanupDeleteDetailCoversAllSourceDirs(t *testing.T) {
	store := openRunHistoryTestStore(t)

	dirA := t.TempDir()
	dirB := t.TempDir()
	writeStrmFixture(t, filepath.Join(dirA, "广告A.txt"), "ad-a")
	writeStrmFixture(t, filepath.Join(dirA, "保留A.mkv"), "keep-a")
	if err := os.MkdirAll(filepath.Join(dirA, "空目录A"), 0o755); err != nil {
		t.Fatalf("创建空目录失败: %v", err)
	}
	writeStrmFixture(t, filepath.Join(dirB, "广告B.txt"), "ad-b")
	writeStrmFixture(t, filepath.Join(dirB, "保留B.mkv"), "keep-b")

	rule := createCleanupRule(t, store, "清理模式-多目录", []string{dirA, dirB})
	service := NewService(store)
	run, err := service.PrepareRuleRun(buildRuleExecuteRequest(rule, model.TriggerModeManual))
	if err != nil {
		t.Fatalf("启动清理模式执行失败: %v", err)
	}
	waitRunSettled(t, service, run.ID)
	assertRunSucceeded(t, service, run.ID)

	detail := representativeDetailFromHistory(t, store, "cleanup")
	if got := detail.Counts[model.BackupFileActionDelete]; got != 3 {
		t.Fatalf("counts.delete = %d, want 3（两个目录的删除都要在代表行里）", got)
	}

	deletes := detailEntriesByAction(detail, model.BackupFileActionDelete)
	seen := make(map[string]bool, len(deletes))
	for _, entry := range deletes {
		seen[entry.Path] = true
	}
	for _, want := range []string{
		filepath.Join(dirA, "广告A.txt"),
		filepath.Join(dirA, "空目录A"),
		filepath.Join(dirB, "广告B.txt"),
	} {
		if !seen[want] {
			t.Fatalf("跨监控目录的删除条目丢了 %s（实际 %+v）", want, seen)
		}
	}

	// 根路径同样要合起来：前端靠它把两个目录的绝对路径都裁成相对路径。
	roots := make(map[string]bool, len(detail.SourceRoots))
	for _, root := range detail.SourceRoots {
		roots[root] = true
	}
	for _, want := range []string{filepath.ToSlash(dirA), filepath.ToSlash(dirB)} {
		if !roots[want] {
			t.Fatalf("source_roots 缺 %s（实际 %+v）", want, detail.SourceRoots)
		}
	}
}

// TestNormalizeExecuteSourceDirsExpandsJSONArray 锁定源目录归一化的一条硬规则：
// 规则表里多监控目录存成 `["a","b"]`（source_dir 字段），而自动触发与手动执行都会把它
// 原样塞进请求 —— 必须展开成两个目录，不能当成一个路径（否则会多跑一个必然失败的监控目录）。
func TestNormalizeExecuteSourceDirsExpandsJSONArray(t *testing.T) {
	encoded := `["D:/media/a","D:/media/b"]`

	got := normalizeExecuteSourceDirs(encoded, []string{"D:/media/a", "D:/media/b"})
	want := []string{"D:/media/a", "D:/media/b"}
	if len(got) != len(want) {
		t.Fatalf("源目录 = %+v, want %+v（JSON 数组串不该被当成一个路径）", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("源目录[%d] = %q, want %q（实际 %+v）", index, got[index], want[index], got)
		}
	}

	// 单个目录仍然是普通路径，不能因为以方括号开头就被误展开。
	single := normalizeExecuteSourceDirs("D:/media/a", nil)
	if len(single) != 1 || single[0] != "D:/media/a" {
		t.Fatalf("单目录归一化结果 = %+v", single)
	}
}
