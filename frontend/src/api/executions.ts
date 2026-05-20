import { getJSON, postJSON } from './http'

export interface RunInstance {
  id: string
  rule_id?: number
  rule_name?: string
  trigger_mode: string
  archive_mode?: string
  status: string
  stage: string
  current_series?: string
  current_volume_or_dir?: string
  processed_files: number
  success_count: number
  skip_count: number
  failure_count: number
  started_at: string
  updated_at: string
  finished_at?: string
}

export interface RunLogEntry {
  id: string
  run_id: string
  level: string
  message: string
  created_at: string
}

export interface PreparedMode {
  archive_mode: string
  source_dir: string
  target_dir: string
  summary: string
}

export interface ManualPreflightResult {
  source_dir: string
  output_dir: string
  allowed: boolean
  image_count: number
  has_nested_dirs: boolean
  has_non_image_files: boolean
  rejected_reasons: string[]
  execution_skeleton: boolean
}

export interface ManualPreflightPayload {
  run: RunInstance
  preflight: ManualPreflightResult
}

export interface PrepareRuleExecutionPayload {
  run: RunInstance
  prepared: PreparedMode
}

export interface RunLogsPayload {
  items: RunLogEntry[]
  total: number
}

export function prepareManualPreflight(sourceDir: string, outputDir?: string) {
  return postJSON<ManualPreflightPayload>('/api/v1/manual/preflight', {
    source_dir: sourceDir,
    output_dir: outputDir || '',
  })
}

export function prepareRuleExecution(ruleID: number, triggerMode = 'once') {
  return postJSON<PrepareRuleExecutionPayload>('/api/v1/executions/prepare-rule', {
    rule_id: ruleID,
    trigger_mode: triggerMode,
  })
}

export function fetchRun(runID: string) {
  return getJSON<RunInstance>(`/api/v1/runs/${runID}`)
}

export function fetchRunLogs(runID: string) {
  return getJSON<RunLogsPayload>(`/api/v1/runs/${runID}/logs`)
}
