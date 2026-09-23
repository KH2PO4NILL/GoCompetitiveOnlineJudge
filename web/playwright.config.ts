import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests/browser',
  outputDir: process.env.GCOJ_E2E_OUTPUT_DIR ?? 'test-results',
  use: { baseURL: 'http://127.0.0.1:5173', screenshot: 'off', trace: 'off', video: 'off' },
  webServer: { command: 'npm run dev', url: 'http://127.0.0.1:5173', reuseExistingServer: !process.env.CI && process.env.GCOJ_AUTH_E2E !== '1' },
})
