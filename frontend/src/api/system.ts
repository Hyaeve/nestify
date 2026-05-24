import { getJSON } from './http'
import type { ApiResponse } from './http'

export interface HealthPayload {
  service: string
  time: string
}

export interface SystemResourcePayload {
  cpu_usage: number
  memory_usage: number
  nestify_memory: string
  uptime: string
}

export function fetchHealth() {
  return getJSON<HealthPayload>('/api/v1/health')
}

export function fetchSystemResource() {
  return getJSON<SystemResourcePayload>('/api/v1/system/resource')
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

