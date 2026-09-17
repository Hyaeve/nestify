<template>
  <div v-if="manifest" class="run-detail">
    <!-- 文件级明细：源路径 / 目标路径左右分列，一行一条（悬浮看完整路径与备注）。
         窗口高度固定，这里不翻页——整份明细一次性铺出来，靠滚动条上下查看，
         所以行高与内边距都压到最小，同样高度里尽量多显示几行。 -->
    <el-table
      :data="files"
      class="run-detail__table"
      size="small"
      height="100%"
      empty-text="本次执行没有文件级明细"
    >
      <el-table-column label="源路径" min-width="300">
        <template #default="scope">
          <el-tooltip placement="top" effect="light" popper-class="run-detail-tip" :show-after="150">
            <template #content>
              <div class="run-detail-tip__path">{{ scope.row.path }}</div>
              <div v-if="scope.row.note" class="run-detail-tip__note">{{ scope.row.note }}</div>
            </template>
            <span class="run-detail__cell">
              <el-icon class="run-detail__icon">
                <Folder v-if="scope.row.dir" />
                <Document v-else />
              </el-icon>
              <span class="run-detail__path">{{ displaySource(scope.row) }}</span>
            </span>
          </el-tooltip>
        </template>
      </el-table-column>

      <el-table-column label="目标路径" min-width="300">
        <template #default="scope">
          <el-tooltip
            v-if="scope.row.target"
            placement="top"
            effect="light"
            popper-class="run-detail-tip"
            :show-after="150"
          >
            <template #content>
              <div class="run-detail-tip__path">{{ scope.row.target }}</div>
            </template>
            <span class="run-detail__cell">
              <span class="run-detail__path">{{ displayTarget(scope.row) }}</span>
            </span>
          </el-tooltip>
          <span v-else class="run-detail__blank">—</span>
        </template>
      </el-table-column>

      <el-table-column label="结果" width="112" align="center">
        <template #default="scope">
          <span class="run-detail__action" :class="actionClass(scope.row.action)">
            {{ actionLabel(scope.row.action) }}
          </span>
        </template>
      </el-table-column>

      <el-table-column label="大小" width="94" align="right">
        <template #default="scope">
          <span class="run-detail__size">{{ formatSize(scope.row.size) }}</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Document, Folder } from '@element-plus/icons-vue'

import {
  backupFileActionClass,
  backupFileActionLabel,
  formatBackupFileSize,
  stripDetailRoot,
  type BackupFileEntry,
  type BackupFileManifest,
  type RunFileAction,
} from '../utils/backupDetail'

const props = defineProps<{
  manifest: BackupFileManifest | null
}>()

const files = computed(() => props.manifest?.files ?? [])

// 源 / 目标列只显示「配置根路径下一级」开始的相对路径：绝对路径太长，两列并排根本读不出差别；
// 完整路径放在悬浮提示里（见模板里的 el-tooltip）。
function displaySource(row: BackupFileEntry) {
  return stripDetailRoot(row.path, props.manifest?.sourceRoots ?? [])
}

function displayTarget(row: BackupFileEntry) {
  return stripDetailRoot(row.target ?? '', props.manifest?.targetRoots ?? [])
}

function actionLabel(action: RunFileAction) {
  return backupFileActionLabel(action)
}

function actionClass(action: RunFileAction) {
  return backupFileActionClass(action)
}

function formatSize(size?: number) {
  return formatBackupFileSize(size)
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

/* 表格高度由组件的 height="100%" 写成内联样式，这里再兜一层 flex，保证它是唯一的伸缩项。 */
.run-detail__table {
  flex: 1 1 auto;
  width: 100%;
  min-height: 0;
}

/* 一行一条：把单元格的上下内边距压到最小，同样高度里能多铺几行。
   注意有**两层**上下内边距：`td.el-table__cell` 一层，内层 `.cell` 一层；全局
   `styles/index.scss` 的 `.el-table .cell { padding-top/bottom: var(--table-cell-padding-y) }`
   给内层留了 12px，只压外层的话行高仍会停在 58px（明细表要单独清零）。 */
.run-detail__table :deep(.el-table__cell) {
  padding: 4px 0;
}

.run-detail__table :deep(.cell) {
  padding-top: 0;
  padding-bottom: 0;
}

.run-detail__cell {
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

.run-detail__blank {
  color: var(--el-text-color-placeholder);
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

.run-detail__size {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}
</style>
