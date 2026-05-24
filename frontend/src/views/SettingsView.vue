<template>
  <el-card class="page-card">
    <template #header>
      <span>系统设置</span>
    </template>

    <div class="settings-grid">
      <section class="settings-panel">
        <div class="settings-panel__title">基础设置</div>
        <el-form label-position="top">
          <el-form-item label="系统版本">
            <el-input :value="appVersion" readonly />
          </el-form-item>
          <el-form-item label="日志保留天数">
            <el-input value="30" />
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
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'

import { updateAdminAccount } from '../api/auth'
import { useAuthStore } from '../stores/auth'

const appVersion = 'v0.9.1'
const authStore = useAuthStore()
const submitting = ref(false)

const adminForm = reactive({
  username: authStore.user?.username ?? '',
  currentPassword: '',
  newPassword: '',
})

onMounted(() => {
  adminForm.username = authStore.user?.username ?? ''
})

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
</style>
