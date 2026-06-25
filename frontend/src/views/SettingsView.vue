<template>
  <el-card class="page-card">
    <template #header>
      <span>系统设置</span>
    </template>

    <div class="settings-grid">
      <section class="settings-panel">
        <div class="settings-panel__title">基础设置</div>
        <el-form label-position="top">
          <el-form-item label="日志保留天数">
            <el-input-number v-model="settingsForm.logRetentionDays" :min="1" :max="3650" />
            <div class="settings-help">默认保留 5 天，超过 {{ settingsForm.logRetentionDays || 5 }} 天的日志会自动清理。</div>
          </el-form-item>
          <el-form-item label="最大日志条数">
            <el-input-number v-model="settingsForm.logRetentionMaxRecords" :min="1" :max="1000000" />
            <div class="settings-help">默认保留 10000 条，超过上限后会自动删除更早的日志。</div>
          </el-form-item>
          <el-form-item label="归巢历史展示方式">
            <el-radio-group v-model="settingsForm.historyViewMode" class="settings-view-mode-group">
              <el-radio-button value="flat">平铺</el-radio-button>
              <el-radio-button value="tree">堆放</el-radio-button>
            </el-radio-group>
            <div class="settings-help">控制规则管理中归巢历史的默认展示方式，保存后刷新或重新进入页面仍会保持当前选择。</div>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="settingsSubmitting" @click="submitSettings">保存设置</el-button>
          </el-form-item>
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
  padding: 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
}

.settings-panel__title {
  margin-bottom: 12px;
  font-size: 16px;
  font-weight: 600;
}

.settings-help {
  margin-top: 8px;
  font-size: 12px;
  line-height: 1.6;
  color: var(--el-text-color-secondary);
}

.settings-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
  flex-wrap: wrap;
}

.settings-view-mode-group {
  display: inline-flex;
}
</style>
