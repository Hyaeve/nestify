import { deleteJSON, getJSON } from './http'

export interface RuleItem {
  id: number
  name: string
  description: string
  enabled: boolean
  monitor_enabled: boolean
  compatibility_mode: 'local' | 'compatibility'
  archive_mode: 'package' | 'collect' | 'cleanup'
  run_mode: 'watch' | 'cron' | 'once'
  source_dir: string
  target_dir: string
  watch_debounce_ms: number
  cron_expression: string
  run_on_start: boolean
  options_json?: string
  package_options_json?: string
  collect_options_json?: string
  filters_json?: string
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

export interface CreateRulePayload {
  name: string
  description: string
  enabled: boolean
  monitor_enabled: boolean
  compatibility_mode: 'local' | 'compatibility'
  archive_mode: 'package' | 'collect' | 'cleanup'
  run_mode: 'watch' | 'cron' | 'once'
  source_dir: string
  target_dir?: string
  watch_debounce_ms?: number
  cron_expression?: string
  run_on_start?: boolean
  options?: Record<string, boolean>
  package_options?: Record<string, boolean>
  collect_options?: Record<string, boolean>
  filters?: string[]
}

export interface UpdateRulePayload extends CreateRulePayload {}

export function fetchRules() {
  return getJSON<RulesListPayload>('/api/v1/rules')
}

export function fetchRule(id: number) {
  return getJSON<RuleItem>(`/api/v1/rules/${id}`)
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

export function deleteRule(id: number) {
	return deleteJSON<Record<string, never>>(`/api/v1/rules/${id}`)
}
