export interface RunHistoryItem {
  id: string
  rule_id?: number
  rule_name?: string
  trigger_mode: string
  archive_mode?: string
  status: string
  processed_files: number
  success_count: number
  skip_count: number
  failure_count: number
  summary: string
  started_at: string
  updated_at?: string
  finished_at?: string
}

import { deleteJSON, getJSON } from './http'

export interface RunHistoryPayload {
  items: RunHistoryItem[]
  total: number
}

export function fetchRunHistory() {
  return getJSON<RunHistoryPayload>('/api/v1/run-history')
}

export function clearRunHistory() {
  return deleteJSON<null>('/api/v1/run-history')
}

export function emptyRunHistory(): RunHistoryItem[] {
  return []
}
