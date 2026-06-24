<template>
  <el-container class="admin-layout">
    <el-aside :width="isCollapsed ? '68px' : '176px'" :class="['admin-layout__aside', { 'is-collapsed': isCollapsed }]">
      <div class="brand">
        <img class="brand__logo" src="/nestify-logo.png" alt="Nestify logo" />
        <div class="brand__meta">
          <div class="brand__name">Nestify</div>
          <div class="brand__version">v6.8</div>
        </div>
      </div>
      <div class="aside-scroll">
        <el-menu router :default-active="route.path" :collapse="isCollapsed" :collapse-transition="false" class="menu">
          <el-menu-item index="/dashboard">
            <el-icon class="nav-icon nav-icon--dashboard">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path class="nav-icon__arc" fill-rule="evenodd" d="M3.35 16.15a8.65 8.65 0 0 1 17.3 0 1.42 1.42 0 0 1-2.84 0 5.81 5.81 0 0 0-11.62 0 1.42 1.42 0 0 1-2.84 0Zm2.92-5.3 1.9 1.12a6.75 6.75 0 0 0-.78 1.9H5.16a8.8 8.8 0 0 1 1.11-3.02Zm4.58-2.37v2.22a6.64 6.64 0 0 0-1.7.62L8.03 9.38a8.62 8.62 0 0 1 2.82-.9Zm2.3 0a8.62 8.62 0 0 1 2.82.9l-1.12 1.94a6.64 6.64 0 0 0-1.7-.62V8.48Zm4.58 2.37a8.8 8.8 0 0 1 1.11 3.02h-2.23a6.75 6.75 0 0 0-.78-1.9l1.9-1.12Z" />
                <path class="nav-icon__needle" d="M12.45 14.35 16.2 9.2a.78.78 0 0 1 1.22.95l-3.85 5.08a1.6 1.6 0 1 1-1.12-.88Z" />
              </svg>
            </el-icon>
            <span>仪表盘</span>
          </el-menu-item>
          <el-menu-item index="/rules">
            <el-icon><Operation /></el-icon>
            <span>规则管理</span>
          </el-menu-item>
          <el-menu-item index="/manual-pack">
            <el-icon class="nav-icon nav-icon--folder">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M4.05 7.35a2.1 2.1 0 0 1 2.1-2.1h4.35l1.75 2.1h5.95a2.1 2.1 0 0 1 2.1 2.1v1.05" />
                <path d="M4.45 10.25h15.45a1.8 1.8 0 0 1 1.76 2.16l-.98 4.9a2.45 2.45 0 0 1-2.4 1.97H5.95a2.45 2.45 0 0 1-2.41-2.01l-.85-4.62a2 2 0 0 1 1.76-2.4Z" />
              </svg>
            </el-icon>
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
      </div>

      <button class="aside-toggle aside-toggle--edge" type="button" @click="toggleAside" :aria-label="isCollapsed ? '展开侧栏' : '收起侧栏'">
        <span class="aside-toggle__line"></span>
        <span class="aside-toggle__chevron" :class="{ 'is-collapsed': isCollapsed }">
          <el-icon>
            <ArrowLeftBold v-if="!isCollapsed" />
            <ArrowRightBold v-else />
          </el-icon>
        </span>
      </button>
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
          <el-button circle class="icon-button" @click="toggleTheme">
            <el-icon>
              <Apple v-if="isAppleTVTheme" />
              <Sunny v-else />
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
  Apple,
  ArrowLeftBold,
  ArrowRightBold,
  Document,
  MoreFilled,
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

const themeCycle: ThemeMode[] = ['light', 'appletv']
const isAppleTVTheme = computed(() => themeMode.value === 'appletv')
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
  const currentIndex = themeCycle.indexOf(themeMode.value)
  themeMode.value = themeCycle[(currentIndex + 1) % themeCycle.length]
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
    position: sticky;
    top: 0;
    display: flex;
    flex-direction: column;
    height: 100vh;
    overflow: hidden;
    background: var(--bg-sidebar);
    border-right: 1px solid var(--border-color);
    box-shadow: 10px 0 30px rgba(2, 6, 23, 0.12);
    backdrop-filter: blur(16px);
    min-width: 68px;
    will-change: width;
    transition:
      width 0.28s cubic-bezier(0.22, 1, 0.36, 1),
      box-shadow 0.28s ease,
      background-color 0.28s ease;
  }

  &__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    height: 52px;
    padding: 0 var(--app-shell-padding-x);
    background: var(--bg-header);
    border-bottom: 1px solid var(--border-color);
    backdrop-filter: blur(18px);
  }

  &__main {
    padding: var(--app-shell-padding-y) var(--app-shell-padding-x);
    background: transparent;
  }
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 72px;
  padding: 14px 10px 12px;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-color);
  background: var(--bg-sidebar);
}

.brand__logo {
  width: 38px;
  height: 38px;
  object-fit: cover;
  border-radius: 12px;
  box-shadow: 0 12px 24px rgba(59, 130, 246, 0.14);
  transition: transform 0.26s cubic-bezier(0.22, 1, 0.36, 1);
}

.brand__meta {
  min-width: 0;
  max-width: 86px;
  overflow: hidden;
  opacity: 1;
  white-space: nowrap;
  transform: translateX(0);
  transition:
    opacity 0.18s ease,
    max-width 0.26s cubic-bezier(0.22, 1, 0.36, 1),
    transform 0.26s cubic-bezier(0.22, 1, 0.36, 1);
}

.brand__name {
  font-size: 16px;
  font-weight: 800;
  line-height: 1.1;
}

.brand__version {
  margin-top: 4px;
  font-size: 11px;
  color: var(--text-secondary);
  letter-spacing: 0.08em;
}

.aside-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 12px 10px 16px;
  scrollbar-gutter: stable;
  transition: padding 0.28s cubic-bezier(0.22, 1, 0.36, 1);
}

.aside-scroll::-webkit-scrollbar {
  width: 6px;
}

.aside-scroll::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.26);
}

.menu {
  padding: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex-wrap: wrap;
}

.page-title__crumb,
.page-title__text {
  font-size: 14px;
  font-weight: 600;
}

.page-title__text {
  min-width: 0;
}

.page-title__crumb,
.page-title__divider {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.admin-layout > .el-container {
  min-width: 0;
}

.admin-layout__main > * {
  width: 100%;
  max-width: var(--app-content-max-width);
  margin: 0 auto;
}

@media (max-width: 1599px) {
  .admin-layout {
    &__aside {
      box-shadow: 8px 0 24px rgba(2, 6, 23, 0.1);
    }

    &__header {
      gap: 12px;
      height: 50px;
    }
  }

  .brand {
    min-height: 68px;
    padding: 12px 10px 10px;
  }

  .brand__logo {
    width: 34px;
    height: 34px;
  }

  :deep(.el-menu-item) {
    height: 42px;
    border-radius: 12px;
  }
}

@media (min-width: 1920px) {
  .admin-layout {
    &__header {
      height: 56px;
    }
  }

  .brand {
    min-height: 76px;
  }

  .page-title__crumb,
  .page-title__text,
  .page-title__divider {
    font-size: 15px;
  }

  :deep(.el-menu-item) {
    height: 46px;
  }
}

.icon-button {
  border-color: var(--border-color);
  background: var(--bg-elevated);
  color: var(--text-primary);
}

.aside-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  min-height: 132px;
  padding: 0;
  color: var(--text-secondary);
  background: transparent;
  border: 0;
  border-radius: 999px;
  cursor: pointer;
  transition:
    color 0.2s ease,
    background-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
}

.aside-toggle--edge {
  position: absolute;
  top: 50%;
  right: 0;
  z-index: 6;
  transform: translateY(-50%);
  width: 16px;
  background: transparent;
}

.aside-toggle:hover {
  color: var(--accent-color);
  box-shadow: none;
}

.aside-toggle:focus-visible {
  outline: none;
}

.aside-toggle__line {
  position: absolute;
  left: 7px;
  width: 2px;
  height: 84px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.26);
  transition:
    width 0.2s ease,
    height 0.2s ease,
    background-color 0.2s ease,
    transform 0.2s ease;
}

.aside-toggle__chevron {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 18px;
  height: 84px;
  color: rgba(148, 163, 184, 0.78);
  opacity: 0;
  transform: translate(-50%, -50%) scale(0.86);
  --aside-toggle-chevron-transform: scaleY(4.25);
  transition:
    color 0.2s ease,
    opacity 0.18s ease,
    transform 0.18s ease;
}

.aside-toggle:hover .aside-toggle__line,
.aside-toggle:focus-visible .aside-toggle__line {
  background: var(--accent-color);
  opacity: 0;
}

.aside-toggle:hover .aside-toggle__chevron,
.aside-toggle:focus-visible .aside-toggle__chevron {
  color: var(--accent-color);
  opacity: 1;
  transform: translate(-50%, -50%) scale(1);
}

.aside-toggle :deep(.el-icon) {
  display: block;
  width: 18px;
  height: 84px;
  font-size: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.aside-toggle :deep(.el-icon svg) {
  width: 18px;
  height: 18px;
  transform: var(--aside-toggle-chevron-transform);
  transform-origin: center;
}

.admin-layout__aside.is-collapsed .brand {
  justify-content: center;
  padding-left: 0;
  padding-right: 0;
}

.admin-layout__aside.is-collapsed .brand__logo {
  transform: translateX(4px);
}

.admin-layout__aside.is-collapsed .brand__meta {
  max-width: 0;
  opacity: 0;
  transform: translateX(-8px);
  pointer-events: none;
}

:deep(.el-menu-item) {
  position: relative;
  height: 44px;
  margin-bottom: 6px;
  padding: 0 14px !important;
  border-radius: 14px;
  color: #222222;
  font-weight: 800;
  transition:
    width 0.28s cubic-bezier(0.22, 1, 0.36, 1),
    padding 0.28s cubic-bezier(0.22, 1, 0.36, 1),
    color 0.18s ease,
    background-color 0.18s ease,
    box-shadow 0.18s ease;
}

:deep(.el-menu-item:hover) {
  color: #1688e8;
  background: #d7ebfb;
}

.admin-layout__aside.is-collapsed .aside-scroll {
  padding: 12px 0 16px;
}

:deep(.el-menu--collapse) {
  width: 68px;
}

:deep(.el-menu--collapse .el-menu-item) {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 68px;
  min-width: 68px;
  height: 54px;
  padding: 0 !important;
  margin: 0 0 10px;
  border-radius: 0;
  background: transparent;
}

:deep(.el-menu--collapse .el-menu-item::before) {
  content: '';
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: -1;
  width: 54px;
  height: 54px;
  border-radius: 18px;
  background: transparent;
  transform: translate(-50%, -50%);
  transition:
    background-color 0.18s ease,
    box-shadow 0.18s ease;
}

:deep(.el-menu--collapse .el-menu-item:hover) {
  background: transparent;
}

:deep(.el-menu--collapse .el-menu-item:hover::before) {
  background: #d7ebfb;
}

:deep(.el-menu-item .el-icon) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 26px;
  width: 26px;
  height: 26px;
  margin-right: 10px;
  font-size: 22px;
}

:deep(.el-menu--collapse .el-menu-item .el-icon) {
  position: absolute;
  left: 50%;
  top: 50%;
  margin: 0;
  transform: translate(-50%, -50%);
}

.nav-icon svg {
  width: 26px;
  height: 26px;
  fill: none;
  stroke: currentColor;
  stroke-width: 2.15;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.nav-icon--dashboard svg {
  width: 30px;
  height: 30px;
  fill: currentColor;
  stroke: none;
}

.nav-icon--dashboard .nav-icon__arc,
.nav-icon--dashboard .nav-icon__needle {
  transform: translateY(0.2px) scale(1.08);
  transform-origin: 12px 13px;
}

.nav-icon--folder svg {
  width: 26px;
  height: 26px;
  stroke-width: 1.65;
  opacity: 0.72;
}

:deep(.el-menu-item.is-active) {
  color: #1e9bff;
  background: #e8f4ff;
  box-shadow: inset 4px 0 0 #1e9bff;
}

:deep(.el-menu--collapse .el-menu-item.is-active) {
  background: transparent;
  box-shadow: none;
}

:deep(.el-menu--collapse .el-menu-item.is-active::before) {
  background: #e8f4ff;
}

:deep(.el-menu-item.is-active:hover) {
  color: #1688e8;
  background: #d7ebfb;
  box-shadow: inset 4px 0 0 #1688e8;
}

:deep(.el-menu--collapse .el-menu-item.is-active:hover) {
  background: transparent;
  box-shadow: none;
}

:deep(.el-menu--collapse .el-menu-item.is-active:hover::before) {
  background: #d7ebfb;
}

:deep(.el-menu-item span) {
  color: inherit;
  max-width: 96px;
  overflow: hidden;
  white-space: nowrap;
  opacity: 1;
  transform: translateX(0);
  transition:
    opacity 0.16s ease,
    transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}

:deep(.el-menu--collapse .el-menu-item span) {
  max-width: 0;
  opacity: 0;
  transform: translateX(-8px);
  pointer-events: none;
}

:global(.header-user-dropdown .el-dropdown-menu) {
  min-width: 128px;
  padding: 6px;
  border-radius: 14px;
}

:global(.header-user-dropdown .el-dropdown-menu__item) {
  min-height: 38px;
  padding: 0 12px;
  border-radius: 10px;
}

:global(.header-user-dropdown .el-dropdown-menu__item .el-icon) {
  margin-right: 8px;
}
</style>

