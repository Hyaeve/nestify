package backup

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"nestify/backend/internal/model"
)

// 测试用的备份任务骨架：单源单目标 + 完成规则「删除源文件」。
func newRetryTestTask(id int64, source, target string) model.BackupTask {
	return model.BackupTask{
		ID:             id,
		Name:           "重试测试",
		Enabled:        true,
		SourceDirs:     []string{source},
		TargetDirs:     []string{target},
		CompletionRule: model.BackupCompletionDeleteSource,
	}
}

// runExecute 在测试里直接驱动 execute：不用 store（recordRunHistory 会自行跳过），
// 但必须先给 states 放一个 Running 的状态位，否则 log / setPhase 无处可写。
func runExecute(svc *Service, task model.BackupTask) {
	svc.mu.Lock()
	svc.states[task.ID] = &taskState{Running: true, Status: "running"}
	svc.mu.Unlock()

	svc.execute(task, false, model.TriggerModeManual)
}

type stateSnapshot struct {
	Failed    int
	Copied    int
	Deleted   int
	Status    string
	Phase     string
	LogsCount int
}

func snapshotState(svc *Service, id int64) stateSnapshot {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	state := svc.states[id]
	if state == nil {
		return stateSnapshot{}
	}
	return stateSnapshot{
		Failed:    state.Failed,
		Copied:    state.Copied,
		Deleted:   state.Deleted,
		Status:    state.Status,
		Phase:     state.Phase,
		LogsCount: len(state.RecentLogs),
	}
}

// 失败重试要把备份间隔压到毫秒级，10 分钟在测试里等不起。
func useFastRetryInterval(t *testing.T) {
	t.Helper()
	original := backupRetryInterval
	backupRetryInterval = time.Millisecond
	t.Cleanup(func() { backupRetryInterval = original })
}

// 首轮失败、重试成功的文件算「已备份」：本次执行不能判成失败，
// 而且补写成功后必须照常按完成规则删掉源文件。
func TestRetryRecoversFailedTargetAndStillDeletesSource(t *testing.T) {
	useFastRetryInterval(t)

	root := t.TempDir()
	source := filepath.Join(root, "src")
	target := filepath.Join(root, "dst")
	writeTestFile(t, filepath.Join(source, "电影", "a.mkv"))
	writeTestFile(t, filepath.Join(source, "剧集", "b.mkv"))

	// 第一次复制「目标端抖动」，之后的尝试放行。
	var mu sync.Mutex
	calls := 0
	copyToTargetHook = func(item sourceItem, target targetDesc) error {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return errors.New("模拟目标端抖动")
		}
		return nil
	}
	t.Cleanup(func() { copyToTargetHook = nil })

	svc := &Service{states: make(map[int64]*taskState)}
	task := newRetryTestTask(1, source, target)
	runExecute(svc, task)

	got := snapshotState(svc, 1)
	if got.Failed != 0 {
		t.Fatalf("重试成功的文件不该计入失败：got %d", got.Failed)
	}
	if got.Copied != 2 {
		t.Fatalf("两个文件最终都应备份成功：got %d", got.Copied)
	}
	if got.Deleted != 2 {
		t.Fatalf("重试成功后仍应按完成规则删除源文件：got %d", got.Deleted)
	}
	if calls != 3 {
		t.Fatalf("应为首轮 2 次 + 重试 1 次：got %d", calls)
	}

	for _, relative := range []string{filepath.Join("电影", "a.mkv"), filepath.Join("剧集", "b.mkv")} {
		if _, err := os.Stat(filepath.Join(target, relative)); err != nil {
			t.Fatalf("目标端应有副本：%s（%v）", relative, err)
		}
		if _, err := os.Stat(filepath.Join(source, relative)); !os.IsNotExist(err) {
			t.Fatalf("源文件应已删除：%s（err=%v）", relative, err)
		}
	}
}

// 重试全部失败的文件：保留源文件、逐条记入明细、整体判成失败；
// 同一个任务里已经备份成功的文件不受牵连，照常删除。
func TestRetryKeepsSourceAndRecordsFinalFailure(t *testing.T) {
	useFastRetryInterval(t)

	root := t.TempDir()
	source := filepath.Join(root, "src")
	target := filepath.Join(root, "dst")
	writeTestFile(t, filepath.Join(source, "ok.mkv"))
	writeTestFile(t, filepath.Join(source, "bad.mkv"))

	copyToTargetHook = func(item sourceItem, target targetDesc) error {
		if item.relative == "bad.mkv" {
			return errors.New("目标端不可写")
		}
		return nil
	}
	t.Cleanup(func() { copyToTargetHook = nil })

	svc := &Service{states: make(map[int64]*taskState)}
	task := newRetryTestTask(2, source, target)
	runExecute(svc, task)

	got := snapshotState(svc, 2)
	if got.Failed != 1 {
		t.Fatalf("重试后仍失败的文件计 1 个：got %d", got.Failed)
	}
	if got.Deleted != 1 {
		t.Fatalf("只有备份成功的那个源文件可删：got %d", got.Deleted)
	}
	if got.Status != "failed" {
		t.Fatalf("存在最终失败时整体状态应为 failed：got %q", got.Status)
	}

	if _, err := os.Stat(filepath.Join(source, "bad.mkv")); err != nil {
		t.Fatalf("最终失败的文件必须保留在源目录：%v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "ok.mkv")); !os.IsNotExist(err) {
		t.Fatalf("备份成功的源文件应已删除（err=%v）", err)
	}
}

// 等待重试期间点「停止」：立刻放弃剩余轮次，且不做任何删除收尾
// （这一轮的结果并不完整，继续删源就是数据丢失）。
func TestRetryWaitHonoursCancelRequest(t *testing.T) {
	original := backupRetryInterval
	backupRetryInterval = 200 * time.Millisecond
	t.Cleanup(func() { backupRetryInterval = original })

	root := t.TempDir()
	source := filepath.Join(root, "src")
	target := filepath.Join(root, "dst")
	writeTestFile(t, filepath.Join(source, "a.mkv"))

	copyToTargetHook = func(item sourceItem, target targetDesc) error {
		return errors.New("目标端不可写")
	}
	t.Cleanup(func() { copyToTargetHook = nil })

	svc := &Service{states: make(map[int64]*taskState)}
	task := newRetryTestTask(3, source, target)

	svc.mu.Lock()
	svc.states[task.ID] = &taskState{Running: true, Status: "running"}
	svc.mu.Unlock()

	done := make(chan struct{})
	go func() {
		svc.execute(task, false, model.TriggerModeManual)
		close(done)
	}()

	// 等执行进入等待重试的阶段后再请求停止。
	time.Sleep(30 * time.Millisecond)
	svc.mu.Lock()
	svc.states[task.ID].CancelRequested = true
	svc.mu.Unlock()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("停止请求应让等待重试立刻结束，而不是干等一个完整间隔")
	}

	got := snapshotState(svc, 3)
	if got.Phase != "已完成" {
		t.Fatalf("停止后应收尾完成：got phase=%q", got.Phase)
	}
	if got.Deleted != 0 {
		t.Fatalf("停止时不允许执行删除收尾：got %d", got.Deleted)
	}
	if _, err := os.Stat(filepath.Join(source, "a.mkv")); err != nil {
		t.Fatalf("源文件必须保留：%v", err)
	}
}

// writeTestFile 建好父目录并写入一个测试文件。
func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败：%v", err)
	}
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatalf("写测试文件失败：%v", err)
	}
}
