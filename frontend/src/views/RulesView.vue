<template>
  <div class="rules-page">
    <div class="rules-tabs">
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'rules' }" @click="activeTab = 'rules'">归档规则</button>
      <button type="button" class="rules-tabs__item" :class="{ 'is-active': activeTab === 'history' }" @click="activeTab = 'history'">归档历史</button>
    </div>

    <el-alert v-if="errorMessage" :closable="false" type="error" :title="errorMessage" class="rules-error" />

    <el-card v-show="activeTab === 'rules'" class="page-card rules-card">
      <template #header>
        <div class="rules-card__header">
          <div>
            <div class="rules-card__title">归档规则</div>
          </div>
          <el-button type="primary" round @click="openCreateDialog">+ 添加规则</el-button>
        </div>
      </template>

      <el-table v-loading="loading" :data="rules">
        <el-table-column prop="name" label="规则名称" min-width="140" />
        <el-table-column label="模式" width="100">
          <template #default="scope">
            <el-tag type="info" effect="plain">{{ scope.row.archive_mode === 'package' ? 'CBZ' : '普通' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="整理方式" width="100">
          <template #default="scope">
            <el-tag effect="plain">{{ scope.row.archive_mode === 'package' ? '打包' : '移动' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="执行模式" width="110">
          <template #default="scope">
            <el-tag :type="scope.row.compatibility_mode === 'compatibility' ? 'warning' : 'success'" effect="plain">
              {{ scope.row.compatibility_mode === 'compatibility' ? '兼容模式' : '本地模式' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="source_dir" label="源路径" min-width="280" show-overflow-tooltip />
        <el-table-column prop="target_dir" label="目标路径" min-width="280" show-overflow-tooltip />
        <el-table-column label="监控" width="88">
          <template #default="scope">
            <el-tag :type="scope.row.monitor_enabled ? 'success' : 'info'" effect="plain">{{ scope.row.monitor_enabled ? '开' : '关' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="88">
          <template #default="scope">
            <el-tag :type="scope.row.enabled ? 'success' : 'info'">{{ scope.row.enabled ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="170" fixed="right">
          <template #default="scope">
            <el-button link type="primary" @click="openEditDialog(scope.row.id)">编辑</el-button>
            <el-button link type="success" @click="prepareExecution(scope.row.id)">整理</el-button>
            <el-button link type="danger" @click="removeRule(scope.row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card v-show="activeTab === 'history'" class="page-card history-card">
      <template #header>
        <div class="rules-card__header">
          <div class="rules-card__title">归档历史</div>
          <div class="history-actions">
            <el-button type="success" plain @click="clearHistory('success')">删除成功</el-button>
            <el-button type="warning" plain @click="clearHistory('skip')">删除跳过</el-button>
            <el-button type="danger" plain @click="clearHistory('failed')">删除失败</el-button>
          </div>
        </div>
      </template>

      <div class="history-summary">
        <span>成功 {{ successCount }}</span>
        <span>跳过 {{ skipCount }}</span>
        <span>失败 {{ failedCount }}</span>
      </div>

      <el-table :data="historyItems">
        <el-table-column label="规则 / 摘要" min-width="360">
          <template #default="scope">
            <div class="history-rule">
              <div class="history-rule__title">{{ scope.row.rule_name || '未知规则' }}</div>
              <div class="history-rule__desc">{{ scope.row.summary || latestPreparedSummary }}</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="scope">
            <span class="history-status" :class="`is-${scope.row.status}`">{{ scope.row.status }}</span>
          </template>
        </el-table-column>
        <el-table-column label="统计" width="140">
          <template #default="scope">{{ scope.row.success_count }}/{{ scope.row.skip_count }}/{{ scope.row.failure_count }}</template>
        </el-table-column>
        <el-table-column label="时间" min-width="180">
          <template #default="scope">{{ formatDateTime(scope.row.started_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90">
          <template #default="scope">
            <el-button link type="danger" @click="removeHistoryItem(scope.$index)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="createDialogVisible" title="新增规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="createForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="归档模式"><el-radio-group v-model="createForm.archive_mode" class="archive-mode-group"><el-radio-button value="package">打包归档</el-radio-button><el-radio-button value="collect">收集归档</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="createForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="createForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="createForm.compatibility_mode">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
          <div class="mode-config-panel__description">本地模式适用于本机目录操作；兼容模式用于挂载网盘等兼容性场景。</div>
        </el-form-item>
        <div class="mode-config-panel">
          <div class="mode-config-panel__header"><div><div class="mode-config-panel__title">{{ getModeTitle(createForm.archive_mode) }}</div><div class="mode-config-panel__description">{{ getModeDescription(createForm.archive_mode) }}</div></div><el-tag type="primary">当前模式</el-tag></div>
          <el-row v-if="createForm.archive_mode === 'package'" :gutter="12"><el-col v-for="option in packageModeOptions" :key="option.key" :span="12"><label class="mode-option-card"><el-checkbox v-model="createForm.package_options[option.key]">{{ option.label }}</el-checkbox><span class="mode-option-card__description">{{ option.description }}</span></label></el-col></el-row>
          <el-row v-else :gutter="12"><el-col v-for="option in collectModeOptions" :key="option.key" :span="12"><label class="mode-option-card"><el-checkbox v-model="createForm.collect_options[option.key]">{{ option.label }}</el-checkbox><span class="mode-option-card__description">{{ option.description }}</span></label></el-col></el-row>
        </div>
        <el-form-item v-if="createForm.schedule_enabled" label="计划表达式"><el-input v-model="createForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="createForm.source_dir"><template #append><el-button @click="openDirectoryPicker('create', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="createForm.target_dir"><template #append><el-button @click="openDirectoryPicker('create', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="createForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="createForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="createDialogVisible = false">取消</el-button><el-button type="primary" :loading="creating" @click="submitCreateRule">创建</el-button></template>
    </el-dialog>

    <el-dialog v-model="editDialogVisible" title="编辑规则" width="640px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="editForm.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12"><el-form-item label="归档模式"><el-radio-group v-model="editForm.archive_mode" class="archive-mode-group"><el-radio-button value="package">打包归档</el-radio-button><el-radio-button value="collect">收集归档</el-radio-button></el-radio-group></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="触发方式"><el-space wrap><el-switch v-model="editForm.monitor_enabled" inline-prompt active-text="新文件触发" inactive-text="新文件触发" /><el-switch v-model="editForm.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" /></el-space></el-form-item></el-col>
        </el-row>
        <el-form-item label="执行适配模式">
          <el-radio-group v-model="editForm.compatibility_mode">
            <el-radio-button value="local">本地模式</el-radio-button>
            <el-radio-button value="compatibility">兼容模式</el-radio-button>
          </el-radio-group>
          <div class="mode-config-panel__description">本地模式适用于本机目录操作；兼容模式用于挂载网盘等兼容性场景。</div>
        </el-form-item>
        <div class="mode-config-panel">
          <div class="mode-config-panel__header"><div><div class="mode-config-panel__title">{{ getModeTitle(editForm.archive_mode) }}</div><div class="mode-config-panel__description">{{ getModeDescription(editForm.archive_mode) }}</div></div><el-tag type="primary">当前模式</el-tag></div>
          <el-row v-if="editForm.archive_mode === 'package'" :gutter="12"><el-col v-for="option in packageModeOptions" :key="option.key" :span="12"><label class="mode-option-card"><el-checkbox v-model="editForm.package_options[option.key]">{{ option.label }}</el-checkbox><span class="mode-option-card__description">{{ option.description }}</span></label></el-col></el-row>
          <el-row v-else :gutter="12"><el-col v-for="option in collectModeOptions" :key="option.key" :span="12"><label class="mode-option-card"><el-checkbox v-model="editForm.collect_options[option.key]">{{ option.label }}</el-checkbox><span class="mode-option-card__description">{{ option.description }}</span></label></el-col></el-row>
        </div>
        <el-form-item v-if="editForm.schedule_enabled" label="计划表达式"><el-input v-model="editForm.cron_expression" /></el-form-item>
        <el-form-item label="源路径"><el-input v-model="editForm.source_dir"><template #append><el-button @click="openDirectoryPicker('edit', 'source_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-form-item label="目标路径"><el-input v-model="editForm.target_dir"><template #append><el-button @click="openDirectoryPicker('edit', 'target_dir')">选择目录</el-button></template></el-input></el-form-item>
        <el-row :gutter="16"><el-col :span="12"><el-form-item label="启用规则"><el-switch v-model="editForm.enabled" /></el-form-item></el-col><el-col :span="12"><el-form-item label="立即运行一次（启动后）"><el-switch v-model="editForm.run_on_start" /></el-form-item></el-col></el-row>
      </el-form>
      <template #footer><el-button @click="editDialogVisible = false">取消</el-button><el-button type="primary" :loading="editing" @click="submitUpdateRule">保存</el-button></template>
    </el-dialog>

    <DirectoryPickerDialog v-model="directoryPickerVisible" title="选择目录" :initial-path="directoryPickerInitialPath" @selected="applyDirectorySelection" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'
import { fetchRunLogs, prepareRuleExecution } from '../api/executions'
import { createRule, deleteRule, fetchRule, fetchRules, updateRule, type RuleItem } from '../api/rules'
import { emptyRunHistory, fetchRunHistory, type RunHistoryItem } from '../api/runHistory'

type ArchiveMode = 'package' | 'collect'
type PackageOptionKey = 'preserve_structure' | 'include_manifest' | 'verify_after_archive' | 'cleanup_source_after_archive' | 'package_nested_folders'
type CollectOptionKey = 'recursive_collect' | 'deduplicate_same_name' | 'keep_latest_only' | 'collect_related_files'
type HistoryStatus = 'success' | 'skip' | 'failed'

const packageModeOptions = [
  { key: 'preserve_structure', label: '保留目录结构', description: '按源目录层级打包归档，避免目标目录结构混乱。' },
  { key: 'include_manifest', label: '生成归档清单', description: '为每次打包写入清单，方便后续核对归档内容。' },
  { key: 'verify_after_archive', label: '归档后校验', description: '完成后再次检查结果，减少丢包或缺失问题。' },
  { key: 'cleanup_source_after_archive', label: '成功后清理源文件', description: '确认已归档后再清理原目录中的已处理文件。' },
  { key: 'package_nested_folders', label: '打包嵌套子目录', description: '遇到多层子文件夹时，按原有层级在归档目录内生成对应 CBZ；关闭时默认跳过并记录。' },
] as const

const collectModeOptions = [
  { key: 'recursive_collect', label: '递归收集子目录', description: '自动扫描所有子目录，把符合条件的文件一并收集。' },
  { key: 'deduplicate_same_name', label: '同名文件去重', description: '遇到重复文件时自动跳过重复项，减少覆盖冲突。' },
  { key: 'keep_latest_only', label: '仅保留最新文件', description: '存在多个版本时优先保留最新文件。' },
  { key: 'collect_related_files', label: '收集关联文件', description: '在收集主文件时同步带上相关说明或附属文件。' },
] as const

function createDefaultPackageOptions(): Record<PackageOptionKey, boolean> {
  return { preserve_structure: true, include_manifest: true, verify_after_archive: true, cleanup_source_after_archive: false, package_nested_folders: false }
}
function createDefaultCollectOptions(): Record<CollectOptionKey, boolean> {
  return { recursive_collect: true, deduplicate_same_name: true, keep_latest_only: false, collect_related_files: true }
}
function parseOptionJSON<T extends Record<string, boolean>>(raw: string | undefined, defaults: T): T {
  if (!raw) return { ...defaults }
  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>
    const normalized = { ...defaults }
    for (const key of Object.keys(defaults)) {
      if (typeof parsed[key] === 'boolean') normalized[key as keyof T] = parsed[key] as T[keyof T]
    }
    return normalized
  } catch {
    return { ...defaults }
  }
}
function getModeTitle(mode: ArchiveMode) { return mode === 'package' ? '打包归档功能' : '收集归档功能' }
function getModeDescription(mode: ArchiveMode) { return mode === 'package' ? '选择打包归档后，下面会展开当前模式专属的规则功能，可按需勾选。' : '选择收集归档后，下面会展开当前模式专属的规则功能，可按需勾选。' }
function resolveRunMode(monitorEnabled: boolean, scheduleEnabled: boolean): 'watch' | 'cron' | 'once' { if (scheduleEnabled) return 'cron'; if (monitorEnabled) return 'watch'; return 'once' }
function formatDateTime(value: string) { return new Date(value).toLocaleString('zh-CN', { hour12: false }) }

const activeTab = ref<'rules' | 'history'>('rules')
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
const latestPreparedSummary = ref('')
const rules = ref<RuleItem[]>([])
const historyItems = ref<RunHistoryItem[]>(emptyRunHistory())

const successCount = computed(() => historyItems.value.filter((item) => item.status === 'success').length)
const skipCount = computed(() => historyItems.value.filter((item) => item.status === 'skip').length)
const failedCount = computed(() => historyItems.value.filter((item) => item.status === 'failed').length)

const createForm = reactive({ name: '', description: '', enabled: true, monitor_enabled: true, schedule_enabled: false, compatibility_mode: 'local' as 'local' | 'compatibility', archive_mode: 'package' as ArchiveMode, source_dir: '', target_dir: '', cron_expression: '', watch_debounce_ms: 2000, run_on_start: true, package_options: createDefaultPackageOptions(), collect_options: createDefaultCollectOptions() })
const editForm = reactive({ name: '', description: '', enabled: true, monitor_enabled: true, schedule_enabled: false, compatibility_mode: 'local' as 'local' | 'compatibility', archive_mode: 'package' as ArchiveMode, source_dir: '', target_dir: '', cron_expression: '', watch_debounce_ms: 2000, run_on_start: true, package_options: createDefaultPackageOptions(), collect_options: createDefaultCollectOptions() })
createForm.compatibility_mode = 'local'

function resetCreateForm() { createForm.name = ''; createForm.description = ''; createForm.enabled = true; createForm.monitor_enabled = true; createForm.schedule_enabled = false; createForm.compatibility_mode = 'local'; createForm.archive_mode = 'package'; createForm.source_dir = ''; createForm.target_dir = ''; createForm.cron_expression = ''; createForm.watch_debounce_ms = 2000; createForm.run_on_start = true; createForm.package_options = createDefaultPackageOptions(); createForm.collect_options = createDefaultCollectOptions() }
function resetEditForm() { editForm.name = ''; editForm.description = ''; editForm.enabled = true; editForm.monitor_enabled = true; editForm.schedule_enabled = false; editForm.compatibility_mode = 'local'; editForm.archive_mode = 'package'; editForm.source_dir = ''; editForm.target_dir = ''; editForm.cron_expression = ''; editForm.watch_debounce_ms = 2000; editForm.run_on_start = true; editForm.package_options = createDefaultPackageOptions(); editForm.collect_options = createDefaultCollectOptions() }
function openCreateDialog() { resetCreateForm(); createDialogVisible.value = true }
function openDirectoryPicker(form: 'create' | 'edit', field: 'source_dir' | 'target_dir') { directoryPickerTarget.value = `${form}.${field}` as 'create.source_dir' | 'create.target_dir' | 'edit.source_dir' | 'edit.target_dir'; directoryPickerInitialPath.value = form === 'create' ? (field === 'source_dir' ? createForm.source_dir : createForm.target_dir) : (field === 'source_dir' ? editForm.source_dir : editForm.target_dir); directoryPickerVisible.value = true }
function applyDirectorySelection(path: string) { if (directoryPickerTarget.value === 'create.source_dir') createForm.source_dir = path; if (directoryPickerTarget.value === 'create.target_dir') createForm.target_dir = path; if (directoryPickerTarget.value === 'edit.source_dir') editForm.source_dir = path; if (directoryPickerTarget.value === 'edit.target_dir') editForm.target_dir = path; directoryPickerVisible.value = false }

async function loadRules() { loading.value = true; errorMessage.value = ''; try { rules.value = (await fetchRules()).data?.items ?? [] } catch (error) { errorMessage.value = error instanceof Error ? error.message : '规则列表加载失败' } finally { loading.value = false } }
async function loadHistory() { try { historyItems.value = (await fetchRunHistory()).data?.items ?? [] } catch (error) { errorMessage.value = error instanceof Error ? error.message : '历史记录加载失败' } }
async function submitCreateRule() { creating.value = true; errorMessage.value = ''; try { await createRule({ name: createForm.name, description: createForm.description, enabled: createForm.enabled, monitor_enabled: createForm.monitor_enabled, compatibility_mode: createForm.compatibility_mode, archive_mode: createForm.archive_mode, run_mode: resolveRunMode(createForm.monitor_enabled, createForm.schedule_enabled), source_dir: createForm.source_dir, target_dir: createForm.target_dir, watch_debounce_ms: createForm.watch_debounce_ms, cron_expression: createForm.schedule_enabled ? createForm.cron_expression : '', run_on_start: createForm.run_on_start, package_options: { ...createForm.package_options }, collect_options: { ...createForm.collect_options } }); ElMessage.success('规则创建成功'); createDialogVisible.value = false; resetCreateForm(); await loadRules() } catch (error) { errorMessage.value = error instanceof Error ? error.message : '规则创建失败' } finally { creating.value = false } }
async function openEditDialog(id: number) { editing.value = true; errorMessage.value = ''; try { const rule = (await fetchRule(id)).data; if (!rule) throw new Error('规则详情不存在'); editingRuleID.value = rule.id; editForm.name = rule.name; editForm.description = rule.description; editForm.enabled = rule.enabled; editForm.monitor_enabled = rule.monitor_enabled; editForm.schedule_enabled = rule.run_mode === 'cron'; editForm.compatibility_mode = rule.compatibility_mode || 'local'; editForm.archive_mode = rule.archive_mode; editForm.source_dir = rule.source_dir; editForm.target_dir = rule.target_dir; editForm.cron_expression = rule.cron_expression; editForm.watch_debounce_ms = rule.watch_debounce_ms; editForm.run_on_start = rule.run_on_start; editForm.package_options = parseOptionJSON(rule.package_options_json, createDefaultPackageOptions()); editForm.collect_options = parseOptionJSON(rule.collect_options_json, createDefaultCollectOptions()); editDialogVisible.value = true } catch (error) { errorMessage.value = error instanceof Error ? error.message : '规则详情加载失败' } finally { editing.value = false } }
async function submitUpdateRule() { if (!editingRuleID.value) { errorMessage.value = '缺少规则 ID'; return }; editing.value = true; errorMessage.value = ''; try { await updateRule(editingRuleID.value, { name: editForm.name, description: editForm.description, enabled: editForm.enabled, monitor_enabled: editForm.monitor_enabled, compatibility_mode: editForm.compatibility_mode, archive_mode: editForm.archive_mode, run_mode: resolveRunMode(editForm.monitor_enabled, editForm.schedule_enabled), source_dir: editForm.source_dir, target_dir: editForm.target_dir, watch_debounce_ms: editForm.watch_debounce_ms, cron_expression: editForm.schedule_enabled ? editForm.cron_expression : '', run_on_start: editForm.run_on_start, package_options: { ...editForm.package_options }, collect_options: { ...editForm.collect_options } }); ElMessage.success('规则更新成功'); editDialogVisible.value = false; editingRuleID.value = null; resetEditForm(); await loadRules() } catch (error) { errorMessage.value = error instanceof Error ? error.message : '规则更新失败' } finally { editing.value = false } }
async function prepareExecution(ruleID: number) { loading.value = true; errorMessage.value = ''; try { const response = await prepareRuleExecution(ruleID, 'once'); const run = response.data?.run; latestPreparedSummary.value = response.data?.prepared?.summary ?? ''; if (run) { const logsResponse = await fetchRunLogs(run.id); const items = logsResponse.data?.items ?? []; const summary = items.map((item) => item.message).join(' | ') || latestPreparedSummary.value; historyItems.value.unshift({ id: run.id, rule_id: run.rule_id, rule_name: run.rule_name || '未知规则', trigger_mode: run.trigger_mode, archive_mode: run.archive_mode, summary, status: run.failure_count > 0 || run.status === 'failed' ? 'failed' : run.skip_count > 0 ? 'skip' : 'success', processed_files: run.processed_files, success_count: run.success_count, skip_count: run.skip_count, failure_count: run.failure_count, started_at: run.started_at, updated_at: run.updated_at, finished_at: run.finished_at }) } ElMessage.success('规则执行完成'); activeTab.value = 'history'; await loadRules(); await loadHistory() } catch (error) { errorMessage.value = error instanceof Error ? error.message : '规则执行失败' } finally { loading.value = false } }
async function removeRule(id: number) { try { await ElMessageBox.confirm('确认删除该规则？', '删除规则', { type: 'warning' }); await deleteRule(id); ElMessage.success('规则已删除'); await loadRules() } catch (error) { if (error === 'cancel') return; errorMessage.value = error instanceof Error ? error.message : '删除规则失败' } }
function clearHistory(status: HistoryStatus) { historyItems.value = historyItems.value.filter((item) => item.status !== status) }
function removeHistoryItem(index: number) { historyItems.value.splice(index, 1) }

onMounted(() => { void loadRules(); void loadHistory() })
</script>

<style scoped>
.rules-page { display: flex; flex-direction: column; gap: 16px; }
.rules-tabs { display: flex; gap: 32px; padding: 0 4px; border-bottom: 1px solid var(--el-border-color-lighter); }
.rules-tabs__item { position: relative; padding: 12px 0; font-size: 15px; background: transparent; border: 0; cursor: pointer; color: var(--el-text-color-regular); }
.rules-tabs__item.is-active { color: var(--el-color-primary); font-weight: 600; }
.rules-tabs__item.is-active::after { content: ''; position: absolute; left: 0; right: 0; bottom: -1px; height: 3px; background: var(--el-color-primary); border-radius: 999px; }
.rules-error { margin-bottom: 4px; }
.rules-card__header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.rules-card__title { font-size: 18px; font-weight: 700; color: var(--el-text-color-primary); }
.history-actions { display: flex; gap: 8px; }
.history-summary { display: flex; justify-content: flex-end; gap: 12px; margin-bottom: 12px; color: var(--el-text-color-secondary); }
.history-rule { display: flex; flex-direction: column; gap: 6px; }
.history-rule__title { font-weight: 600; color: var(--el-text-color-primary); }
.history-rule__desc { line-height: 1.6; color: var(--el-text-color-secondary); }
.history-status { display: inline-flex; align-items: center; justify-content: center; min-width: 68px; padding: 6px 10px; border-radius: 10px; border: 2px solid currentColor; font-weight: 700; transform: rotate(-8deg); }
.history-status.is-success { color: #22c55e; }
.history-status.is-skip { color: #f59e0b; }
.history-status.is-failed { color: #ef4444; }
.archive-mode-group { width: 100%; }
.archive-mode-group :deep(.el-radio-button), .archive-mode-group :deep(.el-radio-button__inner) { width: 50%; }
.mode-config-panel { margin-bottom: 18px; padding: 16px; border: 1px solid var(--el-border-color-light); border-radius: 12px; background: var(--el-fill-color-extra-light); }
.mode-config-panel__header { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; margin-bottom: 12px; }
.mode-config-panel__title { font-size: 14px; font-weight: 600; color: var(--el-text-color-primary); }
.mode-config-panel__description { margin-top: 4px; font-size: 12px; line-height: 1.5; color: var(--el-text-color-secondary); }
.mode-option-card { display: flex; flex-direction: column; gap: 6px; min-height: 92px; padding: 14px 16px; margin-bottom: 12px; border: 1px solid var(--el-border-color); border-radius: 10px; background: var(--el-bg-color); cursor: pointer; }
.mode-option-card:hover { border-color: var(--el-color-primary-light-5); }
.mode-option-card__description { padding-left: 24px; font-size: 12px; line-height: 1.5; color: var(--el-text-color-secondary); }
</style>
