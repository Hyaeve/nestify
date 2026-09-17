// 备份任务的触发方式措辞与其它规则区分：实时监控 / 计划扫描 / 手动。
export function backupTriggerLabel(mode?: string): string {
  switch (mode) {
    case 'watch':
      return '实时监控触发'
    case 'cron':
      return '计划扫描触发'
    case 'once':
      return '启动后自动执行'
    default:
      return '手动执行'
  }
}

// ---------------------------------------------------------------------------
// 备份文件明细（run_history.detail_json）
// ---------------------------------------------------------------------------

// 明细动作：备份链路用 upload/skip/fail/delete，strm 链路用 strm/metadata，
// 打包与归档用 pack/move；失败与跳过是各链路共用的结论性动作。
export type RunFileAction =
  | 'upload'
  | 'skip'
  | 'fail'
  | 'delete'
  | 'strm'
  | 'metadata'
  | 'pack'
  | 'move'

// 兼容旧名：明细面板早期只服务备份链路。
export type BackupFileAction = RunFileAction

const runFileActions: RunFileAction[] = ['upload', 'skip', 'fail', 'delete', 'strm', 'metadata', 'pack', 'move']

function isRunFileAction(value: unknown): value is RunFileAction {
  return typeof value === 'string' && (runFileActions as string[]).includes(value)
}

// 明细面板的筛选页签顺序：先列「这次产出了什么」，再列失败与删除。
export const runFileActionOrder: RunFileAction[] = ['upload', 'strm', 'metadata', 'pack', 'move', 'fail', 'delete']

// 明细面板只列「真的动了文件」的结果；跳过只显示数目，不参与筛选。
export type BackupFileFilter = 'all' | RunFileAction

export interface BackupFileEntry {
  path: string
  action: RunFileAction
  size?: number
  target?: string
  note?: string
  dir?: boolean
}

export interface BackupFileManifest {
  kind: string
  files: BackupFileEntry[]
  counts: Record<RunFileAction, number>
  total: number
  truncated: boolean
  // 本次执行的源 / 目标根路径（规则里配置的路径），用来把明细里的绝对路径裁成
  // 「根路径下一级」显示；旧记录没有这两个字段，退化成整条路径。
  sourceRoots: string[]
  targetRoots: string[]
}

// ---------------------------------------------------------------------------
// 路径裁剪：明细里存的是绝对路径，直接铺在两列里会被挤成几个字
// ---------------------------------------------------------------------------

// stripDetailRoot 把绝对路径裁成「根路径下一级开始的相对路径」。
//
// 规则：
//   - 多个根时取**最长**匹配的那一个（多源 / 多目标的根可能互相是前缀）；
//   - 必须落在路径分隔符上才算命中，否则 /media/strm2 会被 /media/strm 裁成 "2"；
//   - 裁完为空（路径正好等于根，例如 strm 的元数据汇总行记的就是源目录本身）时保留原路径，
//     否则那一格会变成空白，反而看不出指的是哪儿。
export function stripDetailRoot(path: string, roots: string[]): string {
  const normalized = (path || '').replace(/\\/g, '/')
  if (!normalized) {
    return ''
  }

  let matched = ''
  for (const root of roots) {
    if (!root || !normalized.startsWith(root)) {
      continue
    }
    const rest = normalized.slice(root.length)
    if (rest !== '' && !rest.startsWith('/')) {
      continue
    }
    if (root.length > matched.length) {
      matched = root
    }
  }
  if (!matched) {
    return normalized
  }

  const rest = normalized.slice(matched.length).replace(/^\/+/, '')
  return rest || normalized
}

// normalizeDetailRoots 把载荷里的根路径整理成前端可用的形式：去尾部斜杠、去重复、丢空值。
function normalizeDetailRoots(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  const roots: string[] = []
  for (const candidate of value) {
    if (typeof candidate !== 'string') {
      continue
    }
    const root = candidate.replace(/\\/g, '/').trim().replace(/\/+$/, '')
    if (!root || roots.includes(root)) {
      continue
    }
    roots.push(root)
  }
  return roots
}

const emptyActionCounts = (): Record<RunFileAction, number> => ({
  upload: 0,
  skip: 0,
  fail: 0,
  delete: 0,
  strm: 0,
  metadata: 0,
  pack: 0,
  move: 0,
})

function normalizeAction(value: unknown): RunFileAction {
  return isRunFileAction(value) ? value : 'upload'
}

// 面板标题按链路类型变化：同一份载荷结构被备份、strm、打包等链路共用。
const runDetailTitles: Record<string, string> = {
  backup: '备份文件',
  strm: 'Strm 与元数据',
  package: '打包产出',
  collect: '收集明细',
  archive: '归档明细',
}

export function runDetailTitle(kind?: string): string {
  return runDetailTitles[(kind || '').trim()] ?? '执行明细'
}

// ---------------------------------------------------------------------------
// 任务详情弹窗标题下的统计项：每一项同时是下面文件明细的筛选入口
// ---------------------------------------------------------------------------

// 统计项标识。成功 / 跳过 / 错误 来自运行记录本身（run_history 的计数），
// Strm / 元数据 来自明细载荷的动作构成。
export type RunDetailSummaryKey = 'success' | 'skip' | 'failure' | 'strm' | 'metadata'

export const runDetailSummaryLabels: Record<RunDetailSummaryKey, string> = {
  success: '成功',
  skip: '跳过',
  failure: '错误',
  strm: 'Strm',
  metadata: '元数据',
}

// 统计项 → 明细动作。点「成功」不该把失败条目带出来，点「Strm」只看真的生成了 strm 的那些。
// 「已删除」不属于任何统计项：它是收尾动作，既不计入成功数、也没有对应的数字可点，
// 只在「全部」视图里出现。
const runDetailSummaryActions: Record<RunDetailSummaryKey, RunFileAction[]> = {
  success: ['upload', 'strm', 'metadata', 'pack', 'move'],
  skip: [],
  failure: ['fail'],
  strm: ['strm'],
  metadata: ['metadata'],
}

export interface RunDetailSummarySegment {
  key: RunDetailSummaryKey
  label: string
  value: number
}

// buildRunDetailSummarySegments 拼出标题下展示（且可点击筛选）的统计项。
// 「Strm / 元数据」只有 strm 链路才区分，其它链路不占篇幅。
export function buildRunDetailSummarySegments(
  counts: { success_count?: number; skip_count?: number; failure_count?: number },
  manifest: BackupFileManifest | null,
): RunDetailSummarySegment[] {
  const safe = (value?: number) => Math.max(0, Number(value || 0))
  const segments: RunDetailSummarySegment[] = [
    { key: 'success', label: runDetailSummaryLabels.success, value: safe(counts.success_count) },
    { key: 'skip', label: runDetailSummaryLabels.skip, value: safe(counts.skip_count) },
    { key: 'failure', label: runDetailSummaryLabels.failure, value: safe(counts.failure_count) },
  ]
  if (manifest && manifest.kind.trim() === 'strm') {
    segments.push({ key: 'strm', label: runDetailSummaryLabels.strm, value: manifest.counts.strm })
    segments.push({ key: 'metadata', label: runDetailSummaryLabels.metadata, value: manifest.counts.metadata })
  }
  return segments
}

// filterRunDetailFiles 按统计项筛明细；未选中任何项（null）时返回整份明细。
//
// 「跳过」在明细载荷里只累计数量、不产生逐条明细，所以筛出来必然是空 ——
// 调用方据此给一句说明文案（见 RunDetailList 的空态），不要让它看起来像「坏了」。
export function filterRunDetailFiles(
  files: BackupFileEntry[],
  key: RunDetailSummaryKey | null,
): BackupFileEntry[] {
  if (!key) {
    return files
  }
  const actions = runDetailSummaryActions[key]
  if (!actions.length) {
    return []
  }
  return files.filter((file) => actions.includes(file.action))
}

// 解析运行明细载荷；无明细（历史记录或该链路尚未采集）时返回 null，调用方据此隐藏面板。
export function parseRunDetail(item?: { archive_mode?: string; detail_json?: string }): BackupFileManifest | null {
  const raw = item?.detail_json?.trim()
  if (!raw) {
    return null
  }

  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return null
  }
  if (!parsed || typeof parsed !== 'object') {
    return null
  }

  const payload = parsed as {
    kind?: unknown
    files?: unknown
    counts?: unknown
    files_total?: unknown
    files_truncated?: unknown
    source_roots?: unknown
    target_roots?: unknown
  }
  const kind = typeof payload.kind === 'string' ? payload.kind.trim() : ''
  const rawFiles = Array.isArray(payload.files) ? payload.files : []
  const files: BackupFileEntry[] = []

  for (const candidate of rawFiles) {
    if (!candidate || typeof candidate !== 'object') {
      continue
    }
    const entry = candidate as Record<string, unknown>
    const path = typeof entry.path === 'string' ? entry.path.trim() : ''
    if (!path) {
      continue
    }
    const action = normalizeAction(entry.action)
    // 「跳过」不列明细（旧记录里可能存过这类条目，这里一并丢掉），只用 counts.skip 显示数目。
    if (action === 'skip') {
      continue
    }
    files.push({
      path,
      action,
      size: typeof entry.size === 'number' ? entry.size : undefined,
      target: typeof entry.target === 'string' ? entry.target : undefined,
      note: typeof entry.note === 'string' ? entry.note : undefined,
      dir: entry.dir === true,
    })
  }

  const counts = emptyActionCounts()
  const rawCounts = payload.counts
  if (rawCounts && typeof rawCounts === 'object') {
    for (const [key, value] of Object.entries(rawCounts as Record<string, unknown>)) {
      // 认不出的动作直接忽略，避免被归到「已上传」污染页签。
      if (typeof value !== 'number' || value <= 0 || !isRunFileAction(key)) {
        continue
      }
      counts[key] += value
    }
  } else {
    for (const file of files) {
      counts[file.action] += 1
    }
  }

  // 全部是「跳过」的执行也要保留（面板要显示跳过数目），彻底没有明细时才返回 null。
  if (files.length === 0 && counts.skip === 0) {
    return null
  }

  // 「共 N 项」只算会被列出的动作：旧记录的 files_total 含跳过，不能直接用。
  const listedTotal = runFileActionOrder.reduce((total, action) => total + counts[action], 0)
  const total = listedTotal > 0 ? listedTotal : files.length

  return {
    kind,
    files,
    counts,
    total,
    truncated: payload.files_truncated === true,
    sourceRoots: normalizeDetailRoots(payload.source_roots),
    targetRoots: normalizeDetailRoots(payload.target_roots),
  }
}

export function backupFileActionLabel(action: RunFileAction): string {
  switch (action) {
    case 'skip':
      return '跳过'
    case 'fail':
      return '失败'
    case 'delete':
      return '已删除'
    case 'strm':
      return '生成 Strm'
    case 'metadata':
      return '同步元数据'
    case 'pack':
      return '已打包'
    case 'move':
      return '已移动'
    default:
      return '已上传'
  }
}

export function backupFileActionClass(action: RunFileAction): string {
  switch (action) {
    case 'skip':
      return 'is-skip'
    case 'fail':
      return 'is-fail'
    case 'delete':
      return 'is-delete'
    case 'strm':
      return 'is-strm'
    case 'metadata':
      return 'is-metadata'
    case 'pack':
      return 'is-pack'
    case 'move':
      return 'is-move'
    default:
      return 'is-upload'
  }
}

export function formatBackupFileSize(bytes?: number): string {
  const value = Number(bytes || 0)
  if (!Number.isFinite(value) || value <= 0) {
    return '—'
  }

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }

  const digits = size >= 100 || unitIndex === 0 ? 0 : 1
  return `${size.toFixed(digits)} ${units[unitIndex]}`
}
