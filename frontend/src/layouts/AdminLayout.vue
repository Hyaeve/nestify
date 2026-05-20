<template>
  <el-container class="admin-layout">
    <el-aside width="220px" class="admin-layout__aside">
      <div class="brand">
        <img class="brand__logo" src="/nestify-logo.svg" alt="Nestify logo" />
        <div class="brand__meta">
          <div class="brand__name">Nestify</div>
          <div class="brand__version">V0.1</div>
        </div>
      </div>
      <el-menu router default-active="/dashboard" class="menu">
        <el-menu-item index="/dashboard">仪表盘</el-menu-item>
        <el-menu-item index="/rules">规则管理</el-menu-item>
        <el-menu-item index="/manual-pack">手动打包</el-menu-item>
        <el-menu-item index="/logs">任务日志</el-menu-item>
        <el-menu-item index="/settings">系统设置</el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="admin-layout__header">
        <div class="page-title">
          <img class="page-title__logo" src="/nestify-logo.svg" alt="Nestify logo" />
          <div>
            <div class="page-title__text">Nestify 管理台</div>
            <div class="page-title__version">Version V0.1</div>
          </div>
        </div>
        <div class="header-actions">
          <el-tag type="warning">V0.1</el-tag>
          <el-tag type="success">Skeleton</el-tag>
          <span class="username">{{ authStore.user?.username ?? '未登录' }}</span>
          <el-button link type="primary" @click="handleLogout">退出</el-button>
        </div>
      </el-header>

      <el-main class="admin-layout__main">
        <RouterView />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'

import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const router = useRouter()

async function handleLogout() {
  try {
    await authStore.logout()
    ElMessage.success('已退出登录')
    await router.push('/login')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '退出失败')
  }
}
</script>

<style scoped lang="scss">
.admin-layout {
  min-height: 100vh;

  &__aside {
    background: #ffffff;
    border-right: 1px solid #e9edf3;
  }

  &__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: #ffffff;
    border-bottom: 1px solid #e9edf3;
  }

  &__main {
    background: #f5f7fb;
  }
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 20px 16px;
  color: #1f2d3d;
  border-bottom: 1px solid #eef2f7;
}

.brand__logo {
  width: 40px;
  height: 40px;
  object-fit: contain;
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(245, 185, 66, 0.22);
}

.brand__meta {
  min-width: 0;
}

.brand__name {
  font-size: 20px;
  font-weight: 700;
  line-height: 1.1;
}

.brand__version {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
  letter-spacing: 0.08em;
}

.menu {
  border-right: none;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title__logo {
  width: 36px;
  height: 36px;
  object-fit: contain;
}

.page-title__text {
  font-size: 18px;
  font-weight: 600;
}

.page-title__version {
  margin-top: 2px;
  font-size: 12px;
  color: #909399;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.username {
  color: #606266;
  font-size: 14px;
}
</style>

