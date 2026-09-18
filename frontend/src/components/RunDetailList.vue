<template>
  <div class="run-detail">
    <!-- 文件级明细：一行一条（悬浮看完整路径与备注）。
         备份任务改成两行——第一行源路径，第二行箭头 + 落地的目标文件夹（见 backupMode）。

         面板**常驻**：没有明细时只显示空态，整块不隐藏——任务详情要能一眼看出
         「这次确实没动文件」，无论这条记录是成功、失败还是跳过。

         分页：一次执行动辄成百上千个文件，整份铺出来只能靠滚动条硬翻。
         一页条数**跟随系统设置里的「每页文件数」**（与运行日志 / 文件管理同一处），
         底部只留翻页与总数，不给独立的页大小下拉——条数统一由设置控制。 -->
    <div class="run-detail__body">
      <el-table
        :data="pagedRows"
        class="run-detail__table"
        size="small"
        height="100%"
      >
        <template #empty>
          <div class="run-detail__empty">{{ emptyText }}</div>
        </template>

        <el-table-column :label="sourceColumnLabel" min-width="360">
          <template #default="scope">
            <!-- 悬浮提示：在**条目下方**弹出，鼠标停够 0.5s 才出现（扫过整列时不会一路弹），
                 移开立刻收回（hide-after=0，EP 默认还要再等 200ms）。enterable=false 让指针一旦
                 离开条目就关闭——只做「看一眼完整路径」的用途，不需要能点进提示框里。 -->
            <el-tooltip
              placement="bottom"
              effect="light"
              popper-class="run-detail-tip"
              :show-after="500"
              :hide-after="0"
              :enterable="false"
              :offset="6"
            >
              <template #content>
                <div class="run-detail-tip__path">{{ scope.row.entry.path }}</div>
                <div v-if="scope.row.entry.target" class="run-detail-tip__target">→ {{ scope.row.entry.target }}</div>
                <div v-if="scope.row.entry.note" class="run-detail-tip__note">{{ scope.row.entry.note }}</div>
              </template>
              <span class="run-detail__cell">
                <span class="run-detail__line">
                  <el-icon class="run-detail__icon">
                    <Folder v-if="scope.row.entry.dir" />
                    <Document v-else />
                  </el-icon>
                  <span class="run-detail__path">{{ scope.row.source }}</span>
                </span>
                <!-- 第二行只在备份任务、且有内容时出现：优先「落地的目标文件夹」，
                     没有目标（跳过 / 未传到）就退回备注（跳过原因等）。 -->
                <span v-if="backupMode && scope.row.secondary" class="run-detail__line">
                  <span class="run-detail__arrow" aria-hidden="true">↳</span>
                  <span class="run-detail__target" :class="{ 'is-note': !scope.row.target }">{{ scope.row.secondary }}</span>
                </span>
              </span>
            </el-tooltip>
          </template>
        </el-table-column>

        <el-table-column label="结果" width="126" align="center">
          <template #default="scope">
            <span class="run-detail__action" :class="actionClass(scope.row.action)">
              {{ actionLabel(scope.row.action) }}
            </span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 只要有明细就出这一条：总数让「这次到底动了几个文件」一目了然，
         页数少了也只是「共 N 条」一行，不会白占高度。 -->
    <div v-if="rows.length > 0" class="run-detail__pager">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="rows.length"
        background
        layout="total, prev, pager, next"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Document, Folder } from '@element-plus/icons-vue'

import {
  backupFileActionClass,
  backupFileActionLabel,
  detailTargetFolder,
  filterRunDetailFiles,
  runDetailSummaryLabels,
  stripDetailRoot,
  type BackupFileEntry,
  type BackupFileManifest,
  type RunDetailSummaryKey,
  type RunFileAction,
} from '../utils/backupDetail'
import { defaultPageSize, useSettingsStore } from '../stores/settings'

const props = defineProps<{
  manifest: BackupFileManifest | null
  // 顶部统计栏选中的筛选项：null 表示不筛选，列出全部条目。
  filterKey?: RunDetailSummaryKey | null
}>()

const settingsStore = useSettingsStore()

// 备份任务的明细是「备份操作」：第一行源路径，第二行箭头 + 目标端落地的文件夹。
// 其它链路仍是「源路径 + 结果」，目标路径只在悬浮提示里给（它们的产物是 .strm / 压缩包，
// 没有「备份到哪个文件夹」这层语义）。
const backupMode = computed(() => (props.manifest?.kind || '').trim() === 'backup')
const sourceColumnLabel = computed(() => (backupMode.value ? '备份操作' : '源路径'))

// 一页条数跟随系统设置（基础设置窗口的「每页文件数」）；未加载出设置时按默认值兜底。
const pageSize = computed(() => {
  const size = Number(settingsStore.pageSize)
  return Number.isFinite(size) && size > 0 ? size : defaultPageSize
})

interface RunDetailRow {
  entry: BackupFileEntry
  action: RunFileAction
  // 源路径裁掉源根后的显示值（备份链路记的本来就是相对路径，裁剪不命中即原样）。
  source: string
  // 目标端「落地的文件夹」，已裁掉目标根；没有目标（跳过 / 未传到）时为空。
  target: string
  // 第二行真正显示的内容：优先目标文件夹，没有就退回备注。
  secondary: string
}

// 当前筛选下的**全部**条目（未分页）——总数与翻页都基于它。
const rows = computed<RunDetailRow[]>(() => {
  const manifest = props.manifest
  const sourceRoots = manifest?.sourceRoots ?? []
  const targetRoots = manifest?.targetRoots ?? []

  // kind 传下去：净化链路的「成功」要把删除条目一并筛出来（见 filterRunDetailFiles）。
  return filterRunDetailFiles(manifest?.files ?? [], props.filterKey ?? null, manifest?.kind).map((entry) => {
    const target = entry.target ? detailTargetFolder(entry.target, targetRoots, entry.dir === true) : ''
    return {
      entry,
      action: entry.action,
      source: stripDetailRoot(entry.path, sourceRoots),
      target,
      secondary: target || entry.note || '',
    }
  })
})

const currentPage = ref(1)

// 表格只渲染当前这一页。
const pagedRows = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return rows.value.slice(start, start + pageSize.value)
})

// 换筛选 / 换记录 / 改「每页文件数」都回到第一页，避免停在已不存在的页码上。
watch(
  [() => props.filterKey, () => props.manifest, pageSize],
  () => {
    currentPage.value = 1
  },
)

// 空态要分清「本来就没有明细」与「筛出来是空的」。
const emptyText = computed(() => {
  const key = props.filterKey
  if (!key) {
    return '本次执行没有文件级明细'
  }
  return `本次执行没有「${runDetailSummaryLabels[key]}」明细`
})

function actionLabel(action: RunFileAction) {
  return backupFileActionLabel(action)
}

function actionClass(action: RunFileAction) {
  return backupFileActionClass(action)
}
</script>

<style scoped>
/* 竖直方向被弹窗正文的 flex 分配高度：自己撑满剩余空间，把余下的高度全给表格。 */
.run-detail {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

/* 表格区：独占除翻页条以外的全部高度。包一层是为了让 el-table 的 height:100%
   有一个高度确定、可伸缩的父级（翻页条是它的兄弟，不能再算进 100%）。 */
.run-detail__body {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
}

/* 表格高度由组件的 height="100%" 写成内联样式，这里再兜一层 flex，保证它是唯一的伸缩项。 */
.run-detail__table {
  flex: 1 1 auto;
  width: 100%;
  min-height: 0;
}

/* 翻页条：固定高度、不参与伸缩，右对齐与运行日志页保持一致。 */
.run-detail__pager {
  display: flex;
  flex: 0 0 auto;
  justify-content: flex-end;
  padding: 10px 4px 2px;
}

/* 一行一条：把单元格的上下内边距压到最小，同样高度里能多铺几行。
   注意有**两层**上下内边距：`td.el-table__cell` 一层，内层 `.cell` 一层；全局
   `styles/index.scss` 的 `.el-table .cell { padding-top/bottom: var(--table-cell-padding-y) }`
   给内层留了 12px，只压外层的话行高仍会停在 58px（明细表要单独清零）。
   上下各留 7px：4px 时条目挤成一片、扫不出行，7px 是「分得清行又不浪费高度」的折中。 */
.run-detail__table :deep(.el-table__cell) {
  padding: 7px 0;
}

.run-detail__table :deep(.cell) {
  padding-top: 0;
  padding-bottom: 0;
}

/* 一个条目 = 一列（备份任务是两行，其它链路只有第一行），行内各自是 flex 横排。 */
.run-detail__cell {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.run-detail__line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.run-detail__icon {
  flex: 0 0 auto;
  color: #6f97a6;
  font-size: 14px;
}

/* 单行省略：路径超宽也不换行，悬浮提示给完整路径。 */
.run-detail__path {
  min-width: 0;
  overflow: hidden;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 第二行行首的箭头：与上一行的图标（14px）同宽，两行文字因此左对齐。 */
.run-detail__arrow {
  flex: 0 0 14px;
  width: 14px;
  color: #6f97a6;
  font-size: 13px;
  line-height: 1;
  text-align: center;
}

/* 目标端落地的文件夹（已裁掉目标根）：备份语义色 + 比源路径小一档。 */
.run-detail__target {
  min-width: 0;
  overflow: hidden;
  font-size: 12px;
  font-weight: 600;
  color: #5f7fa8;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 没有目标路径时第二行退回备注（跳过原因 / 失败原因），用更弱的中性色区分。 */
.run-detail__target.is-note {
  font-weight: 500;
  color: var(--el-text-color-secondary);
}

/* 筛选后没有条目时的说明文案（el-table 的 empty 插槽）。 */
.run-detail__empty {
  padding: 18px 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
}

.run-detail__action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 68px;
  padding: 1px 8px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}

/* 结果配色一律「浅底 + 深字」，与项目其它标签保持一致。 */
.run-detail__action.is-upload {
  color: #2f8f5b;
  background: rgba(47, 143, 91, 0.12);
}

/* Strm 语义色，与链路模式标识一致。 */
.run-detail__action.is-strm {
  color: #2f8f9d;
  background: rgba(47, 143, 157, 0.13);
}

.run-detail__action.is-metadata {
  color: #b07d18;
  background: rgba(176, 125, 24, 0.13);
}

.run-detail__action.is-pack {
  color: #0975b8;
  background: rgba(32, 159, 238, 0.13);
}

.run-detail__action.is-move {
  color: #46708f;
  background: rgba(70, 112, 143, 0.13);
}

.run-detail__action.is-fail {
  color: #c4562f;
  background: rgba(196, 86, 47, 0.12);
}

.run-detail__action.is-delete {
  color: #5f7fa8;
  background: rgba(95, 127, 168, 0.14);
}

.run-detail__action.is-skip {
  color: #8a8f98;
  background: rgba(138, 143, 152, 0.14);
}
</style>
