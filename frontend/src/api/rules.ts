import { deleteJSON, getJSON } from './http'

export interface CronPreviewPayload {
  expression: string
  next_runs: string[]
}

export interface RuleItem {
  id: number
  sort_order: number
  name: string
  description: string
  enabled: boolean
  monitor_enabled: boolean
  compatibility_mode: 'local' | 'compatibility'
  archive_mode: 'package' | 'collect' | 'cleanup' | 'transform' | 'link'
  rule_type?: 'archive' | 'cleanup' | 'link'
  link_mode?: 'soft' | 'hard' | 'strm'
  run_mode: 'watch' | 'cron' | 'once'
  source_dir: string
  target_dir: string
  watch_debounce_ms: number
  cron_expression: string
  run_on_start: boolean
  options_json?: string
  option_values_json?: string
  package_options_json?: string
  collect_options_json?: string
  filters_json?: string
  whitelist_json?: string
  match_filters_json?: string
  nest_filters_json?: string
  transform_rules_json?: string
  transform_filters_json?: string
  last_run_status: string
  last_success_count: number
  last_skip_count: number
  last_failure_count: number
  created_at: string
  updated_at: string
}

export interface RulesListPayload {
  items: RuleItem[]
  total: number
  page: number
  page_size: number
}

export interface FetchRulesParams {
  page?: number
  page_size?: number
  rule_type?: 'archive' | 'cleanup' | 'link'
}

export interface CreateRulePayload {
  name: string
  description: string
  enabled: boolean
  monitor_enabled: boolean
  compatibility_mode: 'local' | 'compatibility'
  archive_mode: 'package' | 'collect' | 'cleanup' | 'transform' | 'link'
  rule_type?: 'archive' | 'cleanup' | 'link'
  link_mode?: 'soft' | 'hard' | 'strm'
  run_mode: 'watch' | 'cron' | 'once'
  source_dir: string
  target_dir?: string
  watch_debounce_ms?: number
  cron_expression?: string
  run_on_start?: boolean
  options?: Record<string, boolean>
  option_values?: Record<string, number>
  package_options?: Record<string, boolean>
  collect_options?: Record<string, boolean>
  filters?: string[]
  whitelist?: string[]
  match_filters?: string[]
  nest_filters?: string[]
  transform_rules?: string[]
  transform_filters?: string[]
}

export interface UpdateRulePayload extends CreateRulePayload {}

export interface RuleReorderItem {
  id: number
  sort_order: number
}

function buildRulesURL(params: FetchRulesParams = {}) {
  const query = new URLSearchParams()

  if (typeof params.page === 'number' && params.page > 0) {
    query.set('page', String(params.page))
  }

  if (typeof params.page_size === 'number' && params.page_size > 0) {
    query.set('page_size', String(params.page_size))
  }

  if (params.rule_type) {
    query.set('rule_type', params.rule_type)
  }

  const queryString = query.toString()
  return queryString ? `/api/v1/rules?${queryString}` : '/api/v1/rules'
}

export function fetchRules(params: FetchRulesParams = {}) {
  return getJSON<RulesListPayload>(buildRulesURL(params))
}

export function fetchRule(id: number) {
  return getJSON<RuleItem>(`/api/v1/rules/${id}`)
}

export function fetchCronPreview(expression: string) {
  return getJSON<CronPreviewPayload>(`/api/v1/rules/cron-preview?expression=${encodeURIComponent(expression)}`)
}

export function createRule(payload: CreateRulePayload) {
	return fetch('/api/v1/rules', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			Accept: 'application/json',
		},
		credentials: 'include',
		body: JSON.stringify(payload),
	}).then(async (response) => {
    const data = await response.json()
    if (!response.ok) {
      throw new Error(data.message || 'Create rule failed')
    }
    return data
  })
}

export function updateRule(id: number, payload: UpdateRulePayload) {
	return fetch(`/api/v1/rules/${id}`, {
		method: 'PUT',
		headers: {
			'Content-Type': 'application/json',
			Accept: 'application/json',
		},
		credentials: 'include',
		body: JSON.stringify(payload),
	}).then(async (response) => {
    const data = await response.json()
    if (!response.ok) {
      throw new Error(data.message || 'Update rule failed')
    }
    return data
	})
}

export function reorderRules(payload: RuleReorderItem[]) {
	return fetch('/api/v1/rules/reorder', {
		method: 'PUT',
		headers: {
			'Content-Type': 'application/json',
			Accept: 'application/json',
		},
		credentials: 'include',
		body: JSON.stringify(payload),
	}).then(async (response) => {
		const data = await response.json()
		if (!response.ok) {
			throw new Error(data.message || 'Reorder rules failed')
		}
		return data
	})
}

export function deleteRule(id: number) {
	return deleteJSON<Record<string, never>>(`/api/v1/rules/${id}`)
}
