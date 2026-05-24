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
                <el-tag :type="getStatusType(item.status)" effect="plain" size="small">{{ getStatusText(item.status) }}</el-tag>
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
        <el-card class="page-card resource-card resource-card--vertical">
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
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

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

const runningTaskCount = computed(() => rules.value.filter((item) => item.last_run_status === 'running').length)
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
    summaryItems.value = items.slice(0, 5)
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

.summary-item__name {
  font-weight: 600;
  color: var(--text-primary);
}

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

:deep(.el-card__body) {
  position: relative;
}

:deep(.el-tag) {
  border-radius: 999px;
}
</style>

