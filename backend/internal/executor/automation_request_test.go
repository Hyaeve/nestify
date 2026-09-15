package executor

import (
	"testing"

	"nestify/backend/internal/model"
)

// TestBuildRuleExecuteRequestCarriesAllLists 锁定「启动后立即运行 / 计划执行 / 新文件触发」
// 三条自动触发链路的请求映射：这些链路只经过 buildRuleExecuteRequest，
// 漏字段不会报错、只会静默失效（曾漏 MetadataFilters，导致定时运行不同步元数据文件）。
func TestBuildRuleExecuteRequestCarriesAllLists(t *testing.T) {
	rule := model.Rule{
		ID:                   7,
		Name:                 "strm-规则",
		ArchiveMode:          "link",
		RuleType:             "link",
		LinkMode:             "strm",
		CompatibilityMode:    "local",
		SourceDir:            "webdav://3/影视",
		SourceDirs:           []string{"webdav://3/影视"},
		TargetDir:            "/mnt/media",
		OptionsJSON:          `{"strm_overwrite":true}`,
		OptionValuesJSON:     `{"strm_min_video_mb":200}`,
		FiltersJSON:          `["mkv","mp4"]`,
		MetadataFiltersJSON:  `["jpg","srt"]`,
		WhitelistJSON:        `["/预告"]`,
		MatchFiltersJSON:     `["花絮"]`,
		NestFiltersJSON:      `["特典"]`,
		TransformRulesJSON:   `[]`,
		TransformFiltersJSON: `["片段"]`,
	}

	request := buildRuleExecuteRequest(rule, model.TriggerModeCron)

	if request.RuleID != 7 || request.RuleName != "strm-规则" || request.LinkMode != "strm" {
		t.Fatalf("规则基本信息未映射：%+v", request)
	}
	if request.TriggerMode != model.TriggerModeCron {
		t.Fatalf("触发方式 = %q", request.TriggerMode)
	}
	if len(request.Filters) != 2 || request.Filters[0] != "mkv" {
		t.Fatalf("媒体后缀未映射：%v", request.Filters)
	}
	// 关键回归点：元数据后缀必须一起带上，否则计划执行时不落地封面与字幕。
	if len(request.MetadataFilters) != 2 || request.MetadataFilters[0] != "jpg" || request.MetadataFilters[1] != "srt" {
		t.Fatalf("元数据后缀未映射：%v", request.MetadataFilters)
	}
	if len(request.Whitelist) != 1 || request.Whitelist[0] != "/预告" {
		t.Fatalf("过滤名单未映射：%v", request.Whitelist)
	}
	if len(request.MatchFilters) != 1 || len(request.NestFilters) != 1 || len(request.TransformFilters) != 1 {
		t.Fatalf("其它名单未映射：%v / %v / %v", request.MatchFilters, request.NestFilters, request.TransformFilters)
	}
	if !request.Options["strm_overwrite"] {
		t.Fatalf("布尔参数未映射：%v", request.Options)
	}
	if request.OptionValues["strm_min_video_mb"] != 200 {
		t.Fatalf("数值参数未映射：%v", request.OptionValues)
	}
}
