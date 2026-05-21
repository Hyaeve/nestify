<template>
  <el-card class="page-card">
    <template #header>
      <div style="display: flex; justify-content: space-between; align-items: center;">
        <span>规则管理</span>
        <el-button type="primary" @click="openCreateDialog">新增规则</el-button>
      </div>
    </template>

    <el-alert
      v-if="errorMessage"
      style="margin-bottom: 16px;"
      type="error"
      :closable="false"
      :title="errorMessage"
    />

    <el-table v-loading="loading" :data="rules" border>
      <el-table-column prop="name" label="规则名称" min-width="180" />
      <el-table-column label="归档模式" width="120">
        <template #default="scope">
          <el-tag :type="scope.row.archive_mode === 'package' ? 'success' : 'info'">
            {{ scope.row.archive_mode === 'package' ? '打包归档' : '收集归档' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="运行模式" width="120">
        <template #default="scope">
          <el-space wrap>
            <el-tag v-if="scope.row.monitor_enabled" type="success">新文件触发</el-tag>
            <el-tag v-if="scope.row.run_mode === 'cron'" type="warning">计划执行</el-tag>
            <el-tag v-if="!scope.row.monitor_enabled && scope.row.run_mode !== 'cron'" type="info">手动/单次</el-tag>
          </el-space>
        </template>
      </el-table-column>
      <el-table-column prop="source_dir" label="源路径" min-width="240" />
      <el-table-column prop="target_dir" label="目标路径" min-width="240" />
      <el-table-column label="启用状态" width="120">
        <template #default="scope">
          <el-tag :type="scope.row.enabled ? 'success' : 'info'">
            {{ scope.row.enabled ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最近结果" width="140">
        <template #default="scope">
          <span>{{ scope.row.last_run_status || '尚未执行' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140">
        <template #default="scope">
          <el-button link type="primary" @click="openEditDialog(scope.row.id)">编辑</el-button>
          <el-button link type="success" @click="prepareExecution(scope.row.id)">执行预备</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div style="margin-top: 16px; color: #909399;">
      共 {{ rules.length }} 条规则
    </div>

    <el-descriptions v-if="latestPreparedRun" style="margin-top: 16px;" :column="1" border>
      <el-descriptions-item label="最近运行 ID">{{ latestPreparedRun.id }}</el-descriptions-item>
      <el-descriptions-item label="规则名称">{{ latestPreparedRun.rule_name }}</el-descriptions-item>
      <el-descriptions-item label="当前状态">{{ latestPreparedRun.status }}</el-descriptions-item>
      <el-descriptions-item label="当前阶段">{{ latestPreparedRun.stage }}</el-descriptions-item>
      <el-descriptions-item label="执行摘要">{{ latestPreparedSummary }}</el-descriptions-item>
      <el-descriptions-item label="最近日志">
        <el-space direction="vertical" alignment="start">
          <span v-for="log in latestPreparedLogs" :key="log.id">[{{ log.level }}] {{ log.message }}</span>
        </el-space>
      </el-descriptions-item>
    </el-descriptions>
  </el-card>

  <el-dialog v-model="createDialogVisible" title="新增规则" width="640px">
    <el-form label-position="top">
      <el-form-item label="规则名称">
        <el-input v-model="createForm.name" placeholder="例如：Manga Package Rule" />
      </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="归档模式">
              <el-radio-group v-model="createForm.archive_mode" class="archive-mode-group">
                <el-radio-button value="package">打包归档</el-radio-button>
                <el-radio-button value="collect">收集归档</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="触发方式">
              <el-space wrap>
                <el-switch v-model="createForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" />
                <el-switch v-model="createForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" />
              </el-space>
            </el-form-item>
          </el-col>
        </el-row>

        <div class="mode-config-panel">
          <div class="mode-config-panel__header">
            <div>
              <div class="mode-config-panel__title">{{ getModeTitle(createForm.archive_mode) }}</div>
              <div class="mode-config-panel__description">{{ getModeDescription(createForm.archive_mode) }}</div>
            </div>
            <el-tag type="primary">当前模式</el-tag>
          </div>

          <el-row v-if="createForm.archive_mode === 'package'" :gutter="12">
            <el-col v-for="option in packageModeOptions" :key="option.key" :span="12">
              <label class="mode-option-card">
                <el-checkbox v-model="createForm.package_options[option.key]">{{ option.label }}</el-checkbox>
                <span class="mode-option-card__description">{{ option.description }}</span>
              </label>
            </el-col>
          </el-row>

          <el-row v-else :gutter="12">
            <el-col v-for="option in collectModeOptions" :key="option.key" :span="12">
              <label class="mode-option-card">
                <el-checkbox v-model="createForm.collect_options[option.key]">{{ option.label }}</el-checkbox>
                <span class="mode-option-card__description">{{ option.description }}</span>
              </label>
            </el-col>
          </el-row>
        </div>

      <el-form-item v-if="createForm.schedule_enabled" label="计划表达式">
        <el-input v-model="createForm.cron_expression" placeholder="例如：0 */30 * * * *" />
      </el-form-item>

      <el-form-item label="源路径">
        <el-input v-model="createForm.source_dir" placeholder="/library/manga">
          <template #append>
            <el-button @click="openDirectoryPicker('create', 'source_dir')">选择目录</el-button>
          </template>
        </el-input>
      </el-form-item>

      <el-form-item label="目标路径">
        <el-input v-model="createForm.target_dir" placeholder="/archive/manga">
          <template #append>
            <el-button @click="openDirectoryPicker('create', 'target_dir')">选择目录</el-button>
          </template>
        </el-input>
      </el-form-item>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="启用规则">
            <el-switch v-model="createForm.enabled" />
          </el-form-item>
        </el-col>

        <el-col :span="12">
          <el-form-item label="立即运行一次（启动后）">
            <el-switch v-model="createForm.run_on_start" />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>

    <template #footer>
      <el-button @click="createDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="creating" @click="submitCreateRule">创建</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="editDialogVisible" title="编辑规则" width="640px">
    <el-form label-position="top">
      <el-form-item label="规则名称">
        <el-input v-model="editForm.name" placeholder="例如：Manga Package Rule" />
      </el-form-item>

        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="归档模式">
              <el-radio-group v-model="editForm.archive_mode" class="archive-mode-group">
                <el-radio-button value="package">打包归档</el-radio-button>
                <el-radio-button value="collect">收集归档</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>

          <el-col :span="12">
            <el-form-item label="触发方式">
              <el-space wrap>
                <el-switch v-model="editForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" />
                <el-switch v-model="editForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" />
              </el-space>
            </el-form-item>
          </el-col>
        </el-row>

        <div class="mode-config-panel">
          <div class="mode-config-panel__header">
            <div>
              <div class="mode-config-panel__title">{{ getModeTitle(editForm.archive_mode) }}</div>
              <div class="mode-config-panel__description">{{ getModeDescription(editForm.archive_mode) }}</div>
            </div>
            <el-tag type="primary">当前模式</el-tag>
          </div>

          <el-row v-if="editForm.archive_mode === 'package'" :gutter="12">
            <el-col v-for="option in packageModeOptions" :key="option.key" :span="12">
              <label class="mode-option-card">
                <el-checkbox v-model="editForm.package_options[option.key]">{{ option.label }}</el-checkbox>
                <span class="mode-option-card__description">{{ option.description }}</span>
              </label>
            </el-col>
          </el-row>

          <el-row v-else :gutter="12">
            <el-col v-for="option in collectModeOptions" :key="option.key" :span="12">
              <label class="mode-option-card">
                <el-checkbox v-model="editForm.collect_options[option.key]">{{ option.label }}</el-checkbox>
                <span class="mode-option-card__description">{{ option.description }}</span>
              </label>
            </el-col>
          </el-row>
        </div>

      <el-form-item v-if="editForm.schedule_enabled" label="计划表达式">
        <el-input v-model="editForm.cron_expression" placeholder="例如：0 */30 * * * *" />
      </el-form-item>

      <el-form-item label="源路径">
        <el-input v-model="editForm.source_dir" placeholder="/library/manga">
          <template #append>
            <el-button @click="openDirectoryPicker('edit', 'source_dir')">选择目录</el-button>
          </template>
        </el-input>
      </el-form-item>

      <el-form-item label="目标路径">
        <el-input v-model="editForm.target_dir" placeholder="/archive/manga">
          <template #append>
            <el-button @click="openDirectoryPicker('edit', 'target_dir')">选择目录</el-button>
          </template>
        </el-input>
      </el-form-item>

      <el-row :gutter="16">
        <el-col :span="12">
          <el-form-item label="启用规则">
            <el-switch v-model="editForm.enabled" />
          </el-form-item>
        </el-col>

        <el-col :span="12">
          <el-form-item label="立即运行一次（启动后）">
            <el-switch v-model="editForm.run_on_start" />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>

    <template #footer>
      <el-button @click="editDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="editing" @click="submitUpdateRule">保存</el-button>
    </template>
  </el-dialog>

  <DirectoryPickerDialog
    v-model="directoryPickerVisible"
    title="选择目录"
    :initial-path="directoryPickerInitialPath"
    @selected="applyDirectorySelection"
  />
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'

import { fetchRunLogs, prepareRuleExecution, type RunInstance, type RunLogEntry } from '../api/executions'
import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import { createRule, fetchRule, fetchRules, updateRule, type RuleItem } from '../api/rules'

type ArchiveMode = 'package' | 'collect'
type PackageOptionKey =
  | 'preserve_structure'
  | 'include_manifest'
  | 'verify_after_archive'
  | 'cleanup_source_after_archive'
type CollectOptionKey =
  | 'recursive_collect'
  | 'deduplicate_same_name'
  | 'keep_latest_only'
  | 'collect_related_files'

const packageModeOptions: Array<{ key: PackageOptionKey; label: string; description: string }> = [
  { key: 'preserve_structure', label: '保留目录结构', description: '按源目录层级打包归档，避免目标目录结构混乱。' },
  { key: 'include_manifest', label: '生成归档清单', description: '为每次打包写入清单，方便后续核对归档内容。' },
  { key: 'verify_after_archive', label: '归档后校验', description: '完成后再次检查结果，减少丢包或缺失问题。' },
  { key: 'cleanup_source_after_archive', label: '成功后清理源文件', description: '确认已归档后再清理原目录中的已处理文件。' },
]

const collectModeOptions: Array<{ key: CollectOptionKey; label: string; description: string }> = [
  { key: 'recursive_collect', label: '递归收集子目录', description: '自动扫描所有子目录，把符合条件的文件一并收集。' },
  { key: 'deduplicate_same_name', label: '同名文件去重', description: '遇到重复文件时自动跳过重复项，减少覆盖冲突。' },
  { key: 'keep_latest_only', label: '仅保留最新文件', description: '存在多个版本时优先保留最新文件。' },
  { key: 'collect_related_files', label: '收集关联文件', description: '在收集主文件时同步带上相关说明或附属文件。' },
]

function createDefaultPackageOptions(): Record<PackageOptionKey, boolean> {
  return {
    preserve_structure: true,
    include_manifest: true,
    verify_after_archive: true,
    cleanup_source_after_archive: false,
  }
}

function createDefaultCollectOptions(): Record<CollectOptionKey, boolean> {
  return {
    recursive_collect: true,
    deduplicate_same_name: true,
    keep_latest_only: false,
    collect_related_files: true,
  }
}

function parseOptionJSON<T extends Record<string, boolean>>(raw: string | undefined, defaults: T): T {
  if (!raw) {
    return { ...defaults }
  }

  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const normalized = { ...defaults }

    for (const key of Object.keys(defaults)) {
      if (typeof parsed[key] === 'boolean') {
        normalized[key as keyof T] = parsed[key] as T[keyof T]
      }
    }

    return normalized
  } catch {
    return { ...defaults }
  }
}

function getModeTitle(mode: ArchiveMode) {
  return mode === 'package' ? '打包归档功能' : '收集归档功能'
}

function getModeDescription(mode: ArchiveMode) {
  return mode === 'package'
    ? '选择打包归档后，下面会展开当前模式专属的规则功能，可按需勾选。'
    : '选择收集归档后，下面会展开当前模式专属的规则功能，可按需勾选。'
}

const loading = ref(false)
const creating = ref(false)
const editing = ref(false)
const errorMessage = ref('')
const createDialogVisible = ref(false)
const editDialogVisible = ref(false)
const editingRuleID = ref<number | null>(null)
const directoryPickerVisible = ref(false)
const directoryPickerInitialPath = ref('')
const directoryPickerTarget = ref<'create.source_dir' | 'create.target_dir' | 'edit.source_dir' | 'edit.target_dir' | null>(null)
const latestPreparedRun = ref<RunInstance | null>(null)
const latestPreparedLogs = ref<RunLogEntry[]>([])
const latestPreparedSummary = ref('')
const rules = ref<RuleItem[]>([])

const createForm = reactive({
  name: '',
  description: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: false,
  archive_mode: 'package' as ArchiveMode,
  source_dir: '',
  target_dir: '',
  cron_expression: '',
  watch_debounce_ms: 2000,
  run_on_start: true,
  package_options: createDefaultPackageOptions(),
  collect_options: createDefaultCollectOptions(),
})

const editForm = reactive({
  name: '',
  description: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: false,
  archive_mode: 'package' as ArchiveMode,
  source_dir: '',
  target_dir: '',
  cron_expression: '',
  watch_debounce_ms: 2000,
  run_on_start: true,
  package_options: createDefaultPackageOptions(),
  collect_options: createDefaultCollectOptions(),
})

function openCreateDialog() {
  resetCreateForm()
  createDialogVisible.value = true
}

function resolveRunMode(monitorEnabled: boolean, scheduleEnabled: boolean): 'watch' | 'cron' | 'once' {
  if (scheduleEnabled) {
    return 'cron'
  }

  if (monitorEnabled) {
    return 'watch'
  }

  return 'once'
}

async function loadRules() {
  loading.value = true
  errorMessage.value = ''

  try {
    const response = await fetchRules()
    rules.value = response.data?.items ?? []
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则列表加载失败'
  } finally {
    loading.value = false
  }
}

async function submitCreateRule() {
  creating.value = true
  errorMessage.value = ''

  try {
    await createRule({
      name: createForm.name,
      description: createForm.description,
      enabled: createForm.enabled,
      monitor_enabled: createForm.monitor_enabled,
      archive_mode: createForm.archive_mode,
      run_mode: resolveRunMode(createForm.monitor_enabled, createForm.schedule_enabled),
      source_dir: createForm.source_dir,
      target_dir: createForm.target_dir,
      watch_debounce_ms: createForm.watch_debounce_ms,
      cron_expression: createForm.schedule_enabled ? createForm.cron_expression : '',
      run_on_start: createForm.run_on_start,
      package_options: { ...createForm.package_options },
      collect_options: { ...createForm.collect_options },
    })

    ElMessage.success('规则创建成功')
    createDialogVisible.value = false
    resetCreateForm()
    await loadRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则创建失败'
  } finally {
    creating.value = false
  }
}

function resetCreateForm() {
  createForm.name = ''
  createForm.description = ''
  createForm.enabled = true
  createForm.monitor_enabled = true
  createForm.schedule_enabled = false
  createForm.archive_mode = 'package'
  createForm.source_dir = ''
  createForm.target_dir = ''
  createForm.cron_expression = ''
  createForm.watch_debounce_ms = 2000
  createForm.run_on_start = true
  createForm.package_options = createDefaultPackageOptions()
  createForm.collect_options = createDefaultCollectOptions()
}

function resetEditForm() {
  editForm.name = ''
  editForm.description = ''
  editForm.enabled = true
  editForm.monitor_enabled = true
  editForm.schedule_enabled = false
  editForm.archive_mode = 'package'
  editForm.source_dir = ''
  editForm.target_dir = ''
  editForm.cron_expression = ''
  editForm.watch_debounce_ms = 2000
  editForm.run_on_start = true
  editForm.package_options = createDefaultPackageOptions()
  editForm.collect_options = createDefaultCollectOptions()
}

function openDirectoryPicker(form: 'create' | 'edit', field: 'source_dir' | 'target_dir') {
  directoryPickerTarget.value = `${form}.${field}` as 'create.source_dir' | 'create.target_dir' | 'edit.source_dir' | 'edit.target_dir'

  if (form === 'create') {
    directoryPickerInitialPath.value = field === 'source_dir' ? createForm.source_dir : createForm.target_dir
  } else {
    directoryPickerInitialPath.value = field === 'source_dir' ? editForm.source_dir : editForm.target_dir
  }

  directoryPickerVisible.value = true
}

function applyDirectorySelection(path: string) {
  switch (directoryPickerTarget.value) {
    case 'create.source_dir':
      createForm.source_dir = path
      break
    case 'create.target_dir':
      createForm.target_dir = path
      break
    case 'edit.source_dir':
      editForm.source_dir = path
      break
    case 'edit.target_dir':
      editForm.target_dir = path
      break
    default:
      break
  }

  directoryPickerVisible.value = false
}

async function openEditDialog(id: number) {
  editing.value = true
  errorMessage.value = ''

  try {
    const response = await fetchRule(id)
    const rule = response.data

    if (!rule) {
      throw new Error('规则详情不存在')
    }

    editingRuleID.value = rule.id
    editForm.name = rule.name
    editForm.description = rule.description
    editForm.enabled = rule.enabled
    editForm.monitor_enabled = rule.monitor_enabled
    editForm.schedule_enabled = rule.run_mode === 'cron'
    editForm.archive_mode = rule.archive_mode
    editForm.source_dir = rule.source_dir
    editForm.target_dir = rule.target_dir
    editForm.cron_expression = rule.cron_expression
    editForm.watch_debounce_ms = rule.watch_debounce_ms
    editForm.run_on_start = rule.run_on_start
    editForm.package_options = parseOptionJSON(rule.package_options_json, createDefaultPackageOptions())
    editForm.collect_options = parseOptionJSON(rule.collect_options_json, createDefaultCollectOptions())

    editDialogVisible.value = true
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则详情加载失败'
  } finally {
    editing.value = false
  }
}

async function submitUpdateRule() {
  if (!editingRuleID.value) {
    errorMessage.value = '缺少规则 ID'
    return
  }

  editing.value = true
  errorMessage.value = ''

  try {
    await updateRule(editingRuleID.value, {
      name: editForm.name,
      description: editForm.description,
      enabled: editForm.enabled,
      monitor_enabled: editForm.monitor_enabled,
      archive_mode: editForm.archive_mode,
      run_mode: resolveRunMode(editForm.monitor_enabled, editForm.schedule_enabled),
      source_dir: editForm.source_dir,
      target_dir: editForm.target_dir,
      watch_debounce_ms: editForm.watch_debounce_ms,
      cron_expression: editForm.schedule_enabled ? editForm.cron_expression : '',
      run_on_start: editForm.run_on_start,
      package_options: { ...editForm.package_options },
      collect_options: { ...editForm.collect_options },
    })

    ElMessage.success('规则更新成功')
    editDialogVisible.value = false
    editingRuleID.value = null
    resetEditForm()
    await loadRules()
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '规则更新失败'
  } finally {
    editing.value = false
  }
}

async function prepareExecution(ruleID: number) {
  loading.value = true
  errorMessage.value = ''

  try {
    const response = await prepareRuleExecution(ruleID, 'once')
    latestPreparedRun.value = response.data?.run ?? null
    latestPreparedSummary.value = response.data?.prepared?.summary ?? ''

    if (latestPreparedRun.value) {
      const logsResponse = await fetchRunLogs(latestPreparedRun.value.id)
      latestPreparedLogs.value = logsResponse.data?.items ?? []
    }

    ElMessage.success('规则执行预备完成')
  } catch (error) {
    latestPreparedRun.value = null
    latestPreparedLogs.value = []
    latestPreparedSummary.value = ''
    errorMessage.value = error instanceof Error ? error.message : '规则执行预备失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  void loadRules()
})
</script>

<style scoped>
.archive-mode-group {
  width: 100%;
}

.archive-mode-group :deep(.el-radio-button),
.archive-mode-group :deep(.el-radio-button__inner) {
  width: 50%;
}

.mode-config-panel {
  margin-bottom: 18px;
  padding: 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-fill-color-extra-light);
}

.mode-config-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.mode-config-panel__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.mode-config-panel__description {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}

.mode-option-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-height: 92px;
  padding: 14px 16px;
  margin-bottom: 12px;
  border: 1px solid var(--el-border-color);
  border-radius: 10px;
  background: var(--el-bg-color);
  cursor: pointer;
}

.mode-option-card:hover {
  border-color: var(--el-color-primary-light-5);
}

.mode-option-card__description {
  padding-left: 24px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}
</style>

