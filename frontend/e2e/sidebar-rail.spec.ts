import { test, expect, type Page } from '@playwright/test'

// Locks the collapsed rail contract: every visible item in the icon-mode
// sidebar shares one centre column, and the toggle controls follow the sidebar
// state. jsdom has no layout, so these assertions measure real bounding boxes
// on the running app (the authenticated principal comes from e2e/auth.setup.ts).

const RAIL_WIDTH = 48 // --sidebar-width-icon (3rem)

// collapseSidebar collapses via the app shortcut and waits for the width
// transition to settle, so measurements never read mid-animation boxes.
async function collapseSidebar(page: Page) {
  await page.keyboard.press('Control+b')
  await expect(page.locator('[data-slot="sidebar"]')).toHaveAttribute('data-state', 'collapsed')
  await expect
    .poll(() => page.locator('[data-slot="sidebar"]').evaluate((el) => Math.round(el.getBoundingClientRect().width)))
    .toBe(RAIL_WIDTH)
}

test('every rail item shares one centre column when collapsed', async ({ page }) => {
  await page.goto('/')
  await collapseSidebar(page)

  const { centres, midpoint } = await page.evaluate(() => {
    const root = document.querySelector('[data-slot="sidebar"]')!
    const rail = root.getBoundingClientRect()
    const targets: (Element | null)[] = [
      ...Array.from(root.querySelectorAll('[data-sidebar="menu-button"] svg')),
      root.querySelector('[data-sidebar="header"] [role="img"]'),
      root.querySelector('[data-sidebar="footer"] [data-slot="avatar"]'),
      root.querySelector('[data-sidebar="footer"] [aria-label="Notifications"] svg'),
    ]
    const centres: number[] = []
    for (const el of targets) {
      if (!el) continue
      const box = el.getBoundingClientRect()
      // Not-laid-out items (the collapsed NavUser chevron) are not rail items.
      if (box.width === 0) continue
      centres.push(box.left + box.width / 2 - rail.left)
    }
    return { centres, midpoint: rail.width / 2 }
  })

  // 4 nav icons + Settings gear + logo tile + avatar + bell.
  expect(centres).toHaveLength(8)
  for (const centre of centres) {
    expect(Math.abs(centre - midpoint)).toBeLessThanOrEqual(1)
  }
})

test('toggle controls follow the sidebar state', async ({ page }) => {
  await page.goto('/')
  const trigger = page.locator('[data-sidebar="trigger"]')
  const collapse = page.locator('button[aria-label="Collapse sidebar"]')

  // Expanded (the default): the control lives in the sidebar, not the header.
  await expect(page.locator('[data-slot="sidebar"]')).toHaveAttribute('data-state', 'expanded')
  await expect(collapse).toBeVisible()
  await expect(trigger).toHaveCount(0)

  await collapseSidebar(page)
  await expect(trigger).toBeVisible()
  await expect(collapse).toBeHidden()

  // Mobile: the header trigger opens the drawer, which renders no desktop
  // collapse control.
  await page.setViewportSize({ width: 480, height: 900 })
  await expect(trigger).toBeVisible()
  await trigger.click()
  await expect(page.getByRole('dialog').getByText('Dashboard', { exact: true })).toBeVisible()
  await expect(collapse).toHaveCount(0)
})
