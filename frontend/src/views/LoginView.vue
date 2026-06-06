<template>
  <div class="login-page">
    <div class="login-page__noise"></div>
    <div class="login-page__spot login-page__spot--cyan"></div>
    <div class="login-page__spot login-page__spot--pink"></div>
    <div class="login-page__spot login-page__spot--violet"></div>
    <div class="login-hero" aria-hidden="true">
      <div class="login-hero__badge">AI 媒体归档应用基座</div>
      <div class="login-hero__title">承载 AI 应用，管理数字资产，连接未来。</div>
      <div class="login-hero__actions">
        <span class="login-hero__button login-hero__button--primary">快速开始</span>
        <span class="login-hero__button">GitHub</span>
        <span class="login-hero__button">AtomGit</span>
      </div>
    </div>
    <div class="login-page__logo-mark" aria-hidden="true">
      <div class="login-page__logo-ring login-page__logo-ring--left"></div>
      <div class="login-page__logo-ring login-page__logo-ring--right"></div>
      <div class="login-page__logo-star"></div>
    </div>

    <el-card class="login-card">
      <div class="login-brand">
        <img class="login-brand__logo" src="/nestify-logo.png" alt="Nestify logo" />
        <div>
          <div class="login-brand__title">Nestify 登录</div>
          <div class="login-brand__subtitle">Version v3.8 · 媒体归档管理台</div>
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
  justify-content: flex-end;
  padding: 24px 8vw;
  overflow: hidden;
  isolation: isolate;
  background:
    radial-gradient(circle at 16% 22%, rgba(39, 201, 255, 0.12), transparent 20%),
    radial-gradient(circle at 84% 18%, rgba(236, 72, 153, 0.18), transparent 24%),
    radial-gradient(circle at 32% 86%, rgba(192, 38, 211, 0.18), transparent 18%),
    linear-gradient(120deg, #0b1721 0%, #131f33 42%, #1b1730 70%, #22122a 100%);
}

.login-page::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  background:
    linear-gradient(90deg, rgba(5, 10, 20, 0.48) 0%, rgba(5, 10, 20, 0.16) 35%, rgba(5, 10, 20, 0.08) 100%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.02), rgba(255, 255, 255, 0));
}

.login-page::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.04), transparent 26%),
    linear-gradient(0deg, rgba(236, 72, 153, 0.08), transparent 28%);
}

.login-page__noise {
  position: absolute;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  opacity: 0.2;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='180' height='180' viewBox='0 0 180 180'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='180' height='180' filter='url(%23n)' opacity='0.9'/%3E%3C/svg%3E");
  background-size: 180px 180px;
  mix-blend-mode: screen;
}

.login-page__spot {
  position: absolute;
  border-radius: 50%;
  z-index: 0;
}

.login-page__spot--cyan {
  top: -12%;
  right: 22%;
  width: 620px;
  height: 620px;
  background: radial-gradient(circle, rgba(28, 215, 224, 0.38) 0%, rgba(28, 215, 224, 0.08) 42%, transparent 72%);
}

.login-page__spot--pink {
  right: -6%;
  top: -4%;
  width: 760px;
  height: 760px;
  background: radial-gradient(circle, rgba(244, 114, 182, 0.42) 0%, rgba(244, 114, 182, 0.12) 38%, transparent 70%);
}

.login-page__spot--violet {
  left: 8%;
  bottom: -16%;
  width: 860px;
  height: 420px;
  background: radial-gradient(circle, rgba(168, 85, 247, 0.3) 0%, rgba(168, 85, 247, 0.12) 40%, transparent 74%);
}

.login-hero {
  position: absolute;
  left: 72px;
  top: 18%;
  z-index: 1;
  max-width: 760px;
  color: #f8eaf1;
}

.login-hero__badge {
  display: inline-flex;
  align-items: center;
  padding: 10px 18px;
  border-radius: 999px;
  border: 1px solid rgba(236, 72, 153, 0.28);
  background: rgba(17, 24, 39, 0.2);
  color: #f472b6;
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.login-hero__title {
  margin-top: 54px;
  font-size: clamp(48px, 5vw, 76px);
  line-height: 1.08;
  font-weight: 800;
  letter-spacing: -0.04em;
  text-wrap: balance;
  text-shadow: 0 6px 30px rgba(15, 23, 42, 0.18);
}

.login-hero__actions {
  display: flex;
  gap: 20px;
  margin-top: 54px;
  flex-wrap: wrap;
}

.login-hero__button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 172px;
  height: 60px;
  padding: 0 28px;
  border-radius: 999px;
  background: rgba(18, 18, 24, 0.88);
  color: #f8fafc;
  font-size: 18px;
  font-weight: 700;
  box-shadow: 0 14px 28px rgba(2, 6, 23, 0.18);
}

.login-hero__button--primary {
  background: linear-gradient(135deg, #ff6ab7 0%, #ec5da9 100%);
  color: #25111d;
}

.login-page__logo-mark {
  position: absolute;
  right: 11%;
  top: 13%;
  width: 520px;
  height: 520px;
  z-index: 1;
}

.login-page__logo-ring {
  position: absolute;
  inset: 0;
  border-radius: 50%;
}

.login-page__logo-ring--left {
  clip-path: ellipse(33% 50% at 36% 50%);
  background: linear-gradient(180deg, #1ce0dd 0%, #2ec6db 36%, #7c71ff 72%, #d538ff 100%);
  transform: rotate(18deg);
}

.login-page__logo-ring--right {
  clip-path: ellipse(33% 50% at 64% 50%);
  background: linear-gradient(180deg, #ff66bb 0%, #f255bb 44%, #d84dd8 100%);
  transform: rotate(18deg);
}

.login-page__logo-star {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 120px;
  height: 120px;
  background: linear-gradient(180deg, #3fe3ff 0%, #8f72ff 100%);
  clip-path: polygon(50% 0%, 63% 36%, 100% 50%, 63% 64%, 50% 100%, 37% 64%, 0% 50%, 37% 36%);
  transform: translate(-50%, -50%) rotate(8deg);
  filter: drop-shadow(0 0 18px rgba(120, 119, 255, 0.35));
}

.login-card {
  position: relative;
  z-index: 1;
  width: 420px;
  border-radius: 20px;
  margin-right: 8%;
  background: rgba(15, 23, 42, 0.9);
  border: 1px solid rgba(148, 163, 184, 0.14);
  box-shadow: 0 28px 60px rgba(2, 6, 23, 0.34);
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

@media (max-width: 1380px) {
  .login-page {
    justify-content: center;
    padding: 32px;
  }

  .login-hero,
  .login-page__logo-mark {
    display: none;
  }

  .login-card {
    margin-right: 0;
  }
}

@media (max-width: 768px) {
  .login-page {
    padding: 16px;
  }

  .login-card {
    width: min(100%, 420px);
  }
}
</style>

