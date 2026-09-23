import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
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
