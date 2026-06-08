<template>
  <div class="dashboard-view">
    <el-row :gutter="16" class="dashboard-row">
      <el-col :span="8">
        <el-card class="page-card metric-card">
          <div class="metric-card__label">总规则数</div>
          <div class="metric-card__value">{{ totalRuleCount }}</div>
          <div class="metric-card__hint">已启用 {{ enabledRuleCount }} 条规则</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card class="page-card metric-card">
          <div class="metric-card__label">今日处理</div>
          <div class="metric-card__value">{{ todayProcessedCount }}</div>
          <div class="metric-card__hint">今日执行 {{ todayRunCount }} 次任务</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card class="page-card metric-card">
          <div class="metric-card__label">运行中任务</div>
          <div class="metric-card__value">{{ runningTaskCount }}</div>
          <div class="metric-card__hint">{{ runningTaskHint }}</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="dashboard-row dashboard-row--detail">
      <el-col :span="14">
        <el-card class="page-card dashboard-card dashboard-card--summary">
          <h3 class="page-section-title">最近执行摘要</h3>
          <div v-if="summaryItems.length" class="summary-list">
            <div v-for="item in summaryItems" :key="item.id" class="summary-item">
              <div class="summary-item__header">
                <span class="summary-item__name">{{ item.rule_name || '未知规则' }}</span>
                <div class="summary-item__badges">
                  <span class="dashboard-mode-tag" :class="dashboardModeTagClass(item.archive_mode)">{{ dashboardModeText(item.archive_mode) }}</span>
                  <el-tag :type="getStatusType(item.status)" effect="plain" size="small">{{ getStatusText(item.status) }}</el-tag>
                </div>
              </div>
              <div class="summary-item__meta">
                <span>{{ formatDate(item.started_at) }}</span>
                <span>{{ item.summary || '无摘要' }}</span>
              </div>
            </div>
          </div>
          <el-empty v-else class="dashboard-empty" description="暂无执行摘要" />
        </el-card>
      </el-col>
      <el-col :span="10">
        <div class="dashboard-side-stack">
          <el-card class="page-card resource-card resource-card--compact">
            <h3 class="page-section-title">系统资源</h3>

            <div class="resource-stack">
              <div class="resource-line">
                <div class="resource-line__head">
                  <span class="resource-line__title">CPU {{ formatPercentage(systemResource?.cpu_usage) }}</span>
                  <span class="resource-line__desc">{{ systemResource?.cpu_model || '未知型号' }}</span>
                </div>
                <el-progress :percentage="systemResource?.cpu_usage ?? 0" :show-text="false" />
              </div>

              <div class="resource-line">
                <div class="resource-line__head">
                  <span class="resource-line__title">内存 {{ formatPercentage(systemResource?.memory_usage) }}</span>
                  <span class="resource-line__desc">{{ formatMemorySummary }}</span>
                </div>
                <el-progress :percentage="systemResource?.memory_usage ?? 0" :show-text="false" color="#409eff" />
              </div>

              <div class="resource-kv">
                <span class="resource-kv__label">Nestify 内存占用</span>
                <span class="resource-kv__value resource-kv__value--primary">{{ systemResource?.nestify_memory || '0 B' }}</span>
              </div>

              <div class="resource-kv">
                <span class="resource-kv__label">运行时间</span>
                <span class="resource-kv__value">{{ systemResource?.uptime || '0分' }}</span>
              </div>

              <div class="resource-kv">
                <span class="resource-kv__label">服务标识</span>
                <span class="resource-kv__value">{{ healthService }}</span>
              </div>
            </div>

            <el-alert
              v-if="healthError"
              class="resource-card__alert"
              type="error"
              :closable="false"
              :title="healthError"
            />
          </el-card>

          <el-card class="page-card dashboard-card task-preview-card">
            <div class="task-preview-card__header">
              <h3 class="page-section-title">进行中任务预览</h3>
              <el-tag :type="runningPreviewItems.length ? 'success' : 'info'" effect="plain" size="small">
                {{ runningPreviewItems.length ? `${runningPreviewItems.length} 个任务` : '暂无任务' }}
              </el-tag>
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
                      <el-tag type="warning" effect="plain" size="small">进行中</el-tag>
                    </div>
                  </div>

                <el-progress :percentage="estimateRunProgress(item)" :stroke-width="8" :show-text="false" status="success" />

                <div class="task-preview-item__stats">
                  <span>成功 {{ item.success_count }}</span>
                  <span>跳过 {{ item.skip_count }}</span>
                  <span>失败 {{ item.failure_count }}</span>
                </div>

                <div class="task-preview-item__path" :title="item.sourceDir">{{ item.sourceDir || '未配置源路径' }}</div>
              </div>
            </div>

            <el-empty v-else class="task-preview-empty" description="当前没有正在执行的规则任务" />
          </el-card>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { fetchRun, fetchRunLogs, type RunInstance } from '../api/executions'
import { fetchHealth, fetchSystemResource, type HealthPayload, type SystemResourcePayload } from '../api/system'
import { emptyRunHistory, fetchRunHistory, type RunHistoryItem } from '../api/runHistory'
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
const runningPreviewItems = computed(() => runningRuns.value.slice(0, 3))

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
	const currentTarget = item.current_volume_or_dir || item.current_series || '正在扫描源目录'
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
    const runningRuleItems = rules.value.filter((item) => item.last_run_status === 'running').slice(0, 6)
    const loadedRuns = await Promise.all(runningRuleItems.map(async (item) => {
      try {
        const logs = await fetchRunLogs(String(item.id))
        const runID = logs.data?.items?.[0]?.run_id
        if (!runID) return null
        const run = await fetchRun(runID)
        if (!run.data) return null
        return {
          ...run.data,
          ruleName: item.name,
          runModeText: runModeText(item.run_mode),
          sourceDir: item.source_dir,
        }
      } catch {
        return null
      }
    }))
    runningRuns.value = loadedRuns.filter((item): item is RunningPreviewItem => item !== null && item.status === 'running')
  } catch {
    rules.value = []
    runningRuns.value = []
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
})
</script>

<style scoped lang="scss">
.dashboard-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dashboard-row {
  margin-top: 0 !important;
}

.dashboard-row--detail {
  align-items: stretch;
}

.dashboard-row--detail :deep(.el-col) {
  display: flex;
}

.metric-card {
  min-height: 124px;
  border: 1px solid var(--border-color);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.02), transparent), var(--bg-panel);
}

.metric-card__label {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
}

.metric-card__value {
  margin-top: 14px;
  font-size: 30px;
  line-height: 1;
  font-weight: 700;
  color: var(--text-primary);
}

.metric-card__hint {
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.dashboard-card {
  min-height: 360px;
  border: 1px solid var(--border-color);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.02), transparent), var(--bg-panel);
}

.dashboard-card--summary {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.dashboard-side-stack {
  display: flex;
  flex-direction: column;
  gap: 16px;
  width: 100%;
  height: 100%;
}

.summary-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.summary-item {
  padding: 12px 14px;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: var(--bg-elevated);
}

.summary-item__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.summary-item__badges {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.summary-item__name {
  font-weight: 600;
  color: var(--text-primary);
}

.dashboard-mode-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 54px;
  padding: 4px 10px;
  border-radius: 8px;
  border: 1px solid currentColor;
  font-size: 12px;
  line-height: 1.2;
  background: #fff;
}

.dashboard-mode-tag--package { color: #d58a2f; background: rgba(213, 138, 47, 0.08); }
.dashboard-mode-tag--collect { color: #8a74d6; background: rgba(138, 116, 214, 0.1); }
.dashboard-mode-tag--cleanup { color: #5f9f45; background: rgba(95, 159, 69, 0.12); }
.dashboard-mode-tag--transform { color: #64b9d8; background: rgba(100, 185, 216, 0.12); }
.dashboard-mode-tag--hardlink { color: #2f3136; background: rgba(47, 49, 54, 0.08); }

.summary-item__meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.dashboard-empty {
  min-height: 280px;
}

.resource-card {
  min-height: 220px;
  border: 1px solid var(--border-color);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.02), transparent), var(--bg-panel);
}

.resource-card--vertical {
  min-height: 360px;
}

.resource-card--compact {
  min-height: auto;
}

.resource-stack {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.resource-line {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.resource-line__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.resource-line__title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
}

.resource-line__desc {
  font-size: 14px;
  color: var(--text-secondary);
  text-align: right;
}

.resource-kv {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.resource-kv__label {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.resource-kv__value {
  font-size: 14px;
  color: var(--text-tertiary);
}

.resource-kv__value--primary {
  font-size: 18px;
  font-weight: 700;
  color: var(--el-color-primary);
}

.resource-card__alert {
  margin-top: 16px;
}

.task-preview-card {
  flex: none;
  min-height: auto;
}

.task-preview-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.task-preview-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-preview-item {
  padding: 12px 14px;
  border: 1px solid var(--border-color);
  border-radius: 12px;
  background: var(--bg-elevated);
}

.task-preview-item__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.task-preview-item__badges {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.task-preview-item__name {
  font-size: 15px;
  font-weight: 700;
  color: var(--text-primary);
}

.task-preview-item__meta {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

.task-preview-item__detail {
  margin-top: 6px;
  font-size: 12px;
  color: var(--text-tertiary);
  line-height: 1.5;
}

.task-preview-item__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-secondary);
}

.task-preview-item__path {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-preview-empty {
  min-height: 120px;
}

@media (max-width: 1280px) {
  .dashboard-row--detail :deep(.el-col) {
    max-width: 100%;
    flex: 0 0 100%;
  }
}

:deep(.el-card__body) {
  position: relative;
}

:deep(.el-tag) {
  border-radius: 999px;
}
</style>

