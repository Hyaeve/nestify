import { defineStore } from 'pinia'

import { fetchSettings } from '../api/system'

export interface StartupPageOption {
  label: string
  value: string
  path: string
}

// 启动页面可选项：value 与后端 default_page 对应，path 为登录后跳转的路由。
export const startupPageOptions: StartupPageOption[] = [
  { label: '仪表盘', value: 'dashboard', path: '/dashboard' },
  { label: '规则管理', value: 'rules', path: '/rules' },
  { label: '文件管理', value: 'manual-pack', path: '/manual-pack' },
  { label: '命名工坊', value: 'naming-workshop', path: '/naming-workshop' },
  { label: '运行日志', value: 'logs', path: '/logs' },
  { label: '系统设置', value: 'settings', path: '/settings' },
]

export const pageSizeOptions = [50, 100, 150, 200]
export const defaultPageSize = 50

export function resolveStartupPath(value?: string): string {
  const matched = startupPageOptions.find((item) => item.value === value)
  return matched ? matched.path : '/dashboard'
}

interface SettingsState {
  defaultPage: string
  pageSize: number
  loaded: boolean
  loading: boolean
}

export const useSettingsStore = defineStore('settings', {
  state: (): SettingsState => ({
    defaultPage: 'dashboard',
    pageSize: defaultPageSize,
    loaded: false,
    loading: false,
  }),
  actions: {
    async ensureLoaded(force = false) {
      if (this.loading) {
        return
      }
      if (this.loaded && !force) {
        return
      }

      this.loading = true
      try {
        const response = await fetchSettings()
        if (response.data) {
          this.defaultPage = response.data.default_page || 'dashboard'
          this.pageSize = response.data.page_size || defaultPageSize
        }
        this.loaded = true
      } catch {
        // 读取失败时保持默认值，不阻断页面。
      } finally {
        this.loading = false
      }
    },
    applyLocal(defaultPage: string, pageSize: number) {
      this.defaultPage = defaultPage || 'dashboard'
      this.pageSize = pageSize || defaultPageSize
      this.loaded = true
    },
  },
})
