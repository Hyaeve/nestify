<template>
  <div class="dashboard-view">
    <section class="dashboard-hero">
      <div class="dashboard-hero__content">
        <div class="dashboard-hero__eyebrow">DASHBOARD</div>
        <h1>运行总览</h1>
        <p>集中查看规则状态、今日处理量、实时任务与系统资源，快速掌握 Nestify 当前运行情况。</p>
      </div>
      <div class="dashboard-hero__top-right">
        <div class="dashboard-hero__meta">
          <span>检查时间：{{ healthTime }}</span>
        </div>
        <div class="dashboard-hero__status">
          <span class="dashboard-live-dot"></span>
          <span>{{ healthStatus }}</span>
        </div>
      </div>
    </section>

    <section class="dashboard-content">
      <!-- 左侧大窗口：任务预览 -->
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
                <div class="task-preview-item__name">
                  <span class="task-preview-item__kind" :class="`task-preview-item__kind--${item.kind}`">{{ item.kind === 'backup' ? '备份' : '规则' }}</span>
                  {{ item.ruleName }}
                </div>
                <div class="task-preview-item__meta">{{ item.metaText }}</div>
                <div class="task-preview-item__detail">{{ runDetailText(item) }}</div>
              </div>
              <div class="task-preview-item__badges">
                <span v-if="item.kind === 'rule'" class="dashboard-mode-tag" :class="dashboardModeTagClass(item.archive_mode)">{{ dashboardModeText(item.archive_mode) }}</span>
                <el-tag type="warning" effect="light" size="small">进行中</el-tag>
              </div>
            </div>

            <el-progress :percentage="item.progress" :stroke-width="8" :show-text="false" status="success" />

            <div class="task-preview-item__stats">
              <span>扫描 {{ item.scanned }}</span>
              <span>成功 {{ item.success_count }}</span>
              <span>跳过 {{ item.skip_count }}</span>
              <span>失败 {{ item.failure_count }}</span>
            </div>

            <div class="task-preview-item__path" :title="item.sourceDir">{{ item.sourceDir || '未配置源路径' }}</div>
            <div class="task-preview-item__logs">
              <div v-if="item.logsLoading" class="task-preview-item__logs-loading">执行情况加载中...</div>
              <template v-else>
                <div v-for="(log, logIndex) in item.logs" :key="logIndex" class="task-preview-item__log-line">
                  {{ log }}
                </div>
                <div v-if="!item.logs.length" class="task-preview-item__logs-empty">暂无执行日志</div>
              </template>
            </div>
          </div>
        </div>

        <el-empty v-else class="task-preview-empty" description="当前没有正在执行的任务" />
      </div>

      <!-- 右侧：上执行摘要 + 下系统资源 -->
      <div class="dashboard-side-stack">
        <div class="dashboard-panel dashboard-panel--summary">
          <div class="dashboard-panel__header">
            <div>
              <div class="dashboard-panel__eyebrow">EXECUTION</div>
              <h3 class="page-section-title">执行摘要</h3>
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

        <div class="dashboard-panel resource-card">
          <div class="dashboard-panel__header">
            <div>
              <div class="dashboard-panel__eyebrow">RESOURCE</div>
              <h3 class="page-section-title">系统资源</h3>
            </div>
            <el-tag :type="healthTagType" effect="light" size="small">{{ healthStatus }}</el-tag>
          </div>

          <div class="resource-stack resource-stack--compact">
            <div class="resource-metric resource-metric--compact">
              <div class="resource-metric__head">
                <div class="resource-metric__main">
                  <span class="resource-metric__icon">⚙</span>
                  <span class="resource-metric__label">CPU</span>
                </div>
                <span class="resource-metric__value">{{ formatPercentage(systemResource?.cpu_usage) }}</span>
              </div>
              <el-progress class="resource-progress" :percentage="systemResource?.cpu_usage ?? 0" :show-text="false" :stroke-width="8" color="#2563eb" />
            </div>

            <div class="resource-metric resource-metric--compact">
              <div class="resource-metric__head">
                <div class="resource-metric__main">
                  <span class="resource-metric__icon">▣</span>
                  <span class="resource-metric__label">内存</span>
                </div>
                <span class="resource-metric__value">{{ formatPercentage(systemResource?.memory_usage) }}</span>
              </div>
              <el-progress class="resource-progress" :percentage="systemResource?.memory_usage ?? 0" :show-text="false" :stroke-width="8" color="#7c3aed" />
            </div>

            <div class="resource-highlight resource-highlight--compact">
              <span class="resource-highlight__label">Nestify 内存</span>
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
import { fetchRunningBackups } from '../api/backups'

const health = ref<HealthPayload | null>(null)
const healthError = ref('')
const loading = ref(false)
const summaryItems = ref<RunHistoryItem[]>(emptyRunHistory())
const systemResource = ref<SystemResourcePayload | null>(null)
const rules = ref<RuleItem[]>([])
const runHistoryItems = ref<RunHistoryItem[]>(emptyRunHistory())

// 统一的任务预览条目（规则任务 + 备份任务）
interface PreviewTask {
  id: string
  kind: 'rule' | 'backup'
  ruleName: string
  metaText: string
  archive_mode?: string
  sourceDir: string
  progress: number
  scanned: number
  success_count: number
  skip_count: number
  failure_count: number
  logs: string[]
  logsLoading: boolean
  detailText: string
}

const runningPreviewItems = ref<PreviewTask[]>([])
const dashboardExecutionHints = ref<Record<number, string>>({})
const previewRefreshing = ref(false)
let previewPollTimer: number | null = null

const healthStatus = computed(() => {
  if (loading.value) return '检查中'
  if (health.value) return '已连接'
  return '未连接'
})

const healthTime = computed(() => formatHealthTime(health.value?.time))
const healthTagType = computed(() => (health.value ? 'success' : loading.value ? 'warning' : 'danger'))

function formatHealthTime(value?: string) {
  if (!value) {
    return '尚未获取'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    const match = value.match(/(?:T|\s)(\d{2}:\d{2})/)
    return match?.[1] ?? value
  }

  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
}

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

function dashboardModeText(mode?: string) {
  if (mode === 'package') return '打包'
  if (mode === 'collect') return '收集'
  if (mode === 'cleanup') return '清理'
  if (mode === 'transform') return '转换'
  if (mode === 'link') return '链路'
  if (mode === 'naming') return '命名'
  return '未知'
}

function dashboardModeTagClass(mode?: string) {
  if (mode === 'package') return 'dashboard-mode-tag--package'
  if (mode === 'collect') return 'dashboard-mode-tag--collect'
  if (mode === 'cleanup') return 'dashboard-mode-tag--cleanup'
  if (mode === 'transform') return 'dashboard-mode-tag--transform'
  if (mode === 'link') return 'dashboard-mode-tag--hardlink'
  if (mode === 'naming') return 'dashboard-mode-tag--naming'
  return ''
}

function runModeText(mode: RuleItem['run_mode']) {
  if (mode === 'watch') return '监听模式'
  if (mode === 'cron') return '定时模式'
  return '手动模式'
}

function runDetailText(item: PreviewTask) {
  return item.detailText
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
  } catch {
    rules.value = []
  }
}

// 拉取规则任务（running）
async function loadRuleRunningTasks(): Promise<PreviewTask[]> {
  try {
    const runs = (await fetchRuns()).data?.items ?? []
    const runningRuleItems = rules.value.reduce<Record<number, RuleItem>>((map, item) => {
      map[item.id] = item
      return map
    }, {})

    interface RuleTaskDraft extends PreviewTask {
      rawRunId: string
    }

    const running: RuleTaskDraft[] = runs
      .filter((item) => item.status === 'running' && typeof item.rule_id === 'number')
      .map((item) => {
        const rule = item.rule_id ? runningRuleItems[item.rule_id] : undefined
        const triggerMode = rule ? runModeText(rule.run_mode) : (item.trigger_mode === 'cron' ? '定时模式' : item.trigger_mode === 'watch' ? '监听模式' : '手动模式')
        const executionHint = item.rule_id ? dashboardExecutionHints.value[item.rule_id] : ''
        const currentTarget = executionHint || item.current_volume_or_dir || item.current_series || '正在扫描源目录'
        const draft: RuleTaskDraft = {
          id: `rule-${item.id}`,
          kind: 'rule',
          ruleName: rule?.name || item.rule_name || '未知规则',
          metaText: `${dashboardModeText(item.archive_mode)} · ${triggerMode}`,
          archive_mode: item.archive_mode,
          sourceDir: rule?.source_dir || '',
          progress: estimateProgress(item),
          scanned: item.processed_files ?? 0,
          success_count: item.success_count,
          skip_count: item.skip_count,
          failure_count: item.failure_count,
          logs: [],
          logsLoading: true,
          detailText: `当前执行：${currentTarget} · ${dashboardModeText(item.archive_mode)}`,
          rawRunId: item.id,
        }
        return draft
      })

    // 加载日志
    await Promise.all(running.map(async (item) => {
      try {
        const response = await fetchRunLogs(item.rawRunId)
        item.logs = (response.data?.items ?? []).slice(-5).map((log: RunLogEntry) => {
          const time = new Date(log.created_at).toLocaleTimeString('zh-CN', { hour12: false })
          return `[${time}] ${log.message}`
        })
      } catch {
        item.logs = []
      } finally {
        item.logsLoading = false
      }
    }))

    return running.map(({ rawRunId: _rawRunId, ...rest }) => rest)
  } catch {
    return []
  }
}

// 拉取备份任务（running）
async function loadBackupRunningTasks(): Promise<PreviewTask[]> {
  try {
    const response = await fetchRunningBackups()
    const items = response.data?.items ?? []
    return items.map((snapshot) => {
      const triggerLabel = snapshot.phase || '备份中'
      const total = snapshot.scanned + snapshot.copied + snapshot.skipped + snapshot.deleted + snapshot.failed
      const progress = total <= 0 ? 12 : Math.min(92, Math.max(18, total % 100))
      return {
        id: `backup-${snapshot.task_id}`,
        kind: 'backup' as const,
        ruleName: snapshot.task_name || `备份任务 #${snapshot.task_id}`,
        metaText: triggerLabel,
        sourceDir: '',
        progress,
        scanned: snapshot.scanned,
        success_count: snapshot.copied,
        skip_count: snapshot.skipped,
        failure_count: snapshot.failed,
        logs: (snapshot.recent_logs ?? []).slice(-5),
        logsLoading: false,
        detailText: snapshot.progress || snapshot.phase || '备份中',
      }
    })
  } catch {
    return []
  }
}

async function refreshRunningPreview() {
  previewRefreshing.value = true
  try {
    await loadRules()
    const [ruleTasks, backupTasks] = await Promise.all([
      loadRuleRunningTasks(),
      loadBackupRunningTasks(),
    ])
    runningPreviewItems.value = [...ruleTasks, ...backupTasks]
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

function estimateProgress(item: RunInstance) {
  const total = item.success_count + item.skip_count + item.failure_count
  if (total <= 0) {
    return 12
  }
  return Math.min(92, Math.max(18, total % 100))
}

onMounted(() => {
  void loadHealth()
  void loadSummary()
  void loadSystemResource()
  void loadRules()
  void refreshRunningPreview()
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
.dashboard-hero__top-right,
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

.dashboard-hero__top-right {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  flex: 0 0 auto;
}

.dashboard-hero__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.dashboard-hero__meta span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 132px;
  min-height: 44px;
  padding: 0 12px;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  background: rgba(219, 234, 254, 0.66);
  color: #2563eb;
  font-size: 12px;
  font-weight: 800;
  white-space: nowrap;
}

.dashboard-hero__status {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  flex: 0 0 auto;
  width: 132px;
  min-height: 44px;
  padding: 8px 12px;
  border: 1px solid #86efac;
  border-radius: 999px;
  background: rgba(220, 252, 231, 0.72);
  color: #16a34a;
  font-size: 13px;
  font-weight: 900;
  white-space: nowrap;
  backdrop-filter: blur(12px);
}

.dashboard-live-dot {
  width: 9px;
  height: 9px;
  border-radius: 999px;
  background: #34d399;
  box-shadow: 0 0 0 6px rgba(52, 211, 153, 0.18);
}

.dashboard-content {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(340px, 0.6fr);
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

.task-preview-item__name {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.task-preview-item__kind {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 11px;
  line-height: 1.4;
  font-weight: 800;
}

.task-preview-item__kind--backup {
  color: #5b7a6e;
  background: #eef3f1;
}

.task-preview-item__kind--rule {
  color: #2563eb;
  background: #eff6ff;
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
.dashboard-mode-tag--naming { color: #0f8f79; background: rgba(15, 159, 135, 0.12); }

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

.resource-stack--compact {
  gap: 10px;
}

.resource-metric {
  padding: 16px;
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: linear-gradient(180deg, #f8fbff 0%, #f3f8ff 100%);
}

.resource-metric--compact {
  padding: 12px 14px;
  border-radius: 14px;
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

.resource-highlight--compact {
  padding: 12px 14px;
  border-radius: 14px;
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

@media (max-width: 1280px) {
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

  .dashboard-hero {
    padding: 28px 24px;
  }
}

:deep(.el-tag) {
  border-radius: 999px;
}
</style>

