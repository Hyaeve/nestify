import { getJSON } from './http'

export interface HealthPayload {
  service: string
  time: string
}

export function fetchHealth() {
  return getJSON<HealthPayload>('/api/v1/health')
}

