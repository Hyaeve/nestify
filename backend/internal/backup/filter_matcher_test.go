package backup

import (
	"testing"

	"nestify/backend/internal/model"
)

func extensionWhitelist(exts ...string) model.BackupFilterRule {
	return model.BackupFilterRule{
		Type:       model.BackupFilterExtension,
		Whitelist:  true,
		MatchFile:  true,
		Extensions: exts,
	}
}

// 扩展名白名单是「允许清单」：只有命中的扩展名放行，其余文件一律排除。
func TestExtensionWhitelistOnlyAllowsListedExtensions(t *testing.T) {
	matcher := newFilterMatcher([]model.BackupFilterRule{extensionWhitelist("mp4", "mkv")})

	cases := []struct {
		name     string
		expected bool
	}{
		{"movie.mp4", false},
		{"movie.MP4", false}, // 不区分大小写
		{"movie.mkv", false},
		{"subtitle.ass", true},
		{"poster.jpg", true},
		{"README", true}, // 无扩展名同样不在白名单内
	}

	for _, item := range cases {
		if got := matcher.excluded("剧集/"+item.name, item.name, false, 1024); got != item.expected {
			t.Fatalf("%s: excluded=%v, want %v", item.name, got, item.expected)
		}
	}
}

// 扩展名黑名单与之相反：命中的排除，其余放行。
func TestExtensionBlacklistExcludesListedExtensions(t *testing.T) {
	matcher := newFilterMatcher([]model.BackupFilterRule{{
		Type:       model.BackupFilterExtension,
		Blacklist:  true,
		MatchFile:  true,
		Extensions: []string{"tmp", "nfo"},
	}})

	if !matcher.excluded("a.tmp", "a.tmp", false, 1) {
		t.Fatal("tmp 应被黑名单排除")
	}
	if matcher.excluded("a.mp4", "a.mp4", false, 1) {
		t.Fatal("mp4 不应被黑名单排除")
	}
}

// 关键回归：扩展名白名单绝不能作用于目录。
// 否则即便勾了「文件夹」，目录也会被判为「未命中白名单」而整棵剪掉，一个文件都备份不了。
func TestExtensionWhitelistNeverExcludesDirectories(t *testing.T) {
	rule := extensionWhitelist("mp4")
	// 模拟历史 / 误操作数据：勾上了「文件夹」。
	rule.MatchDir = true

	matcher := newFilterMatcher([]model.BackupFilterRule{rule})

	if matcher.excluded("剧集", "剧集", true, 0) {
		t.Fatal("扩展名白名单不应排除目录，否则会把整棵目录树剪掉")
	}
	if matcher.excluded("剧集/第一季", "第一季", true, 0) {
		t.Fatal("扩展名白名单不应排除子目录")
	}
	// 目录放行之后，白名单依旧只放行列表内的扩展名。
	if !matcher.excluded("剧集/第一季/notes.txt", "notes.txt", false, 1) {
		t.Fatal("非白名单扩展名的文件应被排除")
	}
	if matcher.excluded("剧集/第一季/01.mp4", "01.mp4", false, 1) {
		t.Fatal("白名单扩展名的文件应被放行")
	}
}

// 体积规则同样只对文件生效，不应影响目录遍历。
func TestSizeWhitelistNeverExcludesDirectories(t *testing.T) {
	matcher := newFilterMatcher([]model.BackupFilterRule{{
		Type:      model.BackupFilterSize,
		Whitelist: true,
		MatchDir:  true,
		MatchFile: true,
		MinSize:   1,
		SizeUnit:  "MB",
	}})

	if matcher.excluded("剧集", "剧集", true, 0) {
		t.Fatal("体积白名单不应排除目录")
	}
	// 小于 1MB 的文件不满足体积白名单 → 排除；满足 → 放行。
	if !matcher.excluded("small.txt", "small.txt", false, 1024) {
		t.Fatal("小于白名单下限的文件应被排除")
	}
	if matcher.excluded("big.mkv", "big.mkv", false, 5*1024*1024) {
		t.Fatal("位于白名单区间内的文件应被放行")
	}
}

// 白名单与黑名单可以叠加：黑名单优先命中即排除，没有任何白名单命中时也排除。
func TestWhitelistAndBlacklistCombination(t *testing.T) {
	matcher := newFilterMatcher([]model.BackupFilterRule{
		extensionWhitelist("mp4", "mkv"),
		{
			Type:       model.BackupFilterExtension,
			Blacklist:  true,
			MatchFile:  true,
			Extensions: []string{"ts"},
		},
	})

	if !matcher.excluded("a.ass", "a.ass", false, 1) {
		t.Fatal("ass 不在白名单内，应被排除")
	}
	if !matcher.excluded("b.ts", "b.ts", false, 1) {
		t.Fatal("ts 在白名单外，任何情况都应排除")
	}
	if matcher.excluded("c.mkv", "c.mkv", false, 1) {
		t.Fatal("mkv 在白名单内且未被黑名单命中，应放行")
	}
}

// 没有配置任何规则时不做任何过滤。
func TestNoRulesKeepsEverything(t *testing.T) {
	matcher := newFilterMatcher(nil)
	if matcher.excluded("a.txt", "a.txt", false, 1) {
		t.Fatal("无规则时不应排除任何文件")
	}
	if matcher.excluded("目录", "目录", true, 0) {
		t.Fatal("无规则时不应排除任何目录")
	}
}
