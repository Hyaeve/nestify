<template>
  <div class="login-page">
    <div class="login-page__glow login-page__glow--left"></div>
    <div class="login-page__glow login-page__glow--right"></div>

    <el-card class="login-card">
      <div class="login-brand">
        <img class="login-brand__logo" src="/nestify-logo.png" alt="Nestify logo" />
        <div>
          <div class="login-brand__title">Nestify 登录</div>
          <div class="login-brand__subtitle">Version v8.0 · 媒体归档管理台</div>
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
    radial-gradient(circle at 18% 18%, rgba(191, 219, 254, 0.46), transparent 28%),
    radial-gradient(circle at 78% 14%, rgba(207, 250, 254, 0.42), transparent 26%),
    radial-gradient(circle at 80% 84%, rgba(226, 232, 240, 0.58), transparent 30%),
    linear-gradient(135deg, #eef7fb 0%, #e6f0f7 46%, #f6fbff 100%);
}

.login-page::before {
  content: '';
  position: absolute;
  inset: -18%;
  z-index: 0;
  background:
    radial-gradient(circle at 16% 76%, rgba(125, 211, 252, 0.34), transparent 24%),
    radial-gradient(circle at 74% 24%, rgba(147, 197, 253, 0.28), transparent 22%),
    radial-gradient(circle at 62% 72%, rgba(226, 232, 240, 0.42), transparent 28%),
    radial-gradient(circle at 35% 30%, rgba(255, 255, 255, 0.72), transparent 30%);
  filter: blur(96px) saturate(106%);
  opacity: 0.86;
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
  opacity: 0.11;
  mix-blend-mode: soft-light;
}

.login-page__glow {
  position: absolute;
  width: 280px;
  height: 280px;
  border-radius: 50%;
  filter: blur(68px);
  opacity: 0.36;
  z-index: 0;
}

.login-page__glow--left {
  top: 8%;
  left: 8%;
  background: rgba(147, 197, 253, 0.34);
}

.login-page__glow--right {
  right: 10%;
  bottom: 8%;
  background: rgba(165, 243, 252, 0.3);
}

.login-card {
  position: relative;
  z-index: 1;
  width: 500px;
  padding: 10px;
  border-radius: 26px;
  background: rgba(255, 255, 255, 0.62);
  border: 1px solid rgba(226, 232, 240, 0.72);
  box-shadow:
    0 28px 80px rgba(15, 23, 42, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.72);
  backdrop-filter: blur(22px) saturate(120%);
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
  box-shadow: 0 18px 42px rgba(59, 130, 246, 0.12);
}

.login-brand__title {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
}

.login-brand__subtitle {
  margin-top: 4px;
  font-size: 14px;
  color: #64748b;
}

:deep(.el-card__body) {
  position: relative;
  z-index: 1;
}

:deep(.el-form-item__label) {
  color: #334155;
  font-weight: 700;
}

:deep(.el-input__wrapper) {
  min-height: 42px;
  border-radius: 14px;
  background: rgba(248, 250, 252, 0.72);
  box-shadow: 0 0 0 1px rgba(203, 213, 225, 0.72) inset;
  backdrop-filter: blur(12px);
}

:deep(.el-input__wrapper.is-focus) {
  box-shadow:
    0 0 0 1px rgba(56, 189, 248, 0.72) inset,
    0 10px 28px rgba(14, 165, 233, 0.12);
}

.login-button {
  width: 100%;
  min-height: 42px;
  border: 0;
  border-radius: 14px;
  background: linear-gradient(135deg, #38bdf8 0%, #2563eb 100%);
  box-shadow: 0 16px 34px rgba(37, 99, 235, 0.18);
  font-weight: 800;
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

