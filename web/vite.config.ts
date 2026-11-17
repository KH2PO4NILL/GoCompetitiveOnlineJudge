import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      '/api/v1/admin': { target: process.env.GCOJ_ADMIN_UPSTREAM ?? 'http://127.0.0.1:8001' },
      '/api/v1': { target: process.env.GCOJ_APP_UPSTREAM ?? 'http://127.0.0.1:8002' },
    },
  },
  build: { sourcemap: false },
})
