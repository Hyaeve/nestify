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

          <el-select v-model="logsSortBy" class="logs-toolbar__select logs-toolbar__select--sort" placeholder="排序" @change="handleFiltersChange">
            <el-option label="修改时间" value="modified_at" />
            <el-option label="文件名称" value="name" />
          </el-select>

          <el-tooltip :content="logsSortOrder === 'asc' ? '正序' : '倒序'" placement="top" :show-after="300">
            <el-button class="logs-action logs-action--sort" circle :aria-label="logsSortOrder === 'asc' ? '正序' : '倒序'" @click="toggleLogsSortOrder">
              <svg class="logs-action__icon" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M7 5.2v13.6" />
                <path v-if="logsSortOrder === 'asc'" d="M3.9 8.35 7 5.2l3.1 3.15" />
                <path v-else d="m3.9 15.65 3.1 3.15 3.1-3.15" />
                <path d="M13 7h7" />
                <path d="M13 12h5.2" />
                <path d="M13 17h3.4" />
              </svg>
            </el-button>
          </el-tooltip>

          <el-input
            v-model="keywordInput"
            class="logs-toolbar__search"
            clearable
            placeholder="搜索日志内容..."
            @keyup.enter="handleSearch"
          />

          <el-button class="logs-action logs-action--clear" type="danger" :disabled="!historyItems.length" @click="handleClearHistory">
            <svg class="logs-action__icon" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M4.5 7.5h15" />
              <path d="M9 7.5V5.8a1.3 1.3 0 0 1 1.3-1.3h3.4A1.3 1.3 0 0 1 15 5.8v1.7" />
              <path d="M7.5 7.5l.6 10.5a1.5 1.5 0 0 0 1.5 1.4h5.4a1.5 1.5 0 0 0 1.5-1.4l.6-10.5" />
              <path d="M10 11v4.5" />
              <path d="M14 11v4.5" />
            </svg>
          </el-button>
          <el-button class="logs-action logs-action--refresh" circle :loading="loading" @click="loadHistory">
            <svg class="logs-action__icon" viewBox="0 0 24 24" aria-hidden="true">
              <path d="M19 7v4h-4" />
              <path d="M5.5 17a8 8 0 0 1 13.5-6" />
              <path d="M5 17v-4h4" />
              <path d="M18.5 7A8 8 0 0 1 5 13" />
            </svg>
          </el-button>
          <div class="logs-view-toggle" role="group" aria-label="日志展示方式">
            <el-tooltip content="平铺" placement="top" :show-after="300">
              <button
                type="button"
                class="logs-view-toggle__button"
                :class="{ 'is-active': logsViewMode === 'flat' }"
                aria-label="平铺"
                :aria-pressed="logsViewMode === 'flat'"
                @click="logsViewMode = 'flat'"
              >
                <svg class="logs-view-toggle__icon" viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M5 6.5h14" />
                  <path d="M5 12h14" />
                  <path d="M5 17.5h14" />
                </svg>
              </button>
            </el-tooltip>
            <el-tooltip content="折叠" placement="top" :show-after="300">
              <button
                type="button"
                class="logs-view-toggle__button"
                :class="{ 'is-active': logsViewMode === 'tree' }"
                aria-label="折叠"
                :aria-pressed="logsViewMode === 'tree'"
                @click="logsViewMode = 'tree'"
              >
                <svg class="logs-view-toggle__icon" viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M6 6.5h12" />
                  <path d="M6 6.5v11" />
                  <path d="M9 12h9" />
                  <path d="M9 17.5h9" />
                  <path d="M6 12h3" />
                  <path d="M6 17.5h3" />
                </svg>
              </button>
            </el-tooltip>
          </div>
        </div>

        <div class="logs-summary">
          <span>共 {{ filteredTotal }} 条结果</span>
          <span v-if="searchKeyword">关键字：{{ searchKeyword }}</span>
        </div>
      </div>

      <div class="logs-table-shell">
        <el-table v-if="logsViewMode === 'flat'" v-loading="loading" :data="historyItems" class="logs-table" empty-text="暂无任务日志">
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

        <el-table v-else v-loading="loading" :data="logTreeRows" class="logs-table logs-tree-table" row-key="id" empty-text="暂无任务日志" @row-click="openLogDetailDialog">
          <el-table-column label="折叠任务" min-width="520">
            <template #default="scope">
              <button type="button" class="logs-detail-card" @click.stop="openLogDetailDialog(scope.row)">
                <span class="logs-detail-card__title">{{ scope.row.title }}</span>
                <span class="logs-detail-card__desc">{{ scope.row.description }}</span>
              </button>
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

          <el-table-column label="数量" width="110" align="center">
            <template #default="scope">{{ scope.row.processed_files }}</template>
          </el-table-column>

          <el-table-column label="时间" min-width="180">
            <template #default="scope">
              <span class="logs-time">{{ formatDateTime(scope.row.started_at) }}</span>
            </template>
          </el-table-column>

          <el-table-column label="操作" width="90" align="center">
            <template #default="scope">
              <el-button link type="primary" @click.stop="openLogDetailDialog(scope.row)">详情</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <el-dialog v-model="logDetailDialogVisible" class="logs-detail-dialog" title="任务详情" width="920px" destroy-on-close>
        <template v-if="selectedLogGroup">
          <div class="logs-detail-summary">
            <div>
              <div class="logs-detail-summary__title">{{ selectedLogGroup.title }}</div>
              <div class="logs-detail-summary__desc">{{ selectedLogGroup.description }}</div>
            </div>
            <div class="logs-detail-summary__tags">
              <el-tag class="logs-level-tag" :type="statusTagType(selectedLogGroup.status)" effect="light">{{ statusLabel(selectedLogGroup.status) }}</el-tag>
              <span class="logs-mode-tag" :class="historyModeTagClass(selectedLogGroup)">{{ historyModeLabel(selectedLogGroup) }}</span>
            </div>
          </div>
          <el-table :data="pagedLogDetailRows" class="logs-table logs-detail-table" empty-text="暂无明细">
            <el-table-column label="明细" min-width="520">
              <template #default="scope">
                <div class="logs-message logs-message--child">
                  <div class="logs-message__title">{{ scope.row.title }}</div>
                  <div class="logs-message__meta">
                    <span>{{ scope.row.description }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="级别" width="120" align="center">
              <template #default="scope">
                <el-tag class="logs-level-tag" :type="statusTagType(scope.row.status)" effect="light">{{ statusLabel(scope.row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="数量" width="110" align="center">
              <template #default="scope">{{ scope.row.processed_files }}</template>
            </el-table-column>
            <el-table-column label="时间" min-width="180">
              <template #default="scope">
                <span class="logs-time">{{ formatDateTime(scope.row.started_at) }}</span>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="selectedLogGroupChildren.length > logDetailPageSize" class="logs-detail-pagination">
            <el-pagination
              v-model:current-page="logDetailCurrentPage"
              background
              layout="total, prev, pager, next"
              :page-size="logDetailPageSize"
              :total="selectedLogGroupChildren.length"
            />
          </div>
        </template>
      </el-dialog>

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
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import { clearRunHistory, fetchRunHistory, type RunHistoryItem, type RunHistorySummary } from '../api/runHistory'
import { formatRunHistorySummary } from '../utils/runHistorySummary'

type LogsViewMode = 'flat' | 'tree'
type LogTreeRow = RunHistoryItem & {
  id: string
  title: string
  description: string
  is_group: boolean
  children?: LogTreeRow[]
}

function createDefaultHistorySummary(): RunHistorySummary {
  return {
    total: 0,
    today: 0,
    success: 0,
    failed: 0,
    skipped: 0,
  }
}

const logsViewModeStorageKey = 'nestify.logs.viewMode'

function normalizeLogsViewMode(value: unknown): LogsViewMode {
  return value === 'tree' ? 'tree' : 'flat'
}

function readLogsViewModePreference(): LogsViewMode {
  if (typeof window === 'undefined') {
    return 'flat'
  }

  return normalizeLogsViewMode(window.localStorage.getItem(logsViewModeStorageKey))
}

function persistLogsViewModePreference(mode: LogsViewMode) {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(logsViewModeStorageKey, mode)
}

const loading = ref(false)
const historyItems = ref<RunHistoryItem[]>([])
const historySummary = ref<RunHistorySummary>(createDefaultHistorySummary())
const filteredTotal = ref(0)
const keywordInput = ref('')
const searchKeyword = ref('')
const statusFilter = ref<'all' | 'success' | 'failed' | 'skip'>('all')
const ruleTypeFilter = ref<'all' | 'archive' | 'cleanup' | 'link'>('all')
const logsSortBy = ref<'name' | 'modified_at'>('modified_at')
const logsSortOrder = ref<'asc' | 'desc'>('desc')
const logsPageSizeOptions = [25, 50]
const logsPageSize = ref(25)
const logsCurrentPage = ref(1)
const logsViewMode = ref<LogsViewMode>(readLogsViewModePreference())
const logDetailDialogVisible = ref(false)
const selectedLogGroup = ref<LogTreeRow | null>(null)
const logDetailPageSize = 25
const logDetailCurrentPage = ref(1)

const totalLogs = computed(() => historySummary.value.total)
const todayLogs = computed(() => historySummary.value.today)
const successLogs = computed(() => historySummary.value.success)
const failedLogs = computed(() => historySummary.value.failed)
const skippedLogs = computed(() => historySummary.value.skipped)
const logTreeRows = computed(() => buildLogTreeRows(historyItems.value))
const selectedLogGroupChildren = computed(() => selectedLogGroup.value?.children ?? [])
const pagedLogDetailRows = computed(() => {
  const start = (logDetailCurrentPage.value - 1) * logDetailPageSize
  return selectedLogGroupChildren.value.slice(start, start + logDetailPageSize)
})

async function loadHistory() {
  loading.value = true
  try {
    const response = await fetchRunHistory({
      page: logsCurrentPage.value,
      page_size: logsPageSize.value,
      keyword: searchKeyword.value || undefined,
      status: statusFilter.value !== 'all' ? statusFilter.value : undefined,
      rule_type: ruleTypeFilter.value !== 'all' ? ruleTypeFilter.value : undefined,
      sort_by: logsSortBy.value,
      sort_order: logsSortOrder.value,
      view_mode: logsViewMode.value,
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

function toggleLogsSortOrder() {
  logsSortOrder.value = logsSortOrder.value === 'asc' ? 'desc' : 'asc'
  handleFiltersChange()
}

function resetFilters() {
  keywordInput.value = ''
  searchKeyword.value = ''
  statusFilter.value = 'all'
  ruleTypeFilter.value = 'all'
  logsSortBy.value = 'modified_at'
  logsSortOrder.value = 'desc'
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

function buildLogGroupKey(item: RunHistoryItem) {
  return [item.rule_id ?? 'manual', item.rule_name || '', item.trigger_mode || '', item.archive_mode || '', item.link_mode || '', item.started_at || ''].join('|')
}

function triggerModeText(mode: string) {
  if (mode === 'cron') return '定时'
  if (mode === 'watch') return '监听'
  return '手动'
}

function resolveLogGroupStatus(items: RunHistoryItem[]) {
  if (items.some((item) => item.status === 'failed')) return 'failed'
  if (items.some((item) => item.status === 'skip')) return 'skip'
  return 'success'
}

function buildLogChildRow(item: RunHistoryItem): LogTreeRow {
  return {
    ...item,
    id: `item-${item.id}`,
    title: formatRunHistorySummary(item.summary) || item.summary || '未记录具体条目',
    description: `${item.rule_name || '手动任务'} · ${triggerModeText(item.trigger_mode)} · 成功 ${item.success_count} / 警告 ${item.skip_count} / 错误 ${item.failure_count}`,
    is_group: false,
  }
}

function buildLogTreeRows(items: RunHistoryItem[]): LogTreeRow[] {
  const groups = new Map<string, RunHistoryItem[]>()
  for (const item of items) {
    const key = buildLogGroupKey(item)
    const current = groups.get(key)
    if (current) {
      current.push(item)
    } else {
      groups.set(key, [item])
    }
  }

  return Array.from(groups.entries()).map(([key, groupItems]) => {
    const first = groupItems[0]
    const processed = groupItems.reduce((total, item) => total + Math.max(0, Number(item.processed_files || 0)), 0) || groupItems.length
    const success = groupItems.reduce((total, item) => total + Math.max(0, Number(item.success_count || 0)), 0)
    const skipped = groupItems.reduce((total, item) => total + Math.max(0, Number(item.skip_count || 0)), 0)
    const failed = groupItems.reduce((total, item) => total + Math.max(0, Number(item.failure_count || 0)), 0)

    return {
      ...first,
      id: `group-${key}`,
      status: resolveLogGroupStatus(groupItems),
      processed_files: processed,
      success_count: success,
      skip_count: skipped,
      failure_count: failed,
      title: `${historyModeLabel(first)}任务 · ${formatDateTime(first.started_at)}`,
      description: `${first.rule_name || '手动任务'} · ${triggerModeText(first.trigger_mode)} · 操作 ${processed} 个文件或文件夹 · 共 ${groupItems.length} 条明细`,
      is_group: true,
      children: groupItems.map(buildLogChildRow),
    }
  })
}

function openLogDetailDialog(row: LogTreeRow) {
  if (!row.is_group) return
  selectedLogGroup.value = row
  logDetailCurrentPage.value = 1
  logDetailDialogVisible.value = true
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
      if (item?.link_mode === 'strm') return 'Strm'
      return item?.link_mode === 'hard' ? '硬链' : '软链'
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
      if (item?.link_mode === 'strm') return 'logs-mode-tag--strm'
      return item?.link_mode === 'hard' ? 'logs-mode-tag--hardlink' : 'logs-mode-tag--softlink'
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

watch(logsViewMode, (mode) => {
  persistLogsViewModePreference(mode)
  logsCurrentPage.value = 1
  void loadHistory()
})

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

.logs-toolbar__select--sort {
  width: 128px;
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

.logs-view-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 42px;
  margin-left: auto;
  padding: 4px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 16px;
  background: rgba(248, 250, 252, 0.72);
  backdrop-filter: blur(8px);
}

.logs-view-toggle__button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 0;
  border-radius: 12px;
  color: #64748b;
  background: transparent;
  cursor: pointer;
  transition:
    color 0.18s ease,
    background 0.18s ease,
    box-shadow 0.18s ease,
    transform 0.18s ease;
}

.logs-view-toggle__button:hover {
  color: #2563eb;
  background: #eff6ff;
}

.logs-view-toggle__button.is-active {
  color: #2563eb;
  background: rgba(37, 99, 235, 0.14);
  box-shadow: inset 0 0 0 1px rgba(37, 99, 235, 0.18), 0 8px 18px rgba(37, 99, 235, 0.1);
}

.logs-view-toggle__button:active {
  transform: translateY(1px);
}

.logs-view-toggle__icon {
  width: 20px;
  height: 20px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.logs-action {
  min-height: 42px;
  border: 0;
  border-radius: 14px;
  font-weight: 700;
}

.logs-action--clear {
  min-width: 42px;
  color: #dc2626;
  background: #fee2e2;
}

.logs-action--refresh,
.logs-action--sort {
  width: 42px;
  min-width: 42px;
  color: #2563eb;
  background: #dbeafe;
}

.logs-action--sort {
  border: 1px solid rgba(37, 99, 235, 0.16);
  background: rgba(255, 255, 255, 0.82);
}

.logs-action--sort:not(.is-disabled):hover {
  color: #2f6fd6;
  border-color: #b7d8ff;
  background: #edf7ff;
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.1);
  transform: translateY(-1px);
}

.logs-action__icon {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
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
.logs-mode-tag--strm { color: #2f8f9d; background: rgba(47, 143, 157, 0.12); }

.logs-tree-table :deep(.el-table__row) { cursor: pointer; }

.logs-detail-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  padding: 0;
  text-align: left;
  background: transparent;
  border: 0;
  cursor: pointer;
}

.logs-detail-card__title {
  color: #0f172a;
  font-size: 14px;
  font-weight: 800;
  line-height: 1.55;
}

.logs-detail-card__desc {
  color: #64748b;
  font-size: 12px;
  line-height: 1.6;
}

.logs-detail-summary {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 16px;
  padding: 16px;
  border: 1px solid #eef2f7;
  border-radius: 18px;
  background: #f8fafc;
}

.logs-detail-summary__title {
  color: #0f172a;
  font-size: 16px;
  font-weight: 900;
}

.logs-detail-summary__desc {
  margin-top: 6px;
  color: #64748b;
  line-height: 1.6;
}

.logs-detail-summary__tags {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.logs-detail-table {
  margin-top: 8px;
}

.logs-detail-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.logs-message {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.logs-message--child {
  padding-left: 6px;
  border-left: 3px solid #e2e8f0;
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

