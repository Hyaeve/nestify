package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func writeStrmFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 %s 失败: %v", path, err)
	}
}

func readStrmFixture(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return string(content)
}

func requireStrmMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s 不应存在（err=%v）", path, err)
	}
}

// TestExecuteStrmRuleCopiesMetadataAsRealFiles 锁定 strm 链路的核心约定：
// 媒体后缀生成 .strm（内容为可直接播放的源路径），
// 元数据后缀（字幕 / 图片 / nfo）复制成同名实体文件，而不是也生成 .strm。
func TestExecuteStrmRuleCopiesMetadataAsRealFiles(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "S01E01.mkv"), "video-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "S01E01.srt"), "subtitle-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "poster.jpg"), "poster-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "剧集", "notes.txt"), "ignored-bytes")

	service := NewService(nil)
	stats, err := service.executeStrmRule("run-metadata", ExecuteRuleRequest{
		Filters:         []string{"mkv"},
		MetadataFilters: []string{"srt", "jpg"},
	}, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("executeStrmRule 出错: %v", err)
	}

	// 媒体：生成 strm，内容为源文件的本地路径。
	if got, want := readStrmFixture(t, filepath.Join(targetDir, "剧集", "S01E01.strm")), filepath.Join(sourceDir, "剧集", "S01E01.mkv")+"\n"; got != want {
		t.Fatalf("S01E01.strm 内容 = %q, want %q", got, want)
	}

	// 元数据：同名实体文件，内容为真实字节而不是地址。
	if got := readStrmFixture(t, filepath.Join(targetDir, "剧集", "S01E01.srt")); got != "subtitle-bytes" {
		t.Fatalf("S01E01.srt 内容 = %q，元数据必须是实体文件", got)
	}
	if got := readStrmFixture(t, filepath.Join(targetDir, "剧集", "poster.jpg")); got != "poster-bytes" {
		t.Fatalf("poster.jpg 内容 = %q，元数据必须是实体文件", got)
	}

	// 元数据不应再被转换成 .strm。
	requireStrmMissing(t, filepath.Join(targetDir, "剧集", "S01E01.srt.strm"))
	requireStrmMissing(t, filepath.Join(targetDir, "剧集", "poster.strm"))

	// 未命中任何后缀的文件不落地。
	requireStrmMissing(t, filepath.Join(targetDir, "剧集", "notes.txt"))

	// 统计：1 个 strm + 2 个元数据，其中元数据单独计数。
	if stats.MetadataCount != 2 {
		t.Fatalf("MetadataCount = %d, want 2", stats.MetadataCount)
	}
	if stats.SuccessCount != 3 {
		t.Fatalf("SuccessCount = %d, want 3（1 strm + 2 元数据）", stats.SuccessCount)
	}
}

// TestExecuteStrmRuleSkipsExistingMetadataWithoutOverwrite 确认元数据的
// 「已存在即跳过 / 覆盖生成」语义与 strm 保持一致。
func TestExecuteStrmRuleSkipsExistingMetadataWithoutOverwrite(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeStrmFixture(t, filepath.Join(sourceDir, "S01E01.nfo"), "new-nfo")
	writeStrmFixture(t, filepath.Join(targetDir, "S01E01.nfo"), "old-nfo")

	service := NewService(nil)
	request := ExecuteRuleRequest{
		Filters:         []string{"mkv"},
		MetadataFilters: []string{"nfo"},
	}

	stats, err := service.executeStrmRule("run-skip", request, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("executeStrmRule 出错: %v", err)
	}
	if got := readStrmFixture(t, filepath.Join(targetDir, "S01E01.nfo")); got != "old-nfo" {
		t.Fatalf("未启用覆盖生成时不应改写已存在的元数据，实际 %q", got)
	}
	if stats.MetadataCount != 0 || stats.SkipCount != 1 {
		t.Fatalf("统计应为 MetadataCount=0 / SkipCount=1，实际 %d / %d", stats.MetadataCount, stats.SkipCount)
	}

	request.Options = map[string]bool{"strm_overwrite": true}
	stats, err = service.executeStrmRule("run-overwrite", request, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("executeStrmRule(覆盖生成) 出错: %v", err)
	}
	if got := readStrmFixture(t, filepath.Join(targetDir, "S01E01.nfo")); got != "new-nfo" {
		t.Fatalf("覆盖生成时应改写元数据，实际 %q", got)
	}
	if stats.MetadataCount != 1 {
		t.Fatalf("覆盖生成后 MetadataCount = %d, want 1", stats.MetadataCount)
	}
}

// TestExecuteStrmRuleAllowsMetadataOnly 确认只配元数据后缀时规则仍可执行
// （只做元数据落地，不生成任何 strm）。
func TestExecuteStrmRuleAllowsMetadataOnly(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeStrmFixture(t, filepath.Join(sourceDir, "S01E01.mkv"), "video-bytes")
	writeStrmFixture(t, filepath.Join(sourceDir, "S01E01.nfo"), "nfo-bytes")

	service := NewService(nil)
	stats, err := service.executeStrmRule("run-metadata-only", ExecuteRuleRequest{
		MetadataFilters: []string{"nfo"},
	}, sourceDir, targetDir, &executionStats{})
	if err != nil {
		t.Fatalf("executeStrmRule 出错: %v", err)
	}

	if got := readStrmFixture(t, filepath.Join(targetDir, "S01E01.nfo")); got != "nfo-bytes" {
		t.Fatalf("S01E01.nfo 内容 = %q", got)
	}
	requireStrmMissing(t, filepath.Join(targetDir, "S01E01.strm"))
	if stats.MetadataCount != 1 || stats.SuccessCount != 1 {
		t.Fatalf("统计应为 MetadataCount=1 / SuccessCount=1，实际 %d / %d", stats.MetadataCount, stats.SuccessCount)
	}
}

// TestSplitStrmExtensionSetsPrefersMedia 锁定重叠配置的处理：媒体优先，
// 避免一次误配就让规则不再生成 strm。
func TestSplitStrmExtensionSetsPrefersMedia(t *testing.T) {
	strmExtensions, metadataExtensions := splitStrmExtensionSets([]string{"mkv", "jpg"}, []string{".JPG", "nfo"})

	if _, ok := strmExtensions[".jpg"]; !ok {
		t.Fatalf("重叠后缀应保留在媒体集合：%v", strmExtensions)
	}
	if _, ok := metadataExtensions[".jpg"]; ok {
		t.Fatalf("重叠后缀应从元数据集合移除：%v", metadataExtensions)
	}
	if _, ok := metadataExtensions[".nfo"]; !ok {
		t.Fatalf("元数据后缀应被归一（无点 -> 带点小写）：%v", metadataExtensions)
	}
	if isStrmMetadataFile("poster.jpg", strmExtensions, metadataExtensions) {
		t.Fatal("重叠后缀应按媒体处理，poster.jpg 不应被当成元数据")
	}
	if !isStrmMetadataFile("movie.nfo", strmExtensions, metadataExtensions) {
		t.Fatal("movie.nfo 应按元数据处理")
	}
	if isStrmMetadataFile("movie.mkv", strmExtensions, metadataExtensions) {
		t.Fatal("movie.mkv 不应按元数据处理")
	}
}
