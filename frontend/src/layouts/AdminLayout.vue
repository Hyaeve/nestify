<template>
  <el-container class="admin-layout">
    <el-aside :width="isCollapsed ? '76px' : '220px'" class="admin-layout__aside">
      <div class="brand">
        <img class="brand__logo" src="/nestify-logo.png" alt="Nestify logo" />
        <div v-if="!isCollapsed" class="brand__meta">
          <div class="brand__name">Nestify</div>
          <div class="brand__version">V0.2</div>
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

      <el-button circle class="aside-toggle" @click="toggleAside">
        <el-icon>
          <ArrowLeftBold v-if="!isCollapsed" />
          <ArrowRightBold v-else />
        </el-icon>
      </el-button>
    </el-aside>

    <el-container>
      <el-header class="admin-layout__header">
        <div class="header-left">
          <div class="page-title">
            <span class="page-title__crumb">首页</span>
            <span class="page-title__divider">/</span>
            <span class="page-title__text">{{ currentSectionLabel }}</span>
          </div>
        </div>
        <div class="header-actions">
          <el-tag effect="plain" class="pro-tag">高级版</el-tag>
          <el-button circle class="icon-button" @click="toggleTheme">
            <el-icon>
              <Sunny v-if="!isDark" />
              <MoonNight v-else />
            </el-icon>
          </el-button>
          <el-dropdown trigger="click" placement="bottom-end" popper-class="header-user-dropdown" @command="handleHeaderCommand">
            <el-button circle class="icon-button">
              <el-icon><MoreFilled /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="restart">
                  <el-icon><RefreshRight /></el-icon>
                  重启系统
                </el-dropdown-item>
                <el-dropdown-item command="logout" divided>
                  <el-icon><SwitchButton /></el-icon>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
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
import {
  ArrowLeftBold,
  ArrowRightBold,
  Document,
  FolderOpened,
  MoonNight,
  MoreFilled,
  Odometer,
  Operation,
  RefreshRight,
  Setting,
  Sunny,
  SwitchButton,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'

import { restartSystem } from '../api/system'
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

async function handleHeaderCommand(command: string) {
  if (command === 'restart') {
    await handleRestartSystem()
    return
  }

  if (command === 'logout') {
    await handleLogout()
  }
}

async function handleRestartSystem() {
  try {
    await ElMessageBox.confirm('确认重启系统？系统服务会短暂中断。', '重启系统', {
      type: 'warning',
      confirmButtonText: '确认重启',
      cancelButtonText: '取消',
    })

    await restartSystem()
    ElMessage.success('已发送重启指令')
  } catch (error) {
    if (error === 'cancel') {
      return
    }

    ElMessage.error(error instanceof Error ? error.message : '重启系统失败')
  }
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
    position: relative;
    overflow: visible;
    background: var(--bg-sidebar);
    border-right: 1px solid var(--border-color);
    transition: width 0.2s ease;
  }

  &__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    height: 62px;
    padding: 0 28px;
    background: rgba(255, 255, 255, 0.94);
    border-bottom: 1px solid var(--border-color);
    backdrop-filter: blur(18px);
  }

  &__main {
    padding: 24px 28px;
    background: transparent;
  }
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 72px;
  padding: 14px 16px;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-color);
}

.brand__logo {
  width: 42px;
  height: 42px;
  object-fit: cover;
  border-radius: 12px;
  box-shadow: 0 12px 24px rgba(59, 130, 246, 0.14);
}

.brand__meta {
  min-width: 0;
}

.brand__name {
  font-size: 18px;
  font-weight: 700;
  line-height: 1.1;
}

.brand__version {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-secondary);
  letter-spacing: 0.08em;
}

.menu {
  padding: 12px 10px 18px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-title__crumb,
.page-title__text {
  font-size: 15px;
  font-weight: 600;
}

.page-title__crumb,
.page-title__divider {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-secondary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.icon-button {
  border-color: var(--border-color);
  background: var(--bg-elevated);
  color: var(--text-primary);
}

.pro-tag {
  border-radius: 999px;
  color: #34a853;
  border-color: rgba(52, 168, 83, 0.22);
  background: rgba(52, 168, 83, 0.08);
}

.aside-toggle {
  position: absolute;
  top: 50%;
  right: -16px;
  z-index: 5;
  width: 32px;
  height: 32px;
  transform: translateY(-50%);
  border-color: var(--border-color);
  background: var(--bg-panel-strong);
  color: var(--text-secondary);
  box-shadow: var(--shadow-soft);
}

:deep(.el-menu-item) {
  height: 48px;
  margin-bottom: 6px;
  border-radius: 16px;
}

:deep(.el-menu--collapse .el-menu-item) {
  padding: 0;
  justify-content: center;
}

:deep(.el-menu-item .el-icon) {
  font-size: 18px;
}

:deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.14), rgba(64, 158, 255, 0.22));
  color: var(--accent-color);
  box-shadow: inset 0 0 0 1px rgba(64, 158, 255, 0.08);
}

:global(.header-user-dropdown .el-dropdown-menu) {
  min-width: 160px;
  padding: 8px;
  border-radius: 16px;
}

:global(.header-user-dropdown .el-dropdown-menu__item) {
  min-height: 44px;
  border-radius: 12px;
}

:global(.header-user-dropdown .el-dropdown-menu__item .el-icon) {
  margin-right: 10px;
}
</style>

