<template>
  <div class="file-manager-view" @click="hideContextMenu">
    <el-card class="page-card file-manager-card">
      <div class="file-manager-card__content">
        <el-alert
          v-if="errorMessage"
          class="file-manager-card__alert"
          type="error"
          :closable="false"
          :title="errorMessage"
        />

        <div class="toolbar-row">
          <el-tooltip content="新建文件夹" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--folder" circle aria-label="新建文件夹" @click="createFolderDialogVisible = true">
              <el-icon><FolderAdd /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip content="上传文件夹" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--upload" type="primary" circle aria-label="上传文件夹" @click="triggerUpload">
              <el-icon><UploadFilled /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip content="刷新" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--refresh" :loading="loading" circle aria-label="刷新" @click="reloadEntries">
              <template v-if="!loading">
                <el-icon><Refresh /></el-icon>
              </template>
            </el-button>
          </el-tooltip>
          <el-tooltip content="最近访问" placement="top" :show-after="500">
            <el-dropdown trigger="click" :disabled="recentVisitedPaths.length === 0" @command="handleRecentVisitedCommand">
              <el-button class="toolbar-action toolbar-action--recent" circle aria-label="最近访问">
                <el-icon><Clock /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item v-for="path in recentVisitedPaths" :key="path" :command="path">{{ path }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </el-tooltip>
          <span class="toolbar-row__split" />
          <el-tooltip content="移动" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--move" :disabled="!selectedCount" circle aria-label="移动" @click="openMoveDialog()">
              <el-icon><Right /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip content="复制" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--copy" :disabled="!selectedCount" circle aria-label="复制" @click="openCopyDialog()">
              <el-icon><CopyDocument /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip content="解压" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--extract" :disabled="!canExtractSelectedArchives || extracting" :loading="extracting" circle aria-label="解压" @click="extractSelectedArchives()">
              <template v-if="!extracting">
                <el-icon><Download /></el-icon>
              </template>
            </el-button>
          </el-tooltip>
          <el-tooltip content="收集" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--collect" :disabled="!canCollectSelectedFolders || collecting" :loading="collecting" circle aria-label="收集" @click="collectSelectedFolders()">
              <template v-if="!collecting">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="toolbar-action__icon toolbar-action__icon--collect">
                  <path d="M3.75 7.75a2 2 0 0 1 2-2h4.2l1.7 1.8h6.6a2 2 0 0 1 2 2v7.5a2 2 0 0 1-2 2H5.75a2 2 0 0 1-2-2Z" />
                  <path d="M12 10.5v5" />
                  <path d="M9.75 13.25 12 15.5l2.25-2.25" />
                </svg>
              </template>
            </el-button>
          </el-tooltip>
          <el-tooltip content="删除" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--delete" type="danger" plain :disabled="!selectedCount" circle aria-label="删除" @click="removeItems()">
              <el-icon><Delete /></el-icon>
            </el-button>
          </el-tooltip>
          <span class="toolbar-row__split" />
          <el-tooltip content="选择目录" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--browse" circle aria-label="选择目录" @click="openPicker('browse')">
              <el-icon><Folder /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip content="上级目录" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--parent" :disabled="!parentPath" circle aria-label="上级目录" @click="openParent">
              <el-icon><Back /></el-icon>
            </el-button>
          </el-tooltip>
          <el-tooltip content="回收站" placement="top" :show-after="500">
            <el-button class="toolbar-action toolbar-action--recycle" circle aria-label="回收站" @click="openRecycleBin">
              <svg viewBox="0 0 24 24" aria-hidden="true" class="toolbar-action__icon toolbar-action__icon--recycle">
                <path d="M9 4.5h6" />
                <path d="M10 3h4a1 1 0 0 1 1 1v.5H9V4a1 1 0 0 1 1-1Z" />
                <path d="M5.5 6.5h13" />
                <path d="M7 6.5l.8 11.2A2 2 0 0 0 9.79 19.5h4.42a2 2 0 0 0 1.99-1.8L17 6.5" />
                <path d="M10 10v6" />
                <path d="M14 10v6" />
              </svg>
            </el-button>
          </el-tooltip>
          <div class="toolbar-row__search">
            <el-input
              v-model="searchKeyword"
              clearable
              class="toolbar-row__search-input"
              placeholder="搜索当前层级文件或文件夹"
            />
          </div>
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
          <span>{{ filteredEntries.length }} 个项目</span>
          <span class="summary-row__sort">
            文件排序：
            <el-select v-model="sortBy" size="small" class="summary-row__sort-select">
              <el-option label="修改时间" value="modified_at" />
              <el-option label="文件名称" value="name" />
              <el-option label="文件类型" value="type" />
            </el-select>
            <el-select v-model="sortOrder" size="small" class="summary-row__sort-order-select">
              <el-option label="倒序" value="desc" />
              <el-option label="正序" value="asc" />
            </el-select>
          </span>
        </div>

        <el-table
          ref="tableRef"
          v-loading="loading"
          :data="pagedEntries"
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
                <el-tooltip v-if="scope.row.is_dir" :content="isStarred(scope.row.path) ? '取消星标' : '添加星标'" placement="top" :show-after="500">
                  <el-button
                    link
                    class="entry-star"
                    :class="{ 'is-active': isStarred(scope.row.path) }"
                    :aria-label="isStarred(scope.row.path) ? '取消星标' : '添加星标'"
                    @click.stop="toggleFolderStar(scope.row.path)"
                  >
                    <el-icon class="entry-star__icon">
                      <StarFilled v-if="isStarred(scope.row.path)" />
                      <Star v-else />
                    </el-icon>
                  </el-button>
                </el-tooltip>
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
                        <el-dropdown-item command="rename"><el-icon><EditPen /></el-icon>重命名</el-dropdown-item>
                        <el-dropdown-item command="move"><el-icon><Right /></el-icon>移动</el-dropdown-item>
                        <el-dropdown-item command="copy"><el-icon><CopyDocument /></el-icon>复制</el-dropdown-item>
                        <el-dropdown-item v-if="isArchiveEntry(scope.row)" command="extract"><el-icon><Download /></el-icon>解压到当前目录</el-dropdown-item>
                        <el-dropdown-item command="delete" divided><el-icon><Delete /></el-icon>删除</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <div class="table-pagination">
          <span class="footer-count">共 {{ filteredEntries.length }} 条</span>
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            background
            layout="sizes, prev, pager, next"
            :page-sizes="pageSizeOptions"
            :total="filteredEntries.length"
            @current-change="handleCurrentPageChange"
            @size-change="handlePageSizeChange"
          />
        </div>

        <input ref="uploadInputRef" type="file" multiple webkitdirectory directory class="file-upload-input" @change="handleUploadSelected" />
      </div>
    </el-card>

    <div
      v-if="contextMenu.visible && contextMenu.entry"
      class="context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @click.stop
    >
      <button v-if="contextMenu.entry.is_dir" type="button" class="context-menu__item" @click="openEntryFromContext"><el-icon class="context-menu__item-icon"><FolderOpened /></el-icon><span class="context-menu__item-label">打开</span></button>
      <button type="button" class="context-menu__item" @click="packEntryFromContext"><el-icon class="context-menu__item-icon"><Files /></el-icon><span class="context-menu__item-label">打包</span></button>
      <div class="context-menu__divider" />
      <button type="button" class="context-menu__item" @click="renameEntryFromContext"><el-icon class="context-menu__item-icon"><EditPen /></el-icon><span class="context-menu__item-label">重命名</span></button>
      <button type="button" class="context-menu__item" @click="moveEntryFromContext"><el-icon class="context-menu__item-icon"><Right /></el-icon><span class="context-menu__item-label">移动</span></button>
      <button type="button" class="context-menu__item" @click="copyEntryFromContext"><el-icon class="context-menu__item-icon"><CopyDocument /></el-icon><span class="context-menu__item-label">复制</span></button>
      <button v-if="isArchiveEntry(contextMenu.entry)" type="button" class="context-menu__item" @click="extractEntryFromContext"><el-icon class="context-menu__item-icon"><Download /></el-icon><span class="context-menu__item-label">解压</span></button>
      <button type="button" class="context-menu__item context-menu__item--danger" @click="deleteEntryFromContext"><el-icon class="context-menu__item-icon"><Delete /></el-icon><span class="context-menu__item-label">删除</span></button>
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
          <el-input v-model="copyTargetPath" class="path-action-input" placeholder="请输入或选择目标目录">
            <template #append>
              <el-button class="path-action-input__button" @click="openPicker('copy')">选择目录</el-button>
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
          <el-input v-model="moveTargetPath" class="path-action-input" placeholder="请输入或选择目标目录">
            <template #append>
              <el-button class="path-action-input__button" @click="openPicker('move')">选择目录</el-button>
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
          <el-input v-model="packOutputDir" class="path-action-input" placeholder="请输入或选择输出目录">
            <template #append>
              <el-button class="path-action-input__button" @click="openPicker('pack')">选择目录</el-button>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item label="归档名称">
          <el-input v-model="packArchiveName" placeholder="例如：archive.cbz，可留空自动生成" />
        </el-form-item>
        <el-form-item>
          <el-checkbox v-model="packNestSourceFolder">是否嵌套源文件夹</el-checkbox>
          <div class="pack-form__hint">勾选后会按源文件夹名称保留一层目录；取消后会直接输出到目标路径，不再额外套源文件夹。</div>
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
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Back, Clock, CopyDocument, Delete, Document, Download, EditPen, Files, Folder, FolderAdd, FolderOpened, MoreFilled, Refresh, Right, Star, StarFilled, UploadFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import {
  browseDirectories,
  collectItems,
  copyItems,
  createFolder,
  deleteItems,
  extractArchives,
  fetchBrowseRoots,
  moveItems,
  packItemsAsCBZ,
  renameItem,
  uploadFiles,
  type BrowseRoot,
  type DirectoryEntry,
} from '../api/paths'

interface FileManagerEntry extends DirectoryEntry {}

interface BreadcrumbItem {
  label: string
  path: string
}

type PickerMode = 'browse' | 'copy' | 'move' | 'pack'
type SortBy = 'modified_at' | 'name' | 'type'
type SortOrder = 'asc' | 'desc'

const STARRED_FOLDERS_STORAGE_KEY = 'nestify:file-manager:starred-folders'
const RECENT_VISITED_PATHS_STORAGE_KEY = 'nestify:file-manager:recent-visited-paths'

const directoryPath = ref('')
const directoryPickerVisible = ref(false)
const pickerMode = ref<PickerMode>('browse')
const loading = ref(false)
const extracting = ref(false)
const collecting = ref(false)
const errorMessage = ref('')
const roots = ref<BrowseRoot[]>([])
const entries = ref<FileManagerEntry[]>([])
const parentPath = ref('')
const selectedRows = ref<FileManagerEntry[]>([])
const searchKeyword = ref('')
const sortBy = ref<SortBy>('modified_at')
const sortOrder = ref<SortOrder>('desc')
const starredFolders = ref<string[]>([])
const recentVisitedPaths = ref<string[]>([])
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
const packNestSourceFolder = ref(true)
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
const canExtractSelectedArchives = computed(() => selectedRows.value.length > 0 && selectedRows.value.every((item) => isArchiveEntry(item)))
const canCollectSelectedFolders = computed(() => selectedRows.value.length > 0 && selectedRows.value.every((item) => item.is_dir))
const starredFolderSet = computed(() => new Set(starredFolders.value.map((item) => normalizePath(item))))
const sortedEntries = computed(() => [...entries.value].sort(compareEntries))
const filteredEntries = computed(() => {
	const keyword = searchKeyword.value.trim().toLowerCase()
	if (!keyword) {
		return sortedEntries.value
	}
	return sortedEntries.value.filter((entry) => entry.name.toLowerCase().includes(keyword) || entry.path.toLowerCase().includes(keyword))
})
const pageSizeOptions = [25, 50, 100]
const pageSize = ref(25)
const currentPage = ref(1)
const pagedEntries = computed(() => {
	const start = (currentPage.value - 1) * pageSize.value
	return filteredEntries.value.slice(start, start + pageSize.value)
})

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

async function openRecycleBin() {
  directoryPath.value = '/data/recycle'
  await openCurrentPath()
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

function getEntryExtension(entry: FileManagerEntry) {
  if (entry.is_dir) return 'folder'
  const index = entry.name.lastIndexOf('.')
  if (index <= 0 || index === entry.name.length - 1) {
    return ''
  }
  return entry.name.slice(index + 1).toLowerCase()
}

function compareEntries(a: FileManagerEntry, b: FileManagerEntry) {
  const starredDelta = Number(isStarred(b.path)) - Number(isStarred(a.path))
  if (starredDelta !== 0) {
    return starredDelta
  }

  if (a.is_dir !== b.is_dir) {
    return a.is_dir ? -1 : 1
  }

  let result = 0

  if (sortBy.value === 'modified_at') {
    result = compareByModifiedAt(a, b)
  } else if (sortBy.value === 'type') {
    result = getEntryExtension(a).localeCompare(getEntryExtension(b), 'zh-CN', { sensitivity: 'base' })
  } else {
    result = compareByName(a, b)
  }

  if (result !== 0) {
    return applySortOrder(result)
  }

  return compareByName(a, b)
}

function compareByName(a: FileManagerEntry, b: FileManagerEntry) {
  return a.name.localeCompare(b.name, 'zh-CN', { sensitivity: 'base', numeric: true })
}

function applySortOrder(result: number) {
  return sortOrder.value === 'asc' ? result : -result
}

function handleCurrentPageChange(page: number) {
	currentPage.value = page
}

function handlePageSizeChange(size: number) {
	pageSize.value = size
	currentPage.value = 1
}

function compareByModifiedAt(a: FileManagerEntry, b: FileManagerEntry) {
  const aTime = a.modified_at ? new Date(a.modified_at).getTime() : 0
  const bTime = b.modified_at ? new Date(b.modified_at).getTime() : 0
  if (Number.isNaN(aTime) || Number.isNaN(bTime)) {
    return b.modified_at.localeCompare(a.modified_at)
  }
  return bTime - aTime
}

function loadStarredFolders() {
  try {
    const raw = window.localStorage.getItem(STARRED_FOLDERS_STORAGE_KEY)
    if (!raw) {
      starredFolders.value = []
      return
    }

    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      starredFolders.value = []
      return
    }

    starredFolders.value = parsed.filter((item): item is string => typeof item === 'string').map((item) => normalizePath(item))
  } catch {
    starredFolders.value = []
  }
}

function loadRecentVisitedPaths() {
  try {
    const raw = window.localStorage.getItem(RECENT_VISITED_PATHS_STORAGE_KEY)
    if (!raw) {
      recentVisitedPaths.value = []
      return
    }

    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      recentVisitedPaths.value = []
      return
    }

    recentVisitedPaths.value = dedupeRecentVisitedPaths(parsed
      .filter((item): item is string => typeof item === 'string')
      .map((item) => normalizePath(item))
      .filter(Boolean)
    )
  } catch {
    recentVisitedPaths.value = []
  }
}

function getPathDepth(path: string) {
  const normalizedPath = normalizePath(path)
  if (!normalizedPath || normalizedPath === '/') {
    return 0
  }

  return normalizedPath.split('/').filter(Boolean).length
}

function isAncestorPath(ancestor: string, target: string) {
  const normalizedAncestor = normalizePath(ancestor)
  const normalizedTarget = normalizePath(target)
  if (!normalizedAncestor || !normalizedTarget || normalizedAncestor === normalizedTarget) {
    return false
  }
  return normalizedTarget.startsWith(`${normalizedAncestor}/`)
}

function dedupeRecentVisitedPaths(paths: string[]) {
  const result: string[] = []

  for (const rawPath of paths) {
    const path = normalizePath(rawPath)
    const depth = getPathDepth(path)
    if (depth < 2) {
      continue
    }

    const exactIndex = result.findIndex((item) => item === path)
    if (exactIndex >= 0) {
      result.splice(exactIndex, 1)
      result.unshift(path)
      continue
    }

    const descendantIndex = result.findIndex((item) => isAncestorPath(path, item))
    if (descendantIndex >= 0) {
      continue
    }

    const filtered = result.filter((item) => !isAncestorPath(item, path))
    result.length = 0
    result.push(path, ...filtered)

    if (result.length >= 5) {
      break
    }
  }

  return result.slice(0, 5)
}

function persistRecentVisitedPath(path: string) {
  const normalizedPath = normalizePath(path)
  if (!normalizedPath) {
    return
  }

  recentVisitedPaths.value = dedupeRecentVisitedPaths([normalizedPath, ...recentVisitedPaths.value])
  window.localStorage.setItem(RECENT_VISITED_PATHS_STORAGE_KEY, JSON.stringify(recentVisitedPaths.value))
}

function persistStarredFolders() {
  window.localStorage.setItem(STARRED_FOLDERS_STORAGE_KEY, JSON.stringify(starredFolders.value))
}

function isStarred(path: string) {
  return starredFolderSet.value.has(normalizePath(path))
}

function toggleFolderStar(path: string) {
  const targetPath = normalizePath(path)
  if (starredFolderSet.value.has(targetPath)) {
    starredFolders.value = starredFolders.value.filter((item) => normalizePath(item) !== targetPath)
  } else {
    starredFolders.value = [...starredFolders.value, targetPath]
  }
  persistStarredFolders()
}

async function handleRecentVisitedCommand(path: string) {
  if (!path || path === directoryPath.value) {
    return
  }

  await openPath(path)
}

function isArchiveEntry(entry: FileManagerEntry) {
  if (entry.is_dir) return false
  return /\.(zip|cbz)$/i.test(entry.name)
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
    const previousPath = directoryPath.value
    const response = await browseDirectories(directoryPath.value)
    directoryPath.value = response.data?.current_path ?? directoryPath.value
    parentPath.value = response.data?.parent_path ?? ''
    entries.value = response.data?.entries ?? []
    if (previousPath && previousPath !== directoryPath.value) {
      persistRecentVisitedPath(previousPath)
    }
    if (directoryPath.value) {
      persistRecentVisitedPath(directoryPath.value)
    }
    currentPage.value = 1
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
  if (command === 'extract') {
    void extractSelectedArchives(entry)
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

function extractEntryFromContext() {
  if (contextMenu.value.entry) {
    void extractSelectedArchives(contextMenu.value.entry)
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

  const relativePaths = files.map((file) => (file as File & { webkitRelativePath?: string }).webkitRelativePath || file.name)

  try {
    await uploadFiles(directoryPath.value, files, relativePaths)
    ElMessage.success(`已上传 ${files.length} 个项目`)
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
  packNestSourceFolder.value = true
  packDialogVisible.value = true
}

async function submitPackCBZ() {
  try {
    const response = await packItemsAsCBZ(
      packCandidates.value.map((item) => item.path),
      packOutputDir.value,
      packArchiveName.value,
      packNestSourceFolder.value,
    )
    ElMessage.success(`CBZ 已生成：${response.data?.output_path ?? ''}`)
    packDialogVisible.value = false
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : 'CBZ 打包失败'
  }
}

async function extractSelectedArchives(entry?: FileManagerEntry) {
  const items = getSelection(entry)
  if (items.length === 0) {
    ElMessage.warning('请先选择压缩包')
    return
  }

  const invalidItem = items.find((item) => !isArchiveEntry(item))
  if (invalidItem) {
    ElMessage.warning('仅支持批量解压 zip 或 cbz 压缩包')
    return
  }

  try {
    await ElMessageBox.confirm(`确认解压 ${items.length} 个压缩包到当前目录吗？`, '解压确认', {
      type: 'warning',
      confirmButtonText: '解压',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }

  extracting.value = true
  errorMessage.value = ''

  try {
    const response = await extractArchives(items.map((item) => item.path), directoryPath.value)
    ElMessage.success(`已解压 ${response.data?.total ?? items.length} 个压缩包`)
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '解压失败'
  } finally {
    extracting.value = false
  }
}

async function collectSelectedFolders(entry?: FileManagerEntry) {
  const items = getSelection(entry)
  if (items.length === 0) {
    ElMessage.warning('请先选择文件夹')
    return
  }

  const invalidItem = items.find((item) => !item.is_dir)
  if (invalidItem) {
    ElMessage.warning('收集功能仅支持文件夹')
    return
  }

  const removeSubfolders = ref(false)

  try {
    await ElMessageBox.confirm(
      h('div', { class: 'collect-confirm' }, [
        h('div', { class: 'collect-confirm__text' }, `确认收集 ${items.length} 个文件夹下的所有子文件到各自根目录吗？`),
        h('label', { class: 'collect-confirm__checkbox' }, [
          h('input', {
            type: 'checkbox',
            checked: removeSubfolders.value,
            onChange: (event: Event) => {
              removeSubfolders.value = (event.target as HTMLInputElement).checked
            },
          }),
          h('span', '是否删除子文件夹'),
        ]),
      ]),
      '收集确认',
      {
        type: 'warning',
        confirmButtonText: '开始收集',
        cancelButtonText: '取消',
        distinguishCancelAndClose: true,
      },
    )
  } catch {
    return
  }

  collecting.value = true
  errorMessage.value = ''

  try {
    const response = await collectItems(items.map((item) => item.path), removeSubfolders.value)
    ElMessage.success(`已完成 ${response.data?.total ?? items.length} 个文件夹的收集`)
    await openCurrentPath()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '收集失败'
  } finally {
    collecting.value = false
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

onMounted(() => {
  loadStarredFolders()
  loadRecentVisitedPaths()
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

.toolbar-row__search {
  margin-left: auto;
}

.toolbar-row__search-input {
  width: 220px;
}

.toolbar-row__split {
  width: 1px;
  height: 28px;
  background: var(--el-border-color-lighter);
}

.toolbar-action {
  transition: color 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.toolbar-action :deep(.el-icon),
.toolbar-action :deep(svg) {
  width: 18px;
  height: 18px;
}

.toolbar-action--folder:not(.is-disabled):hover,
.toolbar-action--folder:not(.is-disabled):focus-visible {
  color: #409eff;
  border-color: #b7d8ff;
  background: #edf7ff;
}

.toolbar-action--upload:not(.is-disabled):hover,
.toolbar-action--upload:not(.is-disabled):focus-visible {
  box-shadow: 0 10px 24px rgba(64, 158, 255, 0.28);
}

.toolbar-action:not(.is-disabled):hover,
.toolbar-action:not(.is-disabled):focus-visible {
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
}

.toolbar-action--move:not(.is-disabled):hover,
.toolbar-action--move:not(.is-disabled):focus-visible {
  color: #1f9d8b;
  border-color: #9be7d7;
  background: #effcf8;
}

.toolbar-action--refresh:not(.is-disabled):hover,
.toolbar-action--refresh:not(.is-disabled):focus-visible {
  color: #e67e22;
  border-color: #ffbf80;
  background: #fff3e8;
}

.toolbar-action--recent:not(.is-disabled):hover,
.toolbar-action--recent:not(.is-disabled):focus-visible {
  color: #6b7280;
  border-color: #d1d5db;
  background: #f8fafc;
}

.toolbar-action--copy:not(.is-disabled):hover,
.toolbar-action--copy:not(.is-disabled):focus-visible {
  color: #2f6fd6;
  border-color: #b7d8ff;
  background: #edf7ff;
}

.toolbar-action--extract:not(.is-disabled):hover,
.toolbar-action--extract:not(.is-disabled):focus-visible {
  color: #b7791f;
  border-color: #f5d27b;
  background: #fff8dc;
}

.toolbar-action--collect:not(.is-disabled):hover,
.toolbar-action--collect:not(.is-disabled):focus-visible {
  color: #7c52c8;
  border-color: #d9c2ff;
  background: #f5efff;
}

.toolbar-action__icon--collect {
  fill: none;
  stroke: currentColor;
  stroke-width: 1.75;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.toolbar-action--delete:not(.is-disabled):hover,
.toolbar-action--delete:not(.is-disabled):focus-visible {
  color: #e35d6a;
  border-color: #f4c2c8;
  background: #fff1f3;
}

.toolbar-action--recycle {
  padding: 8px;
}

.toolbar-action--browse:not(.is-disabled):hover,
.toolbar-action--browse:not(.is-disabled):focus-visible,
.toolbar-action--parent:not(.is-disabled):hover,
.toolbar-action--parent:not(.is-disabled):focus-visible {
  color: #409eff;
  border-color: #b7d8ff;
  background: #edf7ff;
}

.toolbar-action__icon {
  width: 18px;
  height: 18px;
  display: block;
}

.toolbar-action__icon--recycle {
  fill: none;
  stroke: currentColor;
  stroke-width: 1.75;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.toolbar-action--recycle:not(.is-disabled):hover,
.toolbar-action--recycle:not(.is-disabled):focus-visible {
  color: #e67e22;
  border-color: #ffd2a6;
  background: #fff7ed;
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
  padding: 4px 8px;
  font-size: 16px;
  color: #6b7280;
  background: transparent;
  border: 0;
  border-radius: 8px;
  cursor: pointer;
  transition: color 0.2s ease, background-color 0.2s ease;
}

.path-row__crumb::after {
  content: '/';
  margin-left: 8px;
  color: #c0c4cc;
}

.path-row__crumb:hover {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.path-row__crumb.is-current {
  color: var(--el-color-primary);
  font-weight: 600;
  background: var(--el-color-primary-light-9);
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

.summary-row__sort {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.summary-row__sort-select {
  width: 132px;
}

.summary-row__sort-order-select {
  width: 96px;
}

.entry-name {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 8px 10px;
  text-align: left;
  background: transparent;
  border: 0;
  border-radius: 12px;
  transition: background-color 0.2s ease;
}

.entry-star {
  flex-shrink: 0;
  min-width: auto;
  margin-right: -6px;
  color: #c0c4cc;
}

.entry-star.is-active,
.entry-star:hover {
  color: #f5b942;
}

.entry-star__icon {
  font-size: 18px;
  line-height: 1;
}

.entry-name.is-dir {
  cursor: pointer;
}

.entry-name:hover {
  background: var(--el-color-primary-light-9);
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
  transition: color 0.2s ease;
}

.entry-name.is-dir .entry-name__title {
  color: #1677ff;
}

.entry-name__path {
  margin-top: 4px;
  color: var(--text-secondary);
  font-size: 12px;
  transition: color 0.2s ease;
}

.entry-name:hover .entry-name__title,
.entry-name:hover .entry-name__path {
  color: var(--el-color-primary);
}

.entry-actions {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 16px;
}

.entry-actions__icon {
  font-size: 16px;
  min-width: 20px;
  padding: 0;
}

.entry-actions :deep(.el-dropdown) {
  margin-left: 4px;
}

.entry-actions__icon--warning {
  color: #f59e0b;
}

.table-pagination {
	margin-top: 16px;
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	flex-wrap: wrap;
}

.footer-count {
	color: var(--text-secondary);
	font-size: 14px;
}

.pack-form__hint {
  margin-top: 6px;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

:deep(.collect-confirm) {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

:deep(.collect-confirm__text) {
  line-height: 1.7;
}

:deep(.collect-confirm__checkbox) {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-primary);
  cursor: pointer;
}

:deep(.path-action-input .el-input-group__append) {
  padding: 0;
  overflow: hidden;
}

:deep(.path-action-input .el-input-group__append .el-button) {
  height: 100%;
  margin: 0;
  border: 0;
  border-radius: 0;
}

.path-action-input__button {
  min-width: 110px;
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
  display: flex;
  align-items: center;
  gap: 10px;
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

.context-menu__item-icon {
  flex-shrink: 0;
  font-size: 16px;
}

.context-menu__item-label {
  flex: 1;
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
