import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

import AdminLayout from '../layouts/AdminLayout.vue'
import DashboardView from '../views/DashboardView.vue'
import LoginView from '../views/LoginView.vue'
import LogsView from '../views/LogsView.vue'
import ManualPackView from '../views/ManualPackView.vue'
import RulesView from '../views/RulesView.vue'
import SettingsView from '../views/SettingsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: LoginView,
    },
    {
      path: '/',
      component: AdminLayout,
      redirect: '/dashboard',
      children: [
        {
          path: 'dashboard',
          name: 'dashboard',
          component: DashboardView,
        },
        {
          path: 'rules',
          name: 'rules',
          component: RulesView,
        },
        {
          path: 'manual-pack',
          name: 'manual-pack',
          component: ManualPackView,
        },
        {
          path: 'logs',
          name: 'logs',
          component: LogsView,
        },
        {
          path: 'settings',
          name: 'settings',
          component: SettingsView,
        },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const authStore = useAuthStore()
  await authStore.initializeSession()

  if (to.path === '/login') {
    if (authStore.isAuthenticated) {
      return '/dashboard'
    }

    return true
  }

  if (!authStore.isAuthenticated) {
    return '/login'
  }

  return true
})

export default router

