import { expect, test } from '@playwright/test'
import { login, tinyWebP } from './helpers'

test('an expired session stops queued uploads before the login redirect completes', async ({ page }) => {
  await login(page)
  await page.getByLabel('选择图片文件').setInputFiles([
    { name: 'expired-first.webp', mimeType: 'image/webp', buffer: tinyWebP },
    { name: 'expired-queued.webp', mimeType: 'image/webp', buffer: tinyWebP },
  ])
  let requests = 0
  page.on('request', request => {
    if (request.method() === 'POST' && new URL(request.url()).pathname === '/api/v1/images') requests += 1
  })
  await page.context().clearCookies()
  const rejected = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/api/v1/images')
  await page.getByRole('button', { name: '上传 2 个文件', exact: true }).click()
  expect((await rejected).status()).toBe(401)
  await expect(page).toHaveURL(/\/admin\/login$/)
  await expect(page.getByLabel('管理员邮箱')).toBeVisible()
  expect(requests).toBe(1)
})

test('unfinished uploads can stay or leave without starting queued requests', async ({ page }) => {
  await login(page)
  let requests = 0
  let release!: () => void
  const pending = new Promise<void>(resolve => { release = resolve })
  await page.route('**/api/v1/images', async route => {
    if (route.request().method() !== 'POST') return route.continue()
    requests += 1
    await pending
    await route.abort().catch(() => undefined)
  })
  try {
    await page.getByLabel('选择图片文件').setInputFiles([
      { name: 'leave-first.webp', mimeType: 'image/webp', buffer: tinyWebP },
      { name: 'leave-queued.webp', mimeType: 'image/webp', buffer: tinyWebP },
    ])
    const library = page.locator('.workspace-dock').getByRole('link', { name: '图片库', exact: true })
    await library.click()
    await page.getByRole('button', { name: '留在此页', exact: true }).click()
    await expect(page).toHaveURL(/\/admin\/upload$/)
    await page.getByRole('button', { name: '上传 2 个文件', exact: true }).click()
    await expect.poll(() => requests).toBe(1)
    await library.click()
    await page.getByRole('button', { name: '停止并离开', exact: true }).click()
    await expect(page.getByRole('heading', { name: '图片库', exact: true })).toBeVisible()
    release()
    await expect(page.locator('[data-sonner-toast]')).toHaveCount(0)
    await page.locator('.workspace-dock').getByRole('link', { name: '上传图片', exact: true }).click()
    await expect(page.locator('.upload-row')).toHaveCount(0)
    expect(requests).toBe(1)
  } finally { release() }
})
