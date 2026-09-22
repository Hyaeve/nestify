<template>
  <div class="run-detail">
    <!-- 文件级明细：一行一条（悬浮看完整路径与备注）。
         **产出型成功条目占两行**：第一行源路径，第二行箭头 + 产物落点
         （备份 = 落地的文件夹、完整路径含目标根；strm / 软链硬链 / 打包 = 完整目标路径）。
         跳过 / 失败 / 删除**只有一行**——一行文件路径就够，原因在悬浮提示里。

         **转换 / 命名例外**：这两条链路是原地改名，源与目标只差最后一段，整条路径铺出来
         全是重复的噪音，所以两行都只给名字（第一行原名字、第二行新名，文件夹条目同理），
         列名也跟着叫「名称」；完整路径仍然留在悬浮提示里。

         面板**常驻**：没有明细时只显示空态，整块不隐藏——任务详情要能一眼看出
         「这次确实没动文件」，无论这条记录是成功、失败还是跳过。

         **虚拟滚动（轮 114）**：一次执行动辄成百上千个文件，原来靠「每页文件数」翻页硬翻；
         现在整份条目交给 VirtualList，只渲染可视区那几十行，一次滚到底看完，
         不再有页码（数据本来就整份在 manifest 里，翻页只是为了少建 DOM）。 -->
    <div class="run-detail__head">
      <span class="run-detail__head-cell">{{ sourceColumnLabel }}</span>
      <span class="run-detail__head-cell run-detail__head-cell--action">结果</span>
    </div>

    <div class="run-detail__body">
      <VirtualList
        v-if="rows.length > 0"
        class="run-detail__list"
        :items="rows"
        :item-size="rowHeight"
        :item-key="rowKey"
        :reset-key="filterKey ?? 'all'"
      >
        <template #default="{ item }">
          <div class="run-detail__item">
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
                <div class="run-detail-tip__path">{{ item.entry.path }}</div>
                <div v-if="item.entry.target" class="run-detail-tip__target">→ {{ item.entry.target }}</div>
                <div v-if="item.entry.note" class="run-detail-tip__note">{{ item.entry.note }}</div>
              </template>
              <span class="run-detail__cell">
                <span class="run-detail__line">
                  <el-icon class="run-detail__icon">
                    <Folder v-if="item.entry.dir" />
                    <Document v-else />
                  </el-icon>
                  <span class="run-detail__path">{{ item.source }}</span>
                </span>
                <!-- 第二行只在「有产物落点」时出现（上传 / strm / 元数据 / 打包 / 移动）；
                     跳过、失败、删除没有落点，只显示上面那行文件路径。 -->
                <span v-if="item.secondary" class="run-detail__line">
                  <span class="run-detail__arrow" aria-hidden="true">↳</span>
                  <span class="run-detail__target">{{ item.secondary }}</span>
                </span>
              </span>
            </el-tooltip>

            <span class="run-detail__action-cell">
              <span class="run-detail__action" :class="actionClass(item.action)">{{ actionLabel(item.action) }}</span>
            </span>
          </div>
        </template>
      </VirtualList>

      <div v-else class="run-detail__empty">{{ emptyText }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Document, Folder } from '@element-plus/icons-vue'

import VirtualList from './VirtualList.vue'
import {
  backupFileActionClass,
  backupFileActionLabel,
  detailBaseName,
  detailTargetDisplay,
  filterRunDetailFiles,
  runDetailSummaryLabels,
  runDetailShowsNameOnly,
  runFileActionHasTarget,
  stripDetailRoot,
  type BackupFileEntry,
  type BackupFileManifest,
  type RunDetailSummaryKey,
  type RunFileAction,
} from '../utils/backupDetail'

const props = defineProps<{
  manifest: BackupFileManifest | null
  // 顶部统计栏选中的筛选项：null 表示不筛选，列出全部条目。
  filterKey?: RunDetailSummaryKey | null
}>()

// 列名：备份任务的明细是「备份操作」（第一行源路径，第二行箭头 + 目标端落地的文件夹），
// 其它链路仍是「源路径」——它们的第二行（strm / 打包等）同样挂在同一列下。
// 转换 / 命名是例外：这两条链路两行都只给「名字」（第一行原名字、第二行新名），
// 列里根本没有路径，列名跟着叫「名称」，免得看着像路径被裁没了。
const backupMode = computed(() => (props.manifest?.kind || '').trim() === 'backup')
const nameOnlyMode = computed(() => runDetailShowsNameOnly(props.manifest?.kind))
const sourceColumnLabel = computed(() => {
  if (backupMode.value) {
    return '备份操作'
  }
  return nameOnlyMode.value ? '名称' : '源路径'
})

interface RunDetailRow {
  entry: BackupFileEntry
  action: RunFileAction
  // 第一行：默认是源路径裁掉源根后的显示值（备份链路记的本来就是相对路径，裁剪不命中即原样）；
  // 转换 / 命名只给名字（文件名 / 文件夹名，见 runDetailShowsNameOnly）。
  source: string
  // 产物落点：备份链路是「落地的文件夹」（完整路径、含目标根），strm / 软链硬链 / 打包是完整目标路径，
  // 转换 / 命名只给改完后的新名字；非产出型动作（跳过 / 失败 / 删除）恒为空 → 那一行只有一条路径。
  target: string
  // 第二行真正显示的内容，当前与 target 一致（没有产物就不显示第二行）。
  secondary: string
}

// 当前筛选下的**全部**条目（不分页）——虚拟滚动直接吃这一份。
const rows = computed<RunDetailRow[]>(() => {
  const manifest = props.manifest
  const kind = manifest?.kind
  const sourceRoots = manifest?.sourceRoots ?? []
  const nameOnly = nameOnlyMode.value

  // kind 传下去：净化链路的「成功」要把删除条目一并筛出来（见 filterRunDetailFiles）。
  return filterRunDetailFiles(manifest?.files ?? [], props.filterKey ?? null, kind).map((entry) => {
    // 第二行只给「成功产出型」动作：跳过 / 失败 / 删除一律单行，只显示文件路径，
    // 原因留给悬浮提示（用户口径：两行只留给真的产出了东西的那条）。
    const target = runFileActionHasTarget(entry.action) ? detailTargetDisplay(kind, entry) : ''
    return {
      entry,
      action: entry.action,
      source: nameOnly ? detailBaseName(entry.path) : stripDetailRoot(entry.path, sourceRoots),
      target,
      secondary: target,
    }
  })
})

/* 行高必须与 CSS 里写死的一致（行是绝对定位的，算错了就地重叠）：
   上下各 7px 内边距 + 20px 行高，两行之间再留 1px 缝 —— 单行 34、两行 55。 */
const ROW_HEIGHT_SINGLE = 34
const ROW_HEIGHT_DOUBLE = 55

function rowHeight(row: RunDetailRow) {
  return row.secondary ? ROW_HEIGHT_DOUBLE : ROW_HEIGHT_SINGLE
}

function rowKey(row: RunDetailRow) {
  return `${row.entry.path}|${row.action}`
}

// 换筛选由 VirtualList 的 resetKey 处理（换筛选项自动回到顶部）。
// 不回去的话，从「全部」切到条目更少的筛选项时，滚动位置会停在原处，
// 看起来像「筛出来是空的」。

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
/* 竖直方向被弹窗正文的 flex 分配高度：自己撑满剩余空间，把余下的高度全给列表。 */
.run-detail {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

/* 表头：与行同一套栅格（名称吃掉剩余宽度 + 结果 126px），列必然对齐。
   背景 / 文字色沿用全局 .el-table 那支表头色，换成自绘表头也不跳色。 */
.run-detail__head {
  display: grid;
  flex: 0 0 auto;
  grid-template-columns: minmax(0, 1fr) 126px;
  padding: 8px 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  background: rgba(148, 163, 184, 0.08);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.run-detail__head-cell {
  padding: 0 12px;
}

.run-detail__head-cell--action {
  text-align: center;
}

/* 列表区：独占除表头以外的全部高度。 */
.run-detail__body {
  position: relative;
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
}

.run-detail__list {
  flex: 1 1 auto;
  min-height: 0;
}

/* 滚动条压到 4px：明细表内一直用细浅滚动条（弹窗里空间紧，6px 会显眼）。 */
.run-detail__list::-webkit-scrollbar {
  width: 4px;
}

/* 一行 = 名称（flex 撑开）+ 结果，两列与表头同栅格。行高由 VirtualList 内联写死，
   这里只管上下内边距与两行之间的缝。 */
.run-detail__item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 126px;
  align-items: center;
  height: 100%;
  padding: 7px 0;
  box-sizing: border-box;
}

/* 一个条目 = 一列（备份任务是两行，其它链路只有第一行），行内各自是 flex 横排。 */
.run-detail__cell {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
  padding: 0 12px;
  box-sizing: border-box;
}

.run-detail__line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  /* 行高定死 20px：上面的行高常量就是按它算的。 */
  height: 20px;
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
  line-height: 20px;
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

/* 目标端落地的文件夹 / 产物路径：备份语义色 + 比源路径小一档。 */
.run-detail__target {
  min-width: 0;
  overflow: hidden;
  font-size: 12px;
  font-weight: 600;
  line-height: 20px;
  color: #5f7fa8;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.run-detail__action-cell {
  display: flex;
  justify-content: center;
}

/* 筛选后没有条目时的说明文案（原来挂在 el-table 的 empty 插槽上）。 */
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

/* 软链 / 硬链建出的链接：靛蓝（与 strm 的青区分开，都是「落地了产物」的语义）。 */
.run-detail__action.is-link {
  color: #5061c2;
  background: rgba(80, 97, 194, 0.13);
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
