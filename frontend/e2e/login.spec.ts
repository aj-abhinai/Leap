import { test, expect } from '@playwright/test'

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
    await page.getByLabel('Password').fill('admin')
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page).not.toHaveURL(/login/)

    if (page.url().endsWith('/change-password')) {
      await expect(page.getByText('Change Password', { exact: true })).toBeVisible()
    } else {
      await expect(page.getByRole('heading', { name: 'Dashboard', level: 1 })).toBeVisible()
    }
  })
})
