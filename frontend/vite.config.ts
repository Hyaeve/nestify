import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  build: {
    sourcemap: false,
    /* 小图一律内联成 data URI（默认阈值 4096 太小，会漏掉 4 KB 以上的图标）。
       侧栏导航图是当 CSS 遮罩用的（`background-color: currentColor` + `mask`）：
       一旦它作为独立文件走 HTTP，冷缓存（容器刚重建 / 重启后第一次打开）时图还没到，
       遮罩只能作用在已解码的那一段上，就会在左上角先画出一块「上圆下直、蓝里带白」的
       色块，等图下完或再刷新一次才恢复正常。内联后遮罩与组件同时就绪，这个时序窗口不存在。
       当前 app 里被 import 的图片只有 icon/ 下那 7 张（最大 5.9 KB），调高阈值不会把大图塞进包。 */
    assetsInlineLimit: 16 * 1024,
    rollupOptions: {
      output: {
        manualChunks: {
          vue: ['vue', 'vue-router', 'pinia'],
          element: ['element-plus', '@element-plus/icons-vue'],
        },
      },
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})

