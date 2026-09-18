package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"nestify/backend/internal/model"
)

// 级联删除（链路规则·Strm 模式的「级联删除」开关，存 rules.options_json.strm_cascade_delete）。
//
// 目标端的 .strm 是源端媒体文件的「投影」：源端文件被删掉之后，目标端那个 .strm 指向的
// 就是一个不存在的地址，留在库里只会让媒体服务器多出一堆点不开的条目。打开这个开关后，
// 每次执行都会顺带把目标端对齐到源端：
//
//   - 目标端某个 .strm 对应的源文件已经不在了 → 删掉这个 .strm；
//   - 目标端某份元数据（封面 / 字幕 / nfo）在源端已经不在了 → 删掉这个文件；
//   - 目标端整个文件夹对应的源目录已经不在了 → 整个文件夹递归删掉
//     （无论里面还剩不剩 strm，jpg / nfo 等元数据一并删）。
//
// 判定依据是「源端现在还剩什么」的快照（strmSourceSnapshot），而不是「这次扫描生成了什么」：
// 后缀名单被改小、视频体积阈值调高、元数据没命中本次名单……这些都不该导致目标端被删 ——
// 只有源端确实没有这个文件了才删。
//
// 三道安全阀（都是「宁可留着」的方向）：
//
//  1. 快照不可信（执行被停止、源端目录列不出来）时整轮不做级联删除，绝不拿半个快照删目标端；
//  2. 过滤名单命中的目录视为不可触碰，它对应的目标端子目录原样保留；
//  3. 源端根目录一个条目都没有时整轮跳过 —— 源盘没挂上、网盘掉线都会长这样，
//     此时把目标端清空是最坏的结局。
//
// 快照在枚举源端时顺手登记，不额外产生任何 IO（本地链路除外：目标端遍历本身就是本地读目录）。
const (
	// strmTargetSuffix 是目标端 strm 文件的后缀（与 strmRelativePath / strmTargetPath 保持一致）。
	strmTargetSuffix = ".strm"
	// 级联删除写进明细的备注文案：同一类删除在详情里只能有一种说法。
	cascadeDeleteNoteFile = "源端已不存在，级联删除"
	cascadeDeleteNoteDir  = "源端目录已不存在，整个目录级联删除"
)

// strmSourceSnapshot 记录一次执行期间「源端现在还有什么」。
//
// 键统一成「目标端相对路径」的小写斜杠形式：目标端的目录结构就是源端目录结构，
// 两边用同一把钥匙就能直接对上（Windows 上大小写不敏感，一律折小写更稳）。
type strmSourceSnapshot struct {
	mu    sync.Mutex
	files map[string]struct{} // 源端文件（相对路径）
	stems map[string]struct{} // 源端文件去掉扩展名的主干，用于把目标端 .strm 反查回源文件
	dirs  map[string]struct{} // 源端目录（相对路径）
	// protected 是禁止触碰的目标端子目录（过滤名单命中 / 源端列不出来）。
	protected map[string]struct{}
	// rootEntries 是源根目录下的直接条目数：为 0 说明源端看着是空的（多半是没挂上），
	// 这时不做级联删除。
	rootEntries int
	// complete 为 false 表示快照不完整（执行被停止 / 源根列不出来），整轮放弃级联删除。
	complete bool
}

func newStrmSourceSnapshot() *strmSourceSnapshot {
	return &strmSourceSnapshot{
		files:     make(map[string]struct{}),
		stems:     make(map[string]struct{}),
		dirs:      make(map[string]struct{}),
		protected: make(map[string]struct{}),
		complete:  true,
	}
}

// normalizeCascadePath 把相对路径折成快照键：去空白、统一斜杠、去首尾斜杠、折小写。
func normalizeCascadePath(rel string) string {
	return strings.ToLower(strings.Trim(filepath.ToSlash(strings.TrimSpace(rel)), "/"))
}

// stemOfCascadePath 求「源文件 → 目标端 strm 文件名」的反向键：去掉扩展名后的主干。
// 与 strmRelativePath / strmTargetPath 的规则一致（只认最后一个点）。
func stemOfCascadePath(key string) string {
	return strings.TrimSuffix(key, filepath.Ext(key))
}

// record 登记一个源端条目（文件或目录）。nil 接收者表示本次没开级联删除，直接忽略。
func (c *strmSourceSnapshot) record(rel string, isDir bool) {
	if c == nil {
		return
	}
	key := normalizeCascadePath(rel)
	if key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if isDir {
		c.dirs[key] = struct{}{}
		return
	}
	c.files[key] = struct{}{}
	c.stems[stemOfCascadePath(key)] = struct{}{}
}

// protectDir 标记一个不可触碰的源目录（过滤名单命中 / 源端列不出来），
// 它对应的目标端子目录整棵保留。
func (c *strmSourceSnapshot) protectDir(rel string) {
	if c == nil {
		return
	}
	key := normalizeCascadePath(rel)
	if key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.protected[key] = struct{}{}
}

// addRootEntries 累加源根目录下的直接条目数（判断源端是不是「看着空的」）。
func (c *strmSourceSnapshot) addRootEntries(count int) {
	if c == nil || count <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rootEntries += count
}

// markIncomplete 标记快照不可信：执行被停止、源根列不出来时调用。
func (c *strmSourceSnapshot) markIncomplete() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.complete = false
}

// hasFile 判断目标端文件 rel 对应的源端文件是否还在。
//
// strm 为 true 时目标端是「源文件名去掉扩展名 + .strm」，源端扩展名已经无从得知，
// 只能按主干比对（后缀名单以后再改也不影响判定）；元数据文件与源端同名同路径，直接比对。
func (c *strmSourceSnapshot) hasFile(rel string, strm bool) bool {
	if c == nil {
		return true
	}
	key := normalizeCascadePath(rel)
	if key == "" {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if strm {
		_, ok := c.stems[strings.TrimSuffix(key, strmTargetSuffix)]
		return ok
	}
	_, ok := c.files[key]
	return ok
}

// hasDir 判断目标端目录 rel 对应的源目录是否还在。
func (c *strmSourceSnapshot) hasDir(rel string) bool {
	if c == nil {
		return true
	}
	key := normalizeCascadePath(rel)
	if key == "" {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.dirs[key]
	return ok
}

// dirProtected 判断目标端目录 rel 是否属于「不可触碰」的子树。
func (c *strmSourceSnapshot) dirProtected(rel string) bool {
	if c == nil {
		return true
	}
	key := normalizeCascadePath(rel)
	if key == "" {
		return true
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.protected[key]
	return ok
}

// unusableReason 返回「这轮不能做级联删除」的原因；空串表示可以正常执行。
func (c *strmSourceSnapshot) unusableReason() string {
	if c == nil {
		return "未开启"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.complete {
		return "源端列举未完成"
	}
	if c.rootEntries == 0 {
		return "源端目录为空"
	}
	return ""
}

// enableCascadeDelete 打开级联删除并准备源端快照。
//
// 必须在开始枚举源端之前调用（与明细采集器同理）：并发列举的多个线程共用同一份快照，
// 谁先建都行，但不能各建一份。
func (s *executionStats) enableCascadeDelete() *strmSourceSnapshot {
	if s == nil {
		return nil
	}
	if s.SourceSnapshot == nil {
		s.SourceSnapshot = newStrmSourceSnapshot()
	}
	return s.SourceSnapshot
}

// cascadeDeletedTotal 返回本次执行级联删掉的目标端条目总数（文件 + 目录）。
func (s *executionStats) cascadeDeletedTotal() int {
	if s == nil {
		return 0
	}
	return s.CascadeDeletedFiles + s.CascadeDeletedDirs
}

// joinCascadeRel 拼接目标端相对路径（始终用斜杠，与快照键一致）。
func joinCascadeRel(dir, name string) string {
	dir = strings.Trim(strings.TrimSpace(dir), "/")
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// cascadeRelativeDir 求当前目录相对源根的位置（源根本身返回空串）。
func cascadeRelativeDir(root, current string) string {
	rel, err := filepath.Rel(root, current)
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}
	return rel
}

// isCascadeManagedFile 判断目标端的这个文件是不是本规则管的东西：
// 自己生成的 .strm，或按配置同步过来的元数据文件。其它文件一律不碰
// （只有整个目录被判定为「源端已不存在」时才会跟着目录一起删）。
func isCascadeManagedFile(name string, metadataExtensions map[string]struct{}) bool {
	if strings.EqualFold(filepath.Ext(name), strmTargetSuffix) {
		return true
	}
	return matchesStrmExtension(name, metadataExtensions)
}

// cascadeDeleteStrmTarget 把目标目录对齐到源端快照。
//
// 只做删除，不做生成：调用点必须先跑完本次生成，删的才是「源端真的没有了」的那些。
func (s *Service) cascadeDeleteStrmTarget(runID, targetRoot string, metadataExtensions map[string]struct{}, stats *executionStats) error {
	snapshot := stats.SourceSnapshot
	if snapshot == nil {
		return nil
	}
	if reason := snapshot.unusableReason(); reason != "" {
		s.appendLog(runID, "warn", fmt.Sprintf("级联删除：%s，本次跳过（避免误删目标端内容）", reason))
		return nil
	}

	beforeFiles, beforeDirs := stats.CascadeDeletedFiles, stats.CascadeDeletedDirs
	if err := s.cascadeDeleteStrmDir(runID, targetRoot, "", metadataExtensions, snapshot, stats); err != nil {
		return err
	}
	// 逐条删除不刷日志（与「跳过」同理，一次全量清理动辄上千条），只留一行汇总；
	// 到底删了哪些文件去任务详情的「已删除」里看，删掉的目录另有单独一行（数量少、动作大）。
	if removed := (stats.CascadeDeletedFiles - beforeFiles) + (stats.CascadeDeletedDirs - beforeDirs); removed > 0 {
		s.appendLog(runID, "info", fmt.Sprintf("级联删除：目标端移除 %d 个文件 / %d 个目录（源端已不存在）",
			stats.CascadeDeletedFiles-beforeFiles, stats.CascadeDeletedDirs-beforeDirs))
	}
	return nil
}

// cascadeDeleteStrmDir 递归对齐目标端目录。
func (s *Service) cascadeDeleteStrmDir(runID, currentDir, relDir string, metadataExtensions map[string]struct{}, snapshot *strmSourceSnapshot, stats *executionStats) error {
	entries, err := os.ReadDir(currentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		s.appendLog(runID, "error", fmt.Sprintf("级联删除：读取目标目录 %s 失败：%v", currentDir, err))
		stats.FailureCount++
		return nil
	}

	for _, entry := range entries {
		if s.runAborted(runID) {
			return errRunCancelled
		}
		rel := joinCascadeRel(relDir, entry.Name())
		entryPath := filepath.Join(currentDir, entry.Name())

		if entry.IsDir() {
			if snapshot.dirProtected(rel) {
				continue
			}
			if !snapshot.hasDir(rel) {
				removedFiles, removedDirs, failed := s.removeCascadeSubtree(runID, entryPath, rel, stats)
				s.appendLog(runID, "info", fmt.Sprintf("级联删除：源目录已不存在，移除目标目录 %s（%d 个文件 / %d 个目录）", entryPath, removedFiles, removedDirs))
				if failed > 0 {
					s.appendLog(runID, "error", fmt.Sprintf("级联删除：%s 下有 %d 项删除失败", entryPath, failed))
				}
				continue
			}
			if err := s.cascadeDeleteStrmDir(runID, entryPath, rel, metadataExtensions, snapshot, stats); err != nil {
				return err
			}
			continue
		}

		if !isCascadeManagedFile(entry.Name(), metadataExtensions) {
			continue
		}
		if snapshot.hasFile(rel, strings.EqualFold(filepath.Ext(entry.Name()), strmTargetSuffix)) {
			continue
		}
		s.removeCascadeEntry(runID, entryPath, rel, cascadeDeleteNoteFile, false, stats)
	}

	return nil
}

// removeCascadeSubtree 递归删掉一个目标端子目录（源端整个目录已经没了），
// 返回删除成功的文件数 / 目录数与失败数。先删子项再删目录本身，非空目录才删得掉。
func (s *Service) removeCascadeSubtree(runID, dirPath, relDir string, stats *executionStats) (int, int, int) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		stats.FailureCount++
		s.appendLog(runID, "error", fmt.Sprintf("级联删除：读取目标目录 %s 失败：%v", dirPath, err))
		return 0, 0, 1
	}

	var files, dirs, failed int
	for _, entry := range entries {
		if s.runAborted(runID) {
			return files, dirs, failed
		}
		childPath := filepath.Join(dirPath, entry.Name())
		childRel := joinCascadeRel(relDir, entry.Name())
		if entry.IsDir() {
			childFiles, childDirs, childFailed := s.removeCascadeSubtree(runID, childPath, childRel, stats)
			files += childFiles
			dirs += childDirs
			failed += childFailed
			continue
		}
		if s.removeCascadeEntry(runID, childPath, childRel, cascadeDeleteNoteFile, false, stats) {
			files++
		} else {
			failed++
		}
	}

	if s.removeCascadeEntry(runID, dirPath, relDir, cascadeDeleteNoteDir, true, stats) {
		dirs++
	} else {
		failed++
	}
	return files, dirs, failed
}

// removeCascadeEntry 删除目标端的一个文件 / 目录。
//
// 明细的 Path 记目标端相对路径（源端已经没有这条路径了，写绝对路径反而读不出来），
// 完整路径放在 Target 里由悬浮提示展示；删除失败计入失败并留一条失败明细。
// 成功不写运行日志 —— 逐条刷屏由调用方压成汇总（见 cascadeDeleteStrmTarget）。
func (s *Service) removeCascadeEntry(runID, entryPath, rel, note string, dir bool, stats *executionStats) bool {
	if err := os.Remove(entryPath); err != nil {
		stats.FailureCount++
		stats.Detail.record(model.RunFileEntry{
			Path:   filepath.ToSlash(rel),
			Action: model.BackupFileActionFail,
			Target: entryPath,
			Dir:    dir,
			Note:   fmt.Sprintf("级联删除失败：%v", err),
		})
		s.appendLog(runID, "error", fmt.Sprintf("cascade remove %s failed: %v", entryPath, err))
		return false
	}

	if dir {
		stats.CascadeDeletedDirs++
	} else {
		stats.CascadeDeletedFiles++
	}
	stats.Detail.record(model.RunFileEntry{
		Path:   filepath.ToSlash(rel),
		Action: model.BackupFileActionDelete,
		Target: entryPath,
		Dir:    dir,
		Note:   note,
	})
	return true
}

// describeCascadeDelete 用在执行摘要里（没有删除时不占篇幅）。
func describeCascadeDelete(files, dirs int) string {
	if total := files + dirs; total > 0 {
		return fmt.Sprintf("，级联删除 %d 项", total)
	}
	return ""
}
