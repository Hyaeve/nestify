<template>
  <el-card class="page-card">
    <template #header>
      <span>系统设置</span>
    </template>

    <div class="settings-grid">
      <section class="settings-panel">
        <div class="settings-panel__title">基础设置</div>
        <el-form label-position="top" class="settings-basic-form">
          <div class="settings-field-card">
            <div class="settings-field-card__main">
              <div class="settings-field-card__label">日志保留天数</div>
              <div class="settings-help">默认保留 5 天，超过 {{ settingsForm.logRetentionDays || 5 }} 天的日志会自动清理。</div>
            </div>
            <el-input-number v-model="settingsForm.logRetentionDays" :min="1" :max="3650" />
          </div>

          <div class="settings-field-card">
            <div class="settings-field-card__main">
              <div class="settings-field-card__label">最大日志条数</div>
              <div class="settings-help">默认保留 10000 条，超过上限后会自动删除更早的日志。</div>
            </div>
            <el-input-number v-model="settingsForm.logRetentionMaxRecords" :min="1" :max="1000000" />
          </div>

          <div class="settings-field-card settings-field-card--mode">
            <div class="settings-field-card__main">
              <div class="settings-field-card__label">归巢历史展示方式</div>
              <div class="settings-help">控制规则管理中归巢历史的默认展示方式，保存后刷新或重新进入页面仍会保持当前选择。</div>
            </div>
            <div class="settings-view-mode-group" role="group" aria-label="归巢历史展示方式">
              <el-tooltip content="平铺" placement="top" :show-after="300">
                <button
                  type="button"
                  class="settings-view-mode-button"
                  :class="{ 'is-active': settingsForm.historyViewMode === 'flat' }"
                  aria-label="平铺"
                  :aria-pressed="settingsForm.historyViewMode === 'flat'"
                  @click="settingsForm.historyViewMode = 'flat'"
                >
                  <svg class="settings-view-mode-icon" viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M5 6.5h14" />
                    <path d="M5 12h14" />
                    <path d="M5 17.5h14" />
                  </svg>
                </button>
              </el-tooltip>
              <el-tooltip content="折叠" placement="top" :show-after="300">
                <button
                  type="button"
                  class="settings-view-mode-button"
                  :class="{ 'is-active': settingsForm.historyViewMode === 'tree' }"
                  aria-label="折叠"
                  :aria-pressed="settingsForm.historyViewMode === 'tree'"
                  @click="settingsForm.historyViewMode = 'tree'"
                >
                  <svg class="settings-view-mode-icon" viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M6 6.5h12" />
                    <path d="M6 6.5v11" />
                    <path d="M9 12h9" />
                    <path d="M9 17.5h9" />
                    <path d="M6 12h3" />
                    <path d="M6 17.5h3" />
                  </svg>
                </button>
              </el-tooltip>
            </div>
          </div>

          <div class="settings-submit-row">
            <el-button type="primary" :loading="settingsSubmitting" @click="submitSettings">保存设置</el-button>
          </div>
        </el-form>
      </section>

      <section class="settings-panel">
        <div class="settings-panel__title">管理员账号</div>
        <el-form label-position="top">
          <el-form-item label="管理员账号">
            <el-input v-model="adminForm.username" placeholder="管理员账号" />
          </el-form-item>
          <el-form-item label="当前密码">
            <el-input v-model="adminForm.currentPassword" type="password" show-password placeholder="请输入当前密码" />
          </el-form-item>
          <el-form-item label="新密码">
            <el-input v-model="adminForm.newPassword" type="password" show-password placeholder="留空则不修改密码" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="submitting" @click="submitAdminChange">保存修改</el-button>
          </el-form-item>
        </el-form>
      </section>

      <section class="settings-panel">
        <div class="settings-panel__title">规则备份</div>
        <div class="settings-help">可导出当前全部规则配置为备份文件，也可导入此前导出的备份以覆盖恢复现有规则。</div>
        <div class="settings-actions">
          <el-button type="primary" :loading="exportingRules" @click="handleExportRulesBackup">导出规则</el-button>
          <el-upload
            :show-file-list="false"
            accept="application/json,.json"
            :auto-upload="false"
            :on-change="handleBackupFileChange"
          >
            <el-button :loading="importingRules">导入规则备份</el-button>
          </el-upload>
        </div>
      </section>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadFile } from 'element-plus'

import { updateAdminAccount } from '../api/auth'
import { exportRulesBackup, fetchSettings, importRulesBackup, updateSettings } from '../api/system'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const submitting = ref(false)
const settingsSubmitting = ref(false)
const exportingRules = ref(false)
const importingRules = ref(false)

const adminForm = reactive({
  username: authStore.user?.username ?? '',
  currentPassword: '',
  newPassword: '',
})

type HistoryViewMode = 'flat' | 'tree'

function normalizeHistoryViewMode(value?: string): HistoryViewMode {
  return value === 'tree' ? 'tree' : 'flat'
}

const settingsForm = reactive({
  logRetentionDays: 5,
  logRetentionMaxRecords: 10000,
  historyViewMode: 'flat' as HistoryViewMode,
})

onMounted(() => {
  adminForm.username = authStore.user?.username ?? ''
  void loadSettings()
})

async function loadSettings() {
  try {
    const response = await fetchSettings()
    if (response.data) {
      settingsForm.logRetentionDays = response.data.log_retention_days || 5
      settingsForm.logRetentionMaxRecords = response.data.log_retention_max_records || 10000
      settingsForm.historyViewMode = normalizeHistoryViewMode(response.data.history_view_mode)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '系统设置加载失败')
  }
}

async function submitSettings() {
  settingsSubmitting.value = true
  try {
    await updateSettings({
      log_retention_days: settingsForm.logRetentionDays,
      log_retention_max_records: settingsForm.logRetentionMaxRecords,
      history_view_mode: settingsForm.historyViewMode,
    })
    ElMessage.success('系统设置已保存')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '系统设置保存失败')
  } finally {
    settingsSubmitting.value = false
  }
}

async function submitAdminChange() {
  submitting.value = true
  try {
    const response = await updateAdminAccount({
      username: adminForm.username,
      current_password: adminForm.currentPassword,
      new_password: adminForm.newPassword,
    })
    if (response.data) {
      authStore.user = response.data
      adminForm.username = response.data.username
    }
    adminForm.currentPassword = ''
    adminForm.newPassword = ''
    ElMessage.success('管理员账号信息已更新')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '管理员账号更新失败')
  } finally {
    submitting.value = false
  }
}

async function handleExportRulesBackup() {
  exportingRules.value = true
  try {
    const blob = await exportRulesBackup()
    const fileName = `nestify-rules-backup-${new Date().toISOString().replace(/[:.]/g, '-')}.json`
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    ElMessage.success('规则备份已导出')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '导出规则备份失败')
  } finally {
    exportingRules.value = false
  }
}

async function handleBackupFileChange(file: UploadFile) {
  if (!file.raw) {
    ElMessage.error('未选择有效备份文件')
    return
  }

  importingRules.value = true
  try {
    await ElMessageBox.confirm('导入备份将覆盖当前全部规则配置，是否继续？', '导入规则备份', { type: 'warning' })
    const text = await file.raw.text()
    const payload = JSON.parse(text) as { version: string; exported_at: string; rules: Array<Record<string, unknown>> }
    const response = await importRulesBackup(payload)
    ElMessage.success(`规则备份已导入，共恢复 ${response.data?.count || 0} 条规则`)
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error instanceof Error ? error.message : '导入规则备份失败')
  } finally {
    importingRules.value = false
  }
}
</script>

<style scoped>
.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.settings-panel {
  padding: 22px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 18px;
  background: var(--el-bg-color);
}

.settings-panel__title {
  margin-bottom: 18px;
  color: #0f172a;
  font-size: 18px;
  font-weight: 800;
}

.settings-help {
  margin-top: 6px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}

.settings-basic-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.settings-field-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 16px 18px;
  border: 1px solid #edf2f7;
  border-radius: 16px;
  background: linear-gradient(180deg, #ffffff 0%, #fbfdff 100%);
}

.settings-field-card__main {
  min-width: 0;
}

.settings-field-card__label {
  color: #1e293b;
  font-size: 14px;
  font-weight: 800;
}

.settings-field-card :deep(.el-input-number) {
  flex: 0 0 auto;
}

.settings-field-card--mode {
  align-items: flex-start;
}

.settings-submit-row {
  display: flex;
  margin-top: 4px;
}

.settings-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
  flex-wrap: wrap;
}

.settings-view-mode-group {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 42px;
  padding: 4px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 16px;
  background: rgba(248, 250, 252, 0.72);
  backdrop-filter: blur(8px);
}

.settings-view-mode-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border: 0;
  border-radius: 12px;
  color: #64748b;
  background: transparent;
  cursor: pointer;
  transition:
    color 0.18s ease,
    background 0.18s ease,
    box-shadow 0.18s ease,
    transform 0.18s ease;
}

.settings-view-mode-button:hover {
  color: #2563eb;
  background: #eff6ff;
}

.settings-view-mode-button.is-active {
  color: #2563eb;
  background: rgba(37, 99, 235, 0.14);
  box-shadow: inset 0 0 0 1px rgba(37, 99, 235, 0.18), 0 8px 18px rgba(37, 99, 235, 0.1);
}

.settings-view-mode-button:active {
  transform: translateY(1px);
}

.settings-view-mode-icon {
  width: 20px;
  height: 20px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
