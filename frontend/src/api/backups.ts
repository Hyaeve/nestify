import { deleteJSON, getJSON, postJSON, putJSON } from './http'

export type BackupCompletionRule = 'none' | 'delete_source' | 'delete_source_dir'
export type BackupReplaceRule = 'skip' | 'overwrite'
export type BackupFilterType = 'name' | 'extension' | 'regex' | 'size'
export type BackupSizeUnit = 'Bytes' | 'KB' | 'MB' | 'GB'

export interface BackupFilterRule {
  id: string
  type: BackupFilterType
  value: string
  blacklist: boolean
  whitelist: boolean
  match_dir: boolean
  match_file: boolean
  min_size: number
  max_size: number
  size_unit: BackupSizeUnit
  extensions?: string[]
}

export interface BackupTask {
  id: number
  sort_order: number
  name: string
  enabled: boolean
  source_dirs: string[]
  target_dirs: string[]
  monitor_enabled: boolean
  completion_rule: BackupCompletionRule
  replace_rule: BackupReplaceRule
  sync_delete_from_target: boolean
  force_full_scan: boolean
  scan_interval_seconds: number
  cron_expression: string
  filter_rules: BackupFilterRule[]
  last_backup_at: string
  last_status: string
  last_summary: string
  last_scanned_files: number
  last_copied_files: number
  last_skipped_files: number
  last_deleted_files: number
  created_at: string
  updated_at: string
}

export interface BackupInput {
  name: string
  enabled: boolean
  source_dirs: string[]
  target_dirs: string[]
  monitor_enabled: boolean
  completion_rule: BackupCompletionRule
  replace_rule: BackupReplaceRule
  sync_delete_from_target: boolean
  force_full_scan: boolean
  scan_interval_seconds: number
  cron_expression: string
  filter_rules: BackupFilterRule[]
}

export interface BackupStatusSnapshot {
  task_id: number
  task_name: string
  running: boolean
  status: string
  phase: string
  progress: string
  scanned: number
  copied: number
  skipped: number
  deleted: number
  failed: number
  last_backup_at: string
  recent_logs: string[]
}

export interface BackupListPayload {
  items: BackupTask[]
}

export function fetchBackups() {
  return getJSON<BackupListPayload>('/api/v1/backups')
}

export function createBackup(payload: BackupInput) {
  return postJSON<BackupTask>('/api/v1/backups', payload)
}

export function updateBackup(id: number, payload: BackupInput) {
  return putJSON<BackupTask>(`/api/v1/backups/${id}`, payload)
}

export function deleteBackup(id: number) {
  return deleteJSON<Record<string, never>>(`/api/v1/backups/${id}`)
}

export function runBackup(id: number, forceFull = false) {
  return postJSON<BackupStatusSnapshot>(`/api/v1/backups/${id}/run`, { force_full: forceFull })
}

/** 停止一个正在执行的备份任务（卡片上的「扫描」按钮再次点击）。 */
export function cancelBackup(id: number) {
  return postJSON<Record<string, never>>(`/api/v1/backups/${id}/cancel`, {})
}

export function setBackupEnabled(id: number, enabled: boolean) {
  return postJSON<BackupTask>(`/api/v1/backups/${id}/enabled`, { enabled })
}

export interface BackupReorderItem {
  id: number
  sort_order: number
}

export function reorderBackups(items: BackupReorderItem[]) {
  return putJSON<Record<string, never>>('/api/v1/backups/reorder', items)
}

export function fetchBackupStatus(id: number) {
  return getJSON<BackupStatusSnapshot>(`/api/v1/backups/${id}/status`)
}

export interface RunningBackupsPayload {
  items: BackupStatusSnapshot[]
}

export function fetchRunningBackups() {
  return getJSON<RunningBackupsPayload>('/api/v1/backups/running')
}
