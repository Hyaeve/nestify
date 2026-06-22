<template>
  <div class="dashboard-view">
    <section class="dashboard-hero">
      <div class="dashboard-hero__content">
        <div class="dashboard-hero__eyebrow">DASHBOARD</div>
        <h1>运行总览</h1>
        <p>集中查看规则状态、今日处理量、实时任务与系统资源，快速掌握 Nestify 当前运行情况。</p>
        <div class="dashboard-hero__meta">
          <span>服务：{{ healthService }}</span>
          <span>检查时间：{{ healthTime }}</span>
        </div>
      </div>
      <div class="dashboard-hero__status">
        <span class="dashboard-live-dot"></span>
        <span>{{ healthStatus }}</span>
        <el-tag :type="healthTagType" effect="light" round>{{ healthStatus }}</el-tag>
      </div>
    </section>

    <section class="dashboard-overview">
      <div class="metric-card metric-card--rules">
        <div class="metric-card__icon">规</div>
        <div>
          <div class="metric-card__label">总规则数</div>
          <div class="metric-card__value">{{ totalRuleCount }}</div>
          <div class="metric-card__hint">已启用 {{ enabledRuleCount }} 条规则</div>
        </div>
      </div>
      <div class="metric-card metric-card--processed">
        <div class="metric-card__icon">处</div>
        <div>
          <div class="metric-card__label">今日处理</div>
          <div class="metric-card__value">{{ todayProcessedCount }}</div>
          <div class="metric-card__hint">今日执行 {{ todayRunCount }} 次任务</div>
        </div>
      </div>
      <div class="metric-card metric-card--running">
        <div class="metric-card__icon">行</div>
        <div>
          <div class="metric-card__label">运行中任务</div>
          <div class="metric-card__value">{{ runningTaskCount }}</div>
          <div class="metric-card__hint">{{ runningTaskHint }}</div>
        </div>
      </div>
      <div class="metric-card metric-card--resource">
        <div class="metric-card__icon">资</div>
        <div>
          <div class="metric-card__label">系统资源</div>
          <div class="metric-card__value">{{ formatPercentage(systemResource?.cpu_usage) }}</div>
          <div class="metric-card__hint">内存 {{ formatPercentage(systemResource?.memory_usage) }}</div>
        </div>
      </div>
    </section>

    <section class="dashboard-content">
      <div class="dashboard-panel dashboard-panel--summary">
        <div class="dashboard-panel__header">
          <div>
            <div class="dashboard-panel__eyebrow">EXECUTION</div>
            <h3 class="page-section-title">最近执行摘要</h3>
          </div>
          <span class="dashboard-panel__count">{{ summaryItems.length }} 条记录</span>
        </div>
        <div v-if="summaryItems.length" class="summary-list">
          <div v-for="item in summaryItems" :key="item.id" class="summary-item">
            <div class="summary-item__header">
              <div class="summary-item__title">
                <span class="summary-item__dot"></span>
                <span class="summary-item__name">{{ item.rule_name || '未知规则' }}</span>
              </div>
              <div class="summary-item__badges">
                <span class="dashboard-mode-tag" :class="dashboardModeTagClass(item.archive_mode)">{{ dashboardModeText(item.archive_mode) }}</span>
                <el-tag :type="getStatusType(item.status)" effect="light" size="small">{{ getStatusText(item.status) }}</el-tag>
              </div>
            </div>
            <div class="summary-item__meta">
              <span>{{ formatDate(item.started_at) }}</span>
              <span>{{ formatRunHistorySummary(item.summary) || '无摘要' }}</span>
            </div>
          </div>
        </div>
        <el-empty v-else class="dashboard-empty" description="暂无执行摘要" />
      </div>

      <div class="dashboard-side-stack">
        <div class="dashboard-panel resource-card">
          <div class="dashboard-panel__header">
            <div>
              <div class="dashboard-panel__eyebrow">RESOURCE</div>
              <h3 class="page-section-title">系统资源</h3>
            </div>
            <el-tag :type="healthTagType" effect="light" size="small">{{ healthStatus }}</el-tag>
          </div>

          <div class="resource-stack">
            <div class="resource-metric">
              <div class="resource-metric__head">
                <div class="resource-metric__main">
                  <span class="resource-metric__icon">⚙</span>
                  <span class="resource-metric__label">CPU</span>
                </div>
                <span class="resource-metric__value">{{ formatPercentage(systemResource?.cpu_usage) }}</span>
              </div>
              <div class="resource-metric__desc">{{ systemResource?.cpu_model || '未知型号' }}</div>
              <el-progress class="resource-progress" :percentage="systemResource?.cpu_usage ?? 0" :show-text="false" :stroke-width="12" color="#2563eb" />
            </div>

            <div class="resource-metric">
              <div class="resource-metric__head">
                <div class="resource-metric__main">
                  <span class="resource-metric__icon">▣</span>
                  <span class="resource-metric__label">内存</span>
                </div>
                <span class="resource-metric__value">{{ formatPercentage(systemResource?.memory_usage) }}</span>
              </div>
              <div class="resource-metric__desc">{{ formatMemorySummary }}</div>
              <el-progress class="resource-progress" :percentage="systemResource?.memory_usage ?? 0" :show-text="false" :stroke-width="12" color="#7c3aed" />
            </div>

            <div class="resource-highlight">
              <span class="resource-highlight__label">Nestify 内存占用</span>
              <span class="resource-highlight__value">{{ systemResource?.nestify_memory || '0 B' }}</span>
            </div>
          </div>

          <el-alert
            v-if="healthError"
            class="resource-card__alert"
            type="error"
            :closable="false"
            :title="healthError"
          />
        </div>

        <div class="dashboard-panel task-preview-card">
          <div class="task-preview-card__header">
            <div>
              <div class="dashboard-panel__eyebrow">LIVE TASKS</div>
              <h3 class="page-section-title">任务预览</h3>
            </div>
            <div class="task-preview-card__actions">
              <el-tag :type="runningPreviewItems.length ? 'success' : 'info'" effect="light" size="small">
                {{ runningPreviewItems.length ? `${runningPreviewItems.length} 个任务` : '暂无任务' }}
              </el-tag>
              <el-button class="task-preview-card__refresh" text size="small" :loading="previewRefreshing" @click="refreshRunningPreview">刷新</el-button>
            </div>
          </div>

          <div v-if="runningPreviewItems.length" class="task-preview-list">
            <div v-for="item in runningPreviewItems" :key="item.id" class="task-preview-item">
              <div class="task-preview-item__header">
                <div>
                  <div class="task-preview-item__name">{{ item.ruleName }}</div>
                  <div class="task-preview-item__meta">{{ dashboardModeText(item.archive_mode) }} · {{ item.runModeText }}</div>
                  <div class="task-preview-item__detail">{{ runDetailText(item) }}</div>
                </div>
                <div class="task-preview-item__badges">
                  <span class="dashboard-mode-tag" :class="dashboardModeTagClass(item.archive_mode)">{{ dashboardModeText(item.archive_mode) }}</span>
                  <el-tag type="warning" effect="light" size="small">进行中</el-tag>
                </div>
              </div>

              <el-progress :percentage="estimateRunProgress(item)" :stroke-width="8" :show-text="false" status="success" />

              <div class="task-preview-item__stats">
                <span>成功 {{ item.success_count }}</span>
                <span>跳过 {{ item.skip_count }}</span>
                <span>失败 {{ item.failure_count }}</span>
              </div>

              <div class="task-preview-item__path" :title="item.sourceDir">{{ item.sourceDir || '未配置源路径' }}</div>
              <div class="task-preview-item__logs">
                <div v-if="previewLogsLoadingMap[item.id]" class="task-preview-item__logs-loading">执行情况加载中...</div>
                <template v-else>
                  <div v-for="log in getPreviewLogs(item.id)" :key="log.id" class="task-preview-item__log-line">
                    {{ formatRunLogLine(log) }}
                  </div>
                  <div v-if="!getPreviewLogs(item.id).length" class="task-preview-item__logs-empty">暂无执行日志</div>
                </template>
              </div>
            </div>
          </div>

          <el-empty v-else class="task-preview-empty" description="当前没有正在执行的规则任务" />
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { fetchRunLogs, fetchRuns, type RunInstance, type RunLogEntry } from '../api/executions'
import { fetchHealth, fetchSystemResource, type HealthPayload, type SystemResourcePayload } from '../api/system'
import { emptyRunHistory, fetchRunHistory, type RunHistoryItem } from '../api/runHistory'
import { formatRunHistorySummary } from '../utils/runHistorySummary'
import { fetchRules, type RuleItem } from '../api/rules'

const health = ref<HealthPayload | null>(null)
const healthError = ref('')
const loading = ref(false)
const summaryItems = ref<RunHistoryItem[]>(emptyRunHistory())
const systemResource = ref<SystemResourcePayload | null>(null)
const rules = ref<RuleItem[]>([])
const runHistoryItems = ref<RunHistoryItem[]>(emptyRunHistory())
type RunningPreviewItem = RunInstance & { ruleName: string; runModeText: string; sourceDir: string }
const runningRuns = ref<RunningPreviewItem[]>([])
const dashboardExecutionHints = ref<Record<number, string>>({})
const previewRefreshing = ref(false)
const previewLogsMap = ref<Record<string, RunLogEntry[]>>({})
const previewLogsLoadingMap = ref<Record<string, boolean>>({})
let previewPollTimer: number | null = null

const healthStatus = computed(() => {
  if (loading.value) return '检查中'
  if (health.value) return '已连接'
  return '未连接'
})

const healthService = computed(() => health.value?.service ?? '未知')
const healthTime = computed(() => health.value?.time ?? '尚未获取')
const healthTagType = computed(() => (health.value ? 'success' : loading.value ? 'warning' : 'danger'))
const formatMemorySummary = computed(() => {
  if (!systemResource.value) {
    return '0 B / 0 B'
  }

  return `${systemResource.value.memory_used} / ${systemResource.value.memory_total}`
})

const totalRuleCount = computed(() => rules.value.length)
const enabledRuleCount = computed(() => rules.value.filter((item) => item.enabled).length)
const runningPreviewItems = computed(() => runningRuns.value.slice(0, 2))

const todayRunCount = computed(() => {
  const today = new Date()
  const key = `${today.getFullYear()}-${today.getMonth()}-${today.getDate()}`

  return runHistoryItems.value.filter((item) => {
    const date = new Date(item.started_at)
    return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}` === key
  }).length
})

const todayProcessedCount = computed(() => {
  const today = new Date()
  const key = `${today.getFullYear()}-${today.getMonth()}-${today.getDate()}`

  return runHistoryItems.value
    .filter((item) => {
      const date = new Date(item.started_at)
      return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}` === key
    })
    .reduce((total, item) => total + (item.processed_files || 0), 0)
})

const runningTaskCount = computed(() => runningRuns.value.length)
const runningTaskHint = computed(() => (runningTaskCount.value > 0 ? '存在正在执行的规则任务' : '当前没有活跃任务'))

function getStatusType(status: string) {
  if (status === 'success' || status === 'succeeded') return 'success'
  if (status === 'skip' || status === 'skipped') return 'warning'
  return 'danger'
}

function getStatusText(status: string) {
  if (status === 'success' || status === 'succeeded') return '成功'
  if (status === 'skip' || status === 'skipped') return '跳过'
  return '失败'
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

function formatPercentage(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '0%'
  }

  return `${value.toFixed(1)}%`
}

function archiveModeText(mode: RuleItem['archive_mode']) {
  if (mode === 'package') return '打包'
  if (mode === 'collect') return '收集'
  if (mode === 'cleanup') return '清理'
  if (mode === 'transform') return '转换'
  if (mode === 'link') return '链路'
  return '未知'
}

function dashboardModeText(mode?: string) {
  if (mode === 'package') return '打包'
  if (mode === 'collect') return '收集'
  if (mode === 'cleanup') return '清理'
  if (mode === 'transform') return '转换'
  if (mode === 'link') return '链路'
  return '未知'
}

function dashboardModeTagClass(mode?: string) {
  if (mode === 'package') return 'dashboard-mode-tag--package'
  if (mode === 'collect') return 'dashboard-mode-tag--collect'
  if (mode === 'cleanup') return 'dashboard-mode-tag--cleanup'
  if (mode === 'transform') return 'dashboard-mode-tag--transform'
  if (mode === 'link') return 'dashboard-mode-tag--hardlink'
  return ''
}

function runDetailText(item: RunningPreviewItem) {
	const executionHint = item.rule_id ? dashboardExecutionHints.value[item.rule_id] : ''
	const currentTarget = executionHint || item.current_volume_or_dir || item.current_series || '正在扫描源目录'
	return `当前执行：${currentTarget} · ${dashboardModeText(item.archive_mode)}`
}

function runModeText(mode: RuleItem['run_mode']) {
  if (mode === 'watch') return '监听模式'
  if (mode === 'cron') return '定时模式'
  return '手动模式'
}

function estimateProgress(item: RuleItem) {
  const total = item.last_success_count + item.last_skip_count + item.last_failure_count
  if (total <= 0) {
    return 12
  }

  return Math.min(92, Math.max(18, total % 100))
}

function estimateRunProgress(item: RunningPreviewItem) {
	const total = item.success_count + item.skip_count + item.failure_count
	if (total <= 0) {
		return 12
	}

	return Math.min(92, Math.max(18, total % 100))
}

function getPreviewLogs(runID: string) {
	return previewLogsMap.value[runID] ?? []
}

function formatRunLogLine(log: RunLogEntry) {
	const time = new Date(log.created_at).toLocaleTimeString('zh-CN', { hour12: false })
	return `[${time}] ${log.message}`
}

async function loadHealth() {
  loading.value = true
  healthError.value = ''

  try {
    const response = await fetchHealth()
    health.value = response.data ?? null
  } catch (error) {
    health.value = null
    healthError.value = error instanceof Error ? error.message : '后端连接失败'
  } finally {
    loading.value = false
  }
}

async function loadSummary() {
  try {
    const items = (await fetchRunHistory()).data?.items ?? []
    runHistoryItems.value = items
    summaryItems.value = items.slice(0, 6)
  } catch {
    runHistoryItems.value = []
    summaryItems.value = []
  }
}

async function loadRules() {
  try {
    rules.value = (await fetchRules()).data?.items ?? []
    const runs = (await fetchRuns()).data?.items ?? []
    const runningRuleItems = rules.value.reduce<Record<number, RuleItem>>((map, item) => {
      map[item.id] = item
      return map
    }, {})
    runningRuns.value = runs
      .filter((item) => item.status === 'running' && typeof item.rule_id === 'number')
      .map((item) => {
        const rule = item.rule_id ? runningRuleItems[item.rule_id] : undefined
        return {
          ...item,
          ruleName: rule?.name || item.rule_name || '未知规则',
          runModeText: rule ? runModeText(rule.run_mode) : (item.trigger_mode === 'cron' ? '定时模式' : item.trigger_mode === 'watch' ? '监听模式' : '手动模式'),
          sourceDir: rule?.source_dir || '',
        }
      })
    dashboardExecutionHints.value = {}
    await loadRunningPreviewLogs(runningRuns.value)
  } catch {
    rules.value = []
    runningRuns.value = []
    dashboardExecutionHints.value = {}
    previewLogsMap.value = {}
    previewLogsLoadingMap.value = {}
  }
}

async function loadRunningPreviewLogs(items: RunningPreviewItem[]) {
	const nextLogsMap: Record<string, RunLogEntry[]> = { ...previewLogsMap.value }
	const nextLoadingMap: Record<string, boolean> = { ...previewLogsLoadingMap.value }

	await Promise.all(items.map(async (item) => {
		nextLoadingMap[item.id] = true
		try {
			const response = await fetchRunLogs(item.id)
			nextLogsMap[item.id] = (response.data?.items ?? []).slice(-5)
		} catch {
			nextLogsMap[item.id] = []
		} finally {
			nextLoadingMap[item.id] = false
		}
	}))

	previewLogsMap.value = nextLogsMap
	previewLogsLoadingMap.value = nextLoadingMap
}

async function refreshRunningPreview() {
	previewRefreshing.value = true
	try {
		await loadRules()
	} finally {
		previewRefreshing.value = false
	}
}

function startPreviewPolling() {
	stopPreviewPolling()
	previewPollTimer = window.setInterval(() => {
		void refreshRunningPreview()
	}, 5000)
}

function stopPreviewPolling() {
	if (previewPollTimer !== null) {
		window.clearInterval(previewPollTimer)
		previewPollTimer = null
	}
}

async function loadSystemResource() {
  try {
    const response = await fetchSystemResource()
    systemResource.value = response.data ?? null
  } catch {
    systemResource.value = null
  }
}

onMounted(() => {
  void loadHealth()
  void loadSummary()
  void loadSystemResource()
  void loadRules()
  startPreviewPolling()
})

onBeforeUnmount(() => {
	stopPreviewPolling()
})
</script>

<style scoped lang="scss">
.dashboard-view {
  display: flex;
  flex-direction: column;
  gap: 22px;
  padding-bottom: 12px;
}

.dashboard-hero {
  position: relative;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  overflow: hidden;
  padding: 34px 36px;
  border-radius: 30px;
  border: 1px solid #dbeafe;
  color: #0f172a;
  background:
    linear-gradient(rgba(37, 99, 235, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(37, 99, 235, 0.045) 1px, transparent 1px),
    radial-gradient(circle at 12% 18%, rgba(186, 230, 253, 0.55), transparent 28%),
    radial-gradient(circle at 86% 20%, rgba(224, 231, 255, 0.9), transparent 24%),
    linear-gradient(100deg, #eff8ff 0%, #ffffff 45%, #f4f7ff 100%);
  background-size: 54px 54px, 54px 54px, auto, auto, auto;
  box-shadow: 0 18px 42px rgba(37, 99, 235, 0.08);
}

.dashboard-hero::after {
  position: absolute;
  right: -42px;
  top: -72px;
  width: 280px;
  height: 280px;
  border-radius: 999px;
  background: radial-gradient(circle, rgba(99, 102, 241, 0.12), transparent 66%);
  content: '';
}

.dashboard-hero__content,
.dashboard-hero__status {
  position: relative;
  z-index: 1;
}

.dashboard-hero__eyebrow,
.dashboard-panel__eyebrow {
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.dashboard-hero__eyebrow {
  margin-bottom: 10px;
  color: #1e9bff;
}

.dashboard-hero h1 {
  margin: 0;
  font-size: 34px;
  line-height: 1.18;
  font-weight: 900;
}

.dashboard-hero p {
  max-width: 680px;
  margin: 12px 0 0;
  color: #475569;
  font-size: 15px;
  line-height: 1.8;
}

.dashboard-hero__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 18px;
}

.dashboard-hero__meta span {
  display: inline-flex;
  align-items: center;
  min-height: 30px;
  padding: 0 12px;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  background: rgba(219, 234, 254, 0.66);
  color: #2563eb;
  font-size: 12px;
  font-weight: 800;
}

.dashboard-hero__status {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex: 0 0 auto;
  min-height: 44px;
  padding: 8px 12px 8px 14px;
  border: 1px solid #86efac;
  border-radius: 999px;
  background: rgba(220, 252, 231, 0.72);
  color: #16a34a;
  font-size: 13px;
  font-weight: 900;
  backdrop-filter: blur(12px);
}

.dashboard-live-dot {
  width: 9px;
  height: 9px;
  border-radius: 999px;
  background: #34d399;
  box-shadow: 0 0 0 6px rgba(52, 211, 153, 0.18);
}

.dashboard-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 18px;
}

.metric-card {
  display: flex;
  align-items: center;
  gap: 16px;
  min-height: 108px;
  padding: 22px;
  border: 1px solid #eef2f7;
  border-radius: 24px;
  background: #ffffff;
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.07);
}

.metric-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 52px;
  height: 52px;
  border-radius: 18px;
  color: #ffffff;
  font-size: 18px;
  font-weight: 900;
  box-shadow: 0 12px 24px rgba(37, 99, 235, 0.18);
}

.metric-card--rules .metric-card__icon { background: linear-gradient(135deg, #2563eb, #60a5fa); }
.metric-card--processed .metric-card__icon { background: linear-gradient(135deg, #7c3aed, #a78bfa); }
.metric-card--running .metric-card__icon { background: linear-gradient(135deg, #f59e0b, #fbbf24); }
.metric-card--resource .metric-card__icon { background: linear-gradient(135deg, #10b981, #34d399); }

.metric-card__label {
  color: #64748b;
  font-size: 13px;
  font-weight: 900;
}

.metric-card__value {
  margin-top: 8px;
  color: #0f172a;
  font-size: 30px;
  line-height: 1;
  font-weight: 900;
}

.metric-card__hint {
  margin-top: 8px;
  color: #94a3b8;
  font-size: 12px;
  font-weight: 700;
}

.dashboard-content {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(360px, 0.7fr);
  gap: 18px;
  align-items: stretch;
}

.dashboard-panel {
  border: 1px solid #eef2f7;
  border-radius: 26px;
  background: #ffffff;
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.06);
}

.dashboard-panel--summary,
.resource-card,
.task-preview-card {
  padding: 24px;
}

.dashboard-panel--summary {
  min-height: 100%;
}

.dashboard-panel__header,
.task-preview-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 18px;
}

.dashboard-panel__eyebrow {
  margin-bottom: 7px;
  color: #2563eb;
}

.page-section-title {
  margin: 0;
  color: #0f172a;
  font-size: 18px;
  line-height: 1.25;
  font-weight: 900;
}

.dashboard-panel__count {
  display: inline-flex;
  align-items: center;
  min-height: 30px;
  padding: 0 12px;
  border-radius: 999px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 12px;
  font-weight: 900;
}

.dashboard-side-stack {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-width: 0;
}

.summary-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.summary-item,
.task-preview-item {
  padding: 16px;
  border: 1px solid #eef2f7;
  border-radius: 18px;
  background: #f8fafc;
}

.summary-item__header,
.task-preview-item__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.summary-item__title {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.summary-item__dot {
  width: 9px;
  height: 9px;
  border-radius: 999px;
  background: #2563eb;
  box-shadow: 0 0 0 5px rgba(37, 99, 235, 0.12);
}

.summary-item__badges,
.task-preview-item__badges,
.task-preview-card__actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
}

.summary-item__name,
.task-preview-item__name {
  color: #0f172a;
  font-size: 15px;
  font-weight: 900;
}

.dashboard-mode-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 54px;
  padding: 4px 10px;
  border: 1px solid currentColor;
  border-radius: 8px;
  background: #fff;
  font-size: 12px;
  line-height: 1.2;
  font-weight: 800;
}

.dashboard-mode-tag--package { color: #d58a2f; background: rgba(213, 138, 47, 0.08); }
.dashboard-mode-tag--collect { color: #8a74d6; background: rgba(138, 116, 214, 0.1); }
.dashboard-mode-tag--cleanup { color: #5f9f45; background: rgba(95, 159, 69, 0.12); }
.dashboard-mode-tag--transform { color: #64b9d8; background: rgba(100, 185, 216, 0.12); }
.dashboard-mode-tag--hardlink { color: #2f3136; background: rgba(47, 49, 54, 0.08); }

.summary-item__meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 10px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.6;
}

.dashboard-empty {
  min-height: 280px;
}

.resource-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.resource-metric {
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: linear-gradient(180deg, #f8fbff 0%, #f3f8ff 100%);
}

.resource-metric__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.resource-metric__main {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
  color: #0f172a;
  font-size: 15px;
  font-weight: 900;
}

.resource-metric__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 10px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 16px;
  line-height: 1;
}

.resource-metric__label,
.resource-metric__value {
  white-space: nowrap;
}

.resource-metric__value {
  color: #2563eb;
  font-size: 18px;
  font-weight: 900;
}

.resource-metric__desc {
  min-width: 0;
  margin-bottom: 10px;
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-progress :deep(.el-progress-bar__outer) {
  background-color: #dbeafe;
  border-radius: 999px;
}

.resource-progress :deep(.el-progress-bar__inner) {
  border-radius: 999px;
}

.resource-highlight {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px;
  border: 1px solid #bfdbfe;
  border-radius: 18px;
  background: #eff6ff;
}

.resource-highlight__label {
  color: #334155;
  font-size: 14px;
  font-weight: 900;
}

.resource-highlight__value {
  color: #2563eb;
  font-size: 20px;
  font-weight: 900;
  white-space: nowrap;
}

.resource-card__alert {
  margin-top: 16px;
}

.task-preview-card {
  flex: 1;
  min-height: 0;
}

.task-preview-card__refresh {
  color: #2563eb;
  font-weight: 900;
}

.task-preview-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-preview-item__header {
  margin-bottom: 10px;
}

.task-preview-item__meta {
  margin-top: 5px;
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.task-preview-item__detail {
  margin-top: 6px;
  color: #94a3b8;
  font-size: 12px;
  line-height: 1.5;
}

.task-preview-item__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 10px;
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.task-preview-item__stats span {
  padding: 4px 8px;
  border-radius: 999px;
  background: #ffffff;
}

.task-preview-item__path {
  margin-top: 10px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-preview-item__logs {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 10px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  color: #475569;
  font-size: 12px;
}

.task-preview-item__log-line {
  line-height: 1.6;
  word-break: break-all;
}

.task-preview-item__logs-loading,
.task-preview-item__logs-empty {
  color: #94a3b8;
}

.task-preview-empty {
  min-height: 120px;
}

:global(:root[data-theme='dark']) .dashboard-hero {
  border-color: rgba(96, 165, 250, 0.24);
  color: #f3f7ff;
  background:
    linear-gradient(rgba(96, 165, 250, 0.065) 1px, transparent 1px),
    linear-gradient(90deg, rgba(96, 165, 250, 0.065) 1px, transparent 1px),
    radial-gradient(circle at 9% 18%, rgba(59, 130, 246, 0.36), transparent 28%),
    radial-gradient(circle at 88% 24%, rgba(124, 58, 237, 0.34), transparent 30%),
    linear-gradient(100deg, rgba(24, 35, 55, 0.96) 0%, rgba(12, 17, 29, 0.96) 46%, rgba(35, 29, 58, 0.94) 100%);
  background-size: 54px 54px, 54px 54px, auto, auto, auto;
  box-shadow:
    0 20px 54px rgba(0, 0, 0, 0.3),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

:global(:root[data-theme='dark']) .dashboard-hero::after {
  background: radial-gradient(circle, rgba(96, 165, 250, 0.18), transparent 66%);
}

:global(:root[data-theme='dark']) .dashboard-hero p {
  color: #cbd5e1;
}

:global(:root[data-theme='dark']) .dashboard-hero__meta span {
  border-color: rgba(96, 165, 250, 0.26);
  background: rgba(37, 99, 235, 0.14);
  color: #93c5fd;
}

:global(:root[data-theme='dark']) .dashboard-hero__status {
  border-color: rgba(74, 222, 128, 0.28);
  background: rgba(22, 163, 74, 0.13);
  color: #86efac;
}

:global(:root[data-theme='dark']) .metric-card,
:global(:root[data-theme='dark']) .dashboard-panel {
  border-color: rgba(148, 163, 184, 0.22);
  background:
    linear-gradient(180deg, rgba(23, 30, 45, 0.96) 0%, rgba(15, 20, 32, 0.94) 100%) !important;
  box-shadow:
    0 18px 46px rgba(0, 0, 0, 0.26),
    inset 0 1px 0 rgba(255, 255, 255, 0.045);
}

:global(:root[data-theme='dark']) .metric-card__label,
:global(:root[data-theme='dark']) .metric-card__hint,
:global(:root[data-theme='dark']) .summary-item__meta,
:global(:root[data-theme='dark']) .task-preview-item__meta,
:global(:root[data-theme='dark']) .task-preview-item__stats,
:global(:root[data-theme='dark']) .task-preview-item__path,
:global(:root[data-theme='dark']) .resource-metric__desc {
  color: #c0cadc;
}

:global(:root[data-theme='dark']) .metric-card__value,
:global(:root[data-theme='dark']) .page-section-title,
:global(:root[data-theme='dark']) .summary-item__name,
:global(:root[data-theme='dark']) .task-preview-item__name,
:global(:root[data-theme='dark']) .resource-metric__main {
  color: #e5eefc;
}

:global(:root[data-theme='dark']) .summary-item,
:global(:root[data-theme='dark']) .task-preview-item {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(22, 29, 44, 0.94) !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

:global(:root[data-theme='dark']) .resource-metric {
  border-color: rgba(96, 165, 250, 0.24);
  background: linear-gradient(180deg, rgba(27, 36, 55, 0.96) 0%, rgba(16, 22, 36, 0.94) 100%) !important;
}

:global(:root[data-theme='dark']) .resource-metric__icon,
:global(:root[data-theme='dark']) .dashboard-panel__count {
  background: rgba(37, 99, 235, 0.16);
  color: #93c5fd;
}

:global(:root[data-theme='dark']) .resource-progress :deep(.el-progress-bar__outer) {
  background-color: rgba(96, 165, 250, 0.18);
}

:global(:root[data-theme='dark']) .resource-highlight {
  border-color: rgba(96, 165, 250, 0.24);
  background: rgba(37, 99, 235, 0.16) !important;
}

:global(:root[data-theme='dark']) .resource-highlight__label {
  color: #cbd5e1;
}

:global(:root[data-theme='dark']) .resource-highlight__value,
:global(:root[data-theme='dark']) .resource-metric__value,
:global(:root[data-theme='dark']) .task-preview-card__refresh {
  color: #60a5fa;
}

:global(:root[data-theme='dark']) .task-preview-item__stats span,
:global(:root[data-theme='dark']) .task-preview-item__logs,
:global(:root[data-theme='dark']) .dashboard-mode-tag {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(10, 15, 26, 0.62);
}

:global(:root[data-theme='dark']) .task-preview-item__logs {
  color: #cbd5e1;
  box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.04);
}

:global(:root[data-theme='dark']) .metric-card--rules .metric-card__icon,
:global(:root[data-theme='dark']) .metric-card--enabled .metric-card__icon,
:global(:root[data-theme='dark']) .metric-card--today .metric-card__icon,
:global(:root[data-theme='dark']) .metric-card--running .metric-card__icon {
  box-shadow: 0 14px 28px rgba(0, 0, 0, 0.22);
}

:global(:root[data-theme='dark']) .task-preview-item__detail,
:global(:root[data-theme='dark']) .task-preview-item__logs-loading,
:global(:root[data-theme='dark']) .task-preview-item__logs-empty {
  color: #7f8da4;
}

:global(:root[data-theme='appletv']) .dashboard-hero {
  border-color: rgba(96, 165, 250, 0.24);
  color: #f3f7ff;
  background:
    linear-gradient(rgba(96, 165, 250, 0.065) 1px, transparent 1px),
    linear-gradient(90deg, rgba(96, 165, 250, 0.065) 1px, transparent 1px),
    radial-gradient(circle at 9% 18%, rgba(59, 130, 246, 0.36), transparent 28%),
    radial-gradient(circle at 88% 24%, rgba(124, 58, 237, 0.34), transparent 30%),
    linear-gradient(100deg, rgba(24, 35, 55, 0.96) 0%, rgba(12, 17, 29, 0.96) 46%, rgba(35, 29, 58, 0.94) 100%);
  background-size: 54px 54px, 54px 54px, auto, auto, auto;
  box-shadow:
    0 20px 54px rgba(0, 0, 0, 0.3),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

:global(:root[data-theme='appletv']) .metric-card,
:global(:root[data-theme='appletv']) .dashboard-panel {
  border-color: rgba(148, 163, 184, 0.22);
  background:
    linear-gradient(180deg, rgba(23, 30, 45, 0.96) 0%, rgba(15, 20, 32, 0.94) 100%) !important;
  box-shadow:
    0 18px 46px rgba(0, 0, 0, 0.26),
    inset 0 1px 0 rgba(255, 255, 255, 0.045);
}

:global(:root[data-theme='appletv']) .summary-item,
:global(:root[data-theme='appletv']) .task-preview-item,
:global(:root[data-theme='appletv']) .resource-metric,
:global(:root[data-theme='appletv']) .resource-highlight {
  border-color: rgba(148, 163, 184, 0.22);
  background: rgba(22, 29, 44, 0.94) !important;
}

:global(:root[data-theme='appletv']) .metric-card__label,
:global(:root[data-theme='appletv']) .metric-card__hint,
:global(:root[data-theme='appletv']) .summary-item__meta,
:global(:root[data-theme='appletv']) .task-preview-item__meta,
:global(:root[data-theme='appletv']) .task-preview-item__stats,
:global(:root[data-theme='appletv']) .task-preview-item__path,
:global(:root[data-theme='appletv']) .resource-metric__desc,
:global(:root[data-theme='appletv']) .resource-highlight__label {
  color: #c0cadc;
}

:global(:root[data-theme='appletv']) .metric-card__value,
:global(:root[data-theme='appletv']) .page-section-title,
:global(:root[data-theme='appletv']) .summary-item__name,
:global(:root[data-theme='appletv']) .task-preview-item__name,
:global(:root[data-theme='appletv']) .resource-metric__main {
  color: #e5eefc;
}

@media (max-width: 1280px) {
  .dashboard-overview {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-content {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .dashboard-hero,
  .dashboard-panel__header,
  .task-preview-card__header,
  .summary-item__header,
  .task-preview-item__header {
    flex-direction: column;
  }

  .dashboard-overview {
    grid-template-columns: 1fr;
  }

  .dashboard-hero {
    padding: 28px 24px;
  }
}

:deep(.el-tag) {
  border-radius: 999px;
}
</style>

