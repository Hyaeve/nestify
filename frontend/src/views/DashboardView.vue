<template>
  <div class="dashboard-view">
    <section class="dashboard-hero">
      <div class="dashboard-hero__content">
        <div class="dashboard-hero__eyebrow">DASHBOARD</div>
        <h1>运行总览</h1>
        <p>集中查看规则状态、今日处理量、实时任务与系统资源，快速掌握 Nestify 当前运行情况。</p>
      </div>
      <div class="dashboard-hero__metrics">
        <div class="hero-metric">
          <div class="hero-metric__head">
            <span class="hero-metric__icon">⚙</span>
            <span class="hero-metric__label">CPU</span>
            <span class="hero-metric__value">{{ formatPercentage(systemResource?.cpu_usage) }}</span>
          </div>
          <el-progress class="hero-metric__progress" :percentage="systemResource?.cpu_usage ?? 0" :show-text="false" :stroke-width="6" color="#2563eb" />
        </div>

        <div class="hero-metric">
          <div class="hero-metric__head">
            <span class="hero-metric__icon">▣</span>
            <span class="hero-metric__label">内存</span>
            <span class="hero-metric__value">{{ formatPercentage(systemResource?.memory_usage) }}</span>
          </div>
          <el-progress class="hero-metric__progress" :percentage="systemResource?.memory_usage ?? 0" :show-text="false" :stroke-width="6" color="#7c3aed" />
        </div>

        <div class="hero-metric hero-metric--highlight">
          <span class="hero-metric__label">Nestify 内存</span>
          <span class="hero-metric__value hero-metric__value--lg">{{ systemResource?.nestify_memory || '0 B' }}</span>
        </div>
      </div>
    </section>

    <el-alert
      v-if="healthError"
      class="dashboard-health-alert"
      type="error"
      :closable="false"
      :title="healthError"
    />

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
              <span v-if="item.kind === 'backup'">上传 {{ item.success_count }}</span>
              <span v-else>成功 {{ item.success_count }}</span>
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

      <!-- 右侧：执行摘要（系统资源三项已移入顶栏右侧区域） -->
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
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

import { fetchRunLogs, fetchRuns, type RunInstance, type RunLogEntry } from '../api/executions'
import { fetchHealth, fetchSystemResource, type SystemResourcePayload } from '../api/system'
import { emptyRunHistory, fetchRunHistory, type RunHistoryItem } from '../api/runHistory'
import { formatRunHistorySummary } from '../utils/runHistorySummary'
import { fetchRules, type RuleItem } from '../api/rules'
import { fetchRunningBackups } from '../api/backups'

const healthError = ref('')
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

// 仅用于在后端不可达时给出提示条（顶栏不再展示「检查时间 / 已连接」）。
async function loadHealth() {
  healthError.value = ''

  try {
    await fetchHealth()
  } catch (error) {
    healthError.value = error instanceof Error ? error.message : '后端连接失败'
  }
}

async function loadSummary() {
  try {
    const items = (await fetchRunHistory()).data?.items ?? []
    runHistoryItems.value = items
    // 执行摘要：最多可见 5 条，超出通过列表滚动展示。
    summaryItems.value = items.slice(0, 50)
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
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  overflow: hidden;
  padding: 30px 36px;
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
.dashboard-hero__metrics {
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

/* 顶栏右侧：系统资源三项统计（替代原「检查时间 / 已连接」） */
.dashboard-hero__metrics {
  display: flex;
  align-items: stretch;
  gap: 12px;
  flex: 0 0 auto;
}

.hero-metric {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 9px;
  min-width: 158px;
  padding: 12px 14px;
  border: 1px solid #bfdbfe;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.74);
  backdrop-filter: blur(10px);
}

.hero-metric__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.hero-metric__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 8px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 13px;
  line-height: 1;
}

.hero-metric__label {
  color: #334155;
  font-size: 12px;
  font-weight: 800;
  white-space: nowrap;
}

.hero-metric__value {
  margin-left: auto;
  color: #2563eb;
  font-size: 16px;
  font-weight: 900;
  white-space: nowrap;
}

.hero-metric--highlight {
  gap: 7px;
  background: rgba(239, 246, 255, 0.86);
}

.hero-metric__value--lg {
  margin-left: 0;
  font-size: 20px;
}

.hero-metric__progress {
  width: 100%;
}

.hero-metric__progress :deep(.el-progress-bar__outer) {
  background-color: #dbeafe;
  border-radius: 999px;
}

.hero-metric__progress :deep(.el-progress-bar__inner) {
  border-radius: 999px;
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
.task-preview-card {
  padding: 24px;
}

.dashboard-panel--summary {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
}

.dashboard-panel__header,
.task-preview-card__header {
  flex: 0 0 auto;
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
  height: 100%;
}

/* 高度上限与任务预览列表一致，配合面板 flex 拉伸，
   使「执行摘要」底部与「任务预览」底部对齐。 */
.summary-list {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  max-height: 360px;
  overflow-y: auto;
  padding-right: 4px;
}

/* 细浅滚动条（执行摘要） */
.summary-list::-webkit-scrollbar {
  width: 5px;
}

.summary-list::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.28);
}

.summary-list::-webkit-scrollbar-thumb:hover {
  background: rgba(148, 163, 184, 0.45);
}

.summary-list::-webkit-scrollbar-track {
  background: transparent;
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

.task-preview-card {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.task-preview-card__refresh {
  color: #2563eb;
  font-weight: 900;
}

.task-preview-list {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  max-height: 360px;
  overflow-y: auto;
  padding-right: 4px;
}

/* 细浅滚动条（任务预览） */
.task-preview-list::-webkit-scrollbar {
  width: 5px;
}

.task-preview-list::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.28);
}

.task-preview-list::-webkit-scrollbar-thumb:hover {
  background: rgba(148, 163, 184, 0.45);
}

.task-preview-list::-webkit-scrollbar-track {
  background: transparent;
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

/* 顶栏一行放不下系统资源三张卡片时，让它们整行换到标题下方 */
@media (max-width: 1180px) {
  .dashboard-hero {
    flex-wrap: wrap;
  }

  .dashboard-hero__metrics {
    width: 100%;
    flex-wrap: wrap;
  }

  .hero-metric {
    flex: 1 1 150px;
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

