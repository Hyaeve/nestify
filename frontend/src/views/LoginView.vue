<template>
  <div class="login-page">
    <div class="login-page__glow login-page__glow--left"></div>
    <div class="login-page__glow login-page__glow--right"></div>

    <el-card class="login-card">
      <div class="login-brand">
        <img class="login-brand__logo" src="/nestify-logo.png" alt="Nestify logo" />
        <div>
          <div class="login-brand__title">Nestify 登录</div>
          <div class="login-brand__subtitle">Version v5.2 · 媒体归档管理台</div>
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
          <el-input v-model="username" autocomplete="username" />
        </el-form-item>

        <el-form-item label="密码">
          <el-input v-model="password" type="password" autocomplete="current-password" show-password @keyup.enter="handleLogin" />
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

const username = ref('')
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
  overflow: hidden;
  isolation: isolate;
  background:
    radial-gradient(circle at 18% 16%, rgba(45, 212, 191, 0.18), transparent 24%),
    radial-gradient(circle at 78% 12%, rgba(168, 85, 247, 0.2), transparent 22%),
    radial-gradient(circle at 82% 82%, rgba(244, 114, 182, 0.18), transparent 26%),
    linear-gradient(135deg, #07111a 0%, #111827 50%, #1b1330 100%);
}

.login-page::before {
  content: '';
  position: absolute;
  inset: -18%;
  z-index: 0;
  background:
    radial-gradient(circle at 18% 78%, rgba(236, 72, 153, 0.42), transparent 22%),
    radial-gradient(circle at 76% 24%, rgba(34, 211, 238, 0.34), transparent 20%),
    radial-gradient(circle at 62% 72%, rgba(192, 132, 252, 0.26), transparent 24%),
    radial-gradient(circle at 35% 30%, rgba(59, 130, 246, 0.22), transparent 26%);
  filter: blur(92px) saturate(118%);
  opacity: 0.95;
  transform: scale(1.05);
}

.login-page::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='220' height='220' viewBox='0 0 220 220'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='1.15' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='220' height='220' filter='url(%23n)' opacity='0.75'/%3E%3C/svg%3E");
  background-size: 180px 180px;
  opacity: 0.16;
  mix-blend-mode: screen;
}

.login-page__glow {
  position: absolute;
  width: 280px;
  height: 280px;
  border-radius: 50%;
  filter: blur(60px);
  opacity: 0.52;
  z-index: 0;
}

.login-page__glow--left {
  top: 8%;
  left: 8%;
  background: rgba(59, 130, 246, 0.34);
}

.login-page__glow--right {
  right: 10%;
  bottom: 8%;
  background: rgba(244, 114, 182, 0.28);
}

.login-card {
  position: relative;
  z-index: 1;
  width: 500px;
  padding: 10px;
  border-radius: 24px;
  background: rgba(15, 23, 42, 0.82);
  border: 1px solid rgba(148, 163, 184, 0.2);
  backdrop-filter: blur(18px);
}

.login-brand {
  display: flex;
  align-items: center;
  gap: 18px;
  margin-bottom: 24px;
}

.login-brand__logo {
  width: 64px;
  height: 64px;
  border-radius: 16px;
  object-fit: cover;
  box-shadow: 0 16px 40px rgba(245, 185, 66, 0.2);
}

.login-brand__title {
  font-size: 20px;
  font-weight: 700;
  color: #f8fafc;
}

.login-brand__subtitle {
  margin-top: 4px;
  font-size: 14px;
  color: #94a3b8;
}

.login-button {
  width: 100%;
  min-height: 40px;
}

@media (max-width: 768px) {
  .login-page {
    padding: 16px;
  }

  .login-card {
    width: min(100%, 500px);
    padding: 6px;
  }
}
</style>

