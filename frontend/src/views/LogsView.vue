<template>
  <div class="logs-view">
    <div class="logs-overview">
      <section class="metric-card">
        <div class="metric-card__label">总日志数</div>
        <div class="metric-card__value">{{ totalLogs }}</div>
      </section>
      <section class="metric-card">
        <div class="metric-card__label">今日日志</div>
        <div class="metric-card__value">{{ todayLogs }}</div>
      </section>
      <section class="metric-card metric-card--success">
        <div class="metric-card__label">成功数</div>
        <div class="metric-card__value">{{ successLogs }}</div>
      </section>
      <section class="metric-card metric-card--danger">
        <div class="metric-card__label">错误数</div>
        <div class="metric-card__value">{{ failedLogs }}</div>
      </section>
      <section class="metric-card metric-card--warning">
        <div class="metric-card__label">跳过数</div>
        <div class="metric-card__value">{{ skippedLogs }}</div>
      </section>
    </div>

    <el-card class="page-card logs-card">
      <div class="logs-card__content">
        <div class="logs-toolbar">
          <el-select v-model="statusFilter" class="logs-toolbar__select" placeholder="状态" @change="handleFiltersChange">
            <el-option label="全部状态" value="all" />
            <el-option label="成功" value="success" />
            <el-option label="错误" value="failed" />
            <el-option label="跳过" value="skip" />
          </el-select>

          <el-select v-model="modeFilter" class="logs-toolbar__select" placeholder="类型" @change="handleFiltersChange">
            <el-option label="全部类型" value="all" />
            <el-option label="打包归档" value="package" />
            <el-option label="收集归档" value="collect" />
            <el-option label="清理归档" value="cleanup" />
          </el-select>

          <el-input
            v-model="keywordInput"
            class="logs-toolbar__search"
            clearable
            placeholder="关键字搜索（规则名 / 摘要）"
            @keyup.enter="handleSearch"
          />

          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="resetFilters">重置</el-button>
          <el-button :loading="loading" @click="loadHistory">刷新</el-button>
          <el-button type="danger" plain :disabled="!historyItems.length" @click="handleClearHistory">清空日志</el-button>
        </div>

        <div class="logs-summary">
          <span>共 {{ filteredTotal }} 条结果</span>
          <span v-if="searchKeyword">关键字：{{ searchKeyword }}</span>
        </div>

        <el-table v-loading="loading" :data="historyItems" border stripe class="logs-table" empty-text="暂无任务日志">
          <el-table-column label="时间" min-width="180">
            <template #default="scope">
              {{ formatDateTime(scope.row.started_at) }}
            </template>
          </el-table-column>

          <el-table-column label="状态" width="90" align="center">
            <template #default="scope">
              <el-tag :type="statusTagType(scope.row.status)" effect="light">{{ statusLabel(scope.row.status) }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column label="规则 / 任务" min-width="180" show-overflow-tooltip>
            <template #default="scope">
              {{ scope.row.rule_name || '手动任务' }}
            </template>
          </el-table-column>

          <el-table-column label="类型" width="100" align="center">
            <template #default="scope">
              {{ archiveModeLabel(scope.row.archive_mode) }}
            </template>
          </el-table-column>

          <el-table-column label="触发方式" width="110" align="center">
            <template #default="scope">
              {{ triggerModeLabel(scope.row.trigger_mode) }}
            </template>
          </el-table-column>

          <el-table-column label="统计" min-width="180">
            <template #default="scope">
              <span class="count-pill count-pill--success">成 {{ scope.row.success_count }}</span>
              <span class="count-pill count-pill--warning">跳 {{ scope.row.skip_count }}</span>
              <span class="count-pill count-pill--danger">错 {{ scope.row.failure_count }}</span>
            </template>
          </el-table-column>

          <el-table-column label="处理数" width="90" align="center">
            <template #default="scope">
              {{ scope.row.processed_files }}
            </template>
          </el-table-column>

          <el-table-column label="消息摘要" min-width="320" show-overflow-tooltip>
            <template #default="scope">
              {{ scope.row.summary || '—' }}
            </template>
          </el-table-column>
        </el-table>

        <div v-if="filteredTotal > 0" class="logs-pagination">
          <el-pagination
            v-model:current-page="logsCurrentPage"
            v-model:page-size="logsPageSize"
            background
            layout="total, sizes, prev, pager, next"
            :page-sizes="logsPageSizeOptions"
            :total="filteredTotal"
            @current-change="handlePageChange"
            @size-change="handlePageSizeChange"
          />
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import { clearRunHistory, fetchRunHistory, type RunHistoryItem, type RunHistorySummary } from '../api/runHistory'

function createDefaultHistorySummary(): RunHistorySummary {
  return {
    total: 0,
    today: 0,
    success: 0,
    failed: 0,
    skipped: 0,
  }
}

const loading = ref(false)
const historyItems = ref<RunHistoryItem[]>([])
const historySummary = ref<RunHistorySummary>(createDefaultHistorySummary())
const filteredTotal = ref(0)
const keywordInput = ref('')
const searchKeyword = ref('')
const statusFilter = ref<'all' | 'success' | 'failed' | 'skip'>('all')
const modeFilter = ref<'all' | 'package' | 'collect' | 'cleanup'>('all')
const logsPageSizeOptions = [25, 50]
const logsPageSize = ref(25)
const logsCurrentPage = ref(1)

const totalLogs = computed(() => historySummary.value.total)
const todayLogs = computed(() => historySummary.value.today)
const successLogs = computed(() => historySummary.value.success)
const failedLogs = computed(() => historySummary.value.failed)
const skippedLogs = computed(() => historySummary.value.skipped)

async function loadHistory() {
  loading.value = true
  try {
    const response = await fetchRunHistory({
      page: logsCurrentPage.value,
      page_size: logsPageSize.value,
      keyword: searchKeyword.value || undefined,
      status: statusFilter.value !== 'all' ? statusFilter.value : undefined,
      archive_mode: modeFilter.value !== 'all' ? modeFilter.value : undefined,
    })
    historyItems.value = response.data?.items ?? []
    filteredTotal.value = response.data?.total ?? 0
    historySummary.value = response.data?.summary ?? createDefaultHistorySummary()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '任务日志加载失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  searchKeyword.value = keywordInput.value.trim()
  logsCurrentPage.value = 1
  void loadHistory()
}

function handleFiltersChange() {
  logsCurrentPage.value = 1
  void loadHistory()
}

function resetFilters() {
  keywordInput.value = ''
  searchKeyword.value = ''
  statusFilter.value = 'all'
  modeFilter.value = 'all'
  logsCurrentPage.value = 1
  void loadHistory()
}

function handlePageChange(page: number) {
  logsCurrentPage.value = page
  void loadHistory()
}

function handlePageSizeChange(pageSize: number) {
  logsPageSize.value = pageSize
  logsCurrentPage.value = 1
  void loadHistory()
}

async function handleClearHistory() {
  try {
    await ElMessageBox.confirm('确认清空全部任务日志吗？此操作不可恢复。', '清空日志', {
      type: 'warning',
      confirmButtonText: '确认清空',
      cancelButtonText: '取消',
    })

    await clearRunHistory()
    historyItems.value = []
    filteredTotal.value = 0
    historySummary.value = createDefaultHistorySummary()
    resetFilters()
    logsCurrentPage.value = 1
    ElMessage.success('任务日志已清空')
  } catch (error) {
    if (error === 'cancel' || error === 'close') {
      return
    }

    ElMessage.error(error instanceof Error ? error.message : '清空任务日志失败')
  }
}

function formatDateTime(value: string) {
  if (!value) {
    return '—'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', { hour12: false })
}

function statusLabel(status: string) {
  if (status === 'success') {
    return '成功'
  }
  if (status === 'failed') {
    return '错误'
  }
  return '跳过'
}

function statusTagType(status: string): 'success' | 'danger' | 'warning' {
  if (status === 'success') {
    return 'success'
  }
  if (status === 'failed') {
    return 'danger'
  }
  return 'warning'
}

function archiveModeLabel(mode?: string) {
  if (mode === 'package') {
    return '打包归档'
  }
  if (mode === 'collect') {
    return '收集归档'
  }
  if (mode === 'cleanup') {
    return '清理归档'
  }
  return '未分类'
}

function triggerModeLabel(mode: string) {
  if (mode === 'cron') {
    return '定时'
  }
  if (mode === 'watch') {
    return '监听'
  }
  return '手动'
}

onMounted(() => {
  void loadHistory()
})
</script>

<style scoped>
.logs-view {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.logs-overview {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 16px;
}

.metric-card {
  padding: 20px 22px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 20px;
  background: var(--el-bg-color-overlay);
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.05);
}

.metric-card__label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.metric-card__value {
  margin-top: 10px;
  font-size: 42px;
  line-height: 1;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.metric-card--success .metric-card__value {
  color: var(--el-color-success);
}

.metric-card--danger .metric-card__value {
  color: var(--el-color-danger);
}

.metric-card--warning .metric-card__value {
  color: var(--el-color-warning);
}

.logs-card__content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.logs-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.logs-toolbar__select {
  width: 150px;
}

.logs-toolbar__search {
  width: min(360px, 100%);
}

.logs-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.logs-pagination {
  display: flex;
  justify-content: flex-end;
}

.logs-table :deep(.el-table__cell) {
  vertical-align: top;
}

.count-pill {
  display: inline-flex;
  align-items: center;
  margin-right: 8px;
  margin-bottom: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.count-pill--success {
  color: var(--el-color-success);
  background: color-mix(in srgb, var(--el-color-success) 14%, transparent);
}

.count-pill--warning {
  color: var(--el-color-warning);
  background: color-mix(in srgb, var(--el-color-warning) 14%, transparent);
}

.count-pill--danger {
  color: var(--el-color-danger);
  background: color-mix(in srgb, var(--el-color-danger) 14%, transparent);
}

@media (max-width: 1440px) {
  .logs-overview {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 960px) {
  .logs-overview {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .logs-toolbar__search {
    width: 100%;
  }
}

@media (max-width: 640px) {
  .logs-overview {
    grid-template-columns: 1fr;
  }
}
</style>

