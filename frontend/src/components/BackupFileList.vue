<template>
  <div v-if="manifest" class="backup-files">
    <div class="backup-files__head">
      <span class="backup-files__title">备份文件</span>

      <div class="backup-files__filters" role="group" aria-label="备份文件筛选">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          class="backup-files__filter"
          :class="{ 'is-active': activeFilter === tab.key }"
          @click="selectFilter(tab.key)"
        >
          {{ tab.label }}
          <span class="backup-files__filter-count">{{ tab.count }}</span>
        </button>

        <!-- 跳过只看数目、不列明细（筛选规则排除 / 目标同名文件会产生海量条目）。 -->
        <span v-if="skipCount > 0" class="backup-files__stat">
          跳过 <strong>{{ skipCount }}</strong> 项 · 仅统计
        </span>
      </div>

      <span class="backup-files__hint">{{ hintText }}</span>
    </div>

    <el-table :data="pagedFiles" class="backup-files__table" size="small" max-height="300" empty-text="本次执行没有该类文件">
      <el-table-column label="文件" min-width="360">
        <template #default="scope">
          <div class="backup-files__path">
            <el-icon class="backup-files__icon">
              <Folder v-if="scope.row.dir" />
              <Document v-else />
            </el-icon>
            <span class="backup-files__name" :title="scope.row.path">{{ scope.row.path }}</span>
          </div>
          <div v-if="scope.row.note" class="backup-files__note">{{ scope.row.note }}</div>
          <div v-if="scope.row.target" class="backup-files__target" :title="scope.row.target">
            → {{ scope.row.target }}
          </div>
        </template>
      </el-table-column>

      <el-table-column label="结果" width="86" align="center">
        <template #default="scope">
          <span class="backup-files__action" :class="actionClass(scope.row.action)">
            {{ actionLabel(scope.row.action) }}
          </span>
        </template>
      </el-table-column>

      <el-table-column label="大小" width="96" align="right">
        <template #default="scope">
          <span class="backup-files__size">{{ formatSize(scope.row.size) }}</span>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="filteredFiles.length > filePageSize" class="backup-files__pagination">
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
  type BackupFileAction,
  type BackupFileFilter,
  type BackupFileManifest,
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

const tabs = computed(() => {
  const counts = props.manifest?.counts
  return [
    { key: 'upload' as BackupFileFilter, label: '已上传', count: counts?.upload ?? 0 },
    { key: 'fail' as BackupFileFilter, label: '失败', count: counts?.fail ?? 0 },
    { key: 'delete' as BackupFileFilter, label: '已删除', count: counts?.delete ?? 0 },
    { key: 'all' as BackupFileFilter, label: '全部', count: props.manifest?.total ?? 0 },
  ]
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

// 打开新记录时重置筛选：默认展示「已上传」，没有上传则回落到「全部」。
watch(
  () => props.manifest,
  (manifest) => {
    activeFilter.value = (manifest?.counts.upload ?? 0) > 0 ? 'upload' : 'all'
    currentPage.value = 1
  },
  { immediate: true },
)

function selectFilter(filter: BackupFileFilter) {
  activeFilter.value = filter
  currentPage.value = 1
}

function actionLabel(action: BackupFileAction) {
  return backupFileActionLabel(action)
}

function actionClass(action: BackupFileAction) {
  return backupFileActionClass(action)
}

function formatSize(size?: number) {
  return formatBackupFileSize(size)
}
</script>

<style scoped>
.backup-files {
  margin-bottom: 16px;
  padding: 14px 16px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 16px;
  background: var(--el-bg-color);
}

.backup-files__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 12px;
  margin-bottom: 12px;
}

.backup-files__title {
  font-size: 14px;
  font-weight: 800;
  color: #325d6d;
}

.backup-files__filters {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  flex: 1 1 auto;
  min-width: 0;
}

.backup-files__filter {
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

.backup-files__filter:hover {
  color: #325d6d;
  border-color: rgba(63, 127, 149, 0.5);
}

.backup-files__filter.is-active {
  color: #325d6d;
  border-color: rgba(63, 127, 149, 0.55);
  background: rgba(63, 127, 149, 0.12);
}

.backup-files__filter-count {
  font-size: 11px;
  font-weight: 800;
  opacity: 0.75;
}

/* 「跳过」只报数目：虚线胶囊，区别于可点击的筛选按钮。 */
.backup-files__stat {
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

.backup-files__stat strong {
  font-weight: 800;
  color: #8a8f98;
}

.backup-files__hint {
  flex: 0 0 auto;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.backup-files__table {
  width: 100%;
}

.backup-files__path {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.backup-files__icon {
  flex: 0 0 auto;
  color: #6f97a6;
  font-size: 15px;
}

.backup-files__name {
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  word-break: break-all;
}

.backup-files__note {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
  word-break: break-all;
}

.backup-files__target {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-placeholder);
  word-break: break-all;
}

.backup-files__action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 46px;
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}

.backup-files__action.is-upload {
  color: #2f8f5b;
  background: rgba(47, 143, 91, 0.12);
}

.backup-files__action.is-fail {
  color: #c4562f;
  background: rgba(196, 86, 47, 0.12);
}

.backup-files__action.is-delete {
  color: #5f7fa8;
  background: rgba(95, 127, 168, 0.14);
}

.backup-files__size {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}

.backup-files__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
}
</style>
