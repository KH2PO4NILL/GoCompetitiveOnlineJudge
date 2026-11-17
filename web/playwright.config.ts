import { defineConfig } from '@playwright/test'

const port = process.env.GCOJ_E2E_PORT ?? '5173'
if (!/^[1-9]\d{0,4}$/.test(port) || Number(port) > 65535) throw new Error('GCOJ_E2E_PORT must be between 1 and 65535')
const baseURL = `http://127.0.0.1:${port}`

export default defineConfig({
  testDir: './tests/browser',
  workers: 2,
  outputDir: process.env.GCOJ_E2E_OUTPUT_DIR ?? 'test-results',
  use: { baseURL, screenshot: 'off', trace: 'off', video: 'off' },
  webServer: { command: `npm run dev -- --port ${port}`, url: baseURL, reuseExistingServer: !process.env.CI && process.env.GCOJ_AUTH_E2E !== '1' },
})
