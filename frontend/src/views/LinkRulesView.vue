<template>
  <div class="rules-page">
    <el-alert v-if="errorMessage" :closable="false" type="error" :title="errorMessage" class="rules-error" />

    <el-card class="page-card rules-card">
      <template #header>
        <div class="rules-card__header">
          <div>
            <div class="rules-card__title">链路规则</div>
            <div class="mode-config-panel__description">共 {{ total }} 条规则，可实时监控新文件创建硬链或软链，也可按计划全量执行。</div>
          </div>
          <el-button type="primary" round @click="openCreateDialog">+ 添加链路规则</el-button>
        </div>
      </template>

      <el-table v-if="items.length" v-loading="loading" :data="items">
        <el-table-column prop="name" label="规则名称" min-width="160" />
        <el-table-column label="链路模式" width="110">
          <template #default="scope">
            <el-tag :type="scope.row.link_mode === 'hard' ? 'danger' : 'primary'" effect="plain">
              {{ scope.row.link_mode === 'hard' ? '硬链模式' : '软链模式' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="source_dir" label="监控目录" min-width="280" show-overflow-tooltip />
        <el-table-column prop="target_dir" label="链路目录" min-width="280" show-overflow-tooltip />
        <el-table-column label="实时监控" width="100">
          <template #default="scope">
            <el-tag :type="scope.row.monitor_enabled ? 'success' : 'info'" effect="plain">{{ scope.row.monitor_enabled ? '开启' : '关闭' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="计划执行" width="120">
          <template #default="scope">
            <el-tag :type="scope.row.run_mode === 'cron' ? 'warning' : 'info'" effect="plain">{{ scope.row.run_mode === 'cron' ? '已启用' : '未启用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Cron" min-width="140">
          <template #default="scope">{{ scope.row.cron_expression || '—' }}</template>
        </el-table-column>
        <el-table-column label="状态" width="88">
          <template #default="scope">
            <el-tag :type="scope.row.enabled ? 'success' : 'info'">{{ scope.row.enabled ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right" align="center">
          <template #default="scope">
            <div class="rule-actions">
              <el-tooltip content="编辑" placement="top">
                <el-button link class="rule-action rule-action--primary" @click="openEditDialog(scope.row.id)">
                  <el-icon><Edit /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="执行" placement="top">
                <el-button link class="rule-action rule-action--success" @click="prepareExecution(scope.row.id)">
                  <el-icon><Operation /></el-icon>
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button link class="rule-action rule-action--danger" @click="removeItem(scope.row.id)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-else description="暂无链路规则，可添加硬链或软链规则" />

      <div v-if="total > 0" class="history-pagination">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          background
          layout="total, sizes, prev, pager, next"
          :page-sizes="pageSizeOptions"
          :total="total"
          @current-change="loadItems"
          @size-change="handleSizeChange"
        />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑链路规则' : '新增链路规则'" width="680px">
      <el-form label-position="top">
        <el-form-item label="规则名称"><el-input v-model="form.name" /></el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="链路模式">
              <el-radio-group v-model="form.link_mode" class="archive-mode-group">
                <el-radio-button value="soft">软链模式</el-radio-button>
                <el-radio-button value="hard">硬链模式</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="触发方式">
              <el-space wrap>
                <el-switch v-model="form.monitor_enabled" inline-prompt active-text="实时监控" inactive-text="实时监控" />
                <el-switch v-model="form.schedule_enabled" inline-prompt active-text="计划执行" inactive-text="计划执行" @change="handleScheduleChange" />
              </el-space>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="监控目录">
          <el-input v-model="form.source_dir" placeholder="选择需要监控的新文件目录" />
        </el-form-item>
        <el-form-item label="链路目录">
          <el-input v-model="form.target_dir" placeholder="选择链接输出目录" />
        </el-form-item>
        <el-form-item label="计划表达式">
          <el-input v-model="form.cron_expression" :disabled="!form.schedule_enabled" placeholder="默认每天 04:30 执行一次" />
          <div class="mode-config-panel__description">默认计划开启后使用 <code>30 4 * * *</code>，即每天四点半全量执行。</div>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="规则状态">
              <el-switch v-model="form.enabled" inline-prompt active-text="启用" inactive-text="停用" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="启动后立即执行">
              <el-switch v-model="form.run_on_start" inline-prompt active-text="执行" inactive-text="不执行" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitForm">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { Delete, Edit, Operation } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { computed, onMounted, reactive, ref } from 'vue'

import { createRule, deleteRule, fetchRule, fetchRules, updateRule, type CreateRulePayload, type RuleItem } from '../api/rules'

const loading = ref(false)
const errorMessage = ref('')
const items = ref<RuleItem[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(25)
const pageSizeOptions = [25, 50]
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const defaultCron = '30 4 * * *'

const form = reactive({
  name: '',
  enabled: true,
  monitor_enabled: true,
  schedule_enabled: true,
  link_mode: 'soft' as 'soft' | 'hard',
  source_dir: '',
  target_dir: '',
  cron_expression: defaultCron,
  run_on_start: false,
})

const runMode = computed(() => {
  if (form.schedule_enabled) return 'cron' as const
  if (form.monitor_enabled) return 'watch' as const
  return 'once' as const
})

function resetForm() {
  form.name = ''
  form.enabled = true
  form.monitor_enabled = true
  form.schedule_enabled = true
  form.link_mode = 'soft'
  form.source_dir = ''
  form.target_dir = ''
  form.cron_expression = defaultCron
  form.run_on_start = false
}

function handleScheduleChange(value: boolean) {
  if (value && !form.cron_expression.trim()) {
    form.cron_expression = defaultCron
  }
}

function buildPayload(): CreateRulePayload {
  return {
    name: form.name.trim(),
    description: '',
    enabled: form.enabled,
    monitor_enabled: form.monitor_enabled,
    compatibility_mode: 'local',
    archive_mode: 'link',
    rule_type: 'link',
    link_mode: form.link_mode,
    run_mode: runMode.value,
    source_dir: form.source_dir.trim(),
    target_dir: form.target_dir.trim(),
    watch_debounce_ms: 2000,
    cron_expression: form.schedule_enabled ? (form.cron_expression.trim() || defaultCron) : '',
    run_on_start: form.run_on_start,
    options: {},
    package_options: {},
    collect_options: {},
    filters: [],
  }
}

async function loadItems() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await fetchRules({ page: currentPage.value, page_size: pageSize.value, rule_type: 'link' })
    items.value = response.data?.items ?? []
    total.value = response.data?.total ?? 0
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '加载链路规则失败'
  } finally {
    loading.value = false
  }
}

function handleSizeChange() {
  currentPage.value = 1
  void loadItems()
}

function openCreateDialog() {
  editingId.value = null
  resetForm()
  dialogVisible.value = true
}

async function openEditDialog(id: number) {
  try {
    const response = await fetchRule(id)
    const rule = response.data
    if (!rule) {
      throw new Error('规则不存在')
    }
    editingId.value = id
    form.name = rule.name
    form.enabled = rule.enabled
    form.monitor_enabled = rule.monitor_enabled
    form.schedule_enabled = rule.run_mode === 'cron' || Boolean(rule.cron_expression)
    form.link_mode = rule.link_mode === 'hard' ? 'hard' : 'soft'
    form.source_dir = rule.source_dir
    form.target_dir = rule.target_dir
    form.cron_expression = rule.cron_expression || defaultCron
    form.run_on_start = rule.run_on_start
    dialogVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载规则失败')
  }
}

async function submitForm() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入规则名称')
    return
  }
  if (!form.source_dir.trim()) {
    ElMessage.warning('请输入监控目录')
    return
  }
  if (!form.target_dir.trim()) {
    ElMessage.warning('请输入链路目录')
    return
  }

  try {
    const payload = buildPayload()
    if (editingId.value) {
      await updateRule(editingId.value, payload)
      ElMessage.success('链路规则已更新')
    } else {
      await createRule(payload)
      ElMessage.success('链路规则已创建')
    }
    dialogVisible.value = false
    await loadItems()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存规则失败')
  }
}

async function removeItem(id: number) {
  try {
    await ElMessageBox.confirm('确认删除该链路规则？', '删除链路规则', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
    await deleteRule(id)
    ElMessage.success('链路规则已删除')
    await loadItems()
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(error instanceof Error ? error.message : '删除规则失败')
  }
}

function prepareExecution(id: number) {
  ElMessage.info(`链路规则 #${id} 可复用现有执行接口进行手动执行`)
}

onMounted(() => {
  void loadItems()
})
</script>

<style scoped lang="scss">
.rules-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.page-card {
  border: 1px solid var(--border-color);
  border-radius: 18px;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.08);
}

.rules-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.rules-card__title {
  font-size: 18px;
  font-weight: 700;
}

.mode-config-panel__description {
  margin-top: 6px;
  color: var(--text-secondary);
  font-size: 13px;
}

.rules-error {
  margin-bottom: 4px;
}

.rule-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.rule-action {
  font-size: 16px;
}

.rule-action--primary {
  color: #409eff;
}

.rule-action--success {
  color: #67c23a;
}

.rule-action--danger {
  color: #f56c6c;
}

.history-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 18px;
}

.archive-mode-group {
  width: 100%;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
