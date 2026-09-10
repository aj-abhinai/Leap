import { test, expect } from '@playwright/test'

// The setup project runs first and completes the forced first-login password
// change, so the bootstrap admin signs in with the ready password by now.
const readyPassword = process.env.E2E_ADMIN_NEW_PASSWORD ?? 'E2e-Password-1!'

// Login is the subject here, so this spec starts signed out even though the
// suite runs with a saved admin session by default.
test.use({ storageState: { cookies: [], origins: [] } })

test.describe('login', () => {
  test('rejects invalid credentials', async ({ page }) => {
    await page.goto('/login')
    await page.getByLabel('Email').fill('wrong@example.com')
    await page.getByLabel('Password').fill('wrong-password')
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page.getByText('Invalid email or password')).toBeVisible()
    await expect(page).toHaveURL(/\/login$/)
  })

  test('logs in with the bootstrap admin', async ({ page }) => {
    await page.goto('/login')
    await page.getByLabel('Email').fill('admin@admin.com')
    await page.getByLabel('Password').fill(readyPassword)
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page).not.toHaveURL(/login/)
    await expect(page.getByRole('heading', { name: 'Dashboard', level: 1 })).toBeVisible()
  })
})
