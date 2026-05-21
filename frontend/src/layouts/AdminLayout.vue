<template>
  <el-container class="admin-layout">
    <el-aside :width="isCollapsed ? '88px' : '260px'" class="admin-layout__aside">
      <div class="brand">
        <img class="brand__logo" src="/nestify-logo.png" alt="Nestify logo" />
        <div v-if="!isCollapsed" class="brand__meta">
          <div class="brand__name">Nestify</div>
          <div class="brand__version">V0.1</div>
        </div>
      </div>
      <el-menu router :default-active="route.path" :collapse="isCollapsed" :collapse-transition="false" class="menu">
        <el-menu-item index="/dashboard">
          <el-icon><Odometer /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-menu-item index="/rules">
          <el-icon><Operation /></el-icon>
          <span>规则管理</span>
        </el-menu-item>
        <el-menu-item index="/manual-pack">
          <el-icon><FolderOpened /></el-icon>
          <span>文件管理</span>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-icon><Document /></el-icon>
          <span>任务日志</span>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <span>系统设置</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="admin-layout__header">
        <div class="header-left">
          <el-button circle class="icon-button" @click="toggleAside">
            <el-icon>
              <Fold v-if="!isCollapsed" />
              <Expand v-else />
            </el-icon>
          </el-button>

          <div class="page-title">
            <img class="page-title__logo" src="/nestify-logo.png" alt="Nestify logo" />
            <div>
              <div class="page-title__text">首页 / {{ currentSectionLabel }}</div>
              <div class="page-title__version">Nestify Console · Version V0.1</div>
            </div>
          </div>
        </div>
        <div class="header-actions">
          <el-tag type="warning">V0.1</el-tag>
          <el-tag type="success">Skeleton</el-tag>
          <el-button circle class="icon-button" @click="toggleTheme">
            <el-icon>
              <Sunny v-if="isDark" />
              <MoonNight v-else />
            </el-icon>
          </el-button>
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
import { computed, ref } from 'vue'
import { Document, Expand, Fold, FolderOpened, MoonNight, Odometer, Operation, Setting, Sunny } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'

import { useAuthStore } from '../stores/auth'
import { getStoredTheme, setTheme, type ThemeMode } from '../utils/theme'

const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()

const isCollapsed = ref(typeof window !== 'undefined' && window.localStorage.getItem('nestify-sidebar-collapsed') === '1')
const themeMode = ref<ThemeMode>(getStoredTheme())

const isDark = computed(() => themeMode.value === 'dark')
const currentSectionLabel = computed(() => {
  const pageTitleMap: Record<string, string> = {
    '/dashboard': '仪表盘',
    '/rules': '规则管理',
    '/manual-pack': '文件管理',
    '/logs': '任务日志',
    '/settings': '系统设置',
  }

  return pageTitleMap[route.path] ?? '管理台'
})

function toggleAside() {
  isCollapsed.value = !isCollapsed.value
  if (typeof window !== 'undefined') {
    window.localStorage.setItem('nestify-sidebar-collapsed', isCollapsed.value ? '1' : '0')
  }
}

function toggleTheme() {
  themeMode.value = themeMode.value === 'dark' ? 'light' : 'dark'
  setTheme(themeMode.value)
}

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
  background: transparent;

  &__aside {
    background: rgba(10, 15, 27, 0.88);
    border-right: 1px solid rgba(71, 85, 105, 0.24);
    backdrop-filter: blur(18px);
    transition: width 0.2s ease;
  }

  &__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    background: rgba(10, 15, 27, 0.72);
    border-bottom: 1px solid rgba(71, 85, 105, 0.24);
    backdrop-filter: blur(18px);
  }

  &__main {
    padding: 24px;
    background: transparent;
  }
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 92px;
  padding: 20px;
  color: #f8fafc;
  border-bottom: 1px solid rgba(71, 85, 105, 0.24);
}

.brand__logo {
  width: 48px;
  height: 48px;
  object-fit: cover;
  border-radius: 14px;
  box-shadow: 0 12px 30px rgba(245, 185, 66, 0.22);
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
  color: #94a3b8;
  letter-spacing: 0.08em;
}

.menu {
  padding: 12px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title__logo {
  width: 42px;
  height: 42px;
  object-fit: cover;
  border-radius: 12px;
}

.page-title__text {
  font-size: 18px;
  font-weight: 600;
}

.page-title__version {
  margin-top: 2px;
  font-size: 12px;
  color: var(--text-secondary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.icon-button {
  border-color: var(--border-color);
  background: var(--bg-elevated);
  color: var(--text-primary);
}

.username {
  color: var(--text-primary);
  font-size: 14px;
}
</style>

