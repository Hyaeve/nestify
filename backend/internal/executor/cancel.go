package executor

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"nestify/backend/internal/model"
)

// errRunCancelled 是「任务被手动停止」的哨兵错误。
// 执行器在循环检查点发现取消信号后返回它，一路上抛到 runExecution，
// runExecution 据此把这次执行收尾成 RunStatusCancelled 而不是「失败」。
var errRunCancelled = errors.New("run cancelled by user")

// registerRunContext 为一次执行登记可取消的上下文。
// runID 是执行链路上唯一的贯穿标识（所有 executor 函数都带它），
// 所以取消不需要改任何函数签名：循环里调 runAborted(runID) 即可。
func (s *Service) registerRunContext(runID string, ctx context.Context, cancel context.CancelFunc) {
	s.mu.Lock()
	s.runContexts[runID] = ctx
	s.runCancels[runID] = cancel
	s.mu.Unlock()
}

// releaseRunContext 在执行 goroutine 收尾时清掉登记，避免 map 无限增长。
func (s *Service) releaseRunContext(runID string) {
	s.mu.Lock()
	delete(s.runContexts, runID)
	delete(s.runCancels, runID)
	s.mu.Unlock()
}

// runAborted 供执行器在循环检查点调用：返回 true 表示这次执行已被要求停止，
// 调用方应立即 return errRunCancelled（或直接结束当前递归）。
//
// 停止是协作式的 —— 已经进入的那次文件复制 / 打包会写完，
// 不会留下半截产物，只是不再开始下一个条目。
func (s *Service) runAborted(runID string) bool {
	s.mu.RLock()
	ctx := s.runContexts[runID]
	s.mu.RUnlock()
	if ctx == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// CancelRun 请求停止一个正在执行的任务。
// 返回 true 表示取消信号已发出（执行线程会在下一个检查点退出）。
func (s *Service) CancelRun(runID string) (bool, error) {
	s.mu.Lock()
	run, ok := s.runs[runID]
	if !ok || run == nil {
		s.mu.Unlock()
		return false, fmt.Errorf("任务不存在")
	}
	cancel := s.runCancels[runID]
	active := run.Status == model.RunStatusPending || run.Status == model.RunStatusRunning
	s.mu.Unlock()

	if !active {
		return false, fmt.Errorf("任务已结束，无需停止")
	}
	if cancel == nil {
		return false, fmt.Errorf("该任务不可停止")
	}

	cancel()
	s.appendLog(runID, "warn", "已请求停止执行，当前文件处理完成后退出")
	return true, nil
}

// ActiveRuns 只返回尚未结束的执行（pending / running）。
// 卡片上的「执行中」状态由前端轮询这个接口得到，比拉全量 runs 更省。
func (s *Service) ActiveRuns() []*model.RunInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*model.RunInstance, 0, len(s.runs))
	for _, run := range s.runs {
		if run == nil {
			continue
		}
		if run.Status != model.RunStatusPending && run.Status != model.RunStatusRunning {
			continue
		}
		items = append(items, s.cloneRun(run))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].StartedAt.Before(items[j].StartedAt)
	})
	return items
}
