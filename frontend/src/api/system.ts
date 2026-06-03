import { getJSON } from './http'
import type { ApiResponse } from './http'

export interface HealthPayload {
  service: string
  time: string
}

export interface SystemResourcePayload {
  cpu_usage: number
  cpu_model: string
  memory_usage: number
  memory_used: string
  memory_total: string
  nestify_memory: string
  uptime: string
}

export interface SettingsPayload {
  id: number
  timezone: string
  log_level: string
  log_retention_days: number
  log_retention_max_records: number
  created_at: string
  updated_at: string
}

export interface UpdateSettingsPayload {
  log_retention_days: number
  log_retention_max_records: number
}

export interface RuleBackupPayload {
  version: string
  exported_at: string
  rules: Array<Record<string, unknown>>
}

export function fetchHealth() {
  return getJSON<HealthPayload>('/api/v1/health')
}

export function fetchSystemResource() {
  return getJSON<SystemResourcePayload>('/api/v1/system/resource')
}

export function fetchSettings() {
  return getJSON<SettingsPayload>('/api/v1/settings')
}

export async function updateSettings(payload: UpdateSettingsPayload) {
  const response = await fetch('/api/v1/settings', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify(payload),
  })

  const result = (await response.json()) as ApiResponse<SettingsPayload>
  if (!response.ok) {
    throw new Error(result.message || '保存系统设置失败')
  }

  return result
}

export async function restartSystem() {
  const response = await fetch('/api/v1/system/restart', {
    method: 'POST',
    headers: {
      Accept: 'application/json',
    },
    credentials: 'include',
  })

  const payload = (await response.json()) as ApiResponse<null>
  if (!response.ok) {
    throw new Error(payload.message || '重启系统失败')
  }

  return payload
}

export async function exportRulesBackup() {
  const response = await fetch('/api/v1/settings/rules-backup', {
    method: 'GET',
    headers: {
      Accept: 'application/json',
    },
    credentials: 'include',
  })

  if (!response.ok) {
    const payload = (await response.json()) as ApiResponse<null>
    throw new Error(payload.message || '导出规则备份失败')
  }

  return response.blob()
}

export async function importRulesBackup(payload: RuleBackupPayload) {
  const response = await fetch('/api/v1/settings/rules-backup', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify(payload),
  })

  const result = (await response.json()) as ApiResponse<{ count: number }>
  if (!response.ok) {
    throw new Error(result.message || '导入规则备份失败')
  }

  return result
}

