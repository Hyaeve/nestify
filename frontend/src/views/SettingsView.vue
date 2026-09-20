<template>
  <el-card class="page-card">
    <template #header>
      <span>系统设置</span>
    </template>

    <div class="settings-layout">
      <!-- 左侧：登录账户（顶部）+ 基础设置 + 规则备份 -->
      <div class="settings-layout__left">
        <section class="settings-panel settings-panel--account">
          <div class="settings-panel__title-row">
            <div class="settings-panel__title">登录账户</div>
            <el-button type="primary" class="settings-btn-lg settings-btn-solid settings-btn-solid--account" :loading="submitting" @click="submitAdminChange">保存修改</el-button>
          </div>
          <el-form label-position="top" @submit.prevent="submitAdminChange">
            <el-form-item label="账户">
              <el-input v-model="adminForm.username" placeholder="登录账户" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input
                v-model="adminForm.password"
                type="password"
                show-password
                placeholder="留空则不修改密码"
                autocomplete="new-password"
              />
            </el-form-item>
          </el-form>
        </section>

        <section class="settings-panel settings-panel--basic">
          <div class="settings-panel__title-row">
            <div class="settings-panel__title">基础设置</div>
            <el-button type="primary" class="settings-btn-lg settings-btn-solid settings-btn-solid--settings" :loading="settingsSubmitting" @click="submitSettings">保存设置</el-button>
          </div>
          <el-form label-position="top" class="settings-basic-form">
            <el-form-item label="启动页面">
              <el-select v-model="settingsForm.defaultPage">
                <el-option v-for="option in startupPageOptions" :key="option.value" :label="option.label" :value="option.value" />
              </el-select>
            </el-form-item>

            <el-form-item label="日志保留天数">
              <el-input-number v-model="settingsForm.logRetentionDays" :min="1" :max="3650" />
            </el-form-item>

            <el-form-item label="最大日志条数">
              <el-input-number v-model="settingsForm.logRetentionMaxRecords" :min="1" :max="1000000" />
            </el-form-item>

            <el-form-item label="每页文件数">
              <el-select v-model="settingsForm.pageSize">
                <el-option v-for="size in pageSizeOptions" :key="size" :label="String(size)" :value="size" />
              </el-select>
            </el-form-item>

          </el-form>
        </section>

        <section class="settings-panel settings-panel--backup">
          <div class="settings-panel__title">规则备份</div>
          <div class="settings-actions">
            <el-button class="settings-btn-lg settings-actions__btn--backup" :loading="exportingRules" @click="handleExportRulesBackup">备份</el-button>
            <el-upload
              :show-file-list="false"
              accept="application/json,.json"
              :auto-upload="false"
              :on-change="handleBackupFileChange"
            >
              <el-button class="settings-btn-lg settings-actions__btn--restore" :loading="importingRules">还原</el-button>
            </el-upload>
          </div>
        </section>
      </div>

      <!-- 右侧：远程挂载（占位更大） -->
      <div class="settings-layout__right">
        <section class="settings-panel settings-panel--mounts">
          <div class="settings-panel__header">
            <div class="settings-panel__title">远程挂载</div>
            <el-button type="primary" class="settings-btn-lg settings-btn-solid settings-btn-solid--mount" @click="openAddMount">添加挂载</el-button>
          </div>

          <div v-if="mounts.length === 0" class="mounts-empty">
            暂无远程挂载。
          </div>

          <div v-else class="mounts-list">
            <div
              v-for="mount in mounts"
              :key="mount.id"
              class="mount-card"
              :class="{ 'mount-card--disabled': !mount.enabled }"
              @click="openEditMount(mount)"
              @contextmenu.prevent="openMountContextMenu($event, mount)"
            >
              <div class="mount-card__icon" :class="{ 'mount-card__icon--openlist': mount.provider === 'openlist' }">
                <img v-if="mount.provider === 'openlist'" class="mount-card__icon-img" src="/Olist.png" alt="" />
                <svg v-else viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M2.5 8.5h19" />
                  <path d="M4 6.5h16a1.5 1.5 0 0 1 1.5 1.5v10a1.5 1.5 0 0 1-1.5 1.5H4A1.5 1.5 0 0 1 2.5 18V8A1.5 1.5 0 0 1 4 6.5Z" />
                  <path d="M2.5 8.5a1.5 1.5 0 0 1 1.5-1.5h4.3l2 2h9.2a1.5 1.5 0 0 1 1.5 1.5" />
                </svg>
              </div>
              <div class="mount-card__body">
                <div class="mount-card__name">{{ mount.name }}</div>
                <div class="mount-card__meta">{{ mount.provider === '115' ? `115 网盘 · ${mount.device}` : mount.base_url }}</div>
                <div class="mount-card__state">
                  <span class="mount-card__dot" :class="{ 'is-on': mount.enabled }"></span>
                  {{ mount.enabled ? '已启用' : '已停用' }}
                </div>
              </div>
              <div class="mount-card__actions" @click.stop>
                <el-dropdown trigger="click" @command="(cmd: string) => handleMountCommand(cmd, mount)">
                  <button type="button" class="mount-card__more" aria-label="更多操作">
                    <svg viewBox="0 0 24 24" aria-hidden="true">
                      <circle cx="5" cy="12" r="1.6" />
                      <circle cx="12" cy="12" r="1.6" />
                      <circle cx="19" cy="12" r="1.6" />
                    </svg>
                  </button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item :command="mount.enabled ? 'disable' : 'enable'">
                        {{ mount.enabled ? '停用' : '启用' }}
                      </el-dropdown-item>
                      <el-dropdown-item command="edit">编辑</el-dropdown-item>
                      <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </div>
          </div>
        </section>

        <section class="settings-panel settings-panel--cache">
          <div class="settings-panel__title">临时缓存</div>
          <el-form label-position="top" class="cache-form">
            <el-form-item label="缓存目录">
              <el-input v-model="cacheForm.cacheDir" placeholder="/tmp">
                <template #append>
                  <el-button class="cache-form__dir-button" @click="openCacheDirPicker">选择目录</el-button>
                </template>
              </el-input>
              <div class="cache-form__hint">处理文件过程中临时文件的存储目录，选择后即保存。</div>
            </el-form-item>

            <el-form-item>
              <div class="cache-switch-row">
                <el-switch v-model="cacheForm.cachePersistEnabled" @change="persistCacheSettings" />
                <span class="cache-switch-row__label">启用缓存持久化</span>
              </div>
              <div class="cache-form__hint">将缓存数据存储在数据库中，以便在应用程序重启后保持。</div>
            </el-form-item>

            <OutlinedGroup label="忽略扩展名">
              <div class="cache-ignore-tags">
                <span v-for="(ext, index) in cacheForm.ignoredExtensions" :key="ext" class="cache-ignore-tag">
                  {{ ext }}
                  <button type="button" class="cache-ignore-tag__remove" @click="removeIgnoredExtension(index)">×</button>
                </span>
                <el-input
                  v-model="cacheIgnoreDraft"
                  field-label="添加扩展名"
                  class="cache-ignore-input"
                  placeholder="回车添加，例如 .tmp"
                  @keyup.enter="commitIgnoredExtension"
                />
              </div>
            </OutlinedGroup>
          </el-form>
        </section>
      </div>
    </div>

    <DirectoryPickerDialog
      v-model="cacheDirPickerVisible"
      title="选择缓存目录"
      :initial-path="cacheForm.cacheDir"
      @selected="handleCacheDirSelected"
    />

    <!-- 挂载右键菜单（自定义，位于卡片上下文） -->
    <div
      v-if="contextMenu.visible && contextMenu.mount"
      class="mount-context-menu"
      :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
      @click.stop
    >
      <button
        type="button"
        class="mount-context-menu__item"
        @click="handleMountCommand(contextMenu.mount!.enabled ? 'disable' : 'enable', contextMenu.mount!)"
      >
        {{ contextMenu.mount!.enabled ? '停用' : '启用' }}
      </button>
      <button type="button" class="mount-context-menu__item" @click="handleMountCommand('edit', contextMenu.mount!)">
        编辑
      </button>
      <button type="button" class="mount-context-menu__item mount-context-menu__item--danger" @click="handleMountCommand('delete', contextMenu.mount!)">
        删除
      </button>
    </div>

    <WebdavMountDialog v-model="mountDialogVisible" :mount="editingMount" @saved="loadMounts" />
  </el-card>
</template>

<script setup lang="ts">
import { OutlinedInput as ElInput, OutlinedInputNumber as ElInputNumber, OutlinedSelect as ElSelect } from '../components/outlinedControls'
import OutlinedGroup from '../components/OutlinedGroup.vue'
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadFile } from 'element-plus'

import { updateAdminAccount } from '../api/auth'
import { exportRulesBackup, fetchSettings, importRulesBackup, updateSettings } from '../api/system'
import {
  deleteMount,
  fetchMounts,
  fetchMountUsage,
  updateMount,
  type WebdavMount,
} from '../api/mounts'
import { useAuthStore } from '../stores/auth'
import { pageSizeOptions, startupPageOptions, useSettingsStore } from '../stores/settings'
import WebdavMountDialog from '../components/WebdavMountDialog.vue'
import DirectoryPickerDialog from '../components/DirectoryPickerDialog.vue'

const authStore = useAuthStore()
const settingsStore = useSettingsStore()
const submitting = ref(false)
const settingsSubmitting = ref(false)
const exportingRules = ref(false)
const importingRules = ref(false)

const adminForm = reactive({
  username: authStore.user?.username ?? '',
  password: '',
})

const settingsForm = reactive({
  logRetentionDays: 5,
  logRetentionMaxRecords: 10000,
  defaultPage: 'dashboard',
  pageSize: 50,
})

// —— 临时缓存设置 ——
const cacheForm = reactive({
  cacheDir: '/tmp',
  cachePersistEnabled: true,
  ignoredExtensions: [] as string[],
})
const cacheIgnoreDraft = ref('')
const cacheDirPickerVisible = ref(false)

// —— WebDAV 挂载状态 ——
const mounts = ref<WebdavMount[]>([])
const mountDialogVisible = ref(false)
const editingMount = ref<WebdavMount | null>(null)
const contextMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  mount: null as WebdavMount | null,
})

onMounted(() => {
  adminForm.username = authStore.user?.username ?? ''
  void loadSettings()
  void loadMounts()
  document.addEventListener('click', closeContextMenu)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeContextMenu)
})

async function loadSettings() {
  try {
    const response = await fetchSettings()
    if (response.data) {
      settingsForm.logRetentionDays = response.data.log_retention_days || 5
      settingsForm.logRetentionMaxRecords = response.data.log_retention_max_records || 10000
      settingsForm.defaultPage = response.data.default_page || 'dashboard'
      settingsForm.pageSize = response.data.page_size || 50
      cacheForm.cacheDir = response.data.cache_dir || '/tmp'
      cacheForm.cachePersistEnabled = response.data.cache_persist_enabled !== false
      cacheForm.ignoredExtensions = response.data.ignored_extensions ?? []
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
      default_page: settingsForm.defaultPage,
      page_size: settingsForm.pageSize,
      cache_dir: cacheForm.cacheDir,
      cache_persist_enabled: cacheForm.cachePersistEnabled,
      ignored_extensions: cacheForm.ignoredExtensions,
      upload_queue_upper_limit: 0,
      upload_queue_lower_limit: 0,
      max_concurrent_scans: 0,
    })
    settingsStore.applyLocal(settingsForm.defaultPage, settingsForm.pageSize)
    ElMessage.success('系统设置已保存')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '系统设置保存失败')
  } finally {
    settingsSubmitting.value = false
  }
}

// —— 临时缓存 ——
function openCacheDirPicker() {
  cacheDirPickerVisible.value = true
}

async function handleCacheDirSelected(path: string) {
  cacheForm.cacheDir = path
  cacheDirPickerVisible.value = false
  await persistCacheSettings()
}

function commitIgnoredExtension() {
  const value = cacheIgnoreDraft.value.trim()
  if (!value) return
  if (!cacheForm.ignoredExtensions.includes(value)) {
    cacheForm.ignoredExtensions.push(value)
  }
  cacheIgnoreDraft.value = ''
  void persistCacheSettings()
}

function removeIgnoredExtension(index: number) {
  cacheForm.ignoredExtensions.splice(index, 1)
  void persistCacheSettings()
}

async function persistCacheSettings() {
  try {
    await updateSettings({
      log_retention_days: settingsForm.logRetentionDays,
      log_retention_max_records: settingsForm.logRetentionMaxRecords,
      default_page: settingsForm.defaultPage,
      page_size: settingsForm.pageSize,
      cache_dir: cacheForm.cacheDir,
      cache_persist_enabled: cacheForm.cachePersistEnabled,
      ignored_extensions: cacheForm.ignoredExtensions,
      upload_queue_upper_limit: 0,
      upload_queue_lower_limit: 0,
      max_concurrent_scans: 0,
    })
    settingsStore.applyLocal(settingsForm.defaultPage, settingsForm.pageSize)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '缓存设置保存失败')
  }
}
async function submitAdminChange() {
  if (!adminForm.username.trim()) {
    ElMessage.error('账户不能为空')
    return
  }
  submitting.value = true
  try {
    const response = await updateAdminAccount({
      username: adminForm.username.trim(),
      password: adminForm.password,
    })
    if (response.data) {
      authStore.user = response.data
      adminForm.username = response.data.username
    }
    adminForm.password = ''
    ElMessage.success('登录账户已更新')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '登录账户更新失败')
  } finally {
    submitting.value = false
  }
}

// —— WebDAV 挂载 ——
async function loadMounts() {
  try {
    const response = await fetchMounts()
    mounts.value = response.data?.items ?? []
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '挂载列表加载失败')
  }
}

function openAddMount() {
  editingMount.value = null
  mountDialogVisible.value = true
}

function openEditMount(mount: WebdavMount) {
  editingMount.value = mount
  mountDialogVisible.value = true
}

function openMountContextMenu(event: MouseEvent, mount: WebdavMount) {
  contextMenu.mount = mount
  contextMenu.x = event.clientX
  contextMenu.y = event.clientY
  contextMenu.visible = true
}

function closeContextMenu() {
  contextMenu.visible = false
}

/**
 * 删除挂载前的引用提示（只提示、不拦截）。
 * 挂载编号（webdav://<id>）是数据库主键、删除后不会重排，其它挂载的既有路径不受影响；
 * 但被删挂载的引用方会在运行时失败，所以先把引用它的规则 / 备份任务列给用户看。
 * 引用检查本身失败不阻断删除流程。
 */
async function describeMountUsage(mountId: number) {
  try {
    // getJSON 返回 ApiResponse<T>，取值前必须先 .data（否则 TS2339）。
    const response = await fetchMountUsage(mountId)
    const ruleNames = response.data?.rule_names ?? []
    const backupNames = response.data?.backup_names ?? []
    const parts: string[] = []
    if (ruleNames.length) {
      parts.push(`${ruleNames.length} 条规则（${ruleNames.join('、')}）`)
    }
    if (backupNames.length) {
      parts.push(`${backupNames.length} 个备份任务（${backupNames.join('、')}）`)
    }
    if (!parts.length) return ''
    return `该挂载仍被 ${parts.join('、')} 引用，删除后它们将执行失败。`
  } catch {
    return ''
  }
}

async function handleMountCommand(command: string, mount: WebdavMount) {
  closeContextMenu()
  if (command === 'edit') {
    openEditMount(mount)
    return
  }
  if (command === 'enable' || command === 'disable') {
    const enabled = command === 'enable'
    try {
      await updateMount(mount.id, {
        device: mount.device,
        request_interval_ms: mount.request_interval_ms,
        cookie: '',
        name: mount.name,
        provider: mount.provider,
        auth_type: mount.auth_type,
        scheme: mount.scheme,
        host: mount.host,
        port: mount.port,
        username: mount.username,
        // 留空表示沿用已保存的口令与令牌，这里只切启用状态。
        password: '',
        token: '',
        base_path: mount.base_path,
        enabled,
        sort_order: mount.sort_order,
      })
      ElMessage.success(enabled ? '挂载已启用' : '挂载已停用')
      await loadMounts()
    } catch (error) {
      ElMessage.error(error instanceof Error ? error.message : '操作失败')
    }
    return
  }
  if (command === 'delete') {
    try {
      await ElMessageBox.confirm(
        `确认删除挂载「${mount.name}」？${await describeMountUsage(mount.id)}`,
        '删除挂载',
        { type: 'warning', confirmButtonText: '仍然删除', cancelButtonText: '取消' },
      )
      await deleteMount(mount.id)
      ElMessage.success('挂载已删除')
      await loadMounts()
    } catch (error) {
      if (error === 'cancel' || error === 'close') return
      ElMessage.error(error instanceof Error ? error.message : '删除失败')
    }
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
    ElMessage.success('规则已备份')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '备份规则失败')
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
    await ElMessageBox.confirm('还原将覆盖当前全部规则配置，是否继续？', '还原规则', { type: 'warning' })
    const text = await file.raw.text()
    const payload = JSON.parse(text) as { version: string; exported_at: string; rules: Array<Record<string, unknown>> }
    const response = await importRulesBackup(payload)
    ElMessage.success(`规则已还原，共恢复 ${response.data?.count || 0} 条规则`)
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error instanceof Error ? error.message : '还原规则失败')
  } finally {
    importingRules.value = false
  }
}
</script>

<style scoped>
.settings-layout {
  display: grid;
  grid-template-columns: minmax(0, 5fr) minmax(0, 7fr);
  gap: 16px;
  align-items: start;
}

.settings-layout__left,
.settings-layout__right {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 左侧与右侧面板统一收窄宽度：两列各占一半 */
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

.settings-panel__title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 18px;
}

.settings-panel__title-row .settings-panel__title {
  margin-bottom: 0;
}

.settings-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}

.settings-panel__header .settings-panel__title {
  margin-bottom: 0;
}

/* —— WebDAV 挂载卡片 —— */
.mounts-empty {
  padding: 22px 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  text-align: center;
}

.mounts-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  /* 固定窗口高度：恰好容纳 4 个挂载横条，超出上下滑动 */
  height: calc(4 * 80px + 3 * 12px);
  overflow-y: auto;
  padding-right: 4px;
}

.mount-card {
  display: flex;
  align-items: center;
  gap: 14px;
  flex: 0 0 auto;
  height: 80px;
  padding: 12px 16px;
  border: 1px solid #edf2f7;
  border-radius: 14px;
  background: linear-gradient(180deg, #ffffff 0%, #fbfdff 100%);
  cursor: pointer;
  transition:
    border-color 0.18s ease,
    box-shadow 0.18s ease,
    transform 0.18s ease;
}

.mount-card:hover {
  border-color: rgba(32, 159, 238, 0.42);
  box-shadow: 0 8px 18px rgba(32, 159, 238, 0.08);
}

.mount-card--disabled {
  opacity: 0.55;
}

.mount-card__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  width: 38px;
  height: 38px;
  border-radius: 11px;
  color: #4f9d69;
  background: #eaf6ee;
}

.mount-card__icon svg {
  width: 22px;
  height: 22px;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
}

/* OpenList 挂载用项目自带的品牌图标，不用线框图形。 */
.mount-card__icon--openlist {
  padding: 3px;
  background: #ffffff;
  box-shadow: inset 0 0 0 1px rgba(15, 23, 42, 0.08);
}

.mount-card__icon-img {
  display: block;
  width: 100%;
  height: 100%;
  border-radius: 8px;
  object-fit: contain;
}

.mount-card__body {
  min-width: 0;
  flex: 1 1 auto;
}

.mount-card__name {
  color: #1e293b;
  font-size: 14px;
  font-weight: 800;
  line-height: 18px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mount-card__meta {
  margin-top: 1px;
  font-size: 12px;
  line-height: 15px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mount-card__state {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  font-size: 12px;
  line-height: 15px;
  color: var(--el-text-color-regular);
}

.mount-card__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #cbd5e1;
}

.mount-card__dot.is-on {
  background: #22c55e;
}

.mount-card__actions {
  flex: 0 0 auto;
}

.mount-card__more {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 10px;
  color: #64748b;
  background: transparent;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.mount-card__more:hover {
  color: #0975b8;
  background: rgba(32, 159, 238, 0.14);
}

.mount-card__more svg {
  width: 18px;
  height: 18px;
  fill: currentColor;
}

/* —— 右键菜单 —— */
.mount-context-menu {
  position: fixed;
  z-index: 3000;
  min-width: 140px;
  padding: 6px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 12px;
  background: var(--el-bg-color);
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.14);
}

.mount-context-menu__item {
  display: block;
  width: 100%;
  padding: 8px 14px;
  border: 0;
  border-radius: 8px;
  text-align: left;
  font-size: 13px;
  color: var(--el-text-color-primary);
  background: transparent;
  cursor: pointer;
}

.mount-context-menu__item:hover {
  background: var(--el-fill-color-light);
}

.mount-context-menu__item--danger {
  color: #ef4444;
}

.mount-context-menu__item--danger:hover {
  background: #fef2f2;
}

/* —— 基础设置表单 —— */
.settings-basic-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.settings-basic-form :deep(.outlined-field--number) {
  width: 100%;
}

.settings-submit-row {
  display: flex;
  margin-top: 4px;
}

.settings-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

/* 系统设置里的动作按钮统一放大一档：原来用 size="small"（12px 字 / 24px 高）偏小，
   跟卡片操作条的 36px / 14px 对齐。 */
.settings-btn-lg {
  height: 36px;
  padding: 0 18px;
  border-radius: 10px;
  font-size: 14px;
}

/* 「保存修改 / 保存设置 / 添加挂载」三个动作按钮：
   配色演进：深青实心 #0e7490（L31）→ 被反馈「发闷」；高明度高饱和实心 #1fc4ad 等
   → 被反馈「辣眼睛」。现在收到中间路线：**浅色底 + 深色字**（同色相淡底 ~.15 +
   同色淡描边 ~.45 + 深一档的字），既不闷也不刺眼，且仍保有可点按感。
   三个按钮各用一个色相区分作用域（青绿=账户 / 天青=基础设置 / 靛蓝=挂载），
   不再强行同色。颜色仍走 --el-button-* 变量，scoped 选择器带上 data-v 属性后
   权重高于 EP 的 .el-button--primary（含它的 :hover/:active 链式选择器），能覆盖住。 */
.settings-btn-solid {
  --el-button-text-color: #26323f;
}

/* 保存修改（登录账户）：青绿。 */
.settings-btn-solid--account {
  --el-button-text-color: #0f8a7a;
  --el-button-bg-color: rgba(31, 196, 173, 0.15);
  --el-button-border-color: rgba(31, 196, 173, 0.45);
  --el-button-hover-text-color: #0b6f62;
  --el-button-hover-bg-color: rgba(31, 196, 173, 0.26);
  --el-button-hover-border-color: rgba(31, 196, 173, 0.75);
  --el-button-active-text-color: #095c51;
  --el-button-active-bg-color: rgba(31, 196, 173, 0.34);
  --el-button-active-border-color: rgba(31, 196, 173, 0.85);
}

/* 保存设置（基础设置）：天青。 */
.settings-btn-solid--settings {
  --el-button-text-color: #0b7cae;
  --el-button-bg-color: rgba(18, 169, 232, 0.14);
  --el-button-border-color: rgba(18, 169, 232, 0.45);
  --el-button-hover-text-color: #08638d;
  --el-button-hover-bg-color: rgba(18, 169, 232, 0.25);
  --el-button-hover-border-color: rgba(18, 169, 232, 0.75);
  --el-button-active-text-color: #064f72;
  --el-button-active-bg-color: rgba(18, 169, 232, 0.33);
  --el-button-active-border-color: rgba(18, 169, 232, 0.85);
}

/* 添加挂载（远程挂载）：靛蓝。 */
.settings-btn-solid--mount {
  --el-button-text-color: #4a56c0;
  --el-button-bg-color: rgba(90, 111, 232, 0.14);
  --el-button-border-color: rgba(90, 111, 232, 0.42);
  --el-button-hover-text-color: #3b45a0;
  --el-button-hover-bg-color: rgba(90, 111, 232, 0.24);
  --el-button-hover-border-color: rgba(90, 111, 232, 0.72);
  --el-button-active-text-color: #313a85;
  --el-button-active-bg-color: rgba(90, 111, 232, 0.32);
  --el-button-active-border-color: rgba(90, 111, 232, 0.82);
}

/* 规则「备份 / 还原」是两种不同性质的动作，各用一种浅色区分开。
   颜色走 Element Plus 的按钮变量而不是直接写 background——EP 的 hover 规则带
   `:not(.is-disabled):not(.is-text)` 链式选择器，直接写会被它的优先级压过。 */
.settings-actions__btn--backup {
  --el-button-text-color: #3d5a7d;
  --el-button-bg-color: rgba(95, 127, 168, 0.14);
  --el-button-border-color: rgba(95, 127, 168, 0.5);
  --el-button-hover-text-color: #33506f;
  --el-button-hover-bg-color: rgba(95, 127, 168, 0.24);
  --el-button-hover-border-color: rgba(95, 127, 168, 0.72);
  --el-button-active-text-color: #33506f;
  --el-button-active-bg-color: rgba(95, 127, 168, 0.3);
  --el-button-active-border-color: rgba(95, 127, 168, 0.72);
}

/* 还原是覆盖式操作，用琥珀浅色提示「会动到现有配置」。 */
.settings-actions__btn--restore {
  --el-button-text-color: #8a5a12;
  --el-button-bg-color: rgba(183, 121, 31, 0.13);
  --el-button-border-color: rgba(183, 121, 31, 0.48);
  --el-button-hover-text-color: #6f470c;
  --el-button-hover-bg-color: rgba(183, 121, 31, 0.23);
  --el-button-hover-border-color: rgba(183, 121, 31, 0.7);
  --el-button-active-text-color: #6f470c;
  --el-button-active-bg-color: rgba(183, 121, 31, 0.29);
  --el-button-active-border-color: rgba(183, 121, 31, 0.7);
}

/* —— 临时缓存 —— */
.cache-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.cache-form__hint {
  /* 占满整行，确保小字说明显示在标题/控件下方而非右侧 */
  flex: 0 0 100%;
  width: 100%;
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.6;
}

.cache-form__dir-button {
  min-width: 88px;
}

.cache-switch-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.cache-switch-row__label {
  color: #1e293b;
  font-size: 14px;
  font-weight: 800;
}

.cache-ignore-tags {
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.cache-ignore-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 20px;
  padding: 0 8px 0 10px;
  border-radius: 999px;
  font-size: 12px;
  line-height: 1;
  color: #0975b8;
  background: #e9f1f4;
}

.cache-ignore-tag__remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 13px;
  height: 13px;
  border: none;
  border-radius: 50%;
  font-size: 11px;
  line-height: 1;
  color: #0975b8;
  background: transparent;
  cursor: pointer;
}

.cache-ignore-tag__remove:hover {
  background: #dde8ee;
}

.cache-ignore-input {
  flex: 0 1 200px;
  width: auto;
  min-width: 140px;
  max-width: 240px;
}

.cache-ignore-input :deep(.el-input__wrapper) {
  min-height: 28px;
  padding: 1px 10px;
}

.cache-ignore-input :deep(.el-input__inner) {
  height: 24px;
  line-height: 24px;
}

/* 细浅隐藏的滚动条 */
.mounts-list::-webkit-scrollbar {
  width: 5px;
}

.mounts-list::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.28);
}

.mounts-list::-webkit-scrollbar-thumb:hover {
  background: rgba(148, 163, 184, 0.45);
}

.mounts-list::-webkit-scrollbar-track {
  background: transparent;
}

</style>
