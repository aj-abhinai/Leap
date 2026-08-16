import { test, expect } from '@playwright/test'

const views = [
  { title: 'Dashboard', heading: 'Dashboard' },
  { title: 'Contacts', heading: 'Contacts' },
  { title: 'Leads', heading: 'Leads' },
  { title: 'Activities', heading: 'Activities' },
]

test('sidebar navigation renders every view', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('Email').fill('admin@admin.com')
  await page.getByLabel('Password').fill('admin')
  await page.getByRole('button', { name: 'Sign In' }).click()
  await expect(page).not.toHaveURL(/login/)

  const sidebar = page.locator('[data-sidebar="sidebar"]')

  for (const view of views) {
    await sidebar.getByRole('link', { name: view.title, exact: true }).click()
    await expect(page.getByRole('heading', { name: view.heading, level: 1 })).toBeVisible()
  }

  await sidebar.getByRole('link', { name: 'Settings', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible()

  await sidebar.getByRole('button', { name: /admin@admin.com/ }).click()
  await page.getByRole('menuitem', { name: 'Profile' }).click()
  await expect(page).toHaveURL(/profile$/)
  await expect(page.getByLabel('Email')).toHaveValue('admin@admin.com')

  await page.goto('/reminders')
  await expect(page.getByRole('heading', { name: 'Reminders', level: 1 })).toBeVisible()
})
