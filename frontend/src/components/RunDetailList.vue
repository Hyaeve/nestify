<template>
  <div v-if="manifest" class="run-detail">
    <div class="run-detail__head">
      <span class="run-detail__title">{{ panelTitle }}</span>

      <div class="run-detail__filters" role="group" aria-label="执行明细筛选">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          class="run-detail__filter"
          :class="{ 'is-active': activeFilter === tab.key }"
          @click="selectFilter(tab.key)"
        >
          {{ tab.label }}
          <span class="run-detail__filter-count">{{ tab.count }}</span>
        </button>

        <!-- 跳过只看数目、不列明细（筛选规则排除 / 目标已有同名产物会产生海量条目）。 -->
        <span v-if="skipCount > 0" class="run-detail__stat">
          跳过 <strong>{{ skipCount }}</strong> 项 · 仅统计
        </span>
      </div>

      <span class="run-detail__hint">{{ hintText }}</span>
    </div>

    <el-table :data="pagedFiles" class="run-detail__table" size="small" max-height="300" empty-text="本次执行没有该类文件">
      <el-table-column label="文件" min-width="360">
        <template #default="scope">
          <div class="run-detail__path">
            <el-icon class="run-detail__icon">
              <Folder v-if="scope.row.dir" />
              <Document v-else />
            </el-icon>
            <span class="run-detail__name" :title="scope.row.path">{{ scope.row.path }}</span>
          </div>
          <div v-if="scope.row.note" class="run-detail__note">{{ scope.row.note }}</div>
          <div v-if="scope.row.target" class="run-detail__target" :title="scope.row.target">
            → {{ scope.row.target }}
          </div>
        </template>
      </el-table-column>

      <el-table-column label="结果" width="112" align="center">
        <template #default="scope">
          <span class="run-detail__action" :class="actionClass(scope.row.action)">
            {{ actionLabel(scope.row.action) }}
          </span>
        </template>
      </el-table-column>

      <el-table-column label="大小" width="96" align="right">
        <template #default="scope">
          <span class="run-detail__size">{{ formatSize(scope.row.size) }}</span>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="filteredFiles.length > filePageSize" class="run-detail__pagination">
      <el-pagination
        v-model:current-page="currentPage"
        background
        layout="total, prev, pager, next"
        :page-size="filePageSize"
        :total="filteredFiles.length"
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
  formatBackupFileSize,
  runDetailTitle,
  runFileActionOrder,
  type BackupFileFilter,
  type BackupFileManifest,
  type RunFileAction,
} from '../utils/backupDetail'

const props = withDefaults(
  defineProps<{
    manifest: BackupFileManifest | null
    pageSize?: number
  }>(),
  { pageSize: 50 },
)

const filePageSize = props.pageSize
const activeFilter = ref<BackupFileFilter>('all')
const currentPage = ref(1)

// 面板标题跟随载荷类型：备份 / Strm 与元数据 / 打包产出 …
const panelTitle = computed(() => runDetailTitle(props.manifest?.kind))

// 页签按载荷里真实出现的动作动态生成：strm 规则看到「生成 Strm / 同步元数据」，
// 打包规则看到「已打包」，备份规则仍是「已上传 / 失败 / 已删除」。
const tabs = computed(() => {
  const manifest = props.manifest
  if (!manifest) {
    return [] as { key: BackupFileFilter; label: string; count: number }[]
  }
  const items = runFileActionOrder
    .filter((action) => (manifest.counts[action] ?? 0) > 0)
    .map((action) => ({
      key: action as BackupFileFilter,
      label: backupFileActionLabel(action),
      count: manifest.counts[action] ?? 0,
    }))
  items.push({ key: 'all', label: '全部', count: manifest.total })
  return items
})

// 「跳过」不参与筛选，只在筛选行旁边显示总数。
const skipCount = computed(() => props.manifest?.counts.skip ?? 0)

const filteredFiles = computed(() => {
  const files = props.manifest?.files ?? []
  if (activeFilter.value === 'all') {
    return files
  }
  return files.filter((file) => file.action === activeFilter.value)
})

const pagedFiles = computed(() => {
  const start = (currentPage.value - 1) * filePageSize
  return filteredFiles.value.slice(start, start + filePageSize)
})

const hintText = computed(() => {
  const manifest = props.manifest
  if (!manifest) {
    return ''
  }
  const shown = filteredFiles.value.length
  const base = `共 ${manifest.total} 项 · 当前展示 ${shown} 条`
  return manifest.truncated ? `${base}（明细较多，每类仅保留前 200 条）` : base
})

// 打开新记录时重置筛选：默认落在第一个有内容的动作上，没有明细则回落到「全部」。
watch(
  () => props.manifest,
  (manifest) => {
    const first = manifest ? runFileActionOrder.find((action) => (manifest.counts[action] ?? 0) > 0) : undefined
    activeFilter.value = first ?? 'all'
    currentPage.value = 1
  },
  { immediate: true },
)

function selectFilter(filter: BackupFileFilter) {
  activeFilter.value = filter
  currentPage.value = 1
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
.run-detail {
  margin-bottom: 16px;
  padding: 14px 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 16px;
  background: var(--el-bg-color);
}

.run-detail__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 12px;
  margin-bottom: 12px;
}

.run-detail__title {
  font-size: 14px;
  font-weight: 800;
  color: #0975b8;
}

.run-detail__filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  flex: 1 1 auto;
  min-width: 0;
}

.run-detail__filter {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border: 1px solid var(--el-border-color);
  border-radius: 999px;
  background: transparent;
  color: var(--el-text-color-regular);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition:
    color 0.16s ease,
    background-color 0.16s ease,
    border-color 0.16s ease;
}

.run-detail__filter:hover {
  color: #0975b8;
  border-color: rgba(32, 159, 238, 0.5);
}

.run-detail__filter.is-active {
  color: #0975b8;
  border-color: rgba(32, 159, 238, 0.55);
  background: rgba(32, 159, 238, 0.12);
}

.run-detail__filter-count {
  font-size: 11px;
  font-weight: 800;
  opacity: 0.75;
}

/* 「跳过」只报数目：虚线胶囊，区别于可点击的筛选按钮。 */
.run-detail__stat {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 10px;
  border: 1px dashed var(--el-border-color);
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}

.run-detail__stat strong {
  font-weight: 800;
  color: #8a8f98;
}

.run-detail__hint {
  flex: 0 0 auto;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.run-detail__table {
  width: 100%;
}

.run-detail__path {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.run-detail__icon {
  flex: 0 0 auto;
  color: #6f97a6;
  font-size: 15px;
}

.run-detail__name {
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  word-break: break-all;
}

.run-detail__note {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
  word-break: break-all;
}

.run-detail__target {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-placeholder);
  word-break: break-all;
}

.run-detail__action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 68px;
  padding: 2px 8px;
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

.run-detail__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
}
</style>
