export interface RunHistoryItem {
  id: string
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
}

export function emptyRunHistory(): RunHistoryItem[] {
  return []
}
