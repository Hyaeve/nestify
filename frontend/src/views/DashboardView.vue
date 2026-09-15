<template>
  <div class="dashboard-view">
    <section class="dashboard-hero">
      <div class="dashboard-hero__content">
        <div class="dashboard-hero__eyebrow">DASHBOARD</div>
        <h1>运行总览</h1>
        <p>集中查看规则状态、实时任务与执行摘要，快速掌握 Nestify 当前运行情况。</p>
      </div>

      <!-- 顶栏右侧：本项目（Nestify 进程）资源占用 -->
      <div class="dashboard-hero__metrics">
        <div class="hero-metric">
          <div class="hero-metric__head">
            <span class="hero-metric__icon">⚙</span>
            <span class="hero-metric__label">CPU 使用率</span>
            <span class="hero-metric__value">{{ formatPercentage(systemResource?.nestify_cpu_percent) }}</span>
          </div>
          <div class="hero-metric__hint">本项目占用</div>
          <el-progress
            class="hero-metric__progress"
            :percentage="clampPercentage(systemResource?.nestify_cpu_percent)"
            :show-text="false"
            :stroke-width="6"
            color="#6b9fb0"
          />
        </div>

        <div class="hero-metric">
          <div class="hero-metric__head">
            <span class="hero-metric__icon">▣</span>
            <span class="hero-metric__label">内存使用率</span>
            <span class="hero-metric__value">{{ formatPercentage(systemResource?.nestify_memory_percent) }}</span>
          </div>
          <div class="hero-metric__hint">占用 {{ systemResource?.nestify_memory || '0 B' }}</div>
          <el-progress
            class="hero-metric__progress"
            :percentage="clampPercentage(systemResource?.nestify_memory_percent)"
            :show-text="false"
            :stroke-width="6"
            color="#7c3aed"
          />
        </div>

        <div class="hero-metric hero-metric--net">
          <div class="hero-metric__head">
            <span class="hero-metric__icon">⇅</span>
            <span class="hero-metric__label">上传 / 下载</span>
          </div>
          <div class="hero-metric__net">
            <span class="hero-metric__net-row">
              <span class="hero-metric__net-key">上传</span>
              <span class="hero-metric__net-value">{{ systemResource?.nestify_upload_speed || '0 B/s' }}</span>
            </span>
            <span class="hero-metric__net-row">
              <span class="hero-metric__net-key">下载</span>
              <span class="hero-metric__net-value">{{ systemResource?.nestify_download_speed || '0 B/s' }}</span>
            </span>
          </div>
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
      <!-- 左侧大窗口：任务预览（运行中优先，空闲时展示最近完成的任务） -->
      <div class="dashboard-panel task-preview-card">
        <div class="task-preview-card__header">
          <div>
            <div class="dashboard-panel__eyebrow">LIVE TASKS</div>
            <h3 class="page-section-title">任务预览</h3>
          </div>
          <div class="task-preview-card__actions">
            <el-tag :type="runningTasks.length ? 'success' : 'info'" effect="light" size="small">{{ previewBadgeText }}</el-tag>
          </div>
        </div>

        <div v-if="previewTasks.length" class="task-preview-list">
          <div
            v-for="item in previewTasks"
            :key="item.id"
            class="task-preview-item"
            :class="{ 'is-running': item.status === 'running' }"
          >
            <div class="task-preview-item__header">
              <div class="task-preview-item__main">
                <div class="task-preview-item__name">
                  <span class="task-preview-item__kind" :class="`task-preview-item__kind--${item.kind}`">{{ kindLabel(item.kind) }}</span>
                  {{ item.ruleName }}
                </div>
                <div class="task-preview-item__meta">{{ item.metaText }}</div>
                <div v-if="item.detailText" class="task-preview-item__detail">{{ item.detailText }}</div>
              </div>
              <div class="task-preview-item__badges">
                <span v-if="item.modeLabel" class="dashboard-mode-tag" :class="item.modeClass">{{ item.modeLabel }}</span>
                <el-tag :type="statusTagType(item.status)" effect="light" size="small">{{ statusLabel(item.status) }}</el-tag>
              </div>
            </div>

            <el-progress
              v-if="item.status === 'running'"
              :percentage="item.progress"
              :stroke-width="8"
              :show-text="false"
              status="success"
            />

            <div class="task-preview-item__stats">
              <span>扫描 {{ item.scanned }}</span>
              <span v-if="item.kind === 'backup'">上传 {{ item.success_count }}</span>
              <span v-else>成功 {{ item.success_count }}</span>
              <span>跳过 {{ item.skip_count }}</span>
              <span>失败 {{ item.failure_count }}</span>
            </div>

            <div v-if="item.pathLines.length" class="task-preview-item__paths">
              <div v-for="line in item.pathLines" :key="line.label" class="task-preview-item__path">
                <span class="task-preview-item__path-label">{{ line.label }}</span>
                <span class="task-preview-item__path-value" :title="line.value">{{ line.value || '未设置' }}</span>
              </div>
            </div>

            <div v-if="item.status === 'running'" class="task-preview-item__logs">
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

        <el-empty v-else class="task-preview-empty" description="暂无执行中的任务" />
      </div>

      <!-- 右侧：执行摘要 -->
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
                  <span class="summary-item__name">{{ displayRuleName(item) }}</span>
                </div>
                <div class="summary-item__badges">
                  <span v-if="modeLabelOf(item.archive_mode, item.link_mode)" class="dashboard-mode-tag" :class="modeClassOf(item.archive_mode, item.link_mode)">{{ modeLabelOf(item.archive_mode, item.link_mode) }}</span>
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
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import { fetchRunningBackups } from '../api/backups'
import { fetchRunLogs, fetchRuns, type RunInstance, type RunLogEntry } from '../api/executions'
import { emptyRunHistory, fetchRunHistory, type RunHistoryItem } from '../api/runHistory'
import { fetchRules, type RuleItem } from '../api/rules'
import { fetchHealth, fetchSystemResource, type SystemResourcePayload } from '../api/system'
import { formatRunHistorySummary } from '../utils/runHistorySummary'

// 任务预览最多同时展示的条目数（运行中优先，其余用最近完成补齐）。
const PREVIEW_LIMIT = 6

type PreviewKind = 'rule' | 'backup' | 'manual'
type PreviewStatus = 'running' | 'success' | 'failed' | 'skipped'

interface PreviewPathLine {
  label: string
  value: string
}

/** 统一的任务预览条目：规则执行、备份任务、手动解压/压缩/收集都归一到这个结构。 */
interface PreviewTask {
  id: string
  kind: PreviewKind
  ruleName: string
  modeLabel: string
  modeClass: string
  metaText: string
  detailText: string
  status: PreviewStatus
  pathLines: PreviewPathLine[]
  progress: number
  scanned: number
  success_count: number
  skip_count: number
  failure_count: number
  logs: string[]
  logsLoading: boolean
}

const healthError = ref('')
const summaryItems = ref<RunHistoryItem[]>(emptyRunHistory())
const rules = ref<RuleItem[]>([])
const systemResource = ref<SystemResourcePayload | null>(null)

const runningTasks = ref<PreviewTask[]>([])
let previewPollTimer: number | null = null
let resourceWarmTimer: number | null = null

// 任务预览只展示「正在执行」的任务；历史记录统一由右侧「执行摘要」承载。
const previewTasks = computed<PreviewTask[]>(() => runningTasks.value.slice(0, PREVIEW_LIMIT))

const previewBadgeText = computed(() =>
  runningTasks.value.length ? `${runningTasks.value.length} 个任务进行中` : '当前没有执行中的任务',
)

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

function kindLabel(kind: PreviewKind) {
  if (kind === 'backup') return '备份'
  if (kind === 'manual') return '手动'
  return '规则'
}

function statusLabel(status: PreviewStatus) {
  if (status === 'running') return '进行中'
  if (status === 'success') return '已完成'
  if (status === 'failed') return '失败'
  return '跳过'
}

function statusTagType(status: PreviewStatus): 'success' | 'warning' | 'danger' {
  if (status === 'running') return 'warning'
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  return 'warning'
}

function formatPercentage(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return '—'
  }
  if (value > 0 && value < 0.1) {
    return '<0.1%'
  }
  return `${value.toFixed(1)}%`
}

// el-progress 的 percentage 只接受 0~100，这里兜底避免异常数据把进度条撑破。
function clampPercentage(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) {
    return 0
  }
  return Math.min(100, Math.max(0, value))
}

function formatDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value || '—'
  }
  return date.toLocaleString('zh-CN', { hour12: false })
}

/** 预览里的时间更紧凑：只保留月-日 时:分，避免单行被撑开。 */
function formatCompactTime(value?: string) {
  if (!value) {
    return '—'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

/** 触发方式：与规则卡片/运行日志的语义保持一致。 */
function triggerLabel(mode?: string) {
  if (mode === 'watch') return '实时监控'
  if (mode === 'cron') return '计划扫描'
  return '手动'
}

function modeLabelOf(archiveMode?: string, linkMode?: string) {
  switch (archiveMode) {
    case 'package':
      return '打包'
    case 'collect':
      return '收集'
    case 'cleanup':
      return '清理'
    case 'transform':
      return '转换'
    case 'link':
      if (linkMode === 'strm') return 'Strm'
      return linkMode === 'hard' ? '硬链' : '软链'
    case 'naming':
      return '命名'
    case 'backup':
      return '备份'
    case 'extract':
      return '解压'
    default:
      return ''
  }
}

function modeClassOf(archiveMode?: string, linkMode?: string) {
  switch (archiveMode) {
    case 'package':
      return 'dashboard-mode-tag--package'
    case 'collect':
      return 'dashboard-mode-tag--collect'
    case 'cleanup':
      return 'dashboard-mode-tag--cleanup'
    case 'transform':
      return 'dashboard-mode-tag--transform'
    case 'link':
      if (linkMode === 'strm') return 'dashboard-mode-tag--strm'
      return linkMode === 'hard' ? 'dashboard-mode-tag--hardlink' : 'dashboard-mode-tag--softlink'
    case 'naming':
      return 'dashboard-mode-tag--naming'
    case 'backup':
      return 'dashboard-mode-tag--backup'
    case 'extract':
      return 'dashboard-mode-tag--extract'
    default:
      return ''
  }
}

/** 运行日志里的手动任务名称不可读，这里统一映射成中文。 */
function displayRuleName(item: RunHistoryItem) {
  const name = (item.rule_name || '').trim()
  if (name === 'manual-extract') return '手动解压'
  if (name === 'manual-collect') return '手动收集'
  if (name === 'manual-pack') return '手动压缩'
  if (name) return name
  if (item.archive_mode === 'backup') return '备份任务'
  if (item.archive_mode === 'extract') return '手动解压'
  return '手动任务'
}

function normalizeSourceDirs(rule: RuleItem) {
  const list = (rule.source_dirs ?? []).map((dir) => dir.trim()).filter(Boolean)
  if (list.length) {
    return list
  }
  const single = (rule.source_dir || '').trim()
  return single ? [single] : []
}

/**
 * 规则任务的路径展示：净化（清理/转换）只有监控目录、命名只有监控路径，
 * 不能再照搬「源路径 / 目标路径」，按模式适配。
 */
function rulePreviewPaths(rule: RuleItem | undefined, archiveMode?: string): PreviewPathLine[] {
  if (!rule) {
    return []
  }

  const sourceDirs = normalizeSourceDirs(rule)
  const mode = archiveMode || rule.archive_mode

  if (mode === 'cleanup' || mode === 'transform') {
    return [{ label: '监控目录', value: sourceDirs.join('  |  ') }]
  }
  if (mode === 'naming') {
    return [{ label: '监控路径', value: sourceDirs.join('  |  ') }]
  }

  return [
    { label: '源路径', value: sourceDirs.join('  |  ') },
    { label: '目标路径', value: rule.target_dir },
  ]
}

function estimateProgress(item: RunInstance) {
  const total = item.success_count + item.skip_count + item.failure_count
  if (total <= 0) {
    return 12
  }
  return Math.min(92, Math.max(18, total % 100))
}

// 仅用于在后端不可达时给出提示条。
async function loadHealth() {
  healthError.value = ''

  try {
    await fetchHealth()
  } catch (error) {
    healthError.value = error instanceof Error ? error.message : '后端连接失败'
  }
}

// 顶栏右侧：本项目进程的 CPU / 内存 / 网络占用（失败时保留上一次的值，避免闪烁成空）。
async function loadSystemResource() {
  try {
    const response = await fetchSystemResource()
    systemResource.value = response.data ?? null
  } catch {
    // 静默失败：轮询会持续重试，不打断仪表盘其它区域。
  }
}

/** 执行摘要的数据源：运行日志（一条数据源覆盖规则 / 备份 / 手动任务）。 */
async function loadSummary() {
  try {
    const items = (await fetchRunHistory()).data?.items ?? []
    // 执行摘要：最多可见 50 条，超出通过列表滚动展示。
    summaryItems.value = items.slice(0, 50)
  } catch {
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

// 拉取运行中的规则任务（含执行日志）。
async function loadRuleRunningTasks(): Promise<PreviewTask[]> {
  try {
    const runs = (await fetchRuns()).data?.items ?? []
    const ruleMap = rules.value.reduce<Record<number, RuleItem>>((map, item) => {
      map[item.id] = item
      return map
    }, {})

    interface RuleDraft {
      task: PreviewTask
      rawRunId: string
    }

    const drafts: RuleDraft[] = runs
      .filter((item) => item.status === 'running' && typeof item.rule_id === 'number')
      .map((item) => {
        const rule = item.rule_id ? ruleMap[item.rule_id] : undefined
        const currentTarget = item.current_volume_or_dir || item.current_series || '正在扫描源目录'
        return {
          rawRunId: item.id,
          task: {
            id: `rule-${item.id}`,
            kind: 'rule',
            ruleName: rule?.name || item.rule_name || '未知规则',
            modeLabel: modeLabelOf(item.archive_mode, item.link_mode),
            modeClass: modeClassOf(item.archive_mode, item.link_mode),
            metaText: `${triggerLabel(rule?.run_mode || item.trigger_mode)} · 开始于 ${formatCompactTime(item.started_at)}`,
            detailText: `当前执行：${currentTarget}`,
            status: 'running',
            pathLines: rulePreviewPaths(rule, item.archive_mode),
            progress: estimateProgress(item),
            scanned: item.processed_files ?? 0,
            success_count: item.success_count,
            skip_count: item.skip_count,
            failure_count: item.failure_count,
            logs: [],
            logsLoading: true,
          },
        }
      })

    await Promise.all(drafts.map(async (entry) => {
      try {
        const response = await fetchRunLogs(entry.rawRunId)
        entry.task.logs = (response.data?.items ?? []).slice(-5).map((log: RunLogEntry) => {
          const time = new Date(log.created_at).toLocaleTimeString('zh-CN', { hour12: false })
          return `[${time}] ${log.message}`
        })
      } catch {
        entry.task.logs = []
      } finally {
        entry.task.logsLoading = false
      }
    }))

    return drafts.map((entry) => entry.task)
  } catch {
    return []
  }
}

// 拉取运行中的备份任务。
async function loadBackupRunningTasks(): Promise<PreviewTask[]> {
  try {
    const response = await fetchRunningBackups()
    const items = response.data?.items ?? []
    return items.map((snapshot) => {
      const total = snapshot.scanned + snapshot.copied + snapshot.skipped + snapshot.deleted + snapshot.failed
      const progress = total <= 0 ? 12 : Math.min(92, Math.max(18, total % 100))
      const phase = snapshot.progress || snapshot.phase || '备份中'
      return {
        id: `backup-${snapshot.task_id}`,
        kind: 'backup' as const,
        ruleName: snapshot.task_name || `备份任务 #${snapshot.task_id}`,
        modeLabel: '备份',
        modeClass: 'dashboard-mode-tag--backup',
        metaText: `备份中 · ${snapshot.phase || '扫描中'}`,
        detailText: phase,
        status: 'running' as const,
        pathLines: [],
        progress,
        scanned: snapshot.scanned,
        success_count: snapshot.copied,
        skip_count: snapshot.skipped,
        failure_count: snapshot.failed,
        logs: (snapshot.recent_logs ?? []).slice(-5),
        logsLoading: false,
      }
    })
  } catch {
    return []
  }
}

// 真正拉数据：运行中任务 + 执行摘要，两者共用一条刷新链路。
async function reloadPreview() {
  await loadRules()
  const [ruleTasks, backupTasks] = await Promise.all([
    loadRuleRunningTasks(),
    loadBackupRunningTasks(),
  ])
  runningTasks.value = [...ruleTasks, ...backupTasks]
  await loadSummary()
}

function startPreviewPolling() {
  stopPreviewPolling()
  // 静默刷新（没有手动刷新按钮），顶栏资源统计与采样窗口（5s）对齐，同时刷新。
  previewPollTimer = window.setInterval(() => {
    void reloadPreview()
    void loadSystemResource()
  }, 5000)
}

function stopPreviewPolling() {
  if (previewPollTimer !== null) {
    window.clearInterval(previewPollTimer)
    previewPollTimer = null
  }
}

// 首帧只能用来建立采样基准（速率为 0），稍后再取一次得到真实读数，避免顶栏长时间显示 0。
function warmUpSystemResource() {
  if (resourceWarmTimer !== null) {
    window.clearTimeout(resourceWarmTimer)
  }
  resourceWarmTimer = window.setTimeout(() => {
    resourceWarmTimer = null
    void loadSystemResource()
  }, 1500)
}

function stopResourceWarmUp() {
  if (resourceWarmTimer !== null) {
    window.clearTimeout(resourceWarmTimer)
    resourceWarmTimer = null
  }
}

onMounted(() => {
  void loadHealth()
  void reloadPreview()
  void loadSystemResource()
  warmUpSystemResource()
  startPreviewPolling()
})

onBeforeUnmount(() => {
  stopPreviewPolling()
  stopResourceWarmUp()
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
  border: 1px solid rgba(107, 159, 176, 0.2);
  color: #0f172a;
  background:
    linear-gradient(rgba(107, 159, 176, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(107, 159, 176, 0.045) 1px, transparent 1px),
    radial-gradient(circle at 12% 18%, rgba(186, 230, 253, 0.55), transparent 28%),
    radial-gradient(circle at 86% 20%, rgba(224, 231, 255, 0.9), transparent 24%),
    linear-gradient(100deg, #eff8ff 0%, #ffffff 45%, #f4f7ff 100%);
  background-size: 54px 54px, 54px 54px, auto, auto, auto;
  box-shadow: 0 18px 42px rgba(107, 159, 176, 0.08);
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

.dashboard-hero__content {
  position: relative;
  z-index: 1;
  min-width: 0;
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
  color: #3f6c7d;
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

/* 顶栏右侧：本项目（Nestify 进程）占用统计，三栏等宽 */
.dashboard-hero__metrics {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: stretch;
  gap: 12px;
  flex: 0 0 auto;
}

.hero-metric {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  min-width: 164px;
  padding: 12px 14px;
  border: 1px solid rgba(107, 159, 176, 0.42);
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
  flex: 0 0 auto;
  width: 22px;
  height: 22px;
  border-radius: 8px;
  background: rgba(107, 159, 176, 0.14);
  color: #3f6c7d;
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
  color: #3f6c7d;
  font-size: 16px;
  font-weight: 900;
  white-space: nowrap;
}

.hero-metric__hint {
  color: #94a3b8;
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
}

.hero-metric__progress {
  width: 100%;
}

.hero-metric__progress :deep(.el-progress-bar__outer) {
  background-color: rgba(107, 159, 176, 0.2);
  border-radius: 999px;
}

.hero-metric__progress :deep(.el-progress-bar__inner) {
  border-radius: 999px;
}

/* 网络吞吐：上下两行展示上传/下载速率 */
.hero-metric--net {
  gap: 6px;
}

.hero-metric__net {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.hero-metric__net-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
}

.hero-metric__net-key {
  color: #94a3b8;
  font-size: 11px;
  font-weight: 700;
}

.hero-metric__net-value {
  color: #0f766e;
  font-size: 13px;
  font-weight: 900;
  white-space: nowrap;
}

.dashboard-content {
  display: grid;
  /* 任务预览只放执行中的任务，收窄一点；执行摘要因此更宽，长规则名/摘要更好读。 */
  grid-template-columns: minmax(0, 1.2fr) minmax(380px, 0.8fr);
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
  color: #3f6c7d;
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
  background: rgba(107, 159, 176, 0.14);
  color: #3f6c7d;
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
  background: #6b9fb0;
  box-shadow: 0 0 0 5px rgba(107, 159, 176, 0.12);
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

.task-preview-item__main {
  min-width: 0;
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
  color: #5f7fa8;
  background: rgba(95, 127, 168, 0.12);
}

.task-preview-item__kind--rule {
  color: #3f6c7d;
  background: rgba(107, 159, 176, 0.14);
}

.task-preview-item__kind--manual {
  color: #3f6c7d;
  background: #eef3f1;
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
.dashboard-mode-tag--softlink { color: #c47c98; background: rgba(196, 124, 152, 0.12); }
.dashboard-mode-tag--strm { color: #2f8f9d; background: rgba(47, 143, 157, 0.12); }
.dashboard-mode-tag--naming { color: #0f8f79; background: rgba(15, 159, 135, 0.12); }
.dashboard-mode-tag--backup { color: #5f7fa8; background: rgba(95, 127, 168, 0.12); }
.dashboard-mode-tag--extract { color: #a4638a; background: rgba(164, 99, 138, 0.12); }

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

/* 运行中的任务给一个左侧强调条，与「已完成」的历史记录区分开。 */
.task-preview-item.is-running {
  border-color: rgba(107, 159, 176, 0.22);
  background: linear-gradient(180deg, #f8fbff 0%, #f5f9ff 100%);
  box-shadow: inset 3px 0 0 #8fbdcb;
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

.task-preview-item__paths {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 10px;
}

.task-preview-item__path {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.task-preview-item__path-label {
  flex: 0 0 auto;
  color: #94a3b8;
  font-size: 12px;
  font-weight: 700;
}

.task-preview-item__path-value {
  flex: 1 1 auto;
  min-width: 0;
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

/* 顶栏一行放不下三张统计卡时，让它们整行换到标题下方 */
@media (max-width: 1180px) {
  .dashboard-hero {
    flex-wrap: wrap;
  }

  .dashboard-hero__metrics {
    width: 100%;
    flex-wrap: wrap;
  }

  .hero-metric {
    flex: 1 1 160px;
  }
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
