<template>
  <div class="login-page">
    <div class="login-page__glow login-page__glow--left"></div>
    <div class="login-page__glow login-page__glow--right"></div>

    <el-card class="login-card">
      <div class="login-brand">
        <img class="login-brand__logo" src="/nestify-logo.png" alt="Nestify logo" />
        <div>
          <div class="login-brand__title">Nestify 登录</div>
          <div class="login-brand__subtitle">Version V0.9.1 · 媒体归档管理台</div>
        </div>
      </div>

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
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background:
    radial-gradient(circle at top left, rgba(96, 165, 250, 0.24), transparent 30%),
    radial-gradient(circle at bottom right, rgba(56, 189, 248, 0.2), transparent 28%),
    linear-gradient(135deg, #0b1220 0%, #111827 100%);
}

.login-page__glow {
  position: absolute;
  width: 280px;
  height: 280px;
  border-radius: 50%;
  filter: blur(60px);
  opacity: 0.45;
}

.login-page__glow--left {
  top: 10%;
  left: 10%;
  background: rgba(59, 130, 246, 0.4);
}

.login-page__glow--right {
  right: 12%;
  bottom: 12%;
  background: rgba(245, 185, 66, 0.26);
}

.login-card {
  position: relative;
  z-index: 1;
  width: 420px;
  border-radius: 20px;
  background: rgba(15, 23, 42, 0.82);
  border: 1px solid rgba(148, 163, 184, 0.2);
  backdrop-filter: blur(18px);
}

.login-brand {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}

.login-brand__logo {
  width: 58px;
  height: 58px;
  border-radius: 16px;
  object-fit: cover;
  box-shadow: 0 16px 40px rgba(245, 185, 66, 0.2);
}

.login-brand__title {
  font-size: 18px;
  font-weight: 700;
  color: #f8fafc;
}

.login-brand__subtitle {
  margin-top: 4px;
  font-size: 13px;
  color: #94a3b8;
}

.login-button {
  width: 100%;
}
</style>

