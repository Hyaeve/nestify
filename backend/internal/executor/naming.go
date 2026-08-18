package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

type namingRuleConfig struct {
	Content         string `json:"content"`
	Index           int    `json:"index"`
	Position        string `json:"position"`
	Smart           bool   `json:"smart"`
	IgnoreExtension bool   `json:"ignoreExtension"`
	Match           string `json:"match"`
	Replacement     string `json:"replacement"`
	Name            string `json:"name"`
	Pattern         string `json:"pattern"`
	Global          bool   `json:"global"`
}

type namingRuleDefinition struct {
	Category string           `json:"category"`
	Config   namingRuleConfig `json:"config"`
}

func (s *Service) executeNamingRule(runID string, req ExecuteRuleRequest) (executionStats, error) {
	stats := executionStats{}
	sourceDir := filepath.Clean(strings.TrimSpace(req.SourceDir))
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return stats, fmt.Errorf("read naming source dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name()) })
	rules := parseNamingRules(req.TransformRules)
	for index, entry := range entries {
		stats.ProcessedFiles++
		oldName := entry.Name()
		newName := applyNamingRules(oldName, entry.IsDir(), index+1, rules)
		if newName == oldName || strings.TrimSpace(newName) == "" {
			stats.SkipCount++
			continue
		}
		oldPath, newPath := filepath.Join(sourceDir, oldName), filepath.Join(sourceDir, newName)
		if _, statErr := os.Stat(newPath); statErr == nil {
			stats.SkipCount++
			s.appendLog(runID, "warning", fmt.Sprintf("命名跳过，目标已存在：%s", newPath))
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			stats.FailureCount++
			s.appendLog(runID, "error", fmt.Sprintf("重命名失败：%s -> %s：%v", oldName, newName, err))
			continue
		}
		stats.SuccessCount++
		s.appendLog(runID, "info", fmt.Sprintf("已命名：%s -> %s", oldName, newName))
	}
	stats.Summary = fmt.Sprintf("命名完成：成功 %d，跳过 %d，失败 %d", stats.SuccessCount, stats.SkipCount, stats.FailureCount)
	return stats, nil
}

func parseNamingRules(items []string) []namingRuleDefinition {
	result := make([]namingRuleDefinition, 0, len(items))
	for _, item := range items {
		var rule namingRuleDefinition
		if json.Unmarshal([]byte(item), &rule) == nil && rule.Category != "" {
			result = append(result, rule)
		}
	}
	return result
}

func applyNamingRules(name string, isDir bool, order int, rules []namingRuleDefinition) string {
	value := name
	for _, rule := range rules {
		cfg := rule.Config
		switch rule.Category {
		case "replace":
			value = transformNamingBase(value, isDir, cfg.IgnoreExtension, func(v string) string { return strings.ReplaceAll(v, cfg.Match, cfg.Replacement) })
		case "rewrite":
			_, ext := splitNamingExtension(value, isDir)
			value = fmt.Sprintf("%s %d%s", cfg.Name, order, ext)
		case "regex":
			if expression, err := regexp.Compile(cfg.Pattern); err == nil {
				value = transformNamingBase(value, isDir, cfg.IgnoreExtension, func(v string) string {
					if cfg.Global {
						return expression.ReplaceAllString(v, cfg.Replacement)
					}
					location := expression.FindStringIndex(v)
					if location == nil {
						return v
					}
					return v[:location[0]] + expression.ReplaceAllString(v[location[0]:location[1]], cfg.Replacement) + v[location[1]:]
				})
			}
		case "pad":
			value = applyNamingPad(value, isDir, cfg)
		}
	}
	return value
}

func splitNamingExtension(name string, isDir bool) (string, string) {
	if isDir {
		return name, ""
	}
	ext := filepath.Ext(name)
	if ext == name {
		return name, ""
	}
	return strings.TrimSuffix(name, ext), ext
}
func transformNamingBase(name string, isDir, ignore bool, transform func(string) string) string {
	if !ignore {
		return transform(name)
	}
	base, ext := splitNamingExtension(name, isDir)
	return transform(base) + ext
}
func applyNamingPad(name string, isDir bool, cfg namingRuleConfig) string {
	base, ext := name, ""
	if cfg.IgnoreExtension {
		base, ext = splitNamingExtension(name, isDir)
	}
	chars := []rune(base)
	index := cfg.Index - 1
	if index < 0 {
		index = 0
	}
	if index > len(chars) {
		index = len(chars)
	}
	if cfg.Position == "after" && index < len(chars) {
		index++
	}
	content := []rune(cfg.Content)
	if cfg.Smart && index+len(content) <= len(chars) && string(chars[index:index+len(content)]) == cfg.Content {
		return name
	}
	result := append(append(append([]rune{}, chars[:index]...), content...), chars[index:]...)
	if !utf8.ValidString(string(result)) {
		return name
	}
	return string(result) + ext
}
