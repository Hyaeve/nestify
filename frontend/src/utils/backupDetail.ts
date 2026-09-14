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
