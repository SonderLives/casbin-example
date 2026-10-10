import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发期把 /api 代理给 Go 后端（localhost:8080），前端就不存在跨域问题。
// 生产部署时通常由 Nginx 做同样的反向代理。
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
