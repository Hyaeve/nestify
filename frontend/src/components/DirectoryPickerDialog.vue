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
      <div class="picker-roots">
        <span class="picker-label">可浏览根目录</span>
        <el-space wrap>
          <el-button
            v-for="root in roots"
            :key="root.path"
            size="small"
            @click="openPath(root.path)"
          >
            {{ root.name }}
          </el-button>
        </el-space>
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

      <div class="picker-hint">若目录列表因权限受限无法展开，仍可直接选择当前路径继续执行。</div>

      <div class="picker-actions">
        <el-button size="small" :disabled="!parentPath" @click="goParent">上级目录</el-button>
        <el-button size="small" :disabled="!currentPath" @click="reloadCurrent">刷新</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="entries" border>
      <el-table-column prop="name" label="目录名" min-width="260" />
      <el-table-column label="子目录" width="120">
        <template #default="scope">
          <el-tag :type="scope.row.has_children ? 'success' : 'info'">
            {{ scope.row.has_children ? '有' : '无' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120">
        <template #default="scope">
          <el-button link type="primary" @click="openPath(scope.row.path)">进入</el-button>
        </template>
      </el-table-column>
    </el-table>

    <template #footer>
      <el-button @click="visibleProxy = false">取消</el-button>
      <el-button type="primary" :disabled="!currentPath" :loading="confirming" @click="confirmSelection">选择当前目录</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import { browseDirectories, fetchBrowseRoots, validateDirectory, type BrowseRoot, type DirectoryEntry } from '../api/paths'

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
const entries = ref<DirectoryEntry[]>([])
const currentPath = ref('')
const parentPath = ref('')
const pathInput = ref('')

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
      pathInput.value = startPath
      await openPath(startPath)
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '目录根路径加载失败'
  } finally {
    loading.value = false
  }
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
    const response = await browseDirectories(targetPath)
    currentPath.value = response.data?.current_path ?? ''
    parentPath.value = response.data?.parent_path ?? ''
    entries.value = response.data?.entries ?? []
    pathInput.value = currentPath.value
  } catch (error) {
    parentPath.value = ''
    entries.value = []
    errorMessage.value = error instanceof Error ? error.message : '目录浏览失败'
  } finally {
    loading.value = false
  }
}

async function openTypedPath() {
	await openPath(pathInput.value)
}

async function reloadCurrent() {
  if (!currentPath.value) return
  await openPath(currentPath.value)
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

.picker-roots,
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
</style>
