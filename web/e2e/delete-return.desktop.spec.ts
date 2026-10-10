import { expect, test } from '@playwright/test'
import { login, tinyWebP, uploadTinyImage } from './helpers'

test('deleting from details restores filters and clamps the shorter library scroll position', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 700 })
  await login(page)
  const csrf = (await page.context().cookies()).find(cookie => cookie.name === 'imagesilo_csrf')!
  for (let index = 0; index < 4; index += 1) {
    const response = await page.request.post('/api/v1/images', {
      headers: { 'X-CSRF-Token': csrf.value, Origin: new URL(page.url()).origin },
      multipart: { file: { name: `delete-return-${index}.webp`, mimeType: 'image/webp', buffer: tinyWebP } },
    })
    expect(response.ok()).toBe(true)
  }
  await page.goto('/admin/images?q=delete-return-')
  const image = page.locator('.image-card-link').last()
  await image.scrollIntoViewIfNeeded()
  const name = await image.getAttribute('aria-label')
  const position = await page.evaluate(() => window.scrollY)
  expect(position).toBeGreaterThan(100)
  await image.press('Enter')
  await page.getByRole('button', { name: '删除', exact: true }).click()
  await page.getByRole('dialog').getByRole('button', { name: '永久删除', exact: true }).click()
  await expect(page).toHaveURL(/\/admin\/images\?q=delete-return-/)
  await expect(page.locator('.image-card-link')).toHaveCount(3)
  await expect(page.getByRole('link', { name: name!, exact: true })).toHaveCount(0)
  await expect.poll(() => page.evaluate(saved => Math.abs(window.scrollY - Math.min(saved, document.documentElement.scrollHeight - window.innerHeight)), position)).toBeLessThanOrEqual(2)
  await expect(page.locator('input[name="q"]')).toHaveValue('delete-return-')

  // A direct detail visit has no list history state and falls back to the default library.
  const direct = await page.locator('.image-card-link').first().getAttribute('href')
  await page.goto(direct!)
  await page.getByRole('button', { name: '删除', exact: true }).click()
  await page.getByRole('dialog').getByRole('button', { name: '永久删除', exact: true }).click()
  await expect(page).toHaveURL(/\/admin\/images$/)
})


test('a delayed deletion refresh never navigates again after the user goes back', async ({ page }) => {
  await login(page)
  await uploadTinyImage(page, 'delete-delayed-return.webp')
  await page.goto('/admin/images?q=delete-delayed-return')
  await page.locator('.image-card-link').click()
  await expect(page.locator('.detail-core-info')).toBeVisible()
  let release!: () => void
  let requested!: () => void
  const gate = new Promise<void>(resolve => { release = resolve })
  const started = new Promise<void>(resolve => { requested = resolve })
  await page.route('**/api/v1/images?*', async route => {
    requested()
    await gate
    await route.continue().catch(() => undefined)
  })
  try {
    await page.getByRole('button', { name: '删除', exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: '永久删除', exact: true }).click()
    await started
    await page.goBack()
    await expect(page).toHaveURL(/\/admin\/images\?q=delete-delayed-return/)
    const response = page.waitForResponse(res => res.url().includes('/api/v1/images?'))
    release()
    await response
    await expect(page.locator('.empty-state')).toBeVisible()
    await expect(page).toHaveURL(/\/admin\/images\?q=delete-delayed-return/)
    await expect(page.locator('input[name="q"]')).toHaveValue('delete-delayed-return')
  } finally { release() }
})
