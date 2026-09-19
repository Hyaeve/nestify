package executor

import (
	"os"
	"path/filepath"
	"testing"

	"nestify/backend/internal/model"
)

// cascadeRequest 构造一个开了「级联删除」的 strm 请求。
func cascadeRequest(filters, metadataFilters []string) ExecuteRuleRequest {
	request := ExecuteRuleRequest{
		Filters:         filters,
		MetadataFilters: metadataFilters,
	}
	request.Options = map[string]bool{"strm_cascade_delete": true}
	return request
}

// cascadeRun 跑一次 strm 规则并返回统计（不关心错误之外的细节）。
func cascadeRun(t *testing.T, service *Service, runID string, request ExecuteRuleRequest, sourceDir, targetDir string) executionStats {
	t.Helper()
	stats, err := service.executeStrmRule(runID, request, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("%s 执行出错: %v", runID, err)
	}
	return stats
}

// TestCascadeDeleteRemovesStrmForDeletedSourceFile 覆盖用户提出的主场景：
// 源端把 A 文件夹下的 A1 删掉之后，下一次执行要把原先给 A1 生成的 strm 从目标端删掉，
// 而 A2（源端还在）的 strm 必须原样留着。
func TestCascadeDeleteRemovesStrmForDeletedSourceFile(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "A1.mkv"), "a1")
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "A2.mkv"), "a2")

	service := NewService(nil)
	request := cascadeRequest([]string{"mkv"}, nil)
	cascadeRun(t, service, "run-cascade-first", request, sourceDir, targetDir)

	a1Strm := filepath.Join(targetDir, "剧集", "A1.strm")
	a2Strm := filepath.Join(targetDir, "剧集", "A2.strm")
	for _, path := range []string{a1Strm, a2Strm} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("首次执行应生成 %s: %v", path, err)
		}
	}

	// 源端删掉 A1 —— 目标端的 A1.strm 指向的地址已经不存在了。
	if err := os.Remove(filepath.Join(sourceDir, "剧集", "A1.mkv")); err != nil {
		t.Fatalf("删除源文件失败: %v", err)
	}

	stats := cascadeRun(t, service, "run-cascade-second", request, sourceDir, targetDir)

	if _, err := os.Stat(a1Strm); !os.IsNotExist(err) {
		t.Fatalf("源文件已删除，目标端 %s 应当被级联删除（err=%v）", a1Strm, err)
	}
	if _, err := os.Stat(a2Strm); err != nil {
		t.Fatalf("源端仍存在的 A2.strm 不该被删: %v", err)
	}
	if stats.CascadeDeletedFiles != 1 || stats.CascadeDeletedDirs != 0 {
		t.Fatalf("级联删除计数 = %d 文件 / %d 目录, want 1 / 0", stats.CascadeDeletedFiles, stats.CascadeDeletedDirs)
	}

	// 删了什么要能在任务详情里看见：动作是 delete、路径是目标端相对路径 + 原因文案。
	detail := decodeRunDetail(t, &stats)
	deletes := detailEntriesByAction(detail, model.BackupFileActionDelete)
	if len(deletes) != 1 {
		t.Fatalf("删除明细条数 = %d, want 1", len(deletes))
	}
	if deletes[0].Path != "剧集/A1.strm" {
		t.Fatalf("删除明细 Path = %q, want %q", deletes[0].Path, "剧集/A1.strm")
	}
	if deletes[0].Note != cascadeDeleteNoteFile {
		t.Fatalf("删除备注 = %q, want %q", deletes[0].Note, cascadeDeleteNoteFile)
	}
	if deletes[0].Dir {
		t.Fatal("被删的是文件，明细不该标成目录")
	}
	if deletes[0].Target != a1Strm {
		t.Fatalf("删除明细 Target = %q, want %q", deletes[0].Target, a1Strm)
	}
}

// TestCascadeDeleteRemovesWholeTargetDirWhenSourceDirGone 覆盖用户提出的第二个场景，
// 也是用户明确确认过的口径（轮 102）：整个 A 文件夹在源端没了，目标端对应的 A 文件夹要
// **整目录删掉** —— 无论里面还剩不剩 strm，jpg / nfo 等元数据、甚至本规则没生成过的文件
// 都一并删（这是目录级的唯一例外，文件级只删 .strm，见 TestCascadeDeleteKeepsNonStrmFiles）。
func TestCascadeDeleteRemovesWholeTargetDirWhenSourceDirGone(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "A1.mkv"), "a1")
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "poster.jpg"), "jpg")
	// B 留着：既验证「只删源端没有的那个目录」，也让源根不为空（过安全阀）。
	writeStrmFixture(t, filepath.Join(sourceDir, "B", "B1.mkv"), "b1")

	service := NewService(nil)
	request := cascadeRequest([]string{"mkv"}, []string{"jpg"})
	cascadeRun(t, service, "run-cascade-dir-first", request, sourceDir, targetDir)

	// 目标端 A 里再放一个「本规则没生成过」的文件：整目录删除时它也该跟着走。
	writeStrmFixture(t, filepath.Join(targetDir, "A", "readme.txt"), "readme")

	if err := os.RemoveAll(filepath.Join(sourceDir, "A")); err != nil {
		t.Fatalf("删除源目录失败: %v", err)
	}

	stats := cascadeRun(t, service, "run-cascade-dir-second", request, sourceDir, targetDir)

	if _, err := os.Stat(filepath.Join(targetDir, "A")); !os.IsNotExist(err) {
		t.Fatalf("源目录已不存在，目标端 A 应当整目录删除（err=%v）", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "B", "B1.strm")); err != nil {
		t.Fatalf("源端还在的 B 目录不该被动: %v", err)
	}
	// A1.strm + poster.jpg + readme.txt 三个文件 + A 目录本身。
	if stats.CascadeDeletedFiles != 3 || stats.CascadeDeletedDirs != 1 {
		t.Fatalf("级联删除计数 = %d 文件 / %d 目录, want 3 / 1", stats.CascadeDeletedFiles, stats.CascadeDeletedDirs)
	}

	detail := decodeRunDetail(t, &stats)
	deletes := detailEntriesByAction(detail, model.BackupFileActionDelete)
	var dirEntries []model.RunFileEntry
	for _, entry := range deletes {
		if entry.Dir {
			dirEntries = append(dirEntries, entry)
		}
	}
	if len(dirEntries) != 1 {
		t.Fatalf("目录删除明细条数 = %d, want 1", len(dirEntries))
	}
	if dirEntries[0].Path != "A" || dirEntries[0].Note != cascadeDeleteNoteDir {
		t.Fatalf("目录删除明细 = %+v, want Path A / note %q", dirEntries[0], cascadeDeleteNoteDir)
	}
}

// TestCascadeDeleteKeepsNonStrmFiles 用户口径（轮 102）：**文件级级联删除只删 .strm**。
//
// 源端目录还在、只是少了几个文件时，目标端同一个文件夹里的非 strm 文件一律留着 ——
// 本规则复制过去的元数据（nfo）、媒体服务器自己刮削的封面（jpg）、用户手动放进去的文件，
// 三者从文件名上根本分不出来，删错一次就是不可逆的数据丢失。
// （这就是用户报的 bug：这些文件因为「源端同名文件不存在」被级联删掉。）
func TestCascadeDeleteKeepsNonStrmFiles(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "A1.mkv"), "a1")
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "A1.nfo"), "nfo")
	// A2 留在源端：既验证「同目录里源端还在的那条 strm 不动」，也保证 A 目录本身不为空
	// （本轮验的正是「源端目录还在、只是少了文件」这条边界）。
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "A2.mkv"), "a2")

	service := NewService(nil)
	request := cascadeRequest([]string{"mkv"}, []string{"nfo"})
	cascadeRun(t, service, "run-cascade-keep-first", request, sourceDir, targetDir)

	// 目标端除了规则生成/复制的产物，再放两类「不是本规则生成的」文件。
	planted := []string{
		filepath.Join(targetDir, "A", "A1.nfo"),     // 本规则按元数据后缀复制过去的
		filepath.Join(targetDir, "A", "poster.jpg"), // 媒体服务器刮削出来的封面
		filepath.Join(targetDir, "A", "readme.txt"), // 用户手动放进去的
	}
	for _, path := range planted[1:] {
		writeStrmFixture(t, path, "planted")
	}
	for _, path := range planted {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("前置条件：%s 应当存在: %v", path, err)
		}
	}

	// 源端把 A1 的媒体文件与元数据都删掉：目标端 A1.strm 成了失效投影，但非 strm 文件不该动。
	if err := os.Remove(filepath.Join(sourceDir, "A", "A1.mkv")); err != nil {
		t.Fatalf("删除源文件失败: %v", err)
	}
	if err := os.Remove(filepath.Join(sourceDir, "A", "A1.nfo")); err != nil {
		t.Fatalf("删除源元数据失败: %v", err)
	}

	stats := cascadeRun(t, service, "run-cascade-keep-second", request, sourceDir, targetDir)

	if _, err := os.Stat(filepath.Join(targetDir, "A", "A1.strm")); !os.IsNotExist(err) {
		t.Fatalf("源文件已删除，目标端 A1.strm 应当被级联删除（err=%v）", err)
	}
	for _, path := range planted {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("非 strm 文件不该被级联删除：%s（%v）", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(targetDir, "A", "A2.strm")); err != nil {
		t.Fatalf("源端仍在的 A2.strm 不该被删: %v", err)
	}
	if stats.CascadeDeletedFiles != 1 || stats.CascadeDeletedDirs != 0 {
		t.Fatalf("级联删除计数 = %d 文件 / %d 目录, want 1 / 0（只该删掉 A1.strm）",
			stats.CascadeDeletedFiles, stats.CascadeDeletedDirs)
	}

	// 删了什么要能在任务详情里看见，且只该有一条 A1.strm。
	detail := decodeRunDetail(t, &stats)
	deletes := detailEntriesByAction(detail, model.BackupFileActionDelete)
	if len(deletes) != 1 || deletes[0].Path != "A/A1.strm" {
		t.Fatalf("删除明细 = %+v, want 仅 A/A1.strm", deletes)
	}
}

// TestCascadeDeleteKeepsFilteredDir 过滤名单命中的目录在源端「确实还在」，
// 只是本轮不处理它 —— 目标端对应子树必须原样保留（过滤等于跳过，不等于删除）。
func TestCascadeDeleteKeepsFilteredDir(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "正片", "A1.mkv"), "a1")
	writeStrmFixture(t, filepath.Join(sourceDir, "预告", "T1.mkv"), "t1")

	service := NewService(nil)
	request := cascadeRequest([]string{"mkv"}, nil)
	request.Whitelist = []string{"/预告"}
	cascadeRun(t, service, "run-cascade-filter-first", request, sourceDir, targetDir)

	// 目标端预告目录里塞一份「早先生成、后来被过滤名单排除」的残留。
	residual := filepath.Join(targetDir, "预告", "T1.strm")
	writeStrmFixture(t, residual, filepath.Join(sourceDir, "预告", "T1.mkv"))

	stats := cascadeRun(t, service, "run-cascade-filter-second", request, sourceDir, targetDir)

	if _, err := os.Stat(residual); err != nil {
		t.Fatalf("过滤名单命中的目录不该被级联删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "预告")); err != nil {
		t.Fatalf("过滤名单命中的目录不该被级联删除: %v", err)
	}
	if stats.CascadeDeletedFiles != 0 || stats.CascadeDeletedDirs != 0 {
		t.Fatalf("过滤命中的目录不该产生任何级联删除，实际 %d 文件 / %d 目录", stats.CascadeDeletedFiles, stats.CascadeDeletedDirs)
	}
}

// TestCascadeDeleteSkipsWhenSourceRootEmpty 安全阀：源根一个条目都没有时整轮不做级联删除。
// 源盘没挂上（挂载点是空目录）是最容易「一键清空目标端」的场景，必须挡住。
func TestCascadeDeleteSkipsWhenSourceRootEmpty(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "A1.mkv"), "a1")

	service := NewService(nil)
	request := cascadeRequest([]string{"mkv"}, nil)
	cascadeRun(t, service, "run-cascade-empty-first", request, sourceDir, targetDir)

	strmPath := filepath.Join(targetDir, "A", "A1.strm")
	if _, err := os.Stat(strmPath); err != nil {
		t.Fatalf("首次执行应生成 %s: %v", strmPath, err)
	}

	// 源根被清空（等价于源盘没挂上：路径还在、里面什么都没有）。
	if err := os.RemoveAll(filepath.Join(sourceDir, "A")); err != nil {
		t.Fatalf("清空源目录失败: %v", err)
	}

	stats := cascadeRun(t, service, "run-cascade-empty-second", request, sourceDir, targetDir)

	if _, err := os.Stat(strmPath); err != nil {
		t.Fatalf("源根为空时应跳过级联删除，目标端不该被清空: %v", err)
	}
	if stats.CascadeDeletedFiles != 0 || stats.CascadeDeletedDirs != 0 {
		t.Fatalf("源根为空不该产生级联删除，实际 %d 文件 / %d 目录", stats.CascadeDeletedFiles, stats.CascadeDeletedDirs)
	}
}

// TestCascadeDeleteOffLeavesStaleTarget 开关关掉时行为与从前完全一致：
// 源端删掉的文件在目标端留下的 strm 不会被清理（这是「默认行为不变」的兜底）。
func TestCascadeDeleteOffLeavesStaleTarget(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "A1.mkv"), "a1")

	service := NewService(nil)
	request := ExecuteRuleRequest{Filters: []string{"mkv"}}
	cascadeRun(t, service, "run-cascade-off-first", request, sourceDir, targetDir)

	if err := os.Remove(filepath.Join(sourceDir, "A", "A1.mkv")); err != nil {
		t.Fatalf("删除源文件失败: %v", err)
	}

	runStats := &executionStats{}
	stats, err := service.executeStrmRule("run-cascade-off-second", request, sourceDir, targetDir, runStats)
	if err != nil {
		t.Fatalf("执行出错: %v", err)
	}

	strmPath := filepath.Join(targetDir, "A", "A1.strm")
	if _, err := os.Stat(strmPath); err != nil {
		t.Fatalf("未开启级联删除时目标端不该被清理: %v", err)
	}
	if stats.CascadeDeletedFiles != 0 || stats.CascadeDeletedDirs != 0 {
		t.Fatalf("未开启级联删除不该有删除计数，实际 %d 文件 / %d 目录", stats.CascadeDeletedFiles, stats.CascadeDeletedDirs)
	}
	// 快照同理不该被建出来（一分钱开销都不花）。
	if runStats.SourceSnapshot != nil {
		t.Fatal("未开启级联删除时不该创建源端快照")
	}
}

// TestCascadeDeleteIsIdempotent 源端没变化时重复执行不该删任何东西：
// 级联删除的依据是「源端还剩什么」，不是「这次生成顺序」。
func TestCascadeDeleteIsIdempotent(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "A1.mkv"), "a1")
	writeStrmFixture(t, filepath.Join(sourceDir, "A", "poster.jpg"), "jpg")

	service := NewService(nil)
	request := cascadeRequest([]string{"mkv"}, []string{"jpg"})
	cascadeRun(t, service, "run-cascade-idempotent-1", request, sourceDir, targetDir)
	stats := cascadeRun(t, service, "run-cascade-idempotent-2", request, sourceDir, targetDir)

	if stats.CascadeDeletedFiles != 0 || stats.CascadeDeletedDirs != 0 {
		t.Fatalf("源端没变化时不该删除任何东西，实际 %d 文件 / %d 目录", stats.CascadeDeletedFiles, stats.CascadeDeletedDirs)
	}
	for _, path := range []string{
		filepath.Join(targetDir, "A", "A1.strm"),
		filepath.Join(targetDir, "A", "poster.jpg"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("重复执行后 %s 应仍在: %v", path, err)
		}
	}
}

// TestMapRunStatusByCountsCountsCascadeDelete 只做了级联删除、什么都没生成的执行，
// 在运行历史里应当是「成功」而不是「跳过」—— 它确实把目标端清理干净了。
func TestMapRunStatusByCountsCountsCascadeDelete(t *testing.T) {
	cases := []struct {
		name                            string
		success, skip, failure, deleted int
		want                            string
	}{
		{name: "只做级联删除", success: 0, skip: 0, failure: 0, deleted: 3, want: "success"},
		{name: "生成加删除", success: 2, skip: 1, failure: 0, deleted: 1, want: "success"},
		{name: "没开开关全跳过", success: 0, skip: 5, failure: 0, deleted: 0, want: "skip"},
		{name: "有失败就是失败", success: 0, skip: 0, failure: 1, deleted: 4, want: "failed"},
		{name: "什么都没发生", success: 0, skip: 0, failure: 0, deleted: 0, want: "skip"},
	}
	for _, testCase := range cases {
		got := mapRunStatusByCounts(testCase.success, testCase.skip, testCase.failure, testCase.deleted)
		if got != testCase.want {
			t.Fatalf("%s: mapRunStatusByCounts = %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

// TestCascadeSnapshotLookup 直接锁住快照的比对语义：.strm 按主干反查（源端扩展名无关），
// 元数据文件按同名同路径比对，目录按键比对。
func TestCascadeSnapshotLookup(t *testing.T) {
	snapshot := newStrmSourceSnapshot()
	snapshot.addRootEntries(1)
	snapshot.record("剧集/A1.mkv", false)
	snapshot.record("剧集/Season 1", true)
	snapshot.protectDir("预告")

	if snapshot.unusableReason() != "" {
		t.Fatalf("快照应当可用，实际原因 %q", snapshot.unusableReason())
	}
	if !snapshot.hasFile("剧集/A1.strm", true) {
		t.Fatal("A1.strm 应当能按主干反查到源文件 A1.mkv")
	}
	if snapshot.hasFile("剧集/A1.nfo", false) {
		t.Fatal("A1.nfo 在源端并不存在，不该判定为存在")
	}
	if !snapshot.hasDir("剧集/Season 1") {
		t.Fatal("源端目录应当能查到")
	}
	if snapshot.hasDir("剧集/Season 2") {
		t.Fatal("源端不存在的目录不该判定为存在")
	}
	if !snapshot.dirProtected("预告") || snapshot.dirProtected("正片") {
		t.Fatal("保护目录判定不符")
	}

	snapshot.markIncomplete()
	if snapshot.unusableReason() == "" {
		t.Fatal("标记不完整后快照应当不可用")
	}
}
