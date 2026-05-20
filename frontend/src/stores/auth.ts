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
  },
})
