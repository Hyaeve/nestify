package sqlite

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"nestify/backend/internal/config"
	"nestify/backend/internal/model"
)

func parseRuleFiltersJSON(t *testing.T, raw string) []string {
	t.Helper()
	values := make([]string, 0)
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatalf("解析后缀列表 %q 失败: %v", raw, err)
	}
	return values
}

// 升级时要把历史 strm 规则里混在 filters_json 的元数据后缀拆到 metadata_filters_json：
// 否则升级后这些规则仍会把封面 / 字幕 / nfo 生成成 .strm，播放端读不到元数据。
func TestSplitLegacyStrmMetadataFilters(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "nestify-test.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	legacyLinkRule, err := store.CreateRule(model.CreateRuleInput{
		Name:        "历史 strm 规则",
		ArchiveMode: "link",
		RuleType:    "link",
		LinkMode:    "strm",
		RunMode:     "cron",
		SourceDir:   "/data/source",
		TargetDir:   "/data/target",
		Filters:     []string{"mp4", "mkv", ".JPG", "nfo", "ass"},
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	archiveRule, err := store.CreateRule(model.CreateRuleInput{
		Name:        "归档规则",
		ArchiveMode: "package",
		RuleType:    "archive",
		RunMode:     "cron",
		SourceDir:   "/data/source",
		TargetDir:   "/data/target",
		Filters:     []string{"mp4", "jpg"},
	})
	if err != nil {
		t.Fatalf("create archive rule: %v", err)
	}

	if err := store.splitLegacyStrmMetadataFilters(); err != nil {
		t.Fatalf("splitLegacyStrmMetadataFilters: %v", err)
	}

	updated, err := store.GetRuleByID(legacyLinkRule.ID)
	if err != nil || updated == nil {
		t.Fatalf("读取拆分后的 strm 规则失败: %v", err)
	}

	media := parseRuleFiltersJSON(t, updated.FiltersJSON)
	metadata := parseRuleFiltersJSON(t, updated.MetadataFiltersJSON)
	if len(media) != 2 || media[0] != ".mp4" || media[1] != ".mkv" {
		t.Fatalf("媒体后缀 = %v, want [.mp4 .mkv]", media)
	}
	if len(metadata) != 3 || metadata[0] != ".jpg" || metadata[1] != ".nfo" || metadata[2] != ".ass" {
		t.Fatalf("元数据后缀 = %v, want [.jpg .nfo .ass]（大小写归一后按图片/字幕/nfo 归类）", metadata)
	}

	// 非 strm 规则必须原样保留，不能被拆分逻辑波及。
	untouched, err := store.GetRuleByID(archiveRule.ID)
	if err != nil || untouched == nil {
		t.Fatalf("读取归档规则失败: %v", err)
	}
	if got := parseRuleFiltersJSON(t, untouched.FiltersJSON); len(got) != 2 || got[0] != "mp4" || got[1] != "jpg" {
		t.Fatalf("归档规则的后缀被改动: %v", got)
	}
	if got := parseRuleFiltersJSON(t, untouched.MetadataFiltersJSON); len(got) != 0 {
		t.Fatalf("归档规则不应有元数据后缀: %v", got)
	}

	// 幂等：再次执行不应有任何变化。
	if err := store.splitLegacyStrmMetadataFilters(); err != nil {
		t.Fatalf("重复执行失败: %v", err)
	}
	again, err := store.GetRuleByID(legacyLinkRule.ID)
	if err != nil || again == nil {
		t.Fatalf("再次读取失败: %v", err)
	}
	if got := parseRuleFiltersJSON(t, again.FiltersJSON); len(got) != 2 {
		t.Fatalf("重复执行改动了 filters_json: %v", got)
	}
}

// 规则新增的元数据后缀字段必须能原样存取：PUT 是整体覆盖，字段错位会静默丢数据。
func TestRuleMetadataFiltersRoundTrip(t *testing.T) {
	store, err := Open(config.Env{DBPath: filepath.Join(t.TempDir(), "nestify-test.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	rule, err := store.CreateRule(model.CreateRuleInput{
		Name:            "strm 规则",
		ArchiveMode:     "link",
		RuleType:        "link",
		LinkMode:        "strm",
		RunMode:         "cron",
		SourceDir:       "/data/source",
		TargetDir:       "/data/target",
		Filters:         []string{"mp4"},
		MetadataFilters: []string{"jpg", "srt"},
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	stored, err := store.GetRuleByID(rule.ID)
	if err != nil || stored == nil {
		t.Fatalf("读取规则失败: %v", err)
	}
	if got := parseRuleFiltersJSON(t, stored.MetadataFiltersJSON); len(got) != 2 || got[0] != "jpg" || got[1] != "srt" {
		t.Fatalf("元数据后缀 = %v, want [jpg srt]", got)
	}

	updated, err := store.UpdateRule(rule.ID, model.UpdateRuleInput{
		Name:            "strm 规则",
		ArchiveMode:     "link",
		RuleType:        "link",
		LinkMode:        "strm",
		RunMode:         "cron",
		SourceDir:       "/data/source",
		TargetDir:       "/data/target",
		Filters:         []string{"mkv"},
		MetadataFilters: []string{"nfo"},
	})
	if err != nil || updated == nil {
		t.Fatalf("更新规则失败: %v", err)
	}
	if got := parseRuleFiltersJSON(t, updated.FiltersJSON); len(got) != 1 || got[0] != "mkv" {
		t.Fatalf("媒体后缀 = %v, want [mkv]", got)
	}
	if got := parseRuleFiltersJSON(t, updated.MetadataFiltersJSON); len(got) != 1 || got[0] != "nfo" {
		t.Fatalf("更新后的元数据后缀 = %v, want [nfo]", got)
	}
}
