const summaryPatterns: Array<{ pattern: RegExp; format: (...matches: string[]) => string }> = [
  {
    pattern: /^no transform actions enabled$/,
    format: () => '未启用转换操作',
  },
  {
    pattern: /^convert_matching_text enabled but no valid transform rules provided$/,
    format: () => '已启用匹配转换，但未提供有效转换规则',
  },
  {
    pattern: /^filter_matching_text enabled but no valid transform filters provided$/,
    format: () => '已启用匹配过滤，但未提供有效过滤规则',
  },
  {
    pattern: /^no cleanup actions enabled$/,
    format: () => '未启用清理操作',
  },
  {
    pattern: /^cleanup_expired_files enabled but no valid retention days provided$/,
    format: () => '已启用过期清除，但未提供有效保留天数',
  },
  {
    pattern: /^cleanup_matching_files enabled but no valid matchers provided$/,
    format: () => '已启用匹配清理，但未提供有效匹配规则',
  },
  {
    pattern: /^执行失败：(.+)$/,
    format: (reason) => `执行失败：${reason}`,
  },
  {
    pattern: /^read transform directory (.+) failed: (.+)$/,
    format: (directory, reason) => `读取转换目录失败：${directory}；原因：${reason}`,
  },
  {
    pattern: /^renamed directory (.+) -> (.+)$/,
    format: (source, target) => `已重命名文件夹：${source} → ${target}`,
  },
  {
    pattern: /^renamed file (.+) -> (.+)$/,
    format: (source, target) => `已重命名文件：${source} → ${target}`,
  },
  {
    pattern: /^merged directory (.+) -> (.+)$/,
    format: (source, target) => `已合并文件夹：${source} → ${target}`,
  },
  {
    pattern: /^merge directory (.+) into (.+) failed: (.+)$/,
    format: (source, target, reason) => `合并文件夹失败：${source} → ${target}；原因：${reason}`,
  },
  {
    pattern: /^rename target already exists (.+)$/,
    format: (target) => `重命名目标已存在：${target}`,
  },
  {
    pattern: /^rename (.+) failed: (.+)$/,
    format: (source, reason) => `重命名失败：${source}；原因：${reason}`,
  },
  {
    pattern: /^read cleanup directory (.+) failed: (.+)$/,
    format: (directory, reason) => `读取清理目录失败：${directory}；原因：${reason}`,
  },
  {
    pattern: /^remove matched directory (.+) failed: (.+)$/,
    format: (directory, reason) => `删除匹配目录失败：${directory}；原因：${reason}`,
  },
  {
    pattern: /^remove empty directory (.+) failed: (.+)$/,
    format: (directory, reason) => `删除空目录失败：${directory}；原因：${reason}`,
  },
  {
    pattern: /^remove file (.+) failed: (.+)$/,
    format: (file, reason) => `删除文件失败：${file}；原因：${reason}`,
  },
  {
    pattern: /^remove expired file (.+) failed: (.+)$/,
    format: (file, reason) => `删除过期文件失败：${file}；原因：${reason}`,
  },
  {
    pattern: /^trash filtered archive directory (.+) failed: (.+)$/,
    format: (directory, reason) => `移入回收区失败（过滤归档目录）：${directory}；原因：${reason}`,
  },
  {
    pattern: /^trash filtered archive file (.+) failed: (.+)$/,
    format: (file, reason) => `移入回收区失败（过滤归档文件）：${file}；原因：${reason}`,
  },
  {
    pattern: /^move matched file (.+) failed: (.+)$/,
    format: (file, reason) => `移动匹配文件失败：${file}；原因：${reason}`,
  },
  {
    pattern: /^nest matched file (.+) failed: (.+)$/,
    format: (file, reason) => `单件归巢失败：${file}；原因：${reason}`,
  },
  {
    pattern: /^process series (.+) failed: (.+)$/,
    format: (series, reason) => `处理系列失败：${series}；原因：${reason}`,
  },
  {
    pattern: /^left non-image file in source directory (.+): package mode only moves matched custom archive targets$/,
    format: (file) => `已保留源目录中的非图片文件：${file}；打包模式仅移动命中的自定义归档目标`,
  },
  {
    pattern: /^move file (.+) failed: (.+)$/,
    format: (file, reason) => `移动文件失败：${file}；原因：${reason}`,
  },
  {
    pattern: /^pack root images (.+) failed: (.+)$/,
    format: (directory, reason) => `打包根目录图片失败：${directory}；原因：${reason}`,
  },
  {
    pattern: /^packed root images (.+) -> (.+)$/,
    format: (source, target) => `已打包根目录图片：${source} → ${target}`,
  },
  {
    pattern: /^skipped empty series (.+)$/,
    format: (series) => `已跳过空系列目录：${series}`,
  },
  {
    pattern: /^skipped nested directory (.+): recursive_collect disabled$/,
    format: (directory) => `已跳过嵌套目录：${directory}；未启用递归收集`,
  },
  {
    pattern: /^process volume (.+) failed: (.+)$/,
    format: (volume, reason) => `处理分卷失败：${volume}；原因：${reason}`,
  },
  {
    pattern: /^move series file (.+) failed: (.+)$/,
    format: (file, reason) => `移动系列文件失败：${file}；原因：${reason}`,
  },
  {
    pattern: /^skipped nested directory (.+)$/,
    format: (directory) => `已跳过嵌套目录：${directory}`,
  },
  {
    pattern: /^process nested directory (.+) failed: (.+)$/,
    format: (directory, reason) => `处理嵌套目录失败：${directory}；原因：${reason}`,
  },
  {
    pattern: /^move nested file (.+) failed: (.+)$/,
    format: (file, reason) => `移动嵌套文件失败：${file}；原因：${reason}`,
  },
  {
    pattern: /^skipped duplicate file (.+)$/,
    format: (file) => `已跳过重复文件：${file}`,
  },
  {
    pattern: /^trashed filtered archive (directory|file) (.+) -> (.+) \(original path: (.+)\)$/,
    format: (type, source, target) => `已移入回收区（过滤归档${type === 'directory' ? '目录' : '文件'}）：${source} → ${target}`,
  },
  {
    pattern: /^packed series (.+) -> (.+)$/,
    format: (source, target) => `已打包系列：${source} → ${target}`,
  },
  {
    pattern: /^packed volume (.+) -> (.+)$/,
    format: (source, target) => `已打包分卷：${source} → ${target}`,
  },
]

const fallbackReplacements: Array<[RegExp, string]> = [
  [/\brecursive_collect disabled\b/g, '未启用递归收集'],
  [/\bpackage_nested_folders disabled\b/g, '未启用嵌套打包'],
  [/\bsame file already exists\b/g, '相同文件已存在'],
]

export function formatRunHistorySummary(summary?: string) {
  if (!summary?.trim()) {
    return ''
  }

  const text = summary.trim()
  for (const item of summaryPatterns) {
    const match = text.match(item.pattern)
    if (match) {
      return item.format(...match.slice(1))
    }
  }

  return fallbackReplacements.reduce((result, [pattern, replacement]) => result.replace(pattern, replacement), text)
}
