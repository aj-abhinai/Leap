import { test, expect, type Page } from '@playwright/test'

async function login(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Email').fill('admin@admin.com')
  await page.getByLabel('Password').fill('admin')
  await page.getByRole('button', { name: 'Sign In' }).click()
  await expect(page).not.toHaveURL(/login/)
}

function uniqueName() {
  return `E2E Contact ${Date.now()} ${Math.random().toString(36).slice(2, 8)}`
}

test('contact converts into a lead from the contacts table', async ({ page }) => {
  await login(page)
  const name = uniqueName()

  await page.goto('/contacts')
  await page.getByRole('button', { name: 'Add Contact' }).click()
  await page.getByLabel('Name *').fill(name)
  await page.getByPlaceholder('Phone 1').fill('9876500002')
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(page.getByText('Contact created')).toBeVisible()

  const row = page.getByRole('row', { name: new RegExp(name) })
  await expect(row).toBeVisible()

  await row.getByRole('button', { name: 'Create lead from contact' }).click()
  await expect(page).toHaveURL(/\/leads\?contact=/)
  await expect(page.getByText('Linked to')).toBeVisible()

  await page.getByLabel('Notes').fill('E2E lead note')
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(page.getByText('Lead created')).toBeVisible()

  const card = page.locator('div.group.relative.rounded-lg', { hasText: name })
  await expect(card).toBeVisible()

  await card.getByRole('button', { name: 'Edit lead' }).click()
  await page.getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(page.getByText('Lead deleted')).toBeVisible()
  await expect(page.locator('div.group.relative.rounded-lg', { hasText: name })).toHaveCount(0)

  await page.goto('/contacts')
  const leftoverRow = page.getByRole('row', { name: new RegExp(name) })
  await expect(leftoverRow).toBeVisible()
  await leftoverRow.getByRole('button', { name: 'Delete contact' }).click()
  const dialog = page.getByRole('alertdialog')
  await dialog.getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(page.getByText('Contact deleted')).toBeVisible()
})
