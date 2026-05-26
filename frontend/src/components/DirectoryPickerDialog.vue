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

      <div class="picker-hint">采用折叠目录树逐级展开；若因权限或挂载限制无法展开，仍可手动输入路径继续选择。</div>

      <div class="picker-actions">
        <el-button size="small" :disabled="!parentPath" @click="goParent">上级目录</el-button>
        <el-button size="small" :disabled="!currentPath" @click="reloadCurrent">刷新</el-button>
      </div>
    </div>

    <div v-loading="loading" class="picker-tree">
      <el-tree
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
          <div class="picker-tree__node">
            <span class="picker-tree__label">{{ data.label }}</span>
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

import { browseDirectories, fetchBrowseRoots, validateDirectory, type BrowseRoot, type DirectoryEntry } from '../api/paths'

interface TreeNode {
  label: string
  path: string
  leaf: boolean
  parentPath: string
}

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

    const startPath = props.initialPath || currentPath.value || roots.value[0]?.path || ''
    if (startPath) {
      await populateRootLevel(startPath)
      pathInput.value = startPath
      await openPath(startPath)
    } else {
      treeData.value = roots.value.map((root) => ({
        label: root.name,
        path: root.path,
        leaf: false,
        parentPath: '',
      }))
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '目录根路径加载失败'
  } finally {
    loading.value = false
  }
}

function mapDirectoryEntries(entries: DirectoryEntry[], parentPath: string) {
  return entries
    .filter((entry: DirectoryEntry) => entry.is_dir)
    .map((entry: DirectoryEntry) => ({
      label: entry.name,
      path: entry.path,
      leaf: !entry.has_children,
      parentPath,
    }))
}

function findRootPath(path: string) {
  const targetPath = path.trim()
  if (!targetPath) {
    return ''
  }

  const directRoot = roots.value.find((root) => root.path === targetPath)
  if (directRoot) {
    return directRoot.path
  }

  if (roots.value.length === 1) {
    return roots.value[0]?.path ?? ''
  }

  return ''
}

async function populateRootLevel(path: string) {
  const rootPath = findRootPath(path)
  if (!rootPath) {
    treeData.value = roots.value.map((root) => ({
      label: root.name,
      path: root.path,
      leaf: false,
      parentPath: '',
    }))
    return
  }

  const response = await browseDirectories(rootPath)
  treeData.value = mapDirectoryEntries(response.data?.entries ?? [], response.data?.current_path ?? rootPath)
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
    const response = await browseDirectories(targetPath)
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
    const response = await browseDirectories(currentNode.path)
    const children = mapDirectoryEntries(response.data?.entries ?? [], response.data?.current_path ?? currentNode.path)
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

.picker-label {
  color: #606266;
  font-size: 14px;
  min-width: 88px;
}

.picker-hint {
  color: #909399;
  font-size: 13px;
}

.picker-tree {
  min-height: 420px;
  max-height: 520px;
  overflow: auto;
  padding: 8px 12px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
}

.picker-tree__node {
  display: flex;
  align-items: center;
  min-width: 0;
  padding: 6px 0;
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
