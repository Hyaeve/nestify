<template>
  <el-card v-show="visible" class="page-card rules-card backup-rules-card">
    <template #header>
      <div class="rules-card__header">
        <div class="rules-card__title">备份规则</div>
        <div class="rules-card__header-actions">
          <el-button round @click="openResourceLimitDialog">资源限制</el-button>
          <el-button type="primary" round @click="openCreateWizard">+ 添加备份</el-button>
        </div>
      </div>
    </template>

    <el-empty v-if="!loading && backups.length === 0" description="暂无备份规则，点击右上角「添加备份」创建" />

    <div v-else class="backup-grid" ref="backupGridRef">
      <div v-for="task in backups" :key="task.id" class="backup-card" :class="{ 'backup-card--disabled': !task.enabled }" @click="handleBackupCardClick(task)">
        <div class="backup-card__head">
          <div class="backup-card__title-row">
            <button type="button" class="backup-card__drag" aria-label="拖拽排序" title="拖拽排序" @click.stop>
              <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="9" cy="6" r="1.5" /><circle cx="15" cy="6" r="1.5" /><circle cx="9" cy="12" r="1.5" /><circle cx="15" cy="12" r="1.5" /><circle cx="9" cy="18" r="1.5" /><circle cx="15" cy="18" r="1.5" /></svg>
            </button>
            <div class="backup-card__name" :title="task.name">{{ task.name }}</div>
          </div>
          <button type="button" class="backup-card__mode" title="查看实时备份状态" @click.stop="openStatusDialog(task)">备份</button>
        </div>

        <div class="backup-card__body">
          <div class="backup-card__meta">
            <div class="backup-card__meta-label">源路径</div>
            <div class="backup-card__meta-value" v-for="src in visiblePaths(task.source_dirs)" :key="src" :title="src">{{ src }}</div>
            <div v-if="task.source_dirs.length > 2" class="backup-card__meta-more">+{{ task.source_dirs.length - 2 }} 更多</div>
          </div>
          <div class="backup-card__meta">
            <div class="backup-card__meta-label">目标路径</div>
            <div class="backup-card__meta-value" v-for="dst in visiblePaths(task.target_dirs)" :key="dst" :title="dst">{{ dst }}</div>
            <div v-if="task.target_dirs.length > 2" class="backup-card__meta-more">+{{ task.target_dirs.length - 2 }} 更多</div>
          </div>
          <div class="backup-card__meta">
            <div class="backup-card__meta-label">Cron</div>
            <div class="backup-card__meta-value" :class="{ 'is-empty': !task.cron_expression }" :title="task.cron_expression || ''">
              {{ task.cron_expression || '未设置计划' }}
            </div>
          </div>

          <div class="backup-card__tags">
            <span class="backup-card__tag" :class="{ 'is-on': task.monitor_enabled }">实时监控</span>
            <span class="backup-card__tag">{{ completionRuleLabel(task.completion_rule) }}</span>
            <span class="backup-card__tag">{{ replaceRuleLabel(task.replace_rule) }}</span>
            <span v-if="task.force_full_scan" class="backup-card__tag">强制完整扫描</span>
            <span v-if="task.scan_interval_seconds > 0" class="backup-card__tag">间隔 {{ task.scan_interval_seconds }}s</span>
            <span v-if="task.filter_rules.length" class="backup-card__tag">筛选 {{ task.filter_rules.length }} 条</span>
          </div>

          <div class="backup-card__last">
            <span class="backup-card__last-label">上次备份</span>
            <span class="backup-card__last-value">{{ formatTime(task.last_backup_at) }}</span>
          </div>
        </div>

        <div class="backup-card__footer">
          <CardActionBar
            :enabled="task.enabled"
            :busy="updatingIds.has(task.id)"
            execute-title="立即执行一次备份扫描"
            @toggle="toggleEnabled(task)"
            @execute="rescan(task)"
            @edit="openEditWizard(task)"
            @remove="removeTask(task)"
          />
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
        <button
          v-for="(step, index) in wizardSteps"
          :key="index"
          type="button"
          class="backup-wizard-tab"
          :class="{ 'is-active': wizardStep === index }"
          @click="wizardStep = index"
        >
          {{ step }}
        </button>
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
            <div class="backup-form-hint backup-form-hint--top">备份完成后对源文件执行的操作。</div>
            <el-select v-model="wizardForm.completion_rule" style="width: 100%">
              <el-option label="无操作" value="none" />
              <el-option label="删除源文件" value="delete_source" />
              <el-option label="删除源文件和空文件夹" value="delete_source_dir" />
            </el-select>
          </el-form-item>
          <el-form-item label="替换规则">
            <div class="backup-form-hint backup-form-hint--top">目标中已存在同名文件时的处理方式。</div>
            <el-select v-model="wizardForm.replace_rule" style="width: 100%">
              <el-option label="跳过" value="skip" />
              <el-option label="覆盖" value="overwrite" />
            </el-select>
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
            <div class="backup-form-hint backup-form-hint--top">按此间隔自动扫描更改。0 = 禁用。</div>
            <el-input-number v-model="wizardForm.scan_interval_seconds" :min="0" :max="86400" />
          </el-form-item>
          <el-form-item label="Cron 表达式（计划扫描）">
            <el-input v-model="wizardForm.cron_expression" placeholder="填写则启用，不填写则不启用。例如 0 3 * * *" />
          </el-form-item>
        </el-form>
      </div>

      <!-- 第 4 步：筛选规则 -->
      <div v-show="wizardStep === 3" class="backup-wizard-body">
        <div class="backup-filter-toolbar">
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
          <span class="backup-filter-toolbar__hint">添加一条筛选规则，右上角设置名单类型与匹配目标</span>
        </div>

        <div class="backup-filter-list">
          <div v-for="(rule, index) in wizardForm.filter_rules" :key="rule.id" class="backup-filter-row">
            <div class="backup-filter-row__head">
              <span class="backup-filter-row__type">{{ filterTypeLabel(rule.type) }}</span>
              <div class="backup-filter-row__opts">
                <button
                  type="button"
                  class="backup-filter-chip"
                  :class="{ 'is-active': rule.blacklist }"
                  @click="setListMode(rule, 'blacklist')"
                >黑名单</button>
                <button
                  type="button"
                  class="backup-filter-chip"
                  :class="{ 'is-active': rule.whitelist }"
                  @click="setListMode(rule, 'whitelist')"
                >白名单</button>
                <button
                  type="button"
                  class="backup-filter-chip"
                  :class="{ 'is-active': rule.match_dir }"
                  @click="toggleMatch(rule, 'dir')"
                >文件夹</button>
                <button
                  type="button"
                  class="backup-filter-chip"
                  :class="{ 'is-active': rule.match_file }"
                  @click="toggleMatch(rule, 'file')"
                >文件</button>
                <el-button link class="backup-filter-row__remove" @click="removeFilterRule(index)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
            </div>

            <div class="backup-filter-row__body">
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
              <template v-else-if="rule.type === 'regex'">
                <el-input v-model="rule.value" style="width: 100%" :placeholder="filterValuePlaceholder(rule.type)" />
              </template>
              <template v-else>
                <div class="backup-filter-tags">
                  <div v-if="ruleTags(rule).length" class="backup-filter-tags__list">
                    <span v-for="(tag, tagIndex) in ruleTags(rule)" :key="tagIndex" class="backup-filter-tag">
                      {{ tag }}
                      <button type="button" class="backup-filter-tag__remove" @click="removeTag(rule, tagIndex)">×</button>
                    </span>
                  </div>
                  <el-input
                    v-model="rule.draftValue"
                    class="backup-filter-tag-input"
                    :placeholder="filterValuePlaceholder(rule.type)"
                    @keyup.enter="commitTag(rule)"
                  />
                </div>
              </template>
            </div>
          </div>
          <el-empty v-if="wizardForm.filter_rules.length === 0" :image-size="60" description="尚未添加筛选规则" />
        </div>
      </div>

      <template #footer>
        <div class="backup-wizard-footer">
          <el-button v-if="wizardStep === 0" class="backup-wizard-footer__prev" @click="cancelWizard">取消</el-button>
          <el-button v-else class="backup-wizard-footer__prev" @click="prevStep">上一步</el-button>
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
          <div class="backup-status__stat"><span>上传</span><b>{{ statusSnapshot.copied }}</b></div>
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

    <!-- 资源限制 -->
    <el-dialog v-model="resourceLimitVisible" title="资源限制" width="520px">
      <el-form label-position="top">
        <el-form-item label="队列上限">
          <div class="resource-limit__row">
            <el-select v-model="resourceLimitForm.upperMode" style="width: 160px" @change="onUpperModeChange">
              <el-option label="无限制" value="unlimited" />
              <el-option label="自定义" value="custom" />
            </el-select>
            <el-input-number
              v-if="resourceLimitForm.upperMode === 'custom'"
              v-model="resourceLimitForm.upperLimit"
              :min="1"
              :max="100000000"
              :step="1000"
              controls-position="right"
              style="width: 200px"
            />
          </div>
          <div class="resource-limit__hint">待处理上传任务达到此数量时暂停添加，队列回落后恢复。无限制表示不限（大文件夹可能占用更多内存）。</div>
        </el-form-item>

        <el-form-item label="队列下限">
          <div class="resource-limit__row">
            <el-select v-model="resourceLimitForm.lowerMode" style="width: 160px" @change="onLowerModeChange">
              <el-option label="无限制" value="unlimited" />
              <el-option label="自定义" value="custom" />
            </el-select>
            <el-input-number
              v-if="resourceLimitForm.lowerMode === 'custom'"
              v-model="resourceLimitForm.lowerLimit"
              :min="1"
              :max="100000000"
              :step="1000"
              controls-position="right"
              style="width: 200px"
            />
          </div>
          <div class="resource-limit__hint">待处理队列回落到此数量时恢复添加上传任务。未设置时默认为上限的一半。</div>
        </el-form-item>

        <el-form-item label="最大并发扫描数">
          <el-input-number
            v-model="resourceLimitForm.maxConcurrent"
            :min="1"
            :max="64"
            :step="1"
            controls-position="right"
            style="width: 200px"
          />
          <div class="resource-limit__hint">允许同时进行扫描的备份任务数量，超出的任务排队等待。默认 1。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resourceLimitVisible = false">取消</el-button>
        <el-button type="primary" :loading="resourceLimitSaving" @click="saveResourceLimit">保存</el-button>
      </template>
    </el-dialog>

    <DirectoryPickerDialog v-model="pickerVisible" title="选择目录" :initial-path="pickerInitialPath" @selected="applyPathSelection" />
  </el-card>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete } from '@element-plus/icons-vue'
import Sortable from 'sortablejs'
import type { SortableEvent } from 'sortablejs'

import DirectoryPickerDialog from './DirectoryPickerDialog.vue'
import CardActionBar from './CardActionBar.vue'
import {
  createBackup,
  deleteBackup,
  fetchBackupStatus,
  fetchBackups,
  reorderBackups,
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
import { fetchSettings, updateSettings } from '../api/system'

const props = defineProps<{ visible: boolean }>()

const loading = ref(false)
const backups = ref<BackupTask[]>([])
const scanningIds = ref<Set<number>>(new Set())
// 正在提交「启用 / 禁用」请求的任务：用于禁用按钮避免重复点击。
const updatingIds = ref<Set<number>>(new Set())

// —— 资源限制 ——
const resourceLimitVisible = ref(false)
const resourceLimitSaving = ref(false)
const resourceLimitForm = reactive({
  upperMode: 'unlimited' as 'unlimited' | 'custom',
  upperLimit: 10000,
  lowerMode: 'unlimited' as 'unlimited' | 'custom',
  lowerLimit: 5000,
  maxConcurrent: 1,
})

// —— 拖拽排序 ——
const backupGridRef = ref<HTMLElement | null>(null)
let backupSortable: Sortable | null = null
const reordering = ref(false)

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
  filter_rules: EditableFilterRule[]
}

// 带草稿输入框的筛选规则（draftValue 仅前端使用，不落库）
interface EditableFilterRule extends BackupFilterRule {
  draftValue: string
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

onBeforeUnmount(() => {
  destroyBackupSortable()
})

async function loadBackups() {
  loading.value = true
  try {
    const response = await fetchBackups()
    backups.value = response.data?.items ?? []
    await nextTick()
    setupBackupSortable()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '备份规则加载失败')
  } finally {
    loading.value = false
  }
}

// —— 资源限制 ——
function openResourceLimitDialog() {
  resourceLimitVisible.value = true
  void loadResourceLimit()
}

async function loadResourceLimit() {
  try {
    const response = await fetchSettings()
    if (!response.data) return
    const upper = response.data.upload_queue_upper_limit
    const lower = response.data.upload_queue_lower_limit
    resourceLimitForm.upperMode = upper > 0 ? 'custom' : 'unlimited'
    resourceLimitForm.upperLimit = upper > 0 ? upper : 10000
    resourceLimitForm.lowerMode = lower > 0 ? 'custom' : 'unlimited'
    resourceLimitForm.lowerLimit = lower > 0 ? lower : Math.max(1, Math.floor((upper > 0 ? upper : 10000) / 2))
    resourceLimitForm.maxConcurrent = response.data.max_concurrent_scans > 0 ? response.data.max_concurrent_scans : 1
  } catch {
    // 拉取失败时保留默认值
  }
}

function onUpperModeChange(mode: string) {
  if (mode === 'unlimited') {
    resourceLimitForm.upperLimit = 0
  } else {
    resourceLimitForm.upperLimit = 10000
  }
}

function onLowerModeChange(mode: string) {
  if (mode === 'unlimited') {
    resourceLimitForm.lowerLimit = 0
  } else {
    resourceLimitForm.lowerLimit = Math.max(1, Math.floor(resourceLimitForm.upperLimit / 2))
  }
}

async function saveResourceLimit() {
  resourceLimitSaving.value = true
  try {
    const upper = resourceLimitForm.upperMode === 'custom' ? resourceLimitForm.upperLimit : 0
    const lower = resourceLimitForm.lowerMode === 'custom' ? resourceLimitForm.lowerLimit : 0
    const current = await fetchSettings()
    await updateSettings({
      log_retention_days: current.data?.log_retention_days ?? 5,
      log_retention_max_records: current.data?.log_retention_max_records ?? 10000,
      history_view_mode: current.data?.history_view_mode ?? 'flat',
      default_page: current.data?.default_page ?? 'dashboard',
      page_size: current.data?.page_size ?? 50,
      cache_dir: current.data?.cache_dir ?? '/tmp',
      cache_persist_enabled: current.data?.cache_persist_enabled !== false,
      ignored_extensions: current.data?.ignored_extensions ?? [],
      upload_queue_upper_limit: upper,
      upload_queue_lower_limit: lower,
      max_concurrent_scans: resourceLimitForm.maxConcurrent,
    })
    ElMessage.success('资源限制已保存')
    resourceLimitVisible.value = false
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '资源限制保存失败')
  } finally {
    resourceLimitSaving.value = false
  }
}

function visiblePaths(paths: string[]): string[] {
  return paths.slice(0, 2)
}

// —— 拖拽排序 ——
// 记录最近一次拖拽结束时间：拖拽后浏览器会补一个 click，用它避免顺手弹出编辑向导。
let lastBackupDragAt = 0

function handleBackupCardClick(task: BackupTask) {
  if (Date.now() - lastBackupDragAt < 260) return
  openEditWizard(task)
}

function setupBackupSortable() {
  destroyBackupSortable()
  const grid = backupGridRef.value
  if (!grid || backups.value.length < 2) return

  backupSortable = Sortable.create(grid, {
    animation: 180,
    handle: '.backup-card__drag',
    ghostClass: 'backup-card--ghost',
    chosenClass: 'backup-card--chosen',
    dragClass: 'backup-card--drag',
    onStart: () => {
      lastBackupDragAt = Date.now()
    },
    onEnd: (event: SortableEvent) => {
      lastBackupDragAt = Date.now()
      const oldIndex = event.oldIndex
      const newIndex = event.newIndex
      if (oldIndex == null || newIndex == null || oldIndex === newIndex || reordering.value) return
      void persistBackupOrder(oldIndex, newIndex)
    },
  })
}

function destroyBackupSortable() {
  if (backupSortable) {
    backupSortable.destroy()
    backupSortable = null
  }
}

async function persistBackupOrder(oldIndex: number, newIndex: number) {
  const items = [...backups.value]
  const [moved] = items.splice(oldIndex, 1)
  items.splice(newIndex, 0, moved)

  // 先乐观更新本地顺序
  backups.value = items
  reordering.value = true
  try {
    await reorderBackups(items.map((item, index) => ({ id: item.id, sort_order: index + 1 })))
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '调整顺序失败')
    await loadBackups()
  } finally {
    reordering.value = false
    await nextTick()
    setupBackupSortable()
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
  wizardForm.filter_rules = task.filter_rules.map((rule) => {
    // 兼容旧数据：name/extension 类型把 value 字段中逗号/分号/空格分隔的多个值拆入 extensions；
    // regex 保持单值 value 不变。
    const isMulti = rule.type === 'name' || rule.type === 'extension'
    const extensions = [...(rule.extensions ?? [])]
    const legacy = rule.value ?? ''
    if (isMulti && legacy.trim()) {
      for (const part of legacy.split(/[,;|\s]+/)) {
        const trimmed = part.trim()
        if (trimmed && !extensions.includes(trimmed)) {
          extensions.push(trimmed)
        }
      }
    }
    return { ...rule, extensions, draftValue: '' }
  })
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

function prevStep() {
  if (wizardStep.value > 0) {
    wizardStep.value -= 1
  }
}

function cancelWizard() {
  wizardVisible.value = false
}

async function applyWizard() {
  saving.value = true
  try {
    // 提交前：把每条规则尚未回车确认的草稿值并入候选，并剥离 draftValue 字段。
    const filterRules: BackupFilterRule[] = wizardForm.filter_rules
      .map((rule) => {
        const { draftValue, ...rest } = rule
        const extensions = [...(rest.extensions ?? [])]
        const draft = draftValue.trim()
        if (draft && !extensions.includes(draft)) {
          extensions.push(draft)
        }
        return { ...rest, extensions }
      })

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
      filter_rules: filterRules,
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
  const rule: EditableFilterRule = {
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
    draftValue: '',
  }
  wizardForm.filter_rules.push(rule)
}

function removeFilterRule(index: number) {
  wizardForm.filter_rules.splice(index, 1)
}

// 黑名单 / 白名单 二选一
function setListMode(rule: EditableFilterRule, mode: 'blacklist' | 'whitelist') {
  if (mode === 'blacklist') {
    rule.blacklist = true
    rule.whitelist = false
  } else {
    rule.blacklist = false
    rule.whitelist = true
  }
}

// 文件夹 / 文件 至少选一个
function toggleMatch(rule: EditableFilterRule, target: 'dir' | 'file') {
  if (target === 'dir') {
    if (rule.match_dir && !rule.match_file) return
    rule.match_dir = !rule.match_dir
  } else {
    if (rule.match_file && !rule.match_dir) return
    rule.match_file = !rule.match_file
  }
}

// 获取某条规则的候选值列表（多值）
function ruleTags(rule: EditableFilterRule): string[] {
  return rule.extensions ?? []
}

// 回车把草稿值提交为一个候选
function commitTag(rule: EditableFilterRule) {
  const draft = rule.draftValue.trim()
  if (!draft) return
  if (!rule.extensions) rule.extensions = []
  if (!rule.extensions.includes(draft)) {
    rule.extensions.push(draft)
  }
  rule.draftValue = ''
}

// 删除某个候选
function removeTag(rule: EditableFilterRule, tagIndex: number) {
  rule.extensions?.splice(tagIndex, 1)
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
      return '输入扩展名后回车，如 .mkv'
    case 'regex':
      return '例如 \\.(mkv|mp4)$'
    default:
      return '输入名称后回车，如 电视剧'
  }
}

// —— 卡片操作 ——
async function toggleEnabled(task: BackupTask) {
  updatingIds.value = new Set(updatingIds.value).add(task.id)
  try {
    await setBackupEnabled(task.id, !task.enabled)
    ElMessage.success(task.enabled ? '已禁用' : '已启用')
    await loadBackups()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '操作失败')
  } finally {
    const next = new Set(updatingIds.value)
    next.delete(task.id)
    updatingIds.value = next
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

.rules-card__header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.rules-card__title {
  font-size: 18px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

/* —— 资源限制弹窗 —— */
.resource-limit__row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.resource-limit__hint {
  margin-top: 6px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
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
  cursor: pointer;
  transition: box-shadow 0.18s ease, border-color 0.18s ease;
}

.backup-card:hover {
  border-color: #b6c9c1;
  box-shadow: 0 10px 22px rgba(91, 122, 110, 0.08);
}

.backup-card--disabled {
  opacity: 0.62;
}

.backup-card--ghost {
  opacity: 0.4;
}

.backup-card--chosen {
  border-color: #9db8ad;
  box-shadow: 0 6px 16px rgba(91, 122, 110, 0.12);
}

.backup-card--drag {
  opacity: 0.9;
}

.backup-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.backup-card__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  flex: 1 1 auto;
}

.backup-card__drag {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  color: var(--el-text-color-secondary);
  background: transparent;
  border: 0;
  border-radius: 6px;
  cursor: grab;
}

.backup-card__drag svg {
  width: 16px;
  height: 16px;
  fill: currentColor;
}

.backup-card__drag:hover {
  color: var(--el-color-primary);
  background: var(--el-fill-color-light);
}

.backup-card__drag:active {
  cursor: grabbing;
}

.backup-card__name {
  color: #1e293b;
  font-size: 16px;
  font-weight: 800;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 右上角模式标识：与规则卡片保持一致的莫奈低饱和雾霾蓝。 */
/* 与「运行日志」页模式标识（.logs-mode-tag--backup）保持完全一致的观感。 */
.backup-card__mode {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 62px;
  padding: 4px 12px;
  border: 1px solid currentColor;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.2;
  color: #5f7fa8;
  background: rgba(95, 127, 168, 0.12);
  cursor: pointer;
  transition:
    color 0.16s ease,
    background-color 0.16s ease,
    border-color 0.16s ease;
}

.backup-card__mode:hover {
  color: #fff;
  border-color: #5f7fa8;
  background: #5f7fa8;
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

.backup-card__meta-value.is-empty {
  color: var(--el-text-color-placeholder);
  font-style: italic;
}

.backup-card__meta-more {
  font-size: 12px;
  color: var(--el-color-primary);
  cursor: default;
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
  padding-top: 10px;
  border-top: 1px dashed var(--border-color, #e2e8f0);
}

/* 向导（平等 tabs） */
.backup-wizard-steps {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
  padding: 4px;
  border-radius: 12px;
  background: #f1f5f9;
}

.backup-wizard-tab {
  flex: 1 1 0;
  padding: 8px 12px;
  border: none;
  border-radius: 9px;
  font-size: 13px;
  font-weight: 600;
  color: #64748b;
  background: transparent;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.backup-wizard-tab:hover {
  color: #5b7a6e;
}

.backup-wizard-tab.is-active {
  color: #5b7a6e;
  background: #ffffff;
  box-shadow: 0 1px 4px rgba(15, 23, 42, 0.08);
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
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.backup-wizard-footer__prev {
  margin-right: auto;
}

.backup-form-hint {
  /* el-form-item__content 默认是横向 flex，会把小字说明挤到标题右侧；
     这里强制占满整行，使其显示在功能标题下方 */
  flex: 0 0 100%;
  width: 100%;
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
}

.backup-form-hint--top {
  margin-top: 0;
  margin-bottom: 6px;
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
.backup-filter-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.backup-filter-toolbar__hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.backup-filter-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 14px;
}

.backup-filter-row {
  padding: 14px;
  border: 1px solid #edf2f7;
  border-radius: 12px;
  background: #fbfdff;
}

.backup-filter-row__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 12px;
}

.backup-filter-row__type {
  flex: 0 0 auto;
  padding: 2px 8px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 700;
  color: #5b7a6e;
  background: #eef3f1;
}

.backup-filter-row__opts {
  display: flex;
  align-items: center;
  gap: 6px;
}

.backup-filter-chip {
  padding: 3px 10px;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
  font-size: 12px;
  color: #64748b;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s ease;
  line-height: 1.4;
}

.backup-filter-chip:hover {
  border-color: #b6c9c1;
  color: #5b7a6e;
}

.backup-filter-chip.is-active {
  color: #fff;
  background: #5b7a6e;
  border-color: #5b7a6e;
}

.backup-filter-row__remove {
  flex: 0 0 auto;
  color: #ef4444;
  padding: 4px;
}

.backup-filter-row__body {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.backup-filter-row__sep {
  color: var(--el-text-color-secondary);
}

.backup-filter-tags {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  width: 100%;
}

.backup-filter-tags__list {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.backup-filter-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px 3px 10px;
  border-radius: 999px;
  font-size: 12px;
  color: #5b7a6e;
  background: #eef3f1;
}

.backup-filter-tag__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 14px;
  height: 14px;
  border: none;
  border-radius: 50%;
  font-size: 12px;
  line-height: 1;
  color: #5b7a6e;
  background: transparent;
  cursor: pointer;
}

.backup-filter-tag__remove:hover {
  background: #dfe9e4;
}

.backup-filter-tag-input {
  width: 45%;
  max-width: 45%;
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
