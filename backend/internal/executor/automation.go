package executor

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"nestify/backend/internal/model"
)

func (s *Service) StartAutomation(ctx context.Context) error {
	if s.store == nil {
		return nil
	}

	s.automationMu.Lock()
	defer s.automationMu.Unlock()

	if s.automationCancel != nil {
		s.stopAutomationLocked()
	}

	s.automationCtx, s.automationCancel = context.WithCancel(ctx)
	return s.reloadAutomationLocked()
}

func (s *Service) ReloadAutomation() error {
	if s.store == nil {
		return nil
	}

	s.automationMu.Lock()
	defer s.automationMu.Unlock()

	if s.automationCtx == nil {
		return nil
	}

	return s.reloadAutomationLocked()
}

func (s *Service) reloadAutomationLocked() error {
	s.stopCronLocked()
	s.stopWatchersLocked()

	rules, err := s.store.ListRules()
	if err != nil {
		return fmt.Errorf("load rules for automation: %w", err)
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	cronRunner := cron.New(cron.WithParser(parser), cron.WithLocation(time.Local))
	hasCron := false
	log.Printf("executor: cron scheduler location=%s", time.Local.String())

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		if rule.RunOnStart {
			go s.triggerRule(rule, model.TriggerModeOnce)
		}

		switch strings.TrimSpace(rule.RunMode) {
		case model.TriggerModeWatch:
			s.startWatchRuleLocked(rule)
		case model.TriggerModeCron:
			expression := strings.TrimSpace(rule.CronExpression)
			if expression == "" {
				continue
			}
			capturedRule := rule
			if _, addErr := cronRunner.AddFunc(expression, func() {
				s.triggerRule(capturedRule, model.TriggerModeCron)
			}); addErr != nil {
				return fmt.Errorf("register cron for rule %s: %w", rule.Name, addErr)
			}
			hasCron = true
		}
	}

	if hasCron {
		cronRunner.Start()
		s.cronRunner = cronRunner
	}

	return nil
}

func (s *Service) stopAutomationLocked() {
	s.stopCronLocked()
	s.stopWatchersLocked()
	if s.automationCancel != nil {
		s.automationCancel()
		s.automationCancel = nil
	}
	s.automationCtx = nil
}

func (s *Service) stopCronLocked() {
	if s.cronRunner != nil {
		ctx := s.cronRunner.Stop()
		<-ctx.Done()
		s.cronRunner = nil
	}
}

func (s *Service) stopWatchersLocked() {
	for id, cancel := range s.watchCancels {
		cancel()
		delete(s.watchCancels, id)
	}
}

func (s *Service) startWatchRuleLocked(rule model.Rule) {
	if s.automationCtx == nil {
		return
	}

	ctx, cancel := context.WithCancel(s.automationCtx)
	s.watchCancels[rule.ID] = cancel

	interval := time.Duration(rule.WatchDebounceMS) * time.Millisecond
	if interval < 2*time.Second {
		interval = 2 * time.Second
	}
	if isCompatibilityMode(rule.CompatibilityMode) && interval < 10*time.Second {
		interval = 10 * time.Second
	}

	go func() {
		baseline, _ := s.directoryFingerprint(rule.CompatibilityMode, rule.SourceDir)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, err := s.directoryFingerprint(rule.CompatibilityMode, rule.SourceDir)
				if err != nil {
					continue
				}
				if current != baseline {
					baseline = current
					s.triggerRule(rule, model.TriggerModeWatch)
				}
			}
		}
	}()
}

func (s *Service) triggerRule(rule model.Rule, triggerMode string) {
	_, _ = s.PrepareRuleRun(ExecuteRuleRequest{
		RuleID:            rule.ID,
		RuleName:          rule.Name,
		ArchiveMode:       rule.ArchiveMode,
		RuleType:          rule.RuleType,
		LinkMode:          rule.LinkMode,
		TriggerMode:       triggerMode,
		CompatibilityMode: rule.CompatibilityMode,
		SourceDir:         rule.SourceDir,
		TargetDir:         rule.TargetDir,
		Options:           ParseBoolOptionsJSON(rule.OptionsJSON),
		OptionValues:      ParseIntOptionsJSON(rule.OptionValuesJSON),
		PackageOptions:    ParseBoolOptionsJSON(rule.PackageOptionsJSON),
		CollectOptions:    ParseBoolOptionsJSON(rule.CollectOptionsJSON),
		Filters:           ParseStringListJSON(rule.FiltersJSON),
		Whitelist:         ParseStringListJSON(rule.WhitelistJSON),
		MatchFilters:      ParseStringListJSON(rule.MatchFiltersJSON),
		NestFilters:       ParseStringListJSON(rule.NestFiltersJSON),
		TransformRules:    ParseTransformRulesJSON(rule.TransformRulesJSON),
		TransformFilters:  ParseStringListJSON(rule.TransformFiltersJSON),
	})
}

func (s *Service) directoryFingerprint(mode, root string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", nil
	}

	info, err := statWithMode(mode, root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return fmt.Sprintf("file:%s:%d:%d", filepath.Base(root), info.Size(), info.ModTime().UnixNano()), nil
	}

	items := []string{fmt.Sprintf(".|%t|%d|%d", info.IsDir(), info.Size(), info.ModTime().UnixNano())}
	if err := s.collectFingerprint(mode, root, root, &items); err != nil {
		return "", err
	}

	sort.Strings(items)
	return strings.Join(items, "\n"), nil
}

func (s *Service) collectFingerprint(mode, root, current string, items *[]string) error {
	entries, err := readDirWithMode(mode, current)
	if err != nil {
		return err
	}

	sortEntriesNaturally(entries)
	for _, entry := range entries {
		entryPath := filepath.Join(current, entry.Name())
		info, infoErr := entryInfoWithMode(mode, entry)
		if infoErr != nil {
			return infoErr
		}
		rel, relErr := filepath.Rel(root, entryPath)
		if relErr != nil {
			return relErr
		}
		*items = append(*items, fmt.Sprintf("%s|%t|%d|%d", filepath.ToSlash(rel), info.IsDir(), info.Size(), info.ModTime().UnixNano()))
		if entry.IsDir() {
			if err := s.collectFingerprint(mode, root, entryPath, items); err != nil {
				return err
			}
		}
	}

	return nil
}
