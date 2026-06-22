<template>
  <div class="logs-view">
    <section class="logs-hero">
      <div>
        <div class="logs-hero__eyebrow">OBSERVABILITY</div>
        <h1 class="logs-hero__title">运行日志</h1>
        <p class="logs-hero__desc">实时查看任务执行、归档、清理与转换日志，快速检索关键事件和异常信息。</p>
      </div>
      <div class="logs-hero__badge">
        <span class="logs-hero__pulse"></span>
        实时连接
      </div>
    </section>

    <div class="logs-overview">
      <section class="metric-card">
        <div class="metric-card__icon metric-card__icon--primary">▦</div>
        <div class="metric-card__body">
          <div class="metric-card__label">总日志数</div>
          <div class="metric-card__value">{{ totalLogs }}</div>
        </div>
      </section>
      <section class="metric-card">
        <div class="metric-card__icon metric-card__icon--info">◷</div>
        <div class="metric-card__body">
          <div class="metric-card__label">今日日志</div>
          <div class="metric-card__value">{{ todayLogs }}</div>
        </div>
      </section>
      <section class="metric-card metric-card--success">
        <div class="metric-card__icon metric-card__icon--success">✓</div>
        <div class="metric-card__body">
          <div class="metric-card__label">成功数</div>
          <div class="metric-card__value">{{ successLogs }}</div>
        </div>
      </section>
      <section class="metric-card metric-card--danger">
        <div class="metric-card__icon metric-card__icon--danger">!</div>
        <div class="metric-card__body">
          <div class="metric-card__label">错误数</div>
          <div class="metric-card__value">{{ failedLogs }}</div>
        </div>
      </section>
      <section class="metric-card metric-card--warning">
        <div class="metric-card__icon metric-card__icon--warning">⚠</div>
        <div class="metric-card__body">
          <div class="metric-card__label">警告数</div>
          <div class="metric-card__value">{{ skippedLogs }}</div>
        </div>
      </section>
    </div>

    <section class="logs-panel">
      <div class="logs-toolbar-panel">
        <div class="logs-toolbar">
          <el-select v-model="statusFilter" class="logs-toolbar__select" placeholder="状态" @change="handleFiltersChange">
            <el-option label="全部级别" value="all" />
            <el-option label="成功" value="success" />
            <el-option label="错误" value="failed" />
            <el-option label="警告" value="skip" />
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
            placeholder="搜索日志内容..."
            @keyup.enter="handleSearch"
          />

          <el-button class="logs-action logs-action--search" type="primary" @click="handleSearch">搜索</el-button>
          <el-button class="logs-action logs-action--clear" type="danger" :disabled="!historyItems.length" @click="handleClearHistory">清空日志</el-button>
          <el-button class="logs-action logs-action--live" @click="loadHistory">实时</el-button>
          <el-button class="logs-action logs-action--pause" @click="resetFilters">暂停</el-button>
          <el-button class="logs-action logs-action--refresh" circle :loading="loading" @click="loadHistory">↻</el-button>
        </div>

        <div class="logs-summary">
          <span>共 {{ filteredTotal }} 条结果</span>
          <span v-if="searchKeyword">关键字：{{ searchKeyword }}</span>
        </div>
      </div>

      <div class="logs-table-shell">
        <el-table v-loading="loading" :data="historyItems" class="logs-table" empty-text="暂无任务日志">
          <el-table-column label="时间" min-width="180">
            <template #default="scope">
              <span class="logs-time">{{ formatDateTime(scope.row.started_at) }}</span>
            </template>
          </el-table-column>

          <el-table-column label="级别" width="120" align="center">
            <template #default="scope">
              <el-tag class="logs-level-tag" :type="statusTagType(scope.row.status)" effect="light">{{ statusLabel(scope.row.status) }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column label="类型" width="130" align="center">
            <template #default="scope">
              <span class="logs-type">{{ archiveModeLabel(scope.row.archive_mode) }}</span>
            </template>
          </el-table-column>

          <el-table-column label="消息" min-width="520">
            <template #default="scope">
              <div class="logs-message">
                <div class="logs-message__title">
                  {{ formatRunHistorySummary(scope.row.summary) || '—' }}
                </div>
                <div class="logs-message__meta">
                  <span>{{ scope.row.rule_name || '手动任务' }}</span>
                  <span>{{ triggerModeLabel(scope.row.trigger_mode) }}</span>
                  <span>处理 {{ scope.row.processed_files }}</span>
                  <span>成功 {{ scope.row.success_count }}</span>
                  <span>警告 {{ scope.row.skip_count }}</span>
                  <span>错误 {{ scope.row.failure_count }}</span>
                </div>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

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
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import { clearRunHistory, fetchRunHistory, type RunHistoryItem, type RunHistorySummary } from '../api/runHistory'
import { formatRunHistorySummary } from '../utils/runHistorySummary'

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
  gap: 22px;
  padding-bottom: 12px;
}

.logs-hero {
  position: relative;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  overflow: hidden;
  padding: 34px 36px;
  border-radius: 30px;
  color: #ffffff;
  background:
    radial-gradient(circle at 18% 20%, rgba(255, 255, 255, 0.25), transparent 28%),
    linear-gradient(135deg, #2563eb 0%, #4f46e5 52%, #7c3aed 100%);
  box-shadow: 0 24px 54px rgba(37, 99, 235, 0.25);
}

.logs-hero::after {
  position: absolute;
  right: -72px;
  bottom: -88px;
  width: 260px;
  height: 260px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.14);
  content: '';
}

.logs-hero__eyebrow {
  margin-bottom: 8px;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.22em;
  opacity: 0.78;
}

.logs-hero__title {
  margin: 0;
  font-size: 36px;
  line-height: 1.15;
  font-weight: 800;
}

.logs-hero__desc {
  max-width: 620px;
  margin: 12px 0 0;
  font-size: 15px;
  line-height: 1.7;
  color: rgba(255, 255, 255, 0.84);
}

.logs-hero__badge {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border: 1px solid rgba(255, 255, 255, 0.28);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.14);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16);
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.logs-hero__pulse {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #34d399;
  box-shadow: 0 0 0 6px rgba(52, 211, 153, 0.18);
}

.logs-overview {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
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
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
  border-radius: 16px;
  font-size: 20px;
  font-weight: 800;
}

.metric-card__icon--primary {
  color: #2563eb;
  background: #dbeafe;
}

.metric-card__icon--info {
  color: #0891b2;
  background: #cffafe;
}

.metric-card__icon--success {
  color: #16a34a;
  background: #dcfce7;
}

.metric-card__icon--danger {
  color: #dc2626;
  background: #fee2e2;
}

.metric-card__icon--warning {
  color: #d97706;
  background: #fef3c7;
}

.metric-card__body {
  min-width: 0;
}

.metric-card__label {
  font-size: 14px;
  font-weight: 600;
  color: #64748b;
}

.metric-card__value {
  margin-top: 8px;
  font-size: 34px;
  line-height: 1;
  font-weight: 800;
  color: #0f172a;
}

.metric-card--success .metric-card__value {
  color: #16a34a;
}

.metric-card--danger .metric-card__value {
  color: #dc2626;
}

.metric-card--warning .metric-card__value {
  color: #d97706;
}

.logs-panel {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.logs-toolbar-panel,
.logs-table-shell {
  border: 1px solid #eef2f7;
  border-radius: 26px;
  background: #ffffff;
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.06);
}

.logs-toolbar-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
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

.logs-toolbar :deep(.el-input__wrapper),
.logs-toolbar :deep(.el-select__wrapper) {
  min-height: 42px;
  border-radius: 14px;
  box-shadow: 0 0 0 1px #e2e8f0 inset;
}

.logs-action {
  min-height: 42px;
  border: 0;
  border-radius: 14px;
  font-weight: 700;
}

.logs-action--search {
  background: #2563eb;
}

.logs-action--clear {
  background: #ef4444;
}

.logs-action--live {
  color: #047857;
  background: #d1fae5;
}

.logs-action--pause {
  color: #b45309;
  background: #fef3c7;
}

.logs-action--refresh {
  width: 42px;
  color: #2563eb;
  background: #dbeafe;
}

.logs-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  color: #64748b;
  font-size: 14px;
}

.logs-table-shell {
  overflow: hidden;
}

.logs-table {
  --el-table-border-color: transparent;
  --el-table-header-bg-color: #f8fafc;
  --el-table-tr-bg-color: #ffffff;
  --el-table-row-hover-bg-color: #f8fbff;
}

.logs-table :deep(.el-table__header-wrapper th) {
  height: 54px;
  color: #64748b;
  background: #f8fafc;
  font-size: 13px;
  font-weight: 800;
}

.logs-table :deep(.el-table__cell) {
  padding: 16px 0;
  vertical-align: top;
}

.logs-time {
  color: #475569;
  font-weight: 600;
}

.logs-level-tag {
  min-width: 58px;
  border-radius: 999px;
  font-weight: 700;
}

.logs-type {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 5px 10px;
  border-radius: 999px;
  color: #334155;
  background: #f1f5f9;
  font-size: 12px;
  font-weight: 700;
}

.logs-message {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.logs-message__title {
  color: #0f172a;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.55;
  word-break: break-word;
}

.logs-message__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  color: #64748b;
  font-size: 12px;
}

.logs-message__meta span {
  display: inline-flex;
  align-items: center;
  padding: 4px 8px;
  border-radius: 999px;
  background: #f8fafc;
}

.logs-pagination {
  display: flex;
  justify-content: flex-end;
  padding: 0 4px;
}

@media (max-width: 1440px) {
  .logs-overview {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 960px) {
  .logs-hero {
    flex-direction: column;
    padding: 28px;
  }

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

  .logs-hero__title {
    font-size: 30px;
  }
}
</style>

