import { defineStore } from 'pinia'

import { fetchCurrentSession, login, logout, type SessionUser } from '../api/auth'

interface AuthState {
  user: SessionUser | null
  initialized: boolean
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    user: null,
    initialized: false,
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.user),
  },
  actions: {
    async initializeSession() {
      if (this.initialized) {
        return
      }

      try {
        const response = await fetchCurrentSession()
        this.user = response.data ?? null
      } catch {
        this.user = null
      } finally {
        this.initialized = true
      }
    },
    async login(username: string, password: string) {
      const response = await login(username, password)
      this.user = response.data ?? null
      this.initialized = true
    },
    async logout() {
      await logout()
      this.user = null
      this.initialized = true
    },
    /**
     * 会话失效（容器重启 / 会话过期）时清空本地登录态。
     *
     * initialized 保持 true：已经确认未登录，不必再发一次会话检查；
     * 否则路由守卫会「查会话 → 401 → 又跳登录」地往复一次。
     */
    clearSession() {
      this.user = null
      this.initialized = true
    },
  },
})
