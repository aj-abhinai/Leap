import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  test: {
    environment: 'jsdom',
    globals: true,
    // E2E specs are Playwright suites (pnpm test:e2e); keep Vitest scoped to
    // unit/component tests so `pnpm test:run` stays fast and green.
    exclude: ['**/e2e/**', '**/node_modules/**'],
  },
})
