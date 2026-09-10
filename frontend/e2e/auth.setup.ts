import { test as setup, expect } from '@playwright/test'
import { mkdirSync } from 'node:fs'
import { dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const authFile = fileURLToPath(new URL('.auth/admin.json', import.meta.url))
const initialPassword = process.env.E2E_ADMIN_PASSWORD ?? 'admin'
const readyPassword = process.env.E2E_ADMIN_NEW_PASSWORD ?? 'E2e-Password-1!'

// signIn attempts a login and reports whether it stuck; the forced
// password-change redirect also counts as signed in.
async function signIn(page: import('@playwright/test').Page, password: string): Promise<boolean> {
  await page.goto('/login')
  await page.getByLabel('Email').fill('admin@admin.com')
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign In' }).click()
  try {
    await expect(page).not.toHaveURL(/login/, { timeout: 5000 })
    return true
  } catch {
    return false
  }
}

// One prepared principal for the whole suite: the bootstrap admin with the
// forced first-login password change already complete. Specs reuse the saved
// session instead of each logging in (and racing the forced-change redirect).
// A fresh database forces the change once; later runs sign in with the ready
// password directly.
setup('authenticate as the bootstrap admin', async ({ page }) => {
  let password = initialPassword
  if (!(await signIn(page, initialPassword))) {
    if (!(await signIn(page, readyPassword))) {
      throw new Error('bootstrap admin login failed with both the initial and ready passwords')
    }
    password = readyPassword
  }

  if (page.url().includes('/change-password')) {
    await page.getByLabel('Current password').fill(password)
    await page.getByLabel('New password', { exact: true }).fill(readyPassword)
    await page.getByLabel('Confirm new password').fill(readyPassword)
    await page.getByRole('button', { name: 'Change Password' }).click()
    await expect(page).not.toHaveURL(/change-password/)
  }

  mkdirSync(dirname(authFile), { recursive: true })
  await page.context().storageState({ path: authFile })
})
