package executor

import (
	"context"
	"testing"

	"nestify/backend/internal/model"
)

// TestCancelRunStopsActiveRun 锁定「卡片上的执行按钮再次点击即停止」的后端语义：
// CancelRun 会 cancel 掉这次执行登记的上下文，执行器循环里的 runAborted 随之变成 true。
func TestCancelRunStopsActiveRun(t *testing.T) {
	service := NewService(nil)
	run := service.newRun(model.TriggerModeOnce, "collect", "", nil, "demo")

	ctx, cancel := context.WithCancel(context.Background())
	service.registerRunContext(run.ID, ctx, cancel)
	defer service.releaseRunContext(run.ID)

	if service.runAborted(run.ID) {
		t.Fatal("刚登记的执行不应处于已取消状态")
	}

	service.mu.Lock()
	service.runs[run.ID].Status = model.RunStatusRunning
	service.mu.Unlock()

	ok, err := service.CancelRun(run.ID)
	if err != nil || !ok {
		t.Fatalf("CancelRun = (%v, %v)，期望成功", ok, err)
	}
	if !service.runAborted(run.ID) {
		t.Fatal("CancelRun 之后 runAborted 应为 true")
	}

	// 已经结束的执行不能再停。
	service.mu.Lock()
	service.runs[run.ID].Status = model.RunStatusSucceeded
	service.mu.Unlock()
	if _, err := service.CancelRun(run.ID); err == nil {
		t.Fatal("已结束的执行再次停止应当报错")
	}
}

// TestActiveRunsOnlyReturnsUnfinished 确认卡片上的「执行中」状态只由未结束的执行产生。
func TestActiveRunsOnlyReturnsUnfinished(t *testing.T) {
	service := NewService(nil)
	running := service.newRun(model.TriggerModeOnce, "collect", "", nil, "running")
	done := service.newRun(model.TriggerModeOnce, "collect", "", nil, "done")

	service.mu.Lock()
	service.runs[running.ID].Status = model.RunStatusRunning
	service.runs[done.ID].Status = model.RunStatusSucceeded
	service.mu.Unlock()

	items := service.ActiveRuns()
	if len(items) != 1 || items[0].ID != running.ID {
		t.Fatalf("ActiveRuns 应只含运行中的那次执行，实际 %d 条", len(items))
	}
}
