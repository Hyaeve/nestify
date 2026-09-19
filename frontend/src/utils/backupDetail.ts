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
// 打包与归档用 pack/move，软链硬链用 link；失败与跳过是各链路共用的结论性动作。
export type RunFileAction =
  | 'upload'
  | 'skip'
  | 'fail'
  | 'delete'
  | 'strm'
  | 'metadata'
  | 'pack'
  | 'move'
  | 'link'

// 兼容旧名：明细面板早期只服务备份链路。
export type BackupFileAction = RunFileAction

const runFileActions: RunFileAction[] = ['upload', 'skip', 'fail', 'delete', 'strm', 'metadata', 'pack', 'move', 'link']

function isRunFileAction(value: unknown): value is RunFileAction {
  return typeof value === 'string' && (runFileActions as string[]).includes(value)
}

// 会被列出的动作顺序。这个数组参与「共 N 项」的求和，必须覆盖所有会落明细的动作，
// 顺序本身沿用「先列这次产出了什么，再列失败与删除，最后是跳过」。
export const runFileActionOrder: RunFileAction[] = ['upload', 'strm', 'link', 'metadata', 'pack', 'move', 'fail', 'delete', 'skip']

// 明细面板只列「真的动了文件」的结果。
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
  // 本次执行的源 / 目标根路径（规则里配置的路径）。sourceRoots 用来把明细里的源路径裁成
  // 「根路径下一级」显示；targetRoots 现在**不再参与裁剪**（备份目标改为展示完整路径，
  // 见 detailTargetFolder），仅随载荷保留，旧记录没有这两个字段时源路径退化成整条路径。
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

// detailTargetFolder 取明细里目标路径所在的**完整文件夹路径**（含目标根）。
//
// 备份任务的明细里 Target 是落地文件的完整路径，第二行展示它所在的文件夹：
// 只切掉最后一段文件名，其余**原样保留** —— 目标根也一并显示
// （/media/backup/剧集/A/A1.mkv → /media/backup/剧集/A），一眼能看出文件备份到了哪儿。
// 曾经这里会把目标根裁掉（只留「剧集/A」），但用户要看的是完整路径。
// 目录条目（dir）本身就是文件夹，不再切最后一段；
// 文件直接躺在目标根下时返回目标根本身（/media/backup/A2.mkv → /media/backup）。
// 没有目标（跳过 / 未传到 / 删源）时明细里根本没有 Target，调用方据此退回备注。
export function detailTargetFolder(path: string, isDir = false): string {
  const normalized = (path || '').replace(/\\/g, '/').trim()
  if (!normalized) {
    return ''
  }

  // 目录条目本身就是文件夹：去掉尾部斜杠即可。
  if (isDir) {
    return normalized.replace(/\/+$/, '') || normalized
  }

  const trimmed = normalized.replace(/\/+$/, '')
  const slash = trimmed.lastIndexOf('/')
  if (slash < 0) {
    // 只有文件名、没有分隔符（异常数据）：原样显示，总比空着强。
    return trimmed
  }
  // 分隔符在开头（文件就在根下）时返回根本身：/media/backup/A2.mkv → /media/backup。
  return trimmed.slice(0, slash) || '/'
}

// ---------------------------------------------------------------------------
// 明细要不要占两行：只有「真的产出了东西」的成功条目才值得给第二行
// ---------------------------------------------------------------------------

// 产出型动作：备份上传、strm 生成、软链硬链建链、元数据同步、打包、
// 移动（归档 / 收集 / 转换重命名 / 命名重命名）。
// 跳过 / 失败 / 删除都是**结论性**动作 —— 一行文件路径就够，原因留在悬浮提示里。
// 这也是用户定的口径：两行只留给「备份 / strm / 打包 / 收集 / 转换 / 软链硬链 / 命名」
// 这些模式里**成功**的那一条。
const runDetailTargetActions: RunFileAction[] = ['upload', 'strm', 'link', 'metadata', 'pack', 'move']

export function runFileActionHasTarget(action: RunFileAction): boolean {
  return runDetailTargetActions.includes(action)
}

// detailTargetDisplay 给出第二行要展示的产物落点。
//
// 备份链路展示它落地的**文件夹**（完整路径，含目标根）：文件名与第一行的源文件名一模一样，
// 再写一遍是噪音（见 detailTargetFolder）。
// 其它链路展示**完整目标路径**：产物名本身就是信息 ——
// /media/strm/剧集/A/A1.strm（strm 生成）、/media/links/剧集/A1.mkv（软链硬链建出的链接）、
// /media/backup/剧集/剧集-01.cbz（打包）。
// 转换与命名是原地改名，它们的「目标」就是改完之后的完整新路径。
export function detailTargetDisplay(kind: string | undefined, entry: BackupFileEntry): string {
  const raw = (entry.target || '').replace(/\\/g, '/').trim()
  if (!raw) {
    return ''
  }
  if ((kind || '').trim() === 'backup') {
    return detailTargetFolder(raw, entry.dir === true)
  }
  return raw
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
  link: 0,
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
  cleanup: '清理明细',
  transform: '转换明细',
  naming: '命名明细',
  link: '链路明细',
}

export function runDetailTitle(kind?: string): string {
  return runDetailTitles[(kind || '').trim()] ?? '执行明细'
}

// ---------------------------------------------------------------------------
// 任务详情弹窗标题下的统计项：每一项同时是下面文件明细的筛选入口
// ---------------------------------------------------------------------------

// 统计项标识。成功 / 跳过 / 失败 来自运行记录本身（run_history 的计数），
// 删除 / Strm / 元数据 来自明细载荷的动作构成（「删除」只在真有删除时出现）。
//
// 「删除」这个统计项不专属某条链路：备份的删源、strm 链路的级联删除、净化规则删掉的
// 文件与文件夹都会落到删除动作上，所以任何链路只要真的删了东西就会多出这一项。
export type RunDetailSummaryKey = 'success' | 'skip' | 'failure' | 'delete' | 'strm' | 'metadata'

export const runDetailSummaryLabels: Record<RunDetailSummaryKey, string> = {
  success: '成功',
  skip: '跳过',
  failure: '失败',
  delete: '删除',
  strm: 'Strm',
  metadata: '元数据',
}

// 统计项 → 明细动作。点「成功」不该把失败条目带出来，点「Strm」只看真的生成了 strm 的那些。
// 「删除」是独立动作：单独给一个筛选口，删了哪些文件点它就能看到。
const runDetailSummaryActions: Record<RunDetailSummaryKey, RunFileAction[]> = {
  success: ['upload', 'strm', 'link', 'metadata', 'pack', 'move'],
  skip: ['skip'],
  failure: ['fail'],
  delete: ['delete'],
  strm: ['strm'],
  metadata: ['metadata'],
}

export interface RunDetailSummarySegment {
  key: RunDetailSummaryKey
  label: string
  value: number
}

// buildRunDetailSummarySegments 拼出标题下展示（且可点击筛选）的统计项。
// 「Strm / 元数据」只有 strm 链路才区分，其它链路不占篇幅；
// 「删除」只在本次执行真的删过东西时出现（没删除的历史记录一个像素都不变）。
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
  const deleted = safe(manifest?.counts.delete)
  if (deleted > 0) {
    segments.push({ key: 'delete', label: runDetailSummaryLabels.delete, value: deleted })
  }
  if (manifest && manifest.kind.trim() === 'strm') {
    segments.push({ key: 'strm', label: runDetailSummaryLabels.strm, value: manifest.counts.strm })
    segments.push({ key: 'metadata', label: runDetailSummaryLabels.metadata, value: manifest.counts.metadata })
  }
  return segments
}

// filterRunDetailFiles 按统计项筛明细；未选中任何项（null）时返回整份明细。
//
// 「跳过」也有逐条明细，点它就只看被跳过的那些文件。后端不再限制每动作条数
// （轮 96），所以筛出来的条数与统计数字一致。
//
// kind 只在净化链路（cleanup）上用一下：净化规则干的活就是删除，后端也把删除计进
// success_count，所以它的「成功」要连删除条目一起筛出来 —— 否则「成功 N」点开是空的，
// 而下面明明列着 N 条删除。其它链路的删除（备份删源、strm 级联删除）不算成功，
// 不能带进来。
export function filterRunDetailFiles(
  files: BackupFileEntry[],
  key: RunDetailSummaryKey | null,
  kind?: string,
): BackupFileEntry[] {
  if (!key) {
    return files
  }
  const actions = runDetailSummaryActions[key]
  if (!actions.length) {
    return []
  }
  if (key === 'success' && (kind || '').trim() === 'cleanup') {
    return files.filter((file) => actions.includes(file.action) || file.action === 'delete')
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
    // 「跳过」同样逐条列出：任务详情要能看出「到底是哪些文件被跳过了」，
    // 只有统计数字不够（旧记录里也可能存过这类条目，一并展示）。
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

  // 「共 N 项」按 counts 求和（含跳过，跳过也逐条列出）。轮 96 起不再限制每动作条数，
  // counts 与明细条数恒等。
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
      return '删除'
    case 'strm':
      return '生成 Strm'
    case 'link':
      return '创建链路'
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
    case 'link':
      return 'is-link'
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
