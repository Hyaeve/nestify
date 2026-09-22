package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
)

// 锁住「任务粒度列表」：一行一个任务，回的必须是该次执行的**代表行**。
//
// 背景：run_history 是「每处理一项写一行」，记录行数不等于执行次数。
// 仪表盘「执行摘要」要的是「最近执行了哪些任务」，而且点开要能看到该次执行的完整明细 ——
// 明细只在收尾那条记录上，所以任务条目必须指向组内明细最完整的那一条；同时**不能**把
// 组内其余记录行一起带回前端（仪表盘每 5 秒拉一次，一次上万项的执行就是上万行）。
func TestListRunHistoryTaskPageReturnsOneRepresentativeRowPerRun(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "nestify-test.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	ruleID := int64(7)
	firstRun := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	secondRun := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	thirdRun := time.Now().UTC().Truncate(time.Second)
	firstDetail := `{"kind":"cleanup","files":[{"path":"/media/a.mkv","action":"delete"}],"counts":{"delete":1},"files_total":1}`
	secondDetail := `{"kind":"cleanup","files":[{"path":"/media/b.mkv","action":"delete"}],"counts":{"delete":2},"files_total":2}`
	backupDetail := `{"kind":"backup","files":[{"path":"/media/c.mkv","action":"upload"}],"counts":{"upload":1},"files_total":1}`

	// 一次净化执行会落多行：逐项记录不带明细，收尾再补一条带完整明细的。
	items := []model.RunHistoryItem{
		newTaskHistoryItem("cleanup-a-1", &ruleID, "净化任务", "cleanup", firstRun, 1, "", ""),
		newTaskHistoryItem("cleanup-a-2", &ruleID, "净化任务", "cleanup", firstRun, 2, "", ""),
		newTaskHistoryItem("cleanup-a-3", &ruleID, "净化任务", "cleanup", firstRun, 3, firstDetail, "已删除 3 个文件"),
		newTaskHistoryItem("cleanup-b-1", &ruleID, "净化任务", "cleanup", secondRun, 1, "", ""),
		newTaskHistoryItem("cleanup-b-2", &ruleID, "净化任务", "cleanup", secondRun, 2, secondDetail, "已删除 2 个文件"),
		// 另一种模式的任务：确认任务之间不会串组。
		newTaskHistoryItem("backup-c-1", nil, "备份任务", "backup", thirdRun, 1, backupDetail, "备份完成"),
	}
	for _, item := range items {
		if err := store.UpsertRunHistory(item); err != nil {
			t.Fatalf("upsert %s: %v", item.ID, err)
		}
	}

	// 对照组：分组视图会把组内每一行都回给前端（运行日志页靠它数「共几条明细」）。
	grouped, groupedTotal, err := store.ListRunHistoryGroupPage(1, 50, "", "", "", "", "modified_at", "desc")
	if err != nil {
		t.Fatalf("list grouped run history: %v", err)
	}
	if groupedTotal != 3 || len(grouped) != len(items) {
		t.Fatalf("分组视图应是 3 个任务 / %d 行记录，实际 %d / %d", len(items), groupedTotal, len(grouped))
	}

	tasks, total, err := store.ListRunHistoryTaskPage(1, 50, "", "", "", "", "modified_at", "desc")
	if err != nil {
		t.Fatalf("list run history tasks: %v", err)
	}
	if total != 3 {
		t.Fatalf("任务数应是执行次数 3，实际 %d", total)
	}
	if len(tasks) != 3 {
		t.Fatalf("任务视图每个任务应只回一行，实际 %d 行", len(tasks))
	}

	// 最新在最前，且拿到的都是「明细最完整的那条」。
	if tasks[0].ID != "backup-c-1" || tasks[1].ID != "cleanup-b-2" || tasks[2].ID != "cleanup-a-3" {
		t.Fatalf("任务视图应回各组代表行且按开始时间倒序，实际 %s / %s / %s",
			tasks[0].ID, tasks[1].ID, tasks[2].ID)
	}
	// 代表行的计数是该次执行的**最终**统计（收尾那条），不是中途快照。
	if tasks[2].ProcessedFiles != 3 {
		t.Fatalf("代表行应带最终统计，实际 processed_files=%d", tasks[2].ProcessedFiles)
	}
	// 列表类查询一律不回明细：详情弹窗按 id 单条拉。
	for _, task := range tasks {
		if task.DetailJSON != "" {
			t.Fatalf("任务视图不应回明细，实际 %q", task.DetailJSON)
		}
	}

	// 点开任务 = 拿代表行的 id 去拉单条详情，必须能拿到整份明细。
	representative, err := store.GetRunHistoryByID(tasks[1].ID)
	if err != nil {
		t.Fatalf("get representative run history: %v", err)
	}
	if representative.DetailJSON != secondDetail {
		t.Fatalf("代表行应带完整明细（详情弹窗的数据源），实际 %q", representative.DetailJSON)
	}
}

// newTaskHistoryItem 拼一条运行记录：detail 为空表示「逐项记录」，带 detail 的是收尾那条。
func newTaskHistoryItem(id string, ruleID *int64, ruleName, archiveMode string, startedAt time.Time, processed int, detail, summary string) model.RunHistoryItem {
	return model.RunHistoryItem{
		ID:             id,
		RuleID:         ruleID,
		RuleName:       ruleName,
		TriggerMode:    model.TriggerModeManual,
		ArchiveMode:    archiveMode,
		Status:         "success",
		ProcessedFiles: processed,
		SuccessCount:   processed,
		Summary:        summary,
		DetailJSON:     detail,
		StartedAt:      startedAt,
		UpdatedAt:      startedAt.Add(time.Duration(processed) * time.Second),
	}
}
