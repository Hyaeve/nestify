export interface RunHistoryItem {
  id: string
  rule_id?: number
  rule_name?: string
  trigger_mode: string
  archive_mode?: string
  link_mode?: 'soft' | 'hard'
  status: string
  processed_files: number
  success_count: number
  skip_count: number
  failure_count: number
  size_bytes: number
  summary: string
  started_at: string
  updated_at?: string
  finished_at?: string
}

import { deleteJSON, getJSON } from './http'

export type RunHistoryStatus = 'success' | 'failed' | 'skip'
export type RunHistoryArchiveMode = 'package' | 'collect' | 'cleanup'

export interface RunHistorySummary {
  total: number
  today: number
  success: number
  failed: number
  skipped: number
}

export interface RunHistoryPayload {
  items: RunHistoryItem[]
  total: number
  page: number
  page_size: number
  summary?: RunHistorySummary
}

export interface FetchRunHistoryParams {
  page?: number
  page_size?: number
  keyword?: string
  status?: RunHistoryStatus
  archive_mode?: RunHistoryArchiveMode
  rule_type?: 'archive' | 'cleanup' | 'link'
  sort_by?: 'name' | 'modified_at'
  sort_order?: 'asc' | 'desc'
  view_mode?: 'flat' | 'tree'
}

function buildRunHistoryURL(params: FetchRunHistoryParams = {}) {
  const query = new URLSearchParams()

  if (typeof params.page === 'number' && params.page > 0) {
    query.set('page', String(params.page))
  }

  if (typeof params.page_size === 'number' && params.page_size > 0) {
    query.set('page_size', String(params.page_size))
  }

  if (params.keyword?.trim()) {
    query.set('keyword', params.keyword.trim())
  }

  if (params.status) {
    query.set('status', params.status)
  }

  if (params.archive_mode) {
    query.set('archive_mode', params.archive_mode)
  }

  if (params.rule_type) {
    query.set('rule_type', params.rule_type)
  }

  if (params.sort_by) {
    query.set('sort_by', params.sort_by)
  }

  if (params.sort_order) {
    query.set('sort_order', params.sort_order)
  }

  if (params.view_mode) {
    query.set('view_mode', params.view_mode)
  }

  const queryString = query.toString()
  return queryString ? `/api/v1/run-history?${queryString}` : '/api/v1/run-history'
}

export function fetchRunHistory(params: FetchRunHistoryParams = {}) {
  return getJSON<RunHistoryPayload>(buildRunHistoryURL(params))
}

export function clearRunHistory(status?: RunHistoryStatus) {
  const query = new URLSearchParams()
  if (status) {
    query.set('status', status)
  }

  const queryString = query.toString()
  const url = queryString ? `/api/v1/run-history?${queryString}` : '/api/v1/run-history'
  return deleteJSON<null>(url)
}

export function deleteRunHistoryItem(id: string) {
  return deleteJSON<null>(`/api/v1/run-history?id=${encodeURIComponent(id)}`)
}

export function emptyRunHistory(): RunHistoryItem[] {
  return []
}
