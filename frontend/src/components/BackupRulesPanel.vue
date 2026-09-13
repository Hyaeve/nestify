<template>
  <el-card v-show="visible" class="page-card rules-card backup-rules-card">
    <template #header>
      <div class="rules-card__header">
        <div class="rules-card__title">备份规则</div>
        <el-button type="primary" round @click="openCreateWizard">+ 添加备份</el-button>
      </div>
    </template>

    <el-empty v-if="!loading && backups.length === 0" description="暂无备份规则，点击右上角「添加备份」创建" />

    <div v-else class="backup-grid">
      <div v-for="task in backups" :key="task.id" class="backup-card" :class="{ 'backup-card--disabled': !task.enabled }">
        <div class="backup-card__head">
          <div class="backup-card__name" :title="task.name">{{ task.name }}</div>
          <span class="backup-card__status-dot" :class="{ 'is-on': task.enabled }"></span>
        </div>

        <div class="backup-card__body">
          <div class="backup-card__meta">
            <div class="backup-card__meta-label">源路径</div>
            <div class="backup-card__meta-value" v-for="src in task.source_dirs" :key="src" :title="src">{{ src }}</div>
          </div>
          <div class="backup-card__meta">
            <div class="backup-card__meta-label">目标路径</div>
            <div class="backup-card__meta-value" v-for="dst in task.target_dirs" :key="dst" :title="dst">{{ dst }}</div>
          </div>

          <div class="backup-card__tags">
            <span class="backup-card__tag" :class="{ 'is-on': task.monitor_enabled }">实时监控</span>
            <span class="backup-card__tag">{{ completionRuleLabel(task.completion_rule) }}</span>
            <span class="backup-card__tag">{{ replaceRuleLabel(task.replace_rule) }}</span>
            <span v-if="task.force_full_scan" class="backup-card__tag">强制完整扫描</span>
            <span v-if="task.scan_interval_seconds > 0" class="backup-card__tag">间隔 {{ task.scan_interval_seconds }}s</span>
            <span v-if="task.cron_expression" class="backup-card__tag">计划 {{ task.cron_expression }}</span>
            <span v-if="task.filter_rules.length" class="backup-card__tag">筛选 {{ task.filter_rules.length }} 条</span>
          </div>

          <div class="backup-card__last">
            <span class="backup-card__last-label">上次备份</span>
            <span class="backup-card__last-value">{{ formatTime(task.last_backup_at) }}</span>
          </div>
        </div>

        <div class="backup-card__footer">
          <el-switch
            :model-value="task.enabled"
            inline-prompt
            active-text="启用"
            inactive-text="禁用"
            @change="toggleEnabled(task)"
          />
          <div class="backup-card__footer-actions">
            <el-tooltip content="重新扫描（完整扫描）" placement="top">
              <el-button link class="backup-card__action" :disabled="scanningIds.has(task.id)" @click="rescan(task)">
                <svg viewBox="0 0 24 24" aria-hidden="true" class="backup-card__action-icon"><path d="M4.5 12a7.5 7.5 0 0 1 13.2-4.7l2.05 2.2" /><path d="M19.75 5.5v4h-4" /><path d="M19.5 12a7.5 7.5 0 0 1-13.2 4.7l-2.05-2.2" /><path d="M4.25 18.5v-4h4" /></svg>
              </el-button>
            </el-tooltip>
            <el-tooltip content="设置" placement="top">
              <el-button link class="backup-card__action" @click="openEditWizard(task)">
                <el-icon><Setting /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="状态详情" placement="top">
              <el-button link class="backup-card__action" @click="openStatusDialog(task)">
                <el-icon><InfoFilled /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="移除" placement="top">
              <el-button link class="backup-card__action backup-card__action--danger" @click="removeTask(task)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-tooltip>
          </div>
        </div>
      </div>
    </div>

    <!-- 四步向导 -->
    <el-dialog
      v-model="wizardVisible"
      :title="wizardEditing ? '编辑备份规则' : '添加备份规则'"
      width="720px"
      destroy-on-close
      @closed="resetWizard"
    >
      <div class="backup-wizard-steps">
        <div
          v-for="(step, index) in wizardSteps"
          :key="index"
          class="backup-wizard-step"
          :class="{ 'is-active': wizardStep === index, 'is-done': wizardStep > index }"
        >
          <span class="backup-wizard-step__dot">{{ wizardStep > index ? '✓' : index + 1 }}</span>
          <span class="backup-wizard-step__label">{{ step }}</span>
        </div>
      </div>

      <!-- 第 1 步：基本设置 -->
      <div v-show="wizardStep === 0" class="backup-wizard-body">
        <el-form label-position="top">
          <el-form-item label="备份任务名称">
            <el-input v-model="wizardForm.name" placeholder="例如：电视剧自动备份" maxlength="64" />
          </el-form-item>
          <el-form-item label="源路径（可多个）">
            <div class="backup-path-list">
              <div v-for="(path, index) in wizardForm.source_dirs" :key="`src-${index}`" class="backup-path-row">
                <el-input v-model="wizardForm.source_dirs[index]" placeholder="选择源路径">
                  <template #append>
                    <el-button @click="pickPath('source', index)">选择</el-button>
                  </template>
                </el-input>
                <el-button link class="backup-path-row__remove" @click="removePath('source', index)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <el-button text type="primary" @click="addPath('source')">+ 添加源路径</el-button>
            </div>
          </el-form-item>
          <el-form-item label="目标路径（可多个）">
            <div class="backup-path-list">
              <div v-for="(path, index) in wizardForm.target_dirs" :key="`dst-${index}`" class="backup-path-row">
                <el-input v-model="wizardForm.target_dirs[index]" placeholder="选择目标路径">
                  <template #append>
                    <el-button @click="pickPath('target', index)">选择</el-button>
                  </template>
                </el-input>
                <el-button link class="backup-path-row__remove" @click="removePath('target', index)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <el-button text type="primary" @click="addPath('target')">+ 添加目标路径</el-button>
            </div>
          </el-form-item>
        </el-form>
      </div>

      <!-- 第 2 步：备份规则 -->
      <div v-show="wizardStep === 1" class="backup-wizard-body">
        <el-form label-position="top">
          <el-form-item>
            <el-checkbox v-model="wizardForm.monitor_enabled">文件系统监视器</el-checkbox>
            <div class="backup-form-hint">实时监控源文件夹的变化，检测到变更时自动触发备份。</div>
          </el-form-item>
          <el-form-item label="完成规则">
            <el-radio-group v-model="wizardForm.completion_rule">
              <el-radio value="none">无操作</el-radio>
              <el-radio value="delete_source">删除源文件</el-radio>
              <el-radio value="delete_source_dir">删除源文件和空文件夹</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="替换规则">
            <el-radio-group v-model="wizardForm.replace_rule">
              <el-radio value="skip">跳过</el-radio>
              <el-radio value="overwrite">覆盖</el-radio>
            </el-radio-group>
            <div class="backup-form-hint">目标中已存在同名文件时的处理方式。</div>
          </el-form-item>
        </el-form>
      </div>

      <!-- 第 3 步：扫描规则 -->
      <div v-show="wizardStep === 2" class="backup-wizard-body">
        <el-form label-position="top">
          <el-form-item>
            <el-checkbox v-model="wizardForm.sync_delete_from_target">从目标同步删除</el-checkbox>
            <div class="backup-form-hint">从源文件夹删除文件时同步删除目标中的文件。</div>
          </el-form-item>
          <el-form-item>
            <el-checkbox v-model="wizardForm.force_full_scan">启动时强制完整扫描</el-checkbox>
            <div class="backup-form-hint">备份启动时执行完整扫描而不是增量扫描。</div>
          </el-form-item>
          <el-form-item label="自动扫描间隔">
            <el-input-number v-model="wizardForm.scan_interval_seconds" :min="0" :max="86400" />
            <div class="backup-form-hint">按此间隔自动扫描更改。0 = 禁用。</div>
          </el-form-item>
          <el-form-item label="Cron 表达式（计划扫描）">
            <el-input v-model="wizardForm.cron_expression" placeholder="填写则启用，不填写则不启用。例如 0 3 * * *" />
          </el-form-item>
        </el-form>
      </div>

      <!-- 第 4 步：筛选规则 -->
      <div v-show="wizardStep === 3" class="backup-wizard-body">
        <el-dropdown trigger="click" @command="addFilterRule">
          <el-button type="primary" plain>+ 添加规则</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="name">名称</el-dropdown-item>
              <el-dropdown-item command="extension">扩展名</el-dropdown-item>
              <el-dropdown-item command="regex">正则</el-dropdown-item>
              <el-dropdown-item command="size">体积</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>

        <div class="backup-filter-list">
          <div v-for="(rule, index) in wizardForm.filter_rules" :key="rule.id" class="backup-filter-row">
            <div class="backup-filter-row__main">
              <span class="backup-filter-row__type">{{ filterTypeLabel(rule.type) }}</span>
              <div class="backup-filter-row__value">
                <template v-if="rule.type === 'size'">
                  <el-input-number v-model="rule.min_size" :min="0" placeholder="最小" />
                  <el-select v-model="rule.size_unit" style="width: 100px">
                    <el-option label="Bytes" value="Bytes" />
                    <el-option label="KB" value="KB" />
                    <el-option label="MB" value="MB" />
                    <el-option label="GB" value="GB" />
                  </el-select>
                  <span class="backup-filter-row__sep">—</span>
                  <el-input-number v-model="rule.max_size" :min="0" placeholder="最大（留空不限制）" />
                </template>
                <template v-else>
                  <el-input v-model="rule.value" :placeholder="filterValuePlaceholder(rule.type)" />
                </template>
              </div>
            </div>
            <div class="backup-filter-row__opts">
              <el-checkbox-button v-model="rule.blacklist">黑名单</el-checkbox-button>
              <el-checkbox-button v-model="rule.whitelist">白名单</el-checkbox-button>
              <el-checkbox-button v-model="rule.match_dir">文件夹</el-checkbox-button>
              <el-checkbox-button v-model="rule.match_file">文件</el-checkbox-button>
              <el-button link class="backup-filter-row__remove" @click="removeFilterRule(index)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
          </div>
          <el-empty v-if="wizardForm.filter_rules.length === 0" :image-size="60" description="尚未添加筛选规则" />
        </div>
      </div>

      <template #footer>
        <div class="backup-wizard-footer">
          <el-button v-if="wizardStep > 0" @click="wizardStep -= 1">上一步</el-button>
          <el-button @click="wizardVisible = false">取消</el-button>
          <el-button v-if="wizardStep < 3" type="primary" @click="nextStep">下一步</el-button>
          <el-button v-else type="primary" :loading="saving" @click="applyWizard">应用</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 状态详情 -->
    <el-dialog v-model="statusDialogVisible" :title="`备份状态：${statusSnapshot.task_name || ''}`" width="560px">
      <div v-if="statusLoading" class="backup-status-loading">加载中…</div>
      <div v-else class="backup-status">
        <div class="backup-status__line">
          <span class="backup-status__label">状态</span>
          <el-tag :type="statusTagType(statusSnapshot.status)" size="small">{{ statusLabel(statusSnapshot.status) }}</el-tag>
        </div>
        <div class="backup-status__line">
          <span class="backup-status__label">阶段</span>
          <span>{{ statusSnapshot.phase || '—' }}</span>
        </div>
        <div v-if="statusSnapshot.progress" class="backup-status__line">
          <span class="backup-status__label">进度</span>
          <span>{{ statusSnapshot.progress }}</span>
        </div>
        <div class="backup-status__stats">
          <div class="backup-status__stat"><span>扫描</span><b>{{ statusSnapshot.scanned }}</b></div>
          <div class="backup-status__stat"><span>复制</span><b>{{ statusSnapshot.copied }}</b></div>
          <div class="backup-status__stat"><span>跳过</span><b>{{ statusSnapshot.skipped }}</b></div>
          <div class="backup-status__stat"><span>删除</span><b>{{ statusSnapshot.deleted }}</b></div>
          <div class="backup-status__stat"><span>失败</span><b>{{ statusSnapshot.failed }}</b></div>
        </div>
        <div class="backup-status__line">
          <span class="backup-status__label">上次备份</span>
          <span>{{ formatTime(statusSnapshot.last_backup_at) }}</span>
        </div>
        <div v-if="statusSnapshot.recent_logs?.length" class="backup-status__logs">
          <div class="backup-status__logs-title">最近日志</div>
          <div v-for="(log, index) in statusSnapshot.recent_logs" :key="index" class="backup-status__log">{{ log }}</div>
        </div>
      </div>
      <template #footer>
        <el-button @click="statusDialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="refreshStatus">刷新</el-button>
      </template>
    </el-dialog>

    <DirectoryPickerDialog v-model="pickerVisible" title="选择目录" :initial-path="pickerInitialPath" @selected="applyPathSelection" />
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, InfoFilled, Setting } from '@element-plus/icons-vue'

import DirectoryPickerDialog from './DirectoryPickerDialog.vue'
import {
  createBackup,
  deleteBackup,
  fetchBackupStatus,
  fetchBackups,
  runBackup,
  setBackupEnabled,
  updateBackup,
  type BackupCompletionRule,
  type BackupFilterRule,
  type BackupFilterType,
  type BackupReplaceRule,
  type BackupStatusSnapshot,
  type BackupTask,
} from '../api/backups'

const props = defineProps<{ visible: boolean }>()

const loading = ref(false)
const backups = ref<BackupTask[]>([])
const scanningIds = ref<Set<number>>(new Set())

// —— 向导 ——
const wizardSteps = ['基本设置', '备份规则', '扫描规则', '筛选规则']
const wizardVisible = ref(false)
const wizardEditing = ref(false)
const wizardEditingId = ref<number | null>(null)
const wizardStep = ref(0)
const saving = ref(false)

interface WizardForm {
  name: string
  source_dirs: string[]
  target_dirs: string[]
  monitor_enabled: boolean
  completion_rule: BackupCompletionRule
  replace_rule: BackupReplaceRule
  sync_delete_from_target: boolean
  force_full_scan: boolean
  scan_interval_seconds: number
  cron_expression: string
  filter_rules: BackupFilterRule[]
}

function emptyWizardForm(): WizardForm {
  return {
    name: '',
    source_dirs: [''],
    target_dirs: [''],
    monitor_enabled: true,
    completion_rule: 'none',
    replace_rule: 'skip',
    sync_delete_from_target: false,
    force_full_scan: false,
    scan_interval_seconds: 0,
    cron_expression: '',
    filter_rules: [],
  }
}

const wizardForm = reactive<WizardForm>(emptyWizardForm())

// —— 目录选择 ——
const pickerVisible = ref(false)
const pickerInitialPath = ref('')
let pickerTarget: { field: 'source' | 'target'; index: number } | null = null

// —— 状态详情 ——
const statusDialogVisible = ref(false)
const statusLoading = ref(false)
const statusSnapshot = ref<BackupStatusSnapshot>({
  task_id: 0,
  task_name: '',
  running: false,
  status: 'idle',
  phase: '',
  progress: '',
  scanned: 0,
  copied: 0,
  skipped: 0,
  deleted: 0,
  failed: 0,
  last_backup_at: '',
  recent_logs: [],
})
let statusTaskId: number | null = null

onMounted(() => {
  void loadBackups()
})

async function loadBackups() {
  loading.value = true
  try {
    const response = await fetchBackups()
    backups.value = response.data?.items ?? []
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '备份规则加载失败')
  } finally {
    loading.value = false
  }
}

function completionRuleLabel(rule: BackupCompletionRule) {
  switch (rule) {
    case 'delete_source':
      return '删除源文件'
    case 'delete_source_dir':
      return '删除源文件和空文件夹'
    default:
      return '无操作'
  }
}

function replaceRuleLabel(rule: BackupReplaceRule) {
  return rule === 'overwrite' ? '覆盖' : '跳过'
}

function formatTime(value: string) {
  if (!value) return '从未备份'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

// —— 向导导航 ——
function openCreateWizard() {
  Object.assign(wizardForm, emptyWizardForm())
  wizardEditing.value = false
  wizardEditingId.value = null
  wizardStep.value = 0
  wizardVisible.value = true
}

function openEditWizard(task: BackupTask) {
  wizardEditing.value = true
  wizardEditingId.value = task.id
  wizardForm.name = task.name
  wizardForm.source_dirs = [...task.source_dirs]
  wizardForm.target_dirs = [...task.target_dirs]
  wizardForm.monitor_enabled = task.monitor_enabled
  wizardForm.completion_rule = task.completion_rule
  wizardForm.replace_rule = task.replace_rule
  wizardForm.sync_delete_from_target = task.sync_delete_from_target
  wizardForm.force_full_scan = task.force_full_scan
  wizardForm.scan_interval_seconds = task.scan_interval_seconds
  wizardForm.cron_expression = task.cron_expression
  wizardForm.filter_rules = task.filter_rules.map((rule) => ({ ...rule }))
  wizardStep.value = 0
  wizardVisible.value = true
}

function resetWizard() {
  Object.assign(wizardForm, emptyWizardForm())
  wizardEditing.value = false
  wizardEditingId.value = null
  wizardStep.value = 0
}

function nextStep() {
  if (wizardStep.value === 0) {
    if (!wizardForm.name.trim()) {
      ElMessage.error('请填写备份任务名称')
      return
    }
    const hasSource = wizardForm.source_dirs.some((path) => path.trim())
    const hasTarget = wizardForm.target_dirs.some((path) => path.trim())
    if (!hasSource) {
      ElMessage.error('请至少选择一个源路径')
      return
    }
    if (!hasTarget) {
      ElMessage.error('请至少添加一个目标路径')
      return
    }
  }
  wizardStep.value += 1
}

async function applyWizard() {
  saving.value = true
  try {
    const payload = {
      name: wizardForm.name.trim(),
      enabled: true,
      source_dirs: wizardForm.source_dirs.map((p) => p.trim()).filter(Boolean),
      target_dirs: wizardForm.target_dirs.map((p) => p.trim()).filter(Boolean),
      monitor_enabled: wizardForm.monitor_enabled,
      completion_rule: wizardForm.completion_rule,
      replace_rule: wizardForm.replace_rule,
      sync_delete_from_target: wizardForm.sync_delete_from_target,
      force_full_scan: wizardForm.force_full_scan,
      scan_interval_seconds: wizardForm.scan_interval_seconds,
      cron_expression: wizardForm.cron_expression.trim(),
      filter_rules: wizardForm.filter_rules,
    }

    if (wizardEditing.value && wizardEditingId.value != null) {
      await updateBackup(wizardEditingId.value, payload)
      ElMessage.success('备份规则已更新')
    } else {
      await createBackup(payload)
      ElMessage.success('备份规则已创建')
    }

    wizardVisible.value = false
    await loadBackups()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存备份规则失败')
  } finally {
    saving.value = false
  }
}

// —— 路径选择 ——
function addPath(field: 'source' | 'target') {
  if (field === 'source') {
    wizardForm.source_dirs.push('')
  } else {
    wizardForm.target_dirs.push('')
  }
}

function removePath(field: 'source' | 'target', index: number) {
  if (field === 'source') {
    wizardForm.source_dirs.splice(index, 1)
  } else {
    wizardForm.target_dirs.splice(index, 1)
  }
}

function pickPath(field: 'source' | 'target', index: number) {
  pickerTarget = { field, index }
  pickerInitialPath.value = field === 'source' ? wizardForm.source_dirs[index] || '' : wizardForm.target_dirs[index] || ''
  pickerVisible.value = true
}

function applyPathSelection(path: string) {
  if (pickerTarget) {
    if (pickerTarget.field === 'source') {
      wizardForm.source_dirs[pickerTarget.index] = path
    } else {
      wizardForm.target_dirs[pickerTarget.index] = path
    }
  }
  pickerTarget = null
}

// —— 筛选规则 ——
function addFilterRule(type: string) {
  const rule: BackupFilterRule = {
    id: `rule-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    type: type as BackupFilterType,
    value: '',
    blacklist: true,
    whitelist: false,
    match_dir: false,
    match_file: true,
    min_size: 0,
    max_size: 0,
    size_unit: 'MB',
    extensions: [],
  }
  wizardForm.filter_rules.push(rule)
}

function removeFilterRule(index: number) {
  wizardForm.filter_rules.splice(index, 1)
}

function filterTypeLabel(type: BackupFilterType) {
  switch (type) {
    case 'extension':
      return '扩展名'
    case 'regex':
      return '正则'
    case 'size':
      return '体积'
    default:
      return '名称'
  }
}

function filterValuePlaceholder(type: BackupFilterType) {
  switch (type) {
    case 'extension':
      return '例如 .mkv, .mp4'
    case 'regex':
      return '例如 \\.(mkv|mp4)$'
    default:
      return '例如 电视剧'
  }
}

// —— 卡片操作 ——
async function toggleEnabled(task: BackupTask) {
  try {
    await setBackupEnabled(task.id, !task.enabled)
    ElMessage.success(task.enabled ? '已禁用' : '已启用')
    await loadBackups()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '操作失败')
  }
}

async function rescan(task: BackupTask) {
  scanningIds.value = new Set(scanningIds.value).add(task.id)
  try {
    await runBackup(task.id, true)
    ElMessage.success('已开始完整扫描')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '扫描失败')
  } finally {
    const next = new Set(scanningIds.value)
    next.delete(task.id)
    scanningIds.value = next
  }
}

async function removeTask(task: BackupTask) {
  try {
    await ElMessageBox.confirm(`确认移除备份规则「${task.name}」？`, '移除备份规则', { type: 'warning' })
    await deleteBackup(task.id)
    ElMessage.success('备份规则已移除')
    await loadBackups()
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error instanceof Error ? error.message : '移除失败')
  }
}

// —— 状态详情 ——
async function openStatusDialog(task: BackupTask) {
  statusTaskId = task.id
  statusDialogVisible.value = true
  await refreshStatus()
}

async function refreshStatus() {
  if (statusTaskId == null) return
  statusLoading.value = true
  try {
    const response = await fetchBackupStatus(statusTaskId)
    if (response.data) {
      statusSnapshot.value = response.data
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '状态加载失败')
  } finally {
    statusLoading.value = false
  }
}

function statusLabel(status: string) {
  switch (status) {
    case 'running':
      return '执行中'
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    default:
      return '空闲'
  }
}

function statusTagType(status: string): 'success' | 'danger' | 'warning' | 'info' {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'danger'
    case 'running':
      return 'warning'
    default:
      return 'info'
  }
}
</script>

<style scoped>
.rules-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.rules-card__title {
  font-size: 18px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.backup-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}

.backup-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
  border: 1px solid #edf2f7;
  border-radius: 16px;
  background: linear-gradient(180deg, #ffffff 0%, #fbfdff 100%);
  transition: box-shadow 0.18s ease, border-color 0.18s ease;
}

.backup-card:hover {
  border-color: #bcd4ff;
  box-shadow: 0 10px 22px rgba(37, 99, 235, 0.08);
}

.backup-card--disabled {
  opacity: 0.62;
}

.backup-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.backup-card__name {
  color: #1e293b;
  font-size: 16px;
  font-weight: 800;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.backup-card__status-dot {
  flex: 0 0 auto;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #cbd5e1;
}

.backup-card__status-dot.is-on {
  background: #22c55e;
}

.backup-card__body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.backup-card__meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.backup-card__meta-label {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  letter-spacing: 0.04em;
}

.backup-card__meta-value {
  font-size: 13px;
  color: var(--el-text-color-regular);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.backup-card__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.backup-card__tag {
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 12px;
  color: #64748b;
  background: #f1f5f9;
}

.backup-card__tag.is-on {
  color: #15803d;
  background: #ecfdf3;
}

.backup-card__last {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px dashed #edf2f7;
}

.backup-card__last-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.backup-card__last-value {
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.backup-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.backup-card__footer-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.backup-card__action {
  padding: 4px;
}

.backup-card__action-icon {
  width: 18px;
  height: 18px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.backup-card__action--danger {
  color: #ef4444;
}

/* 向导 */
.backup-wizard-steps {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-bottom: 20px;
}

.backup-wizard-step {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #94a3b8;
}

.backup-wizard-step__dot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  font-size: 12px;
  font-weight: 700;
  color: #64748b;
  background: #f1f5f9;
}

.backup-wizard-step.is-active {
  color: #2563eb;
}

.backup-wizard-step.is-active .backup-wizard-step__dot {
  color: #fff;
  background: #2563eb;
}

.backup-wizard-step.is-done .backup-wizard-step__dot {
  color: #15803d;
  background: #ecfdf3;
}

.backup-wizard-step__label {
  font-size: 13px;
  font-weight: 600;
}

.backup-wizard-body {
  max-height: 52vh;
  overflow-y: auto;
  padding: 20px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 16px;
  background: var(--el-bg-color);
}

.backup-wizard-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.backup-form-hint {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
}

.backup-path-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.backup-path-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.backup-path-row__remove {
  flex: 0 0 auto;
  color: #ef4444;
}

/* 筛选规则 */
.backup-filter-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 14px;
}

.backup-filter-row {
  padding: 12px;
  border: 1px solid #edf2f7;
  border-radius: 12px;
  background: #fbfdff;
}

.backup-filter-row__main {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.backup-filter-row__type {
  flex: 0 0 auto;
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 700;
  color: #2563eb;
  background: #eff6ff;
}

.backup-filter-row__value {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1 1 auto;
  min-width: 0;
}

.backup-filter-row__sep {
  color: var(--el-text-color-secondary);
}

.backup-filter-row__opts {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.backup-filter-row__remove {
  margin-left: auto;
  color: #ef4444;
}

/* 状态详情 */
.backup-status-loading {
  padding: 32px;
  text-align: center;
  color: var(--el-text-color-secondary);
}

.backup-status {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.backup-status__line {
  display: flex;
  align-items: center;
  gap: 12px;
}

.backup-status__label {
  flex: 0 0 auto;
  width: 72px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.backup-status__stats {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 8px;
}

.backup-status__stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 10px;
  border-radius: 10px;
  background: #f8fafc;
}

.backup-status__stat span {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.backup-status__stat b {
  font-size: 16px;
  color: #1e293b;
}

.backup-status__logs {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 180px;
  overflow-y: auto;
  padding: 10px;
  border-radius: 10px;
  background: #f8fafc;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.backup-status__logs-title {
  font-size: 12px;
  font-weight: 700;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}

.backup-status__log {
  font-size: 12px;
  color: var(--el-text-color-regular);
  line-height: 1.6;
}
</style>
