<template>
  <div class="login-page">
    <el-card class="login-card">
      <template #header>
        <span>Nestify 登录</span>
      </template>

      <el-alert
        v-if="errorMessage"
        style="margin-bottom: 16px;"
        type="error"
        :closable="false"
        :title="errorMessage"
      />

      <el-form label-position="top" @submit.prevent>
        <el-form-item label="管理员账号">
          <el-input v-model="username" placeholder="admin" />
        </el-form-item>

        <el-form-item label="密码">
          <el-input v-model="password" type="password" placeholder="请输入密码" show-password @keyup.enter="handleLogin" />
        </el-form-item>

        <el-button type="primary" class="login-button" :loading="submitting" @click="handleLogin">登录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const username = ref('admin')
const password = ref('')
const submitting = ref(false)
const errorMessage = ref('')

async function handleLogin() {
  submitting.value = true
  errorMessage.value = ''

  try {
    await authStore.login(username.value, password.value)
    ElMessage.success('登录成功')
    await router.push('/dashboard')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '登录失败'
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="scss">
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #eef4ff 0%, #f7fbff 100%);
}

.login-card {
  width: 420px;
  border-radius: 20px;
}

.login-button {
  width: 100%;
}
</style>

