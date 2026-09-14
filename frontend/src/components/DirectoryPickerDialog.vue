<template>
  <el-dialog v-model="visibleProxy" :title="title" width="760px">
    <el-alert
      v-if="errorMessage"
      style="margin-bottom: 16px;"
      type="error"
      :closable="false"
      :title="errorMessage"
    />

    <div class="picker-toolbar">
      <div class="picker-source">
        <span class="picker-label">目录来源</span>
        <div class="picker-source__group" role="group" aria-label="目录来源">
          <button
            v-for="option in sourceOptions"
            :key="option.value"
            type="button"
            class="picker-source__button"
            :class="{ 'is-active': sourceFilter === option.value }"
            @click="switchSource(option.value)"
          >
            <el-icon class="picker-source__icon"><component :is="option.icon" /></el-icon>
            <span>{{ option.label }}</span>
          </button>
        </div>
        <span class="picker-source__count">本地 {{ localRoots.length }} · 远程 {{ mountRoots.length }}</span>
      </div>

      <div class="picker-current">
        <span class="picker-label">当前目录</span>
        <el-tag type="info">{{ currentPath || '未选择' }}</el-tag>
      </div>

      <div class="picker-input">
        <span class="picker-label">手动路径</span>
        <el-input v-model="pathInput" placeholder="挂载目录无法枚举时，可直接输入路径">
          <template #append>
            <el-button @click="openTypedPath">打开</el-button>
          </template>
        </el-input>
      </div>

      <div class="picker-actions">
        <el-button size="small" :disabled="!parentPath" @click="goParent">上级目录</el-button>
        <el-button size="small" :disabled="!currentPath" @click="reloadCurrent">刷新</el-button>
      </div>
    </div>

    <div v-loading="loading" class="picker-tree">
      <div v-if="!treeData.length && !loading" class="picker-empty">
        {{ emptyHint }}
      </div>
      <el-tree
        v-show="Boolean(treeData.length)"
        ref="treeRef"
        node-key="path"
        :data="treeData"
        :props="treeProps"
        lazy
        :expand-on-click-node="false"
        :load="loadNode"
        @node-click="handleNodeClick"
      >
        <template #default="{ data }">
          <div class="picker-tree__node" :class="{ 'is-mount': data.isMount }">
            <el-icon class="picker-tree__icon">
              <Cloudy v-if="data.isMount" />
              <FolderOpened v-else />
            </el-icon>
            <span class="picker-tree__label">{{ data.label }}</span>
            <span v-if="data.isMount" class="picker-tree__badge">远程</span>
          </div>
        </template>
      </el-tree>
    </div>

    <template #footer>
      <el-button @click="visibleProxy = false">取消</el-button>
      <el-button type="primary" :disabled="!currentPath" :loading="confirming" @click="confirmSelection">选择当前目录</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Cloudy, FolderOpened, Monitor } from '@element-plus/icons-vue'

import { browseAnyDirectory, fetchBrowseRoots, isMountPath, validateDirectory, type BrowseRoot, type DirectoryEntry } from '../api/paths'

interface TreeNode {
  label: string
  path: string
  leaf: boolean
  parentPath: string
  /** 该节点位于 WebDAV 远程挂载内（用于区分图标）。 */
  isMount?: boolean
}

type SourceFilter = 'all' | 'local' | 'mount'

const sourceOptions: Array<{ value: SourceFilter; label: string; icon: unknown }> = [
  { value: 'all', label: '全部来源', icon: FolderOpened },
  { value: 'local', label: '本地目录', icon: Monitor },
  { value: 'mount', label: '远程挂载', icon: Cloudy },
]

const props = defineProps<{
  modelValue: boolean
  title?: string
  initialPath?: string
}>()

const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void
  (event: 'selected', value: string): void
}>()

const loading = ref(false)
const confirming = ref(false)
const errorMessage = ref('')
const roots = ref<BrowseRoot[]>([])
const sourceFilter = ref<SourceFilter>('all')
const currentPath = ref('')
const parentPath = ref('')
const pathInput = ref('')
const treeData = ref<TreeNode[]>([])
const treeRef = ref()

const treeProps = {
  label: 'label',
  children: 'children',
  isLeaf: 'leaf',
}

const visibleProxy = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit('update:modelValue', value),
})

/** 本地物理目录根。 */
const localRoots = computed(() => roots.value.filter((root) => !isMountPath(root.path)))
/** WebDAV 远程挂载根。 */
const mountRoots = computed(() => roots.value.filter((root) => isMountPath(root.path)))
/** 按「目录来源」筛选后的根列表。 */
const filteredRoots = computed(() => {
  if (sourceFilter.value === 'local') {
    return localRoots.value
  }
  if (sourceFilter.value === 'mount') {
    return mountRoots.value
  }
  return roots.value
})

const emptyHint = computed(() => {
  if (sourceFilter.value === 'mount' && !mountRoots.value.length) {
    return '尚未配置远程挂载，可在「系统设置 → 远程挂载」添加'
  }
  if (sourceFilter.value === 'local' && !localRoots.value.length) {
    return '未发现可用的本地目录'
  }
  return '暂无可选目录'
})

function normalizePath(path: string) {
  return path.replace(/\\/g, '/').replace(/\/+$/, '') || '/'
}

function isWithinRoot(rootPath: string, target: string) {
  const root = normalizePath(rootPath)
  const value = normalizePath(target)
  return value === root || value.startsWith(`${root}/`) || root === '/'
}

function rootTreeNodes() {
  return filteredRoots.value.map((root) => ({
    label: root.name,
    path: root.path,
    leaf: false,
    parentPath: '',
    isMount: isMountPath(root.path),
  }))
}

/** 切换「本地目录 / 远程挂载」来源：当前目录仍属新来源则保留，否则回到根列表。 */
function switchSource(value: SourceFilter) {
  if (sourceFilter.value === value) {
    return
  }
  sourceFilter.value = value

  const stillVisible = currentPath.value
    && filteredRoots.value.some((root) => isWithinRoot(root.path, currentPath.value))
  if (stillVisible) {
    treeData.value = rootTreeNodes()
    treeRef.value?.setCurrentKey?.(currentPath.value)
    return
  }

  treeData.value = rootTreeNodes()
  currentPath.value = ''
  parentPath.value = ''
  pathInput.value = ''
}

watch(
  () => props.modelValue,
  (value) => {
    if (value) {
      void initialize()
    }
  },
)

async function initialize() {
  loading.value = true
  errorMessage.value = ''

  try {
    const response = await fetchBrowseRoots()
    roots.value = response.data?.items ?? []

    // 打开时按「上次用过的来源」复位：初始路径是挂载路径就默认远程，否则保留当前筛选。
    if (!currentPath.value && props.initialPath) {
      sourceFilter.value = isMountPath(props.initialPath) ? 'mount' : 'all'
    }

    const startPath = props.initialPath || currentPath.value || ''
    if (startPath) {
      await populateRootLevel(startPath)
      pathInput.value = startPath
      await openPath(startPath)
    } else {
      treeData.value = rootTreeNodes()
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '目录根路径加载失败'
  } finally {
    loading.value = false
  }
}

function mapDirectoryEntries(entries: DirectoryEntry[], parentPath: string, remote = false) {
  return entries
    .filter((entry: DirectoryEntry) => entry.is_dir)
    .map((entry: DirectoryEntry) => ({
      label: entry.name,
      path: entry.path,
      leaf: !entry.has_children,
      parentPath,
      isMount: remote,
    }))
}

function findRootPath(path: string) {
  const targetPath = path.trim()
  if (!targetPath) {
    return ''
  }

  const directRoot = filteredRoots.value.find((root) => root.path === targetPath)
  if (directRoot) {
    return directRoot.path
  }

  if (filteredRoots.value.length === 1) {
    return filteredRoots.value[0]?.path ?? ''
  }

  return ''
}

async function populateRootLevel(path: string) {
  const rootPath = findRootPath(path)
  if (!rootPath) {
    treeData.value = rootTreeNodes()
    return
  }

  const response = await browseAnyDirectory(rootPath)
  treeData.value = mapDirectoryEntries(
    response.data?.entries ?? [],
    response.data?.current_path ?? rootPath,
    isMountPath(rootPath),
  )
}

async function openPath(path: string) {
  const targetPath = path.trim()
  if (!targetPath) {
    return
  }

  loading.value = true
  errorMessage.value = ''
  currentPath.value = targetPath
  pathInput.value = targetPath

  try {
    await populateRootLevel(targetPath)
    const response = await browseAnyDirectory(targetPath)
    currentPath.value = response.data?.current_path ?? ''
    parentPath.value = response.data?.parent_path ?? ''
    pathInput.value = currentPath.value
    treeRef.value?.setCurrentKey?.(currentPath.value)
  } catch (error) {
    parentPath.value = ''
    errorMessage.value = error instanceof Error ? error.message : '目录浏览失败'
  } finally {
    loading.value = false
  }
}

async function loadNode(node: { level: number; data?: TreeNode }, resolve: (data: TreeNode[]) => void) {
  if (node.level === 0) {
    resolve(treeData.value)
    return
  }

  const currentNode = node.data
  if (!currentNode?.path) {
    resolve([])
    return
  }

  try {
    const response = await browseAnyDirectory(currentNode.path)
    const remote = Boolean(currentNode.isMount) || isMountPath(currentNode.path)
    const children = mapDirectoryEntries(
      response.data?.entries ?? [],
      response.data?.current_path ?? currentNode.path,
      remote,
    )
    resolve(children)
  } catch {
    resolve([])
  }
}

function handleNodeClick(data: TreeNode) {
  currentPath.value = data.path
  parentPath.value = data.parentPath
  pathInput.value = data.path
}

async function openTypedPath() {
	await openPath(pathInput.value)
}

async function reloadCurrent() {
  if (!currentPath.value) return
  await initialize()
}

async function goParent() {
  if (!parentPath.value) return
  await openPath(parentPath.value)
}

async function confirmSelection() {
  if (!currentPath.value) return

  confirming.value = true
  errorMessage.value = ''

  try {
    const response = await validateDirectory(currentPath.value)
    const validation = response.data
    if (validation?.allowed && validation.exists && validation.is_dir) {
      emit('selected', currentPath.value)
      visibleProxy.value = false
      return
    }

    emit('selected', currentPath.value)
    visibleProxy.value = false
  } catch (error) {
    emit('selected', currentPath.value)
    visibleProxy.value = false
  } finally {
    confirming.value = false
  }
}
</script>

<style scoped lang="scss">
.picker-toolbar {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 16px;
}

.picker-source {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.picker-source__group {
  display: inline-flex;
  align-items: center;
  padding: 3px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: #f5f7fa;
}

.picker-source__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: #5b6676;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.4;
  cursor: pointer;
  transition: color 0.16s ease, background-color 0.16s ease, box-shadow 0.16s ease;
}

.picker-source__button:hover {
  color: #2f6f4f;
}

.picker-source__button.is-active {
  color: #2f6f4f;
  background: #ffffff;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.1);
}

.picker-source__icon {
  font-size: 15px;
}

.picker-source__count {
  color: #8a94a6;
  font-size: 12px;
}

.picker-current,
.picker-input,
.picker-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.picker-input :deep(.el-input) {
  flex: 1;
  min-width: 280px;
}

.picker-input :deep(.el-input-group__append) {
  padding: 0;
  overflow: hidden;
  border-top-right-radius: 12px;
  border-bottom-right-radius: 12px;
}

.picker-input :deep(.el-input-group__append .el-button) {
  min-width: 92px;
  height: 100%;
  margin: 0;
  padding: 0 20px;
  border: 0;
  border-radius: 0;
}

.picker-input :deep(.el-input__wrapper) {
  border-top-right-radius: 0;
  border-bottom-right-radius: 0;
}

.picker-label {
  color: #606266;
  font-size: 14px;
  min-width: 88px;
}

.picker-tree {
  min-height: 420px;
  max-height: 520px;
  overflow: auto;
  padding: 8px 12px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 16px;
  background: #fff;
}

.picker-tree__node {
  display: flex;
  align-items: center;
  min-width: 0;
  padding: 6px 0;
}

.picker-tree__icon {
  margin-right: 6px;
  color: #c79a54;
  font-size: 15px;
}

.picker-tree__node.is-mount .picker-tree__icon {
  color: #5f7fa8;
}

.picker-tree__badge {
  margin-left: 8px;
  padding: 1px 7px;
  border-radius: 999px;
  color: #5f7fa8;
  background: rgba(95, 127, 168, 0.12);
  font-size: 11px;
  font-weight: 700;
}

.picker-empty {
  padding: 32px 0;
  text-align: center;
  color: #a0a7b4;
  font-size: 13px;
}

.picker-tree__label {
  color: var(--el-text-color-primary);
  line-height: 1.4;
}

.picker-tree :deep(.el-tree-node__content) {
  height: auto;
  min-height: 36px;
  align-items: center;
  padding: 4px 0;
}

.picker-tree :deep(.el-tree-node__expand-icon) {
  margin-top: 0;
}
</style>
