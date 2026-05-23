<template>
  <div class="dashboard-view">
    <el-row :gutter="16" class="dashboard-row">
      <el-col :span="8">
        <el-card class="page-card metric-card">
          <div class="metric-card__label">总规则数</div>
          <div class="metric-card__value">0</div>
          <div class="metric-card__hint">等待规则统计接口接入</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card class="page-card metric-card">
          <div class="metric-card__label">今日处理</div>
          <div class="metric-card__value">0</div>
          <div class="metric-card__hint">今日暂无处理记录</div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card class="page-card metric-card">
          <div class="metric-card__label">运行中任务</div>
          <div class="metric-card__value">0</div>
          <div class="metric-card__hint">当前没有活跃任务</div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="dashboard-row dashboard-row--detail">
      <el-col :span="14">
        <el-card class="page-card dashboard-card">
          <h3 class="page-section-title">当前任务进度</h3>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="后端连接状态">
              <el-tag :type="healthTagType">{{ healthStatus }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="服务标识">
              {{ healthService }}
            </el-descriptions-item>
            <el-descriptions-item label="最近检查时间">
              {{ healthTime }}
            </el-descriptions-item>
          </el-descriptions>

          <el-alert
            v-if="healthError"
            style="margin-top: 16px;"
            type="error"
            :closable="false"
            :title="healthError"
          />
        </el-card>
      </el-col>
      <el-col :span="10">
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
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { fetchHealth, type HealthPayload } from '../api/system'
import { emptyRunHistory, fetchRunHistory, type RunHistoryItem } from '../api/runHistory'

const health = ref<HealthPayload | null>(null)
const healthError = ref('')
const loading = ref(false)
const summaryItems = ref<RunHistoryItem[]>(emptyRunHistory())

const healthStatus = computed(() => {
  if (loading.value) return '检查中'
  if (health.value) return '已连接'
  return '未连接'
})

const healthService = computed(() => health.value?.service ?? '未知')
const healthTime = computed(() => health.value?.time ?? '尚未获取')
const healthTagType = computed(() => (health.value ? 'success' : loading.value ? 'warning' : 'danger'))

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
    summaryItems.value = (await fetchRunHistory()).data?.items.slice(0, 5) ?? []
  } catch {
    summaryItems.value = []
  }
}

onMounted(() => {
  void loadHealth()
  void loadSummary()
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

:deep(.el-card__body) {
  position: relative;
}

:deep(.el-tag) {
  border-radius: 999px;
}
</style>

