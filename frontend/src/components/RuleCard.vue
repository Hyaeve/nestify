<template>
  <article
    class="rule-card"
    :class="{ 'is-disabled': !rule.enabled, 'is-busy': busy }"
    @click="handleCardClick"
    @contextmenu.prevent="emit('contextmenu', rule, $event)"
  >
    <header class="rule-card__head">
      <div class="rule-card__title-row">
        <button type="button" class="rule-card__grip" aria-label="拖拽排序" title="拖拽排序" @click.stop>
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="9" cy="6" r="1.5" />
            <circle cx="15" cy="6" r="1.5" />
            <circle cx="9" cy="12" r="1.5" />
            <circle cx="15" cy="12" r="1.5" />
            <circle cx="9" cy="18" r="1.5" />
            <circle cx="15" cy="18" r="1.5" />
          </svg>
        </button>
        <h3 class="rule-card__title" :title="rule.name">{{ rule.name }}</h3>
      </div>
      <span class="rule-card__mode" :class="modeClass">{{ modeLabel }}</span>
    </header>

    <div class="rule-card__body">
      <div v-for="path in paths" :key="path.label" class="rule-card__row">
        <span class="rule-card__row-label">{{ path.label }}</span>
        <span
          class="rule-card__row-value"
          :class="{ 'is-empty': !path.value }"
          @mouseenter="syncOverflowTitle($event, path.value)"
        >
          {{ path.value || '未设置' }}
        </span>
      </div>

      <div class="rule-card__row">
        <span class="rule-card__row-label">Cron</span>
        <el-tooltip placement="top" effect="light" :show-after="400" :disabled="!rule.cron_expression" @show="emit('cron-preview', rule)">
          <template #content>
            <div class="rule-card__cron-tip">
              <template v-if="cronLoading">加载中...</template>
              <template v-else-if="cronError">{{ cronError }}</template>
              <template v-else>
                <div class="rule-card__cron-tip-title">最近三次执行时间</div>
                <div v-for="item in cronDisplayItems" :key="item" class="rule-card__cron-tip-item">{{ item }}</div>
              </template>
            </div>
          </template>
          <span class="rule-card__row-value rule-card__row-value--mono" :class="{ 'is-empty': !rule.cron_expression }">
            {{ rule.cron_expression || '未设置计划' }}
          </span>
        </el-tooltip>
      </div>

      <div v-if="extraTags.length" class="rule-card__tags">
        <span v-for="tag in extraTags" :key="tag" class="rule-card__tag">{{ tag }}</span>
      </div>
    </div>

    <footer class="rule-card__foot">
      <CardActionBar
        :enabled="rule.enabled"
        :busy="busy"
        :strm-sync="strmSync"
        @toggle="emit('toggle', rule)"
        @execute="emit('execute', rule)"
        @strm-sync="(command) => emit('strm-sync', rule, command)"
        @edit="emit('edit', rule)"
        @remove="emit('remove', rule)"
      />
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'

import CardActionBar from './CardActionBar.vue'

import { syncOverflowTitle } from '../utils/overflowTitle'

import type { RuleItem } from '../api/rules'

const props = withDefaults(
  defineProps<{
    rule: RuleItem
    modeLabel: string
    modeClass: string
    paths?: Array<{ label: string; value: string }>
    extraTags?: string[]
    busy?: boolean
    cronItems?: string[]
    cronLoading?: boolean
    cronError?: string
    /** Strm 模式的链路规则：执行按钮改为「增量同步 / 全量同步」下拉。 */
    strmSync?: boolean
  }>(),
  {
    paths: () => [],
    extraTags: () => [],
    busy: false,
    cronItems: () => [],
    cronLoading: false,
    cronError: '',
    strmSync: false,
  },
)

const emit = defineEmits<{
  (e: 'edit', rule: RuleItem): void
  (e: 'execute', rule: RuleItem): void
  (e: 'strm-sync', rule: RuleItem, command: string): void
  (e: 'remove', rule: RuleItem): void
  (e: 'toggle', rule: RuleItem): void
  (e: 'cron-preview', rule: RuleItem): void
  (e: 'contextmenu', rule: RuleItem, event: MouseEvent): void
}>()

/**
 * 后端 cron-preview 返回的是带时区偏移的 RFC3339（如 2026-09-15T08:00:00+08:00），
 * 原样渲染会在执行时间后面拖一个「+08:00」。这里统一按中国上海时间格式化成
 * 「2026-09-15 08:00:00」，直接给出执行时刻，且不受浏览者本机时区影响。
 */
function formatCronMoment(value: string): string {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value

  const parts = new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(parsed)
  const pick = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value ?? ''

  return `${pick('year')}-${pick('month')}-${pick('day')} ${pick('hour')}:${pick('minute')}:${pick('second')}`
}

const cronDisplayItems = computed(() => (props.cronItems ?? []).map(formatCronMoment))

function handleCardClick() {
  emit('edit', props.rule)
}
</script>

<style scoped lang="scss">
.rule-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 16px 12px;
  border: 1px solid var(--border-color);
  border-radius: 16px;
  background: var(--bg-elevated);
  box-shadow: 0 6px 18px rgba(15, 23, 42, 0.05);
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    transform 0.18s ease,
    opacity 0.18s ease;
}

.rule-card:hover {
  border-color: rgba(32, 159, 238, 0.42);
  box-shadow: 0 12px 26px rgba(32, 159, 238, 0.14);
  transform: translateY(-1px);
}

.rule-card.is-disabled {
  opacity: 0.72;
}

.rule-card.is-busy {
  pointer-events: none;
}

.rule-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.rule-card__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.rule-card__grip {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border: 0;
  border-radius: 6px;
  color: var(--text-secondary);
  background: transparent;
  cursor: grab;
  transition:
    color 0.16s ease,
    background-color 0.16s ease;
}

.rule-card__grip svg {
  width: 16px;
  height: 16px;
  fill: currentColor;
}

.rule-card__grip:hover {
  color: #0975b8;
  background: rgba(32, 159, 238, 0.12);
}

.rule-card__grip:active {
  cursor: grabbing;
}

.rule-card__title {
  margin: 0;
  min-width: 0;
  font-size: 17px;
  font-weight: 800;
  line-height: 1.35;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 与「运行日志」页模式标识（.logs-mode-tag）保持完全一致的观感：矩形 + 同色描边 + 8px 圆角。 */
.rule-card__mode {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 62px;
  padding: 4px 12px;
  border: 1px solid currentColor;
  border-radius: 8px;
  background: #ffffff;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
}

/* 模式标识配色沿用规则页既有色板（组件内独立声明，scoped 样式才能命中）。 */
.rule-card__mode.custom-mode-tag--package { color: #d58a2f; background: rgba(213, 138, 47, 0.08); }
.rule-card__mode.custom-mode-tag--collect { color: #8a74d6; background: rgba(138, 116, 214, 0.1); }
.rule-card__mode.custom-mode-tag--cleanup { color: #5f9f45; background: rgba(95, 159, 69, 0.12); }
.rule-card__mode.custom-mode-tag--transform { color: #64b9d8; background: rgba(100, 185, 216, 0.12); }
.rule-card__mode.custom-mode-tag--hardlink { color: #2f3136; background: rgba(47, 49, 54, 0.08); }
.rule-card__mode.custom-mode-tag--softlink { color: #c47c98; background: rgba(196, 124, 152, 0.12); }
.rule-card__mode.custom-mode-tag--strm { color: #2f8f9d; background: rgba(47, 143, 157, 0.12); }
.rule-card__mode.custom-mode-tag--naming { color: #0f8f79; background: rgba(15, 159, 135, 0.12); }
.rule-card__mode.custom-mode-tag--backup { color: #5f7fa8; background: rgba(95, 127, 168, 0.12); }

.rule-card__body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.rule-card__row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.rule-card__row-label {
  flex: 0 0 64px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.rule-card__row-value {
  flex: 1 1 auto;
  min-width: 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-card__row-value--mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.rule-card__row-value.is-empty {
  color: var(--text-secondary);
  font-style: italic;
}

.rule-card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 2px;
}

.rule-card__tag {
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  color: var(--text-secondary);
  background: rgba(148, 163, 184, 0.14);
}

.rule-card__foot {
  padding-top: 10px;
  border-top: 1px dashed var(--border-color);
}

.rule-card__cron-tip-title {
  margin-bottom: 6px;
  font-weight: 700;
}

.rule-card__cron-tip-item {
  line-height: 1.7;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}
</style>
