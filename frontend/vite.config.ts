import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { resolve } from 'path'

// CSP is enforced by the Go backend's SecurityHeaders middleware
// (internal/middleware/security.go), which must list this hash. This plugin
// recomputes the hash from the actual inline theme script at build time and
// fails the build if it ever drifts from the constant the backend allows —
// a silent mismatch would block dark-mode theming in production.
const themeScriptHash = 'sha256-v4bJLYBuBypQmhQj/loKmpY39P8zNvzk0lA3wiG8QbE='

function cspHashGuard(): Plugin {
  let htmlPath = ''
  return {
    name: 'csp-hash-guard',
    apply: 'build',
    configResolved(config) {
      htmlPath = resolve(config.root, 'index.html')
    },
    buildStart() {
      const html = readFileSync(htmlPath, 'utf8')
      const scripts = [...html.matchAll(/<script>(.*?)<\/script>/gs)]
      if (scripts.length !== 1) {
        this.error(
          `csp-hash-guard: expected exactly one inline <script> in index.html, found ${scripts.length}; ` +
            'any additional inline script would be blocked by the CSP header in production.',
        )
      }
      const hash = 'sha256-' + createHash('sha256').update(scripts[0][1]).digest('base64')
      if (hash !== themeScriptHash) {
        this.error(
          `csp-hash-guard: inline theme script hash changed (${hash}); update ` +
            'internal/middleware/security.go and this constant in sync.',
        )
      }
    },
  }
}

export default defineConfig({
  plugins: [vue(), tailwindcss(), cspHashGuard()],
  resolve: {
    alias: {
      '@': resolve(import.meta.dirname, 'src')
    }
  },
  server: {
    proxy: {
      '/api': 'http://localhost:9000'
    }
  }
})
