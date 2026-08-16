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

function uniqueEmail(name: string) {
  return `${name.replace(/\s+/g, '.').toLowerCase()}@example.com`
}

function contactRow(page: Page, name: string) {
  return page.getByRole('row', { name: new RegExp(name) })
}

async function createContact(page: Page, name: string) {
  await page.goto('/contacts')
  await page.getByRole('button', { name: 'Add Contact' }).click()
  await page.getByLabel('Name *').fill(name)
  await page.getByPlaceholder('Phone 1').fill('9876500001')
  await page.getByPlaceholder('Email 1').fill(uniqueEmail(name))
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(page.getByText('Contact created')).toBeVisible()
}

async function deleteContact(page: Page, name: string) {
  await contactRow(page, name).getByRole('button', { name: 'Delete contact' }).click()
  const dialog = page.getByRole('alertdialog')
  await dialog.getByRole('button', { name: 'Delete', exact: true }).click()
  await expect(page.getByText('Contact deleted')).toBeVisible()
  await expect(contactRow(page, name)).toHaveCount(0)
}

test.describe('contacts', () => {
  test('full lifecycle: create, edit and delete', async ({ page }) => {
    await login(page)
    const name = uniqueName()
    const renamed = `${name} Renamed`

    await createContact(page, name)
    await expect(contactRow(page, name)).toBeVisible()

    await contactRow(page, name).getByRole('button', { name: 'Edit contact' }).click()
    await expect(page.getByText('Edit Contact', { exact: true })).toBeVisible()
    await page.getByLabel('Name *').fill(renamed)
    await page.getByRole('button', { name: 'Update', exact: true }).click()
    await expect(page.getByText('Contact updated')).toBeVisible()
    await expect(contactRow(page, renamed)).toBeVisible()

    await deleteContact(page, renamed)
  })

  test('search finds a contact and filters it out', async ({ page }) => {
    await login(page)
    const name = uniqueName()

    await createContact(page, name)
    await expect(contactRow(page, name)).toBeVisible()

    const search = page.getByPlaceholder('Search contacts...')
    const searchButton = page.getByRole('button', { name: 'Search', exact: true })

    await search.fill(name)
    await searchButton.click()
    await expect(contactRow(page, name)).toBeVisible()

    await search.fill('zzz-no-such-contact')
    await searchButton.click()
    await expect(page.getByText('No contacts found')).toBeVisible()
    await expect(contactRow(page, name)).toHaveCount(0)

    await search.fill(name)
    await searchButton.click()
    await expect(contactRow(page, name)).toBeVisible()

    await deleteContact(page, name)
  })
})
