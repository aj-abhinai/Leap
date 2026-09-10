import { defineConfig, devices } from '@playwright/test'
import { fileURLToPath } from 'node:url'

// Saved once by the setup project (e2e/auth.setup.ts) and reused by every
// spec, so tests start from one deterministic, already-onboarded principal.
const authFile = fileURLToPath(new URL('./e2e/.auth/admin.json', import.meta.url))

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'html' : 'list',
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'setup',
      testMatch: /auth\.setup\.ts/,
    },
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'], storageState: authFile },
      dependencies: ['setup'],
    },
  ],
  webServer: {
    command: 'pnpm dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
  },
})
