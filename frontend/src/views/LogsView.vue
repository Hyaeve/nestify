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

          <el-select v-model="ruleTypeFilter" class="logs-toolbar__select" placeholder="类型" @change="handleFiltersChange">
            <el-option label="全部类型" value="all" />
            <el-option label="归档规则" value="archive" />
            <el-option label="净化规则" value="cleanup" />
            <el-option label="链路规则" value="link" />
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

          <el-table-column label="模式" width="130" align="center">
            <template #default="scope">
              <span class="logs-mode-tag" :class="historyModeTagClass(scope.row)">{{ historyModeLabel(scope.row) }}</span>
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
const ruleTypeFilter = ref<'all' | 'archive' | 'cleanup' | 'link'>('all')
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
      rule_type: ruleTypeFilter.value !== 'all' ? ruleTypeFilter.value : undefined,
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
  ruleTypeFilter.value = 'all'
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

function historyModeLabel(item?: { archive_mode?: string; link_mode?: string }) {
  switch (item?.archive_mode) {
    case 'package':
      return '打包'
    case 'collect':
      return '收集'
    case 'cleanup':
      return '清理'
    case 'transform':
      return '转换'
    case 'link':
      return item?.link_mode === 'soft' ? '软链' : '硬链'
    default:
      return '未知'
  }
}

function historyModeTagClass(item?: { archive_mode?: string; link_mode?: string }) {
  switch (item?.archive_mode) {
    case 'package':
      return 'logs-mode-tag--package'
    case 'collect':
      return 'logs-mode-tag--collect'
    case 'cleanup':
      return 'logs-mode-tag--cleanup'
    case 'transform':
      return 'logs-mode-tag--transform'
    case 'link':
      return item?.link_mode === 'soft' ? 'logs-mode-tag--softlink' : 'logs-mode-tag--hardlink'
    default:
      return ''
  }
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

.logs-mode-tag {
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
}

.logs-mode-tag--package { color: #d58a2f; background: rgba(213, 138, 47, 0.08); }
.logs-mode-tag--collect { color: #8a74d6; background: rgba(138, 116, 214, 0.1); }
.logs-mode-tag--cleanup { color: #5f9f45; background: rgba(95, 159, 69, 0.12); }
.logs-mode-tag--transform { color: #64b9d8; background: rgba(100, 185, 216, 0.12); }
.logs-mode-tag--hardlink { color: #2f3136; background: rgba(47, 49, 54, 0.08); }
.logs-mode-tag--softlink { color: #c47c98; background: rgba(196, 124, 152, 0.12); }

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

:global(:root[data-theme='dark']) .logs-hero {
  border: 1px solid rgba(96, 165, 250, 0.24);
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

:global(:root[data-theme='dark']) .logs-hero::after {
  background: radial-gradient(circle, rgba(96, 165, 250, 0.18), transparent 66%);
}

:global(:root[data-theme='dark']) .logs-hero__desc {
  color: #94a3b8;
}

:global(:root[data-theme='dark']) .logs-hero__badge {
  border-color: rgba(74, 222, 128, 0.28);
  background: rgba(22, 163, 74, 0.13);
  color: #86efac;
}

:global(:root[data-theme='dark']) .metric-card,
:global(:root[data-theme='dark']) .logs-toolbar-panel,
:global(:root[data-theme='dark']) .logs-table-shell {
  border-color: rgba(148, 163, 184, 0.22);
  background:
    linear-gradient(180deg, rgba(23, 30, 45, 0.96) 0%, rgba(15, 20, 32, 0.94) 100%);
  box-shadow:
    0 18px 46px rgba(0, 0, 0, 0.26),
    inset 0 1px 0 rgba(255, 255, 255, 0.045);
}

:global(:root[data-theme='dark']) .metric-card__label,
:global(:root[data-theme='dark']) .logs-summary,
:global(:root[data-theme='dark']) .logs-message__meta,
:global(:root[data-theme='dark']) .logs-time {
  color: #c0cadc;
}

:global(:root[data-theme='dark']) .metric-card__value,
:global(:root[data-theme='dark']) .logs-message__title {
  color: #e5eefc;
}

:global(:root[data-theme='dark']) .logs-toolbar :deep(.el-input__wrapper),
:global(:root[data-theme='dark']) .logs-toolbar :deep(.el-select__wrapper) {
  background: rgba(10, 15, 26, 0.58);
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.24) inset;
}

:global(:root[data-theme='dark']) .logs-toolbar :deep(.el-input__wrapper:hover),
:global(:root[data-theme='dark']) .logs-toolbar :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px rgba(96, 165, 250, 0.38) inset;
}

:global(:root[data-theme='dark']) .logs-table {
  --el-table-header-bg-color: rgba(15, 21, 34, 0.98);
  --el-table-tr-bg-color: rgba(20, 26, 40, 0.94);
  --el-table-row-hover-bg-color: rgba(55, 65, 81, 0.98);
  --el-table-border-color: rgba(148, 163, 184, 0.16);
}

:global(:root[data-theme='dark']) .logs-table :deep(.el-table__header-wrapper th) {
  color: #d8e0ef;
  background: rgba(15, 21, 34, 0.98);
}

:global(:root[data-theme='dark']) .logs-table :deep(.el-table__body tr:nth-child(even) > td.el-table__cell) {
  background: rgba(43, 48, 62, 0.86);
}

:global(:root[data-theme='dark']) .logs-table :deep(.el-table__body tr:nth-child(odd) > td.el-table__cell) {
  background: rgba(18, 24, 38, 0.92);
}

:global(:root[data-theme='dark']) .logs-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: rgba(55, 65, 81, 0.98);
}

:global(:root[data-theme='dark']) .logs-mode-tag,
:global(:root[data-theme='dark']) .logs-message__meta span {
  border-color: rgba(148, 163, 184, 0.18);
  background: rgba(10, 15, 26, 0.62);
}

:global(:root[data-theme='dark']) .logs-table :deep(.el-table__row) {
  color: #cbd5e1;
}

:global(:root[data-theme='dark']) .logs-level-tag,
:global(:root[data-theme='dark']) .logs-mode-tag {
  box-shadow: 0 10px 22px rgba(0, 0, 0, 0.14);
}

:global(:root[data-theme='dark']) .logs-mode-tag--hardlink {
  color: #cbd5e1;
  background: rgba(148, 163, 184, 0.12);
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

