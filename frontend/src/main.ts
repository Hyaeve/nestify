import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'

import App from './App.vue'
import router from './router'
import { setSessionExpiredHandler } from './api/http'
import { useAuthStore } from './stores/auth'
import { applyTheme, getStoredTheme } from './utils/theme'
import './styles/index.scss'

applyTheme(getStoredTheme())

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// 会话失效（容器重启 / 会话过期）时不弹「未登录」，直接回登录窗口。
// 已经在登录页时返回 false，交给调用方按普通错误处理（登录密码错误仍要提示）。
setSessionExpiredHandler(() => {
  useAuthStore(pinia).clearSession()

  if (router.currentRoute.value.path === '/login') {
    return false
  }

  void router.replace('/login').catch(() => undefined)
  return true
})

app.mount('#app')
