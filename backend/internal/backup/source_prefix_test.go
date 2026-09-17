package backup

import (
	"path/filepath"
	"testing"
)

// 单源不加前缀：既有任务的目标结构必须保持原样。
func TestSourceDirPrefixesSingleSourceKeepsLayout(t *testing.T) {
	prefixes := sourceDirPrefixes([]string{"/mnt/media"})
	if len(prefixes) != 1 || prefixes[0] != "" {
		t.Fatalf("单源不应加前缀：%#v", prefixes)
	}

	relative := filepath.Join("电影", "a.mkv")
	if got := sourceKey(prefixes[0], relative); got != relative {
		t.Fatalf("单源的键应保持原样：got %q want %q", got, relative)
	}
}

// 多源各自取一层「源目录名」前缀（末尾斜杠要吃掉）。
func TestSourceDirPrefixesUsesDirNamePerSource(t *testing.T) {
	got := sourceDirPrefixes([]string{"/mnt/disk1/电影", "/mnt/disk2/剧集", "/mnt/disk3/music/"})
	want := []string{"电影", "剧集", "music"}
	if len(got) != len(want) {
		t.Fatalf("前缀数量不符：%#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个前缀 want %q got %q", i, want[i], got[i])
		}
	}
}

// 源目录名重复时追加 -2 / -3，且去重按大小写不敏感处理。
func TestSourceDirPrefixesDeduplicatesNames(t *testing.T) {
	got := sourceDirPrefixes([]string{
		"/mnt/a/电影", "/mnt/b/电影", "/mnt/c/电影",
		"/mnt/d/Movies", "/mnt/e/movies",
	})
	want := []string{"电影", "电影-2", "电影-3", "Movies", "movies-2"}
	if len(got) != len(want) {
		t.Fatalf("前缀数量不符：%#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("第 %d 个前缀 want %q got %q", i, want[i], got[i])
		}
	}
}

// 空串 / "." / 根路径这类取不出目录名的源，退化成「源N」占位。
func TestSourceDirPrefixesFallsBackForRootLikePaths(t *testing.T) {
	got := sourceDirPrefixes([]string{"", ".", "/mnt/media"})
	if got[0] != "源1" || got[1] != "源2" || got[2] != "media" {
		t.Fatalf("退化命名不符：%#v", got)
	}
}

// 回归：跨源同名相对路径不能再互相覆盖（这正是加前缀要解决的问题）。
func TestSourceKeyKeepsSameRelativePathsApart(t *testing.T) {
	prefixes := sourceDirPrefixes([]string{"/mnt/a/电影", "/mnt/b/电影"})
	relative := filepath.Join("流浪地球", "1.mkv")

	index := make(map[string]string)
	index[sourceKey(prefixes[0], relative)] = "/mnt/a/电影/流浪地球/1.mkv"
	index[sourceKey(prefixes[1], relative)] = "/mnt/b/电影/流浪地球/1.mkv"

	if len(index) != 2 {
		t.Fatalf("两个源的同名文件必须各占一个键，实际 %d 条：%#v", len(index), index)
	}
	if _, ok := index[filepath.Join("电影", "流浪地球", "1.mkv")]; !ok {
		t.Fatalf("键应为「源目录名/相对路径」：%#v", index)
	}
}
