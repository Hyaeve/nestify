import type { BackupTask } from '../api/backups'

// 备份任务详情：运行日志 / 归巢历史里点开「备份」任务时展示的结构化信息。
export interface BackupDetailRow {
  label: string
  values: string[]
}

// 备份任务的触发方式措辞与其它规则区分：实时监控 / 计划扫描 / 手动。
export function backupTriggerLabel(mode?: string): string {
  switch (mode) {
    case 'watch':
      return '实时监控触发'
    case 'cron':
      return '计划扫描触发'
    case 'once':
      return '启动后自动执行'
    default:
      return '手动执行'
  }
}

export function backupCompletionRuleLabel(rule?: string): string {
  switch (rule) {
    case 'delete_source':
      return '完成后删除源文件'
    case 'delete_source_dir':
      return '完成后删除源目录（含空文件夹）'
    default:
      return '完成后保留源文件'
  }
}

export function backupReplaceRuleLabel(rule?: string): string {
  return rule === 'overwrite' ? '同名文件覆盖' : '同名文件跳过'
}

// 兼容历史记录：早期运行日志没有 deleted_count 字段，从摘要里的「删除 N」回退解析。
export function resolveBackupDeletedCount(item?: { deleted_count?: number; summary?: string }): number {
  const stored = Number(item?.deleted_count || 0)
  if (stored > 0) {
    return stored
  }
  const match = item?.summary?.match(/删除\s*(\d+)/)
  return match ? Number(match[1]) : 0
}

// ---------------------------------------------------------------------------
// 备份文件明细（run_history.detail_json）
// ---------------------------------------------------------------------------

export type BackupFileAction = 'upload' | 'skip' | 'fail' | 'delete'

// 明细面板只列「真的动了文件」的三种结果；跳过只显示数目，不参与筛选。
export type BackupFileFilter = 'all' | 'upload' | 'fail' | 'delete'

export interface BackupFileEntry {
  path: string
  action: BackupFileAction
  size?: number
  target?: string
  note?: string
  dir?: boolean
}

export interface BackupFileManifest {
  files: BackupFileEntry[]
  counts: Record<BackupFileAction, number>
  total: number
  truncated: boolean
}

const emptyActionCounts = (): Record<BackupFileAction, number> => ({
  upload: 0,
  skip: 0,
  fail: 0,
  delete: 0,
})

function normalizeAction(value: unknown): BackupFileAction {
  if (value === 'skip' || value === 'fail' || value === 'delete') {
    return value
  }
  return 'upload'
}

// 解析备份执行的文件明细；无明细（其它模式或历史记录）时返回 null，调用方据此隐藏面板。
export function parseBackupManifest(item?: { archive_mode?: string; detail_json?: string }): BackupFileManifest | null {
  const raw = item?.detail_json?.trim()
  if (!raw) {
    return null
  }

  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return null
  }
  if (!parsed || typeof parsed !== 'object') {
    return null
  }

  const payload = parsed as {
    files?: unknown
    counts?: unknown
    files_total?: unknown
    files_truncated?: unknown
  }
  const rawFiles = Array.isArray(payload.files) ? payload.files : []
  const files: BackupFileEntry[] = []

  for (const candidate of rawFiles) {
    if (!candidate || typeof candidate !== 'object') {
      continue
    }
    const entry = candidate as Record<string, unknown>
    const path = typeof entry.path === 'string' ? entry.path.trim() : ''
    if (!path) {
      continue
    }
    const action = normalizeAction(entry.action)
    // 「跳过」不列明细（旧记录里可能存过这类条目，这里一并丢掉），只用 counts.skip 显示数目。
    if (action === 'skip') {
      continue
    }
    files.push({
      path,
      action,
      size: typeof entry.size === 'number' ? entry.size : undefined,
      target: typeof entry.target === 'string' ? entry.target : undefined,
      note: typeof entry.note === 'string' ? entry.note : undefined,
      dir: entry.dir === true,
    })
  }

  const counts = emptyActionCounts()
  const rawCounts = payload.counts
  if (rawCounts && typeof rawCounts === 'object') {
    for (const [key, value] of Object.entries(rawCounts as Record<string, unknown>)) {
      const action = normalizeAction(key)
      if (typeof value === 'number' && value > 0) {
        counts[action] += value
      }
    }
  } else {
    for (const file of files) {
      counts[file.action] += 1
    }
  }

  // 全部是「跳过」的执行也要保留（面板要显示跳过数目），彻底没有明细时才返回 null。
  if (files.length === 0 && counts.skip === 0) {
    return null
  }

  // 「共 N 项」只算会被列出的动作：旧记录的 files_total 含跳过，不能直接用。
  const listedTotal = counts.upload + counts.fail + counts.delete
  const total = listedTotal > 0 ? listedTotal : files.length

  return {
    files,
    counts,
    total,
    truncated: payload.files_truncated === true,
  }
}

export function backupFileActionLabel(action: BackupFileAction): string {
  switch (action) {
    case 'skip':
      return '跳过'
    case 'fail':
      return '失败'
    case 'delete':
      return '删除'
    default:
      return '上传'
  }
}

export function backupFileActionClass(action: BackupFileAction): string {
  switch (action) {
    case 'skip':
      return 'is-skip'
    case 'fail':
      return 'is-fail'
    case 'delete':
      return 'is-delete'
    default:
      return 'is-upload'
  }
}

export function formatBackupFileSize(bytes?: number): string {
  const value = Number(bytes || 0)
  if (!Number.isFinite(value) || value <= 0) {
    return '—'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }

  const digits = size >= 100 || unitIndex === 0 ? 0 : 1
  return `${size.toFixed(digits)} ${units[unitIndex]}`
}

function describeScanPlan(task: BackupTask): string {
  const parts: string[] = []
  if (task.monitor_enabled) {
    parts.push('实时监控文件变更')
  }
  if (task.cron_expression) {
    parts.push(`计划 ${task.cron_expression}`)
  }
  if (task.scan_interval_seconds > 0) {
    parts.push(`每 ${task.scan_interval_seconds} 秒轮询`)
  }
  if (task.force_full_scan) {
    parts.push('强制完整扫描')
  }
  return parts.length ? parts.join(' · ') : '仅手动或计划触发'
}

export function buildBackupDetailRows(
  task: BackupTask | null,
  options: { triggerMode?: string; deletedCount?: number; successCount?: number; skipCount?: number; failureCount?: number } = {},
): BackupDetailRow[] {
  const rows: BackupDetailRow[] = []

  if (task) {
    rows.push({ label: '备份规则', values: [task.enabled ? task.name : `${task.name}（已停用）`] })
    rows.push({ label: '源路径', values: task.source_dirs.length ? task.source_dirs : ['—'] })
    rows.push({ label: '目标路径', values: task.target_dirs.length ? task.target_dirs : ['—'] })
  }

  rows.push({ label: '触发方式', values: [backupTriggerLabel(options.triggerMode)] })

  if (task) {
    rows.push({ label: '扫描配置', values: [describeScanPlan(task)] })
    rows.push({ label: '同名处理', values: [backupReplaceRuleLabel(task.replace_rule)] })
    rows.push({ label: '完成后处理', values: [backupCompletionRuleLabel(task.completion_rule)] })
    rows.push({
      label: '同步删除目标',
      values: [
        task.sync_delete_from_target
          ? '已开启 · 源中已不存在的文件会从目标一并删除'
          : '未开启 · 目标中多余的旧文件会保留',
      ],
    })
    rows.push({ label: '筛选规则', values: [task.filter_rules.length ? `${task.filter_rules.length} 条` : '未设置'] })
  }

  const deleted = Math.max(0, Number(options.deletedCount || 0))
  const copied = Math.max(0, Number(options.successCount || 0))
  const skipped = Math.max(0, Number(options.skipCount || 0))
  const failed = Math.max(0, Number(options.failureCount || 0))
  rows.push({ label: '文件结果', values: [`上传 ${copied} · 跳过 ${skipped} · 失败 ${failed}`] })
  rows.push({
    label: '文件删除',
    values: [
      deleted > 0
        ? `本次共删除 ${deleted} 项（源目录清理 / 目标同步删除）`
        : '本次未删除任何文件或文件夹',
    ],
  })

  return rows
}
