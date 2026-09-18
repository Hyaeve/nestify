<template>
  <div class="logs-view">
    <section class="logs-hero">
      <div class="logs-hero__intro">
        <div class="logs-hero__eyebrow">OBSERVABILITY</div>
        <h1 class="logs-hero__title">运行日志</h1>
        <p class="logs-hero__desc">实时查看任务执行、归档、清理与转换日志，快速检索关键事件和异常信息。</p>
      </div>

      <div class="logs-hero__stats">
        <section class="metric-card">
          <div class="metric-card__icon metric-card__icon--primary">▦</div>
          <div class="metric-card__body">
            <div class="metric-card__label">总日志</div>
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
            <div class="metric-card__label">成功</div>
            <div class="metric-card__value">{{ successLogs }}</div>
          </div>
        </section>
        <section class="metric-card metric-card--danger">
          <div class="metric-card__icon metric-card__icon--danger">!</div>
          <div class="metric-card__body">
            <div class="metric-card__label">失败</div>
            <div class="metric-card__value">{{ failedLogs }}</div>
          </div>
        </section>
        <section class="metric-card metric-card--warning">
          <div class="metric-card__icon metric-card__icon--warning">⚠</div>
          <div class="metric-card__body">
            <div class="metric-card__label">跳过</div>
            <div class="metric-card__value">{{ skippedLogs }}</div>
          </div>
        </section>
      </div>
    </section>

    <section class="logs-panel">
      <div class="logs-toolbar-panel">
        <div class="logs-toolbar">
          <el-select v-model="statusFilter" class="logs-toolbar__select" placeholder="状态" @change="handleFiltersChange">
            <el-option label="全部结果" value="all" />
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
            <el-option label="跳过" value="skip" />
          </el-select>

          <el-select v-model="ruleTypeFilter" class="logs-toolbar__select" placeholder="类型" @change="handleFiltersChange">
            <el-option label="全部类型" value="all" />
            <el-option label="归档规则" value="archive" />
            <el-option label="净化规则" value="cleanup" />
            <el-option label="链路规则" value="link" />
            <el-option label="命名规则" value="naming" />
            <el-option label="备份规则" value="backup" />
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
          <div class="logs-summary">
            <span>共 {{ filteredTotal }} 条结果</span>
            <span v-if="searchKeyword">关键字：{{ searchKeyword }}</span>
          </div>

        </div>
      </div>

      <div class="logs-table-shell">
        <el-table v-loading="loading" :data="logTreeRows" class="logs-table logs-tree-table" row-key="id" empty-text="暂无运行日志" @row-click="openLogDetailDialog">
          <!-- 「折叠任务」原来是唯一的弹性列，会把表格剩余宽度全部吃掉（实测 854px），
               后面的列因此被顶到很远。改成定宽后，剩余宽度交给末尾的弹性空列。
               780 = 表格宽 1364 − 后面四列 510 − 右边留白 74（与归巢历史折叠表同款留白），
               时间及其后各列因此整体左移 74px。 -->
          <el-table-column label="折叠任务" width="780">
            <template #default="scope">
              <button type="button" class="logs-detail-card" @click.stop="openLogDetailDialog(scope.row)">
                <span class="logs-detail-card__title">{{ scope.row.title }}</span>
                <span class="logs-detail-card__desc">{{ scope.row.description }}</span>
              </button>
            </template>
          </el-table-column>

          <!-- 时间紧跟在「折叠任务」后面，只到月日与时分秒：标题行已经不重复时间，
               列里再带上年份只是噪音。列序与归巢历史的折叠表保持一致。 -->
          <el-table-column label="时间" width="150" align="center">
            <template #default="scope">
              <span class="logs-time">{{ formatMonthDayTime(scope.row.started_at) }}</span>
            </template>
          </el-table-column>

          <el-table-column label="模式" width="130" align="center">
            <template #default="scope">
              <span class="logs-mode-tag" :class="historyModeTagClass(scope.row)">{{ historyModeLabel(scope.row) }}</span>
            </template>
          </el-table-column>

          <el-table-column label="结果" width="120" align="center">
            <template #default="scope">
              <el-tag class="logs-level-tag" :type="statusTagType(scope.row.status)" effect="light">{{ statusLabel(scope.row.status) }}</el-tag>
            </template>
          </el-table-column>

          <el-table-column label="数量" width="110" align="center">
            <template #default="scope">{{ scope.row.processed_files }}</template>
          </el-table-column>
          <!-- 收尾的弹性空列：专门吃掉表格的剩余宽度。少了它，剩余宽度会被摊回上面各列，
               「折叠任务」又会被撑宽，后面的列就跟着往回跑。 -->
          <el-table-column min-width="1" />
        </el-table>
      </div>

      <el-dialog v-model="logDetailDialogVisible" class="logs-detail-dialog" title="任务详情" width="1080px" top="3vh" destroy-on-close>
        <template v-if="selectedLogGroup">
          <div class="logs-detail-summary">
            <div class="logs-detail-summary__main">
              <div class="logs-detail-summary__title">{{ selectedLogGroup.title }}</div>
              <!-- 统计项兼作筛选入口：点一下只看这一类明细，再点一下取消。
                   右上角不再挂「成功 / 失败」标签——一条执行里成功、跳过、失败本来就同在一行。 -->
              <div class="logs-detail-summary__desc">
                <span class="detail-summary__leading">{{ selectedLogGroupLeading }}</span>
                <span class="detail-summary__sep">·</span>
                <button
                  v-for="segment in selectedLogGroupSegments"
                  :key="segment.key"
                  type="button"
                  class="detail-summary__chip"
                  :class="[`is-${segment.key}`, { 'is-active': detailFilterKey === segment.key }]"
                  @click="toggleDetailFilter(segment.key)"
                >
                  {{ segment.label }}<span class="detail-summary__count">{{ segment.value }}</span>
                </button>
              </div>
            </div>
            <div class="logs-detail-summary__tags">
              <span class="logs-mode-tag" :class="historyModeTagClass(selectedLogGroup)">{{ historyModeLabel(selectedLogGroup) }}</span>
            </div>
          </div>
          <RunDetailList :manifest="selectedRunDetailManifest" :filter-key="detailFilterKey" />
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
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import RunDetailList from '../components/RunDetailList.vue'
import { clearRunHistory, fetchRunHistory, fetchRunHistoryDetail, type RunHistoryItem, type RunHistorySummary } from '../api/runHistory'
import {
  backupTriggerLabel,
  buildRunDetailSummarySegments,
  parseRunDetail,
  type RunDetailSummaryKey,
} from '../utils/backupDetail'
import { pageSizeOptions as settingsPageSizeOptions, useSettingsStore } from '../stores/settings'

type LogTreeRow = RunHistoryItem & {
  // 分组行的 id 加了前缀，仅用于表格 row-key。
  id: string
  // 原始运行日志 id：拉取执行明细（detail_json）时要用它。
  historyId: string
  title: string
  description: string
  is_group: boolean
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

const loading = ref(false)
const historyItems = ref<RunHistoryItem[]>([])
const historySummary = ref<RunHistorySummary>(createDefaultHistorySummary())
const filteredTotal = ref(0)
const keywordInput = ref('')
const searchKeyword = ref('')
const statusFilter = ref<'all' | 'success' | 'failed' | 'skip'>('all')
const ruleTypeFilter = ref<'all' | 'archive' | 'cleanup' | 'link' | 'naming' | 'backup'>('all')
const logsSortBy = ref<'name' | 'modified_at'>('modified_at')
const logsSortOrder = ref<'asc' | 'desc'>('desc')
const settingsStore = useSettingsStore()
const logsPageSizeOptions = settingsPageSizeOptions
const logsPageSize = ref(settingsStore.pageSize || 50)
const logsCurrentPage = ref(1)
const logDetailDialogVisible = ref(false)
const selectedLogGroup = ref<LogTreeRow | null>(null)
// 运行详情（detail_json）：各链路共用，不再只服务备份。
const selectedRunDetail = ref<RunHistoryItem | null>(null)
// 标题下统计项里点中的那一项：null = 不筛选，列出全部文件明细。
const detailFilterKey = ref<RunDetailSummaryKey | null>(null)

const totalLogs = computed(() => historySummary.value.total)
const todayLogs = computed(() => historySummary.value.today)
const successLogs = computed(() => historySummary.value.success)
const failedLogs = computed(() => historySummary.value.failed)
const skippedLogs = computed(() => historySummary.value.skipped)
const logTreeRows = computed(() => buildLogTreeRows(historyItems.value))
// 执行明细：本次执行真的动了哪些文件（备份上传/删除，strm 生成/元数据，打包产出…）。
const selectedRunDetailManifest = computed(() => parseRunDetail(selectedRunDetail.value ?? undefined))
// 详情窗口标题下的第一段文字：规则名 + 触发方式。成功 / 跳过 / 失败不再是死文本，
// 而是后面的可点击统计项（见 selectedLogGroupSegments）。
// strm 任务再补上「Strm N · 元数据 N」：面板里已去掉动作页签，这两类数目只能在这里给。
const selectedLogGroupLeading = computed(() => {
  const group = selectedLogGroup.value
  if (!group) {
    return ''
  }
  return `${group.rule_name || '手动任务'} · ${logTriggerText(group)}`
})
// 统计项：成功 / 跳过 / 失败 + strm 链路额外的 Strm / 元数据。点击即筛选下面的文件明细。
const selectedLogGroupSegments = computed(() =>
  buildRunDetailSummarySegments(selectedLogGroup.value ?? {}, selectedRunDetailManifest.value),
)
// 点中的统计项：再点一次同一个即取消筛选。
function toggleDetailFilter(key: RunDetailSummaryKey) {
  detailFilterKey.value = detailFilterKey.value === key ? null : key
}

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
      // 展示方式固定为折叠（分组）视图，平铺已取消。
      view_mode: 'tree',
    })
    historyItems.value = response.data?.items ?? []
    filteredTotal.value = response.data?.total ?? 0
    historySummary.value = response.data?.summary ?? createDefaultHistorySummary()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '运行日志加载失败')
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
    await ElMessageBox.confirm('确认清空全部运行日志吗？此操作不可恢复。', '清空日志', {
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
    ElMessage.success('运行日志已清空')
  } catch (error) {
    if (error === 'cancel' || error === 'close') {
      return
    }

    ElMessage.error(error instanceof Error ? error.message : '清空运行日志失败')
  }
}

// 折叠列表的「时间」列：只到月日与时分秒（9/17 20:15:03）。
// 折叠条目标题里已经不放时间了，列里再带上年份只是噪音。
function formatMonthDayTime(value?: string) {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return '—'
  }
  const pad = (input: number) => String(input).padStart(2, '0')
  return `${date.getMonth() + 1}/${date.getDate()} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function statusLabel(status: string) {
  if (status === 'success') {
    return '成功'
  }
  if (status === 'failed') {
    return '失败'
  }
  if (status === 'cancelled') {
    return '已停止'
  }
  return '跳过'
}

function statusTagType(status: string): 'success' | 'danger' | 'warning' | 'info' {
  if (status === 'success') {
    return 'success'
  }
  if (status === 'failed') {
    return 'danger'
  }
  if (status === 'cancelled') {
    return 'info'
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

// 备份任务用「实时监控 / 计划扫描 / 手动」表述触发方式，与其它规则区分。
function logTriggerText(item: { archive_mode?: string; trigger_mode: string }) {
  if (item.archive_mode === 'backup') {
    return backupTriggerLabel(item.trigger_mode)
  }
  return triggerModeText(item.trigger_mode)
}

// 折叠条目副行的触发措辞：统一带上「触发」二字
//（实时监控触发 / 计划扫描触发 / 手动执行触发），与归巢历史同一套说法。
function logTriggerPhrase(item?: { trigger_mode?: string }) {
  if (item?.trigger_mode === 'watch') {
    return '实时监控触发'
  }
  if (item?.trigger_mode === 'cron') {
    return '计划扫描触发'
  }
  return '手动执行触发'
}

function resolveLogGroupStatus(items: RunHistoryItem[]) {
  if (items.some((item) => item.status === 'failed')) return 'failed'
  if (items.some((item) => item.status === 'skip')) return 'skip'
  if (items.some((item) => item.status === 'cancelled')) return 'cancelled'
  return 'success'
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
      historyId: first.id,
      status: resolveLogGroupStatus(groupItems),
      processed_files: processed,
      success_count: success,
      skip_count: skipped,
      failure_count: failed,
      // 标题 = 「是什么任务 + 这条规则的自定义名」（如「备份任务 · 剧集追更备份」）：
      // 时间不放在这里，它在右侧的独立列里，两边重复没意义。
      title: `${historyModeLabel(first)}任务 · ${first.rule_name || '手动任务'}`,
      // 副行给「触发方式 + 操作量 + 明细条数」，折叠起来也看得出这次干了多少活。
      description: `${logTriggerPhrase(first)} · 操作 ${processed} 个文件或文件夹 · 共 ${groupItems.length} 条明细`,
      is_group: true,
    }
  })
}

// 执行明细单独拉取：列表接口为避免响应过大不带 detail_json。
// 各链路（备份 / strm / 打包…）共用同一个明细载荷，有就展示、没有就自然隐藏面板。
async function loadSelectedRunDetail(row: LogTreeRow) {
  selectedRunDetail.value = null
  if (!row.historyId) {
    return
  }
  try {
    const payload = await fetchRunHistoryDetail(row.historyId)
    selectedRunDetail.value = payload.data?.item ?? null
  } catch {
    selectedRunDetail.value = null
  }
}

// 详情窗口 = 任务级摘要（标题 + 可点击的统计项 + 模式标签）+ 文件级执行明细。
// 备份任务的规则卡片（源 / 目标 / 扫描配置 / 删除策略）不在这里重复一遍：
// 那一整块就是备份卡片本身，照搬过来只会把明细挤下去。
function openLogDetailDialog(row: LogTreeRow) {
  if (!row.is_group) return
  selectedLogGroup.value = row
  selectedRunDetail.value = null
  // 换一条记录就回到「不筛选」，免得把上一条的筛选态带过来。
  detailFilterKey.value = null
  logDetailDialogVisible.value = true
  void loadSelectedRunDetail(row)
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
    case 'naming':
      return '命名'
    case 'backup':
      return '备份'
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
    case 'naming':
      return 'logs-mode-tag--naming'
    case 'backup':
      return 'logs-mode-tag--backup'
    default:
      return ''
  }
}

onMounted(async () => {
  await settingsStore.ensureLoaded()
  const size = settingsStore.pageSize || 50
  if (logsPageSizeOptions.includes(size)) {
    logsPageSize.value = size
    logsCurrentPage.value = 1
  }
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
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 14px 28px;
  overflow: hidden;
  /* 与仪表盘顶栏（.dashboard-hero）保持同一高度与内边距。 */
  padding: 30px 36px;
  border-radius: 30px;
  border: 1px solid #d9e6ec;
  color: #0f172a;
  background:
    linear-gradient(rgba(32, 159, 238, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(32, 159, 238, 0.04) 1px, transparent 1px),
    radial-gradient(circle at 14% 18%, rgba(182, 197, 201, 0.5), transparent 30%),
    radial-gradient(circle at 86% 20%, rgba(221, 232, 238, 0.8), transparent 26%),
    linear-gradient(100deg, #f1f8fc 0%, #ffffff 46%, #eef4f7 100%);
  background-size: 54px 54px, 54px 54px, auto, auto, auto;
  box-shadow: 0 18px 42px rgba(32, 159, 238, 0.1);
}

.logs-hero::after {
  position: absolute;
  right: -72px;
  bottom: -88px;
  width: 260px;
  height: 260px;
  border-radius: 999px;
  background: radial-gradient(circle, rgba(32, 159, 238, 0.12), transparent 66%);
  content: '';
}

.logs-hero__intro {
  position: relative;
  z-index: 1;
  flex: 1 1 340px;
  min-width: 0;
  max-width: 720px;
}

.logs-hero__eyebrow {
  margin-bottom: 10px;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.16em;
  color: #0975b8;
}

.logs-hero__title {
  margin: 0;
  font-size: 34px;
  line-height: 1.18;
  font-weight: 900;
  color: #0f172a;
}

.logs-hero__desc {
  max-width: 680px;
  margin: 12px 0 0;
  font-size: 15px;
  line-height: 1.8;
  color: #475569;
}

/* 统计栏目放在顶栏右侧区域，不额外转行占用窗口高度。 */
.logs-hero__stats {
  position: relative;
  z-index: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex: 0 1 auto;
}

.metric-card {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 7px 13px;
  border: 1px solid rgba(255, 255, 255, 0.74);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 8px 18px rgba(32, 159, 238, 0.08);
  backdrop-filter: blur(6px);
}

.metric-card__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  flex: 0 0 auto;
  border-radius: 9px;
  font-size: 13px;
  font-weight: 800;
}

.metric-card__icon--primary {
  color: #0975b8;
  background: #e7f0f4;
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
  line-height: 1.2;
}

.metric-card__label {
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
}

.metric-card__value {
  margin-top: 1px;
  font-size: 17px;
  line-height: 1.1;
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

/* 筛选栏与日志条目窗口合并为一个整体窗口：只有一条分隔线，不再是两张独立卡片。 */
.logs-panel {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid #eef2f7;
  border-radius: 26px;
  background: #ffffff;
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.06);
}

.logs-toolbar-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px 18px;
  border-bottom: 1px solid #eef2f7;
  background: linear-gradient(180deg, #fbfdff 0%, #ffffff 100%);
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
  color: #0975b8;
  background: rgba(32, 159, 238, 0.2);
}

.logs-action--sort {
  border: 1px solid rgba(32, 159, 238, 0.16);
  background: rgba(255, 255, 255, 0.82);
}

.logs-action--sort:not(.is-disabled):hover {
  color: #0975b8;
  border-color: rgba(32, 159, 238, 0.42);
  background: rgba(32, 159, 238, 0.14);
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

/* 结果统计并入筛选行右侧，用竖线与筛选区隔开，省掉一整行高度。 */
.logs-summary {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
  padding-left: 14px;
  border-left: 1px solid #e2e8f0;
  color: #64748b;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
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

/* 「结果」列的 成功 / 失败 / 跳过 标签：与「折叠任务」列同一套字体（系统默认 UI 字体），
   不再跟随自托管的鸿蒙字体。 */
.logs-level-tag {
  min-width: 58px;
  border-radius: 999px;
  font-family: var(--font-ui);
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
.logs-mode-tag--naming { color: #0f8f79; background: rgba(15, 159, 135, 0.12); }
/* 备份：莫奈低饱和雾霾蓝，此前未被占用的色相 */
.logs-mode-tag--backup { color: #5f7fa8; background: rgba(95, 127, 168, 0.12); }

.logs-tree-table :deep(.el-table__row) { cursor: pointer; }

/* 折叠任务列：文字整体往右挪一档（表头与内容一起挪，保持对齐）。
   为了让后面几列往左靠，任务列宽度由 min-width 改成了定宽，见模板注释。
   注意 `.logs-tree-table` 本身就是 el-table 根元素，选择器里**不能再写 `.el-table`**
   （那要求 el-table 是它的后代，永远匹配不上）。 */
.logs-tree-table :deep(th:first-child .cell),
.logs-tree-table :deep(td:first-child .cell) { padding-left: 24px; }

/* 折叠任务列的任务条目（标题 + 副行）：走系统默认 UI 字体（--font-ui），
   与页面其它文字一致。原来是自托管的鸿蒙字体，用户反馈不好看，已去掉。
   注意这里必须显式声明：这个节点是 <button>，浏览器对按钮默认用表单控件字体，
   不会继承 body 的 font-family。 */
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
  font-family: var(--font-ui);
}

.logs-detail-card__title {
  color: #0f172a;
  font-size: 16px;
  font-weight: 800;
  line-height: 1.55;
}

.logs-detail-card__desc {
  color: #64748b;
  font-size: 13px;
  line-height: 1.6;
}

/* 详情弹窗高度固定，摘要卡定为不伸缩的顶块：内边距与外边距都收紧，
   让「源路径 / 目标路径」表头尽量贴近上面的标题与统计小字。 */
.logs-detail-summary {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex: 0 0 auto;
  gap: 18px;
  margin-bottom: 8px;
  padding: 8px 14px;
  border: 1px solid #eef2f7;
  border-radius: 14px;
  background: #f8fafc;
}

.logs-detail-summary__title {
  color: #0f172a;
  font-size: 15px;
  font-weight: 900;
}

.logs-detail-summary__main {
  flex: 1 1 auto;
  min-width: 0;
}

/* 前缀文字与可点击的统计项排在一行，靠 gap 分隔，窄了自动换行。 */
.logs-detail-summary__desc {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 3px;
  color: #64748b;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.5;
}

.logs-detail-summary__tags {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.logs-pagination {
  display: flex;
  justify-content: flex-end;
  padding: 0 4px;
}

@media (max-width: 1180px) {
  /* 顶栏放不下时统计栏目整行换到下方，但仍在同一个顶栏窗口内。 */
  .logs-hero__stats {
    width: 100%;
    justify-content: flex-start;
  }
}

@media (max-width: 960px) {
  .logs-hero {
    padding: 26px 24px;
  }

  .logs-toolbar__search {
    width: 100%;
  }
}

@media (max-width: 640px) {
  .logs-hero__title {
    font-size: 28px;
  }
}
</style>

