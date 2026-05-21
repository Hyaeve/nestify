<template>
  <el-card class="page-card file-manager-page">
    <template #header>
      <div class="file-manager-page__header">
        <div>
          <div class="file-manager-page__title">文件管理</div>
          <div class="file-manager-page__subtitle">支持目录浏览、上传、新建、复制、移动、删除与打包 CBZ</div>
        </div>
        <el-tag type="success">Live API</el-tag>
      </div>
    </template>

    <el-alert
      v-if="errorMessage"
      style="margin-bottom: 16px;"
      type="error"
      :closable="false"
      :title="errorMessage"
    />

    <div class="file-toolbar">
      <div class="file-toolbar__path">
        <span class="file-toolbar__label">当前路径</span>
        <el-input v-model="directoryPath" placeholder="请选择或输入已挂载目录" @keyup.enter="openCurrentPath">
          <template #append>
            <el-button @click="openPicker('browse')">选择目录</el-button>
          </template>
        </el-input>
      </div>

      <div class="file-toolbar__roots">
        <span class="file-toolbar__label">根目录</span>
        <el-space wrap>
          <el-button v-for="root in roots" :key="root.path" @click="openRoot(root.path)">{{ root.name }}</el-button>
        </el-space>
      </div>

      <div class="file-toolbar__actions">
        <el-button @click="createFolderDialogVisible = true">新建文件夹</el-button>
        <el-button type="primary" @click="triggerUpload">上传</el-button>
        <el-button :loading="loading" @click="reloadEntries">刷新</el-button>
        <el-button @click="openCurrentPath">打开路径</el-button>
        <el-button :disabled="!parentPath" @click="openParent">上级目录</el-button>
        <el-button type="primary" @click="selectAllEntries">全选</el-button>
        <el-button @click="clearSelection">取消全选</el-button>
        <el-button :disabled="!selectedCount" @click="openCopyDialog()">复制</el-button>
        <el-button type="success" :disabled="!selectedCount" @click="openMoveDialog()">移动</el-button>
        <el-button type="warning" :disabled="!selectedCount" @click="openPackDialog()">打包 CBZ</el-button>
        <el-button type="danger" :disabled="!selectedCount" @click="removeItems()">删除</el-button>
        <el-button :loading="validating" @click="handleValidate">校验目录</el-button>
        <el-button type="success" :loading="preflighting" @click="handlePreflight">执行预检</el-button>
      </div>
    </div>

    <div class="file-summary">
      <el-tag type="info">当前目录：{{ currentPathDisplay }}</el-tag>
      <el-tag type="primary">已选择 {{ selectedCount }} 项</el-tag>
      <el-tag type="success">共 {{ entries.length }} 项</el-tag>
      <el-tag v-if="validation" :type="validation.allowed ? 'success' : 'danger'">
        {{ validation.allowed ? '目录可访问' : '目录受限' }}
      </el-tag>
    </div>

    <el-table
      ref="tableRef"
      v-loading="loading"
      :data="entries"
      border
      @selection-change="handleSelectionChange"
      @row-dblclick="handleRowDblClick"
    >
      <el-table-column type="selection" width="52" />
      <el-table-column label="名称" min-width="320">
        <template #default="scope">
          <div class="entry-name">
            <el-icon class="entry-name__icon">
              <FolderOpened v-if="scope.row.is_dir" />
              <Document v-else />
            </el-icon>
            <div>
              <div class="entry-name__title">{{ scope.row.name }}</div>
              <div class="entry-name__path">{{ scope.row.path }}</div>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="120">
        <template #default="scope">
          <el-tag :type="scope.row.is_dir ? 'warning' : 'info'">{{ scope.row.is_dir ? '文件夹' : '文件' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="子目录" width="120">
        <template #default="scope">
          <el-tag :type="scope.row.has_children ? 'success' : 'info'">{{ scope.row.has_children ? '有' : '无' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="大小" width="140">
        <template #default="scope">
          <span>{{ scope.row.is_dir ? '—' : formatBytes(scope.row.size) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="修改时间" width="190">
        <template #default="scope">
          <span>{{ formatTimestamp(scope.row.modified_at) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="340" fixed="right">
        <template #default="scope">
          <el-space wrap>
            <el-button v-if="scope.row.is_dir" link type="primary" @click="openEntry(scope.row)">进入</el-button>
            <el-button link @click="openCopyDialog(scope.row)">复制</el-button>
            <el-button link type="success" @click="openMoveDialog(scope.row)">移动</el-button>
            <el-button link type="warning" @click="openPackDialog(scope.row)">打包 CBZ</el-button>
            <el-button link type="danger" @click="removeItems(scope.row)">删除</el-button>
          </el-space>
        </template>
      </el-table-column>
    </el-table>

    <div class="file-panels">
      <el-card class="file-panel">
        <template #header>
          <span>目录状态</span>
        </template>
        <el-descriptions v-if="validation" :column="1" border>
          <el-descriptions-item label="路径">{{ validation.path }}</el-descriptions-item>
          <el-descriptions-item label="允许访问">
            <el-tag :type="validation.allowed ? 'success' : 'danger'">{{ validation.allowed ? '是' : '否' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="目录存在">
            <el-tag :type="validation.exists ? 'success' : 'warning'">{{ validation.exists ? '是' : '否' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="可写入">
            <el-tag :type="validation.writable ? 'success' : 'info'">{{ validation.writable ? '是' : '否' }}</el-tag>
          </el-descriptions-item>
        </el-descriptions>
        <div v-else class="placeholder-text">尚未校验当前目录</div>
      </el-card>

      <el-card class="file-panel">
        <template #header>
          <span>最近执行</span>
        </template>

        <el-descriptions v-if="preflightResult" :column="1" border>
          <el-descriptions-item label="输出目录">{{ preflightResult.output_dir }}</el-descriptions-item>
          <el-descriptions-item label="允许执行">
            <el-tag :type="preflightResult.allowed ? 'success' : 'danger'">{{ preflightResult.allowed ? '是' : '否' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="图片数量">{{ preflightResult.image_count }}</el-descriptions-item>
        </el-descriptions>

        <div v-if="latestRun" class="run-summary">
          <div class="run-summary__title">最近运行：{{ latestRun.id }}</div>
          <div class="run-summary__meta">{{ latestRun.trigger_mode }} / {{ latestRun.status }} / {{ latestRun.stage }}</div>
          <div class="run-summary__logs">
            <div v-for="log in latestLogs" :key="log.id" class="run-summary__log">[{{ log.level }}] {{ log.message }}</div>
          </div>
        </div>

        <div v-if="!preflightResult && !latestRun" class="placeholder-text">暂未执行目录预检或归档动作</div>
      </el-card>
    </div>

    <input ref="uploadInputRef" type="file" multiple class="file-upload-input" @change="handleUploadSelected" />
  </el-card>

  <DirectoryPickerDialog
    v-model="directoryPickerVisible"
    title="选择文件管理目录"
    :initial-path="pickerInitialPath"
    @selected="handleDirectorySelected"
  />

  <el-dialog v-model="createFolderDialogVisible" title="新建文件夹" width="520px">
    <el-form label-position="top">
      <el-form-item label="父目录">
        <el-input :model-value="directoryPath" disabled />
      </el-form-item>
      <el-form-item label="文件夹名称">
        <el-input v-model="createFolderName" placeholder="请输入文件夹名称" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="createFolderDialogVisible = false">取消</el-button>
      <el-button type="primary" @click="submitCreateFolder">创建</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="copyDialogVisible" title="复制所选项目" width="620px">
    <el-form label-position="top">
      <el-form-item label="目标目录">
        <el-input v-model="copyTargetPath" placeholder="请输入或选择目标目录">
          <template #append>
            <el-button @click="openPicker('copy')">选择目录</el-button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item label="待复制项目">
        <el-space wrap>
          <el-tag v-for="item in copyCandidates" :key="item.path" type="info">{{ item.name }}</el-tag>
        </el-space>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="copyDialogVisible = false">取消</el-button>
      <el-button type="primary" @click="submitCopyItems">确认复制</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="moveDialogVisible" title="移动所选项目" width="620px">
    <el-form label-position="top">
      <el-form-item label="目标目录">
        <el-input v-model="moveTargetPath" placeholder="请输入或选择目标目录">
          <template #append>
            <el-button @click="openPicker('move')">选择目录</el-button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item label="待移动项目">
        <el-space wrap>
          <el-tag v-for="item in moveCandidates" :key="item.path" type="info">{{ item.name }}</el-tag>
        </el-space>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="moveDialogVisible = false">取消</el-button>
      <el-button type="primary" @click="submitMoveItems">确认移动</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="packDialogVisible" title="打包为 CBZ" width="620px">
    <el-form label-position="top">
      <el-form-item label="输出目录">
        <el-input v-model="packOutputDir" placeholder="请输入或选择输出目录">
          <template #append>
            <el-button @click="openPicker('pack')">选择目录</el-button>
          </template>
        </el-input>
      </el-form-item>
      <el-form-item label="归档名称">
        <el-input v-model="packArchiveName" placeholder="例如：archive.cbz，可留空自动生成" />
      </el-form-item>
      <el-form-item label="待打包项目">
        <el-space wrap>
          <el-tag v-for="item in packCandidates" :key="item.path" type="warning">{{ item.name }}</el-tag>
        </el-space>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="packDialogVisible = false">取消</el-button>
      <el-button type="primary" @click="submitPackCBZ">开始打包</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Document, FolderOpened } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import {
  fetchRunLogs,
  prepareManualPreflight,
  type ManualPreflightResult,
  type RunInstance,
  type RunLogEntry,
} from '../api/executions'
import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import {
  browseDirectories,
  copyItems,
  createFolder,
  deleteItems,
  fetchBrowseRoots,
  moveItems,
  packItemsAsCBZ,
  uploadFiles,
  validateDirectory,
  type BrowseRoot,
  type DirectoryEntry,
  type ValidatePathPayload,
} from '../api/paths'

interface FileManagerEntry extends DirectoryEntry {}

type PickerMode = 'browse' | 'copy' | 'move' | 'pack'

const directoryPath = ref('')
const directoryPickerVisible = ref(false)
const pickerMode = ref<PickerMode>('browse')
const loading = ref(false)
const validating = ref(false)
const preflighting = ref(false)
const errorMessage = ref('')
const validation = ref<ValidatePathPayload | null>(null)
const preflightResult = ref<ManualPreflightResult | null>(null)
const latestRun = ref<RunInstance | null>(null)
const latestLogs = ref<RunLogEntry[]>([])
const roots = ref<BrowseRoot[]>([])
const entries = ref<FileManagerEntry[]>([])
const parentPath = ref('')
const selectedRows = ref<FileManagerEntry[]>([])
const tableRef = ref<any>(null)
const uploadInputRef = ref<HTMLInputElement | null>(null)

const createFolderDialogVisible = ref(false)
const createFolderName = ref('')

const copyDialogVisible = ref(false)
const copyTargetPath = ref('')
const copyCandidates = ref<FileManagerEntry[]>([])

const moveDialogVisible = ref(false)
const moveTargetPath = ref('')
const moveCandidates = ref<FileManagerEntry[]>([])

const packDialogVisible = ref(false)
const packOutputDir = ref('')
const packArchiveName = ref('')
const packCandidates = ref<FileManagerEntry[]>([])

const selectedCount = computed(() => selectedRows.value.length)
const currentPathDisplay = computed(() => directoryPath.value || '未选择')
const pickerInitialPath = computed(() => {
  switch (pickerMode.value) {
    case 'copy':
      return copyTargetPath.value || directoryPath.value
    case 'move':
      return moveTargetPath.value || directoryPath.value
    case 'pack':
      return packOutputDir.value || directoryPath.value
    default:
      return directoryPath.value
  }
})

function openPicker(mode: PickerMode) {
  pickerMode.value = mode
  directoryPickerVisible.value = true
}

function getSelection(entry?: FileManagerEntry) {
  return entry ? [entry] : selectedRows.value
}

function formatBytes(size: number) {
  if (!size) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`
}

function formatTimestamp(value: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

function handleDirectorySelected(path: string) {
  switch (pickerMode.value) {
    case 'copy':
      copyTargetPath.value = path
      break
    case 'move':
      moveTargetPath.value = path
      break
    case 'pack':
      packOutputDir.value = path
      break
    default:
      directoryPath.value = path
      void openCurrentPath()
      break
  }

  directoryPickerVisible.value = false
}

async function initialize() {
  loading.value = true
  errorMessage.value = ''

  try {
    const response = await fetchBrowseRoots()
    roots.value = response.data?.items ?? []

    if (!directoryPath.value && roots.value[0]?.path) {
      directoryPath.value = roots.value[0].path
    }

    if (directoryPath.value) {
      await openCurrentPath()
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '目录初始化失败'
  } finally {
    loading.value = false
  }
}

async function openRoot(path: string) {
  directoryPath.value = path
  await openCurrentPath()
}

async function openCurrentPath() {
  if (!directoryPath.value) return

  loading.value = true
  errorMessage.value = ''

  try {
    const response = await browseDirectories(directoryPath.value)
    directoryPath.value = response.data?.current_path ?? directoryPath.value
    parentPath.value = response.data?.parent_path ?? ''
    entries.value = response.data?.entries ?? []
    clearSelection()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '目录加载失败'
  } finally {
    loading.value = false
  }
}

async function reloadEntries() {
  await openCurrentPath()
}

async function openParent() {
  if (!parentPath.value) return
  directoryPath.value = parentPath.value
  await openCurrentPath()
}

async function openEntry(entry: FileManagerEntry) {
  if (!entry.is_dir) {
    ElMessage.info('当前仅支持进入文件夹')
    return
  }

  directoryPath.value = entry.path
  await openCurrentPath()
}

function handleRowDblClick(row: FileManagerEntry) {
  if (row.is_dir) {
    void openEntry(row)
  }
}

function handleSelectionChange(rows: FileManagerEntry[]) {
  selectedRows.value = rows
}

function clearSelection() {
  tableRef.value?.clearSelection?.()
  selectedRows.value = []
}

function selectAllEntries() {
  tableRef.value?.clearSelection?.()
  entries.value.forEach((entry) => tableRef.value?.toggleRowSelection?.(entry, true))
  selectedRows.value = [...entries.value]
}

async function submitCreateFolder() {
  if (!directoryPath.value) {
    ElMessage.warning('请先选择目录')
    return
  }

  try {
    await createFolder(directoryPath.value, createFolderName.value)
    ElMessage.success('文件夹创建成功')
    createFolderDialogVisible.value = false
    createFolderName.value = ''
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '文件夹创建失败'
  }
}

function triggerUpload() {
  uploadInputRef.value?.click()
}

async function handleUploadSelected(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? [])
  if (files.length === 0 || !directoryPath.value) {
    return
  }

  try {
    await uploadFiles(directoryPath.value, files)
    ElMessage.success(`已上传 ${files.length} 个文件`)
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '上传失败'
  } finally {
    target.value = ''
  }
}

function openCopyDialog(entry?: FileManagerEntry) {
  const items = getSelection(entry)
  if (items.length === 0) {
    ElMessage.warning('请先选择项目')
    return
  }

  copyCandidates.value = items
  copyTargetPath.value = directoryPath.value
  copyDialogVisible.value = true
}

async function submitCopyItems() {
  try {
    await copyItems(copyCandidates.value.map((item) => item.path), copyTargetPath.value)
    ElMessage.success('复制成功')
    copyDialogVisible.value = false
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '复制失败'
  }
}

function openMoveDialog(entry?: FileManagerEntry) {
  const items = getSelection(entry)
  if (items.length === 0) {
    ElMessage.warning('请先选择项目')
    return
  }

  moveCandidates.value = items
  moveTargetPath.value = directoryPath.value
  moveDialogVisible.value = true
}

async function submitMoveItems() {
  try {
    await moveItems(moveCandidates.value.map((item) => item.path), moveTargetPath.value)
    ElMessage.success('移动成功')
    moveDialogVisible.value = false
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '移动失败'
  }
}

function openPackDialog(entry?: FileManagerEntry) {
  const items = getSelection(entry)
  if (items.length === 0) {
    ElMessage.warning('请先选择项目')
    return
  }

  packCandidates.value = items
  packOutputDir.value = directoryPath.value
  packArchiveName.value = ''
  packDialogVisible.value = true
}

async function submitPackCBZ() {
  try {
    const response = await packItemsAsCBZ(
      packCandidates.value.map((item) => item.path),
      packOutputDir.value,
      packArchiveName.value,
    )
    ElMessage.success(`CBZ 已生成：${response.data?.output_path ?? ''}`)
    packDialogVisible.value = false
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'CBZ 打包失败'
  }
}

async function removeItems(entry?: FileManagerEntry) {
  const items = getSelection(entry)
  if (items.length === 0) {
    ElMessage.warning('请先选择项目')
    return
  }

  try {
    await ElMessageBox.confirm(`确认删除 ${items.length} 项吗？此操作不可恢复。`, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await deleteItems(items.map((item) => item.path))
    ElMessage.success('删除成功')
    await openCurrentPath()
  } catch (error) {
    if (error instanceof Error && error.message) {
      errorMessage.value = error.message
    }
  }
}

async function handleValidate() {
  if (!directoryPath.value) {
    errorMessage.value = '请先选择目录'
    return
  }

  validating.value = true
  errorMessage.value = ''

  try {
    const response = await validateDirectory(directoryPath.value)
    validation.value = response.data ?? null
    ElMessage.success('目录校验完成')
  } catch (error) {
    validation.value = null
    errorMessage.value = error instanceof Error ? error.message : '目录校验失败'
  } finally {
    validating.value = false
  }
}

async function handlePreflight() {
  if (!directoryPath.value) {
    errorMessage.value = '请先选择目录'
    return
  }

  preflighting.value = true
  errorMessage.value = ''

  try {
    const response = await prepareManualPreflight(directoryPath.value)
    preflightResult.value = response.data?.preflight ?? null
    latestRun.value = response.data?.run ?? null

    if (latestRun.value) {
      const logsResponse = await fetchRunLogs(latestRun.value.id)
      latestLogs.value = logsResponse.data?.items ?? []
    }

    ElMessage.success('目录预检完成')
  } catch (error) {
    preflightResult.value = null
    latestRun.value = null
    latestLogs.value = []
    errorMessage.value = error instanceof Error ? error.message : '目录预检失败'
  } finally {
    preflighting.value = false
  }
}

onMounted(() => {
  void initialize()
})
</script>

<style scoped lang="scss">
.file-manager-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.file-manager-page__title {
  font-size: 24px;
  font-weight: 700;
}

.file-manager-page__subtitle {
  margin-top: 4px;
  color: var(--text-secondary);
  font-size: 13px;
}

.file-toolbar {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-bottom: 18px;
}

.file-toolbar__path,
.file-toolbar__roots,
.file-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.file-toolbar__path :deep(.el-input) {
  flex: 1;
}

.file-toolbar__label {
  min-width: 72px;
  color: var(--text-secondary);
  font-size: 14px;
}

.file-summary {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.entry-name {
  display: flex;
  align-items: center;
  gap: 12px;
}

.entry-name__icon {
  font-size: 22px;
  color: #f5b942;
}

.entry-name__title {
  font-size: 15px;
  font-weight: 600;
}

.entry-name__path {
  margin-top: 4px;
  color: var(--text-secondary);
  font-size: 12px;
}

.file-panels {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-top: 18px;
}

.file-panel {
  min-height: 220px;
}

.file-upload-input {
  display: none;
}

.run-summary__title {
  font-weight: 600;
}

.run-summary__meta {
  margin-top: 6px;
  color: var(--text-secondary);
  font-size: 13px;
}

.run-summary__logs {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.run-summary__log,
.placeholder-text {
  color: var(--text-secondary);
  font-size: 13px;
}
</style>
