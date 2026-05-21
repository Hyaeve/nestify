<template>
  <div class="file-manager-view" @click="hideContextMenu">
    <el-card class="page-card file-manager-card">
      <template #header>
        <div class="file-manager-card__header">
          <div class="file-manager-card__breadcrumb-wrap">
            <div class="file-manager-card__title-line">
              <span class="file-manager-card__crumb">首页</span>
              <span class="file-manager-card__divider">/</span>
              <span class="file-manager-card__crumb file-manager-card__crumb--active">文件管理</span>
            </div>
          </div>
        </div>
      </template>

      <el-alert
        v-if="errorMessage"
        class="file-manager-card__alert"
        type="error"
        :closable="false"
        :title="errorMessage"
      />

      <div class="toolbar-row">
        <el-button @click="createFolderDialogVisible = true">新建文件夹</el-button>
        <el-button type="primary" @click="triggerUpload">上传</el-button>
        <el-button :loading="loading" @click="reloadEntries">刷新</el-button>
        <span class="toolbar-row__split" />
        <el-button :disabled="!selectedCount" @click="openMoveDialog()">移动</el-button>
        <el-button :disabled="!selectedCount" @click="openCopyDialog()">复制</el-button>
        <el-button type="danger" plain :disabled="!selectedCount" @click="removeItems()">删除</el-button>
        <span class="toolbar-row__split" />
        <el-button @click="openPicker('browse')">选择目录</el-button>
        <el-button :disabled="!parentPath" @click="openParent">上级目录</el-button>
        <el-button :loading="validating" @click="handleValidate">校验目录</el-button>
        <el-button type="success" plain :loading="preflighting" @click="handlePreflight">执行预检</el-button>
      </div>

      <div class="path-row">
        <div class="path-row__breadcrumbs">
          <el-icon class="path-row__icon"><FolderOpened /></el-icon>
          <button
            v-for="(crumb, index) in breadcrumbItems"
            :key="crumb.path"
            type="button"
            class="path-row__crumb"
            :class="{ 'is-current': index === breadcrumbItems.length - 1 }"
            @click.stop="openPath(crumb.path)"
          >
            {{ crumb.label }}
          </button>
        </div>
      </div>

      <div class="summary-row">
        <span>当前目录：{{ currentPathDisplay }}</span>
        <span>已选择 {{ selectedCount }} 项</span>
        <span>{{ entries.length }} 个项目</span>
        <span v-if="validation">{{ validation.allowed ? '目录可访问' : '目录受限' }}</span>
        <span v-if="latestRun">最近任务：{{ latestRun.status }} / {{ latestRun.stage }}</span>
      </div>

      <el-table
        ref="tableRef"
        v-loading="loading"
        :data="entries"
        row-key="path"
        :row-class-name="getRowClassName"
        @selection-change="handleSelectionChange"
        @row-contextmenu="handleRowContextMenu"
      >
        <el-table-column type="selection" width="52" />
        <el-table-column label="名称" min-width="520">
          <template #default="scope">
            <button
              type="button"
              class="entry-name"
              :class="{ 'is-dir': scope.row.is_dir }"
              @click.stop="handleEntryPrimaryAction(scope.row)"
            >
              <el-icon class="entry-name__icon">
                <FolderOpened v-if="scope.row.is_dir" />
                <Document v-else />
              </el-icon>
              <div class="entry-name__text">
                <div class="entry-name__title">{{ scope.row.name }}</div>
                <div class="entry-name__path">{{ scope.row.path }}</div>
              </div>
            </button>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="120">
          <template #default="scope">
            {{ scope.row.is_dir ? '—' : formatBytes(scope.row.size) }}
          </template>
        </el-table-column>
        <el-table-column label="修改时间" width="190">
          <template #default="scope">
            {{ formatTimestamp(scope.row.modified_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="scope">
            <div class="entry-actions">
              <el-tooltip v-if="scope.row.is_dir" content="打开" placement="top">
                <el-button link class="entry-actions__icon" @click.stop="openEntry(scope.row)">
                  <el-icon><Folder /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="打包" placement="top">
                <el-button link class="entry-actions__icon entry-actions__icon--warning" @click.stop="openPackDialog(scope.row)">
                  <el-icon><Files /></el-icon>
                </el-button>
              </el-tooltip>
              <el-dropdown trigger="click" @command="(command: string) => handleMoreCommand(command, scope.row)">
                <el-button link class="entry-actions__icon">
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="rename">重命名</el-dropdown-item>
                    <el-dropdown-item command="move">移动</el-dropdown-item>
                    <el-dropdown-item command="copy">复制</el-dropdown-item>
                    <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="footer-count">{{ entries.length }} 个项目</div>

      <input ref="uploadInputRef" type="file" multiple class="file-upload-input" @change="handleUploadSelected" />
    </el-card>

    <div
      v-if="contextMenu.visible && contextMenu.entry"
      class="context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @click.stop
    >
      <button v-if="contextMenu.entry.is_dir" type="button" class="context-menu__item" @click="openEntryFromContext">打开</button>
      <button type="button" class="context-menu__item" @click="packEntryFromContext">打包</button>
      <div class="context-menu__divider" />
      <button type="button" class="context-menu__item" @click="renameEntryFromContext">重命名</button>
      <button type="button" class="context-menu__item" @click="moveEntryFromContext">移动</button>
      <button type="button" class="context-menu__item" @click="copyEntryFromContext">复制</button>
      <button type="button" class="context-menu__item context-menu__item--danger" @click="deleteEntryFromContext">删除</button>
    </div>

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

    <el-dialog v-model="renameDialogVisible" title="重命名" width="520px">
      <el-form label-position="top">
        <el-form-item label="当前项目">
          <el-input :model-value="renameSourceEntry?.name || ''" disabled />
        </el-form-item>
        <el-form-item label="新名称">
          <el-input v-model="renameTargetName" placeholder="请输入新名称" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitRenameItem">保存</el-button>
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
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Document, Files, Folder, FolderOpened, MoreFilled } from '@element-plus/icons-vue'
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
  renameItem,
  uploadFiles,
  validateDirectory,
  type BrowseRoot,
  type DirectoryEntry,
  type ValidatePathPayload,
} from '../api/paths'

interface FileManagerEntry extends DirectoryEntry {}

interface BreadcrumbItem {
  label: string
  path: string
}

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

const renameDialogVisible = ref(false)
const renameSourceEntry = ref<FileManagerEntry | null>(null)
const renameTargetName = ref('')

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

const contextMenu = ref<{ visible: boolean; x: number; y: number; entry: FileManagerEntry | null }>({
  visible: false,
  x: 0,
  y: 0,
  entry: null,
})

const selectedCount = computed(() => selectedRows.value.length)
const currentPathDisplay = computed(() => directoryPath.value || '未选择')
const selectedPathSet = computed(() => new Set(selectedRows.value.map((item) => item.path)))

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

const breadcrumbItems = computed<BreadcrumbItem[]>(() => {
  if (!directoryPath.value) {
    return []
  }

  const sortedRoots = [...roots.value].sort((a, b) => normalizePath(b.path).length - normalizePath(a.path).length)
  const matchedRoot = sortedRoots.find((root) => isPathWithin(root.path, directoryPath.value))
  if (!matchedRoot) {
    return [{ label: directoryPath.value, path: directoryPath.value }]
  }

  const items: BreadcrumbItem[] = [{ label: '根目录', path: matchedRoot.path }]
  const rootPath = normalizePath(matchedRoot.path)
  const currentPath = normalizePath(directoryPath.value)
  const remainder = currentPath.slice(rootPath.length).replace(/^\/+/, '')

  if (!remainder) {
    return items
  }

  let cursor = matchedRoot.path
  for (const segment of remainder.split('/').filter(Boolean)) {
    cursor = joinPath(cursor, segment)
    items.push({ label: segment, path: cursor })
  }

  return items
})

function normalizePath(path: string) {
  return path.replace(/\\/g, '/').replace(/\/+$/, '') || '/'
}

function isPathWithin(root: string, target: string) {
  const rootPath = normalizePath(root)
  const targetPath = normalizePath(target)
  return targetPath === rootPath || targetPath.startsWith(`${rootPath}/`) || rootPath === '/'
}

function joinPath(base: string, segment: string) {
  if (base.endsWith('/') || base.endsWith('\\')) {
    return `${base}${segment}`
  }
  const separator = base.includes('\\') ? '\\' : '/'
  return `${base}${separator}${segment}`
}

function hideContextMenu() {
  contextMenu.value.visible = false
}

function handleWindowKeyDown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    hideContextMenu()
  }
}

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

function getRowClassName({ row }: { row: FileManagerEntry }) {
  return selectedPathSet.value.has(row.path) ? 'file-manager-row--selected' : ''
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

async function openPath(path: string) {
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
    ElMessage.info('当前仅支持打开文件夹')
    return
  }

  directoryPath.value = entry.path
  await openCurrentPath()
}

function handleEntryPrimaryAction(entry: FileManagerEntry) {
  if (entry.is_dir) {
    void openEntry(entry)
  }
}

function handleSelectionChange(rows: FileManagerEntry[]) {
  selectedRows.value = rows
}

function clearSelection() {
  tableRef.value?.clearSelection?.()
  selectedRows.value = []
}

function handleRowContextMenu(row: FileManagerEntry, _column: unknown, event: MouseEvent) {
  event.preventDefault()
  contextMenu.value = {
    visible: true,
    x: event.clientX,
    y: event.clientY,
    entry: row,
  }
}

function handleMoreCommand(command: string, entry: FileManagerEntry) {
  if (command === 'rename') {
    openRenameDialog(entry)
    return
  }
  if (command === 'move') {
    openMoveDialog(entry)
    return
  }
  if (command === 'copy') {
    openCopyDialog(entry)
    return
  }
  if (command === 'delete') {
    void removeItems(entry)
  }
}

function openRenameDialog(entry: FileManagerEntry) {
  renameSourceEntry.value = entry
  renameTargetName.value = entry.name
  renameDialogVisible.value = true
  hideContextMenu()
}

async function submitRenameItem() {
  if (!renameSourceEntry.value) {
    return
  }

  try {
    await renameItem(renameSourceEntry.value.path, renameTargetName.value)
    ElMessage.success('重命名成功')
    renameDialogVisible.value = false
    renameSourceEntry.value = null
    renameTargetName.value = ''
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '重命名失败'
  }
}

function openEntryFromContext() {
  if (contextMenu.value.entry) {
    void openEntry(contextMenu.value.entry)
  }
  hideContextMenu()
}

function packEntryFromContext() {
  if (contextMenu.value.entry) {
    openPackDialog(contextMenu.value.entry)
  }
  hideContextMenu()
}

function renameEntryFromContext() {
  if (contextMenu.value.entry) {
    openRenameDialog(contextMenu.value.entry)
  }
}

function moveEntryFromContext() {
  if (contextMenu.value.entry) {
    openMoveDialog(contextMenu.value.entry)
  }
  hideContextMenu()
}

function copyEntryFromContext() {
  if (contextMenu.value.entry) {
    openCopyDialog(contextMenu.value.entry)
  }
  hideContextMenu()
}

function deleteEntryFromContext() {
  if (contextMenu.value.entry) {
    void removeItems(contextMenu.value.entry)
  }
  hideContextMenu()
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
  window.addEventListener('keydown', handleWindowKeyDown)
  window.addEventListener('scroll', hideContextMenu, true)
  void initialize()
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleWindowKeyDown)
  window.removeEventListener('scroll', hideContextMenu, true)
})
</script>

<style scoped lang="scss">
.file-manager-view {
  position: relative;
}

.file-manager-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.file-manager-card__title-line {
  display: flex;
  align-items: center;
  gap: 12px;
}

.file-manager-card__crumb {
  font-size: 15px;
  color: var(--text-secondary);
}

.file-manager-card__crumb--active {
  color: var(--text-primary);
  font-weight: 600;
}

.file-manager-card__divider {
  color: var(--text-secondary);
}

.file-manager-card__alert {
  margin-bottom: 16px;
}

.toolbar-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 18px;
}

.toolbar-row__split {
  width: 1px;
  height: 28px;
  background: var(--el-border-color-lighter);
}

.path-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
}

.path-row__breadcrumbs {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.path-row__icon {
  color: #6b7280;
  font-size: 18px;
}

.path-row__crumb {
  position: relative;
  padding: 0;
  font-size: 16px;
  color: #6b7280;
  background: transparent;
  border: 0;
  cursor: pointer;
}

.path-row__crumb::after {
  content: '/';
  margin-left: 8px;
  color: #c0c4cc;
}

.path-row__crumb.is-current {
  color: var(--text-primary);
  font-weight: 600;
}

.path-row__crumb:last-child::after {
  display: none;
}

.summary-row {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
  color: var(--text-secondary);
  font-size: 13px;
}

.entry-name {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 0;
  text-align: left;
  background: transparent;
  border: 0;
}

.entry-name.is-dir {
  cursor: pointer;
}

.entry-name__icon {
  flex-shrink: 0;
  font-size: 24px;
  color: #f5b942;
}

.entry-name__text {
  min-width: 0;
}

.entry-name__title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.entry-name.is-dir .entry-name__title {
  color: #1677ff;
}

.entry-name__path {
  margin-top: 4px;
  color: var(--text-secondary);
  font-size: 12px;
}

.entry-actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

.entry-actions__icon {
  font-size: 16px;
}

.entry-actions__icon--warning {
  color: #f59e0b;
}

.footer-count {
  margin-top: 16px;
  color: var(--text-secondary);
  font-size: 14px;
}

.file-upload-input {
  display: none;
}

.context-menu {
  position: fixed;
  z-index: 3000;
  min-width: 160px;
  padding: 8px;
  background: #fff;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 14px;
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.16);
}

.context-menu__item {
  width: 100%;
  padding: 10px 12px;
  text-align: left;
  color: var(--text-primary);
  background: transparent;
  border: 0;
  border-radius: 10px;
  cursor: pointer;
}

.context-menu__item:hover {
  background: #f3f4f6;
}

.context-menu__item--danger {
  color: #ef4444;
}

.context-menu__divider {
  height: 1px;
  margin: 6px 0;
  background: var(--el-border-color-lighter);
}

:deep(.file-manager-row--selected > td.el-table__cell) {
  background: #f3f4f6 !important;
}

:deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f7f7f8;
}

:deep(.el-table__body tr.current-row > td.el-table__cell) {
  background: #f3f4f6;
}
</style>
