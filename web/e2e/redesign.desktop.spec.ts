import { expect, test, type Page } from '@playwright/test'

import { login, tinyWebP } from './helpers'

async function expectNoHorizontalOverflow(page: Page) {
  const dimensions = await page.evaluate(() => ({
    viewport: document.documentElement.clientWidth,
    document: document.documentElement.scrollWidth,
    body: document.body.scrollWidth,
  }))
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport + 1)
  expect(dimensions.body).toBeLessThanOrEqual(dimensions.viewport + 1)
}

for (const width of [375, 768, 1024, 1440]) {
  test(`redesigned workspace remains usable at ${width}px`, async ({ page }, testInfo) => {
    test.setTimeout(90_000)
    await page.setViewportSize({ width, height: 900 })
    await login(page)

    for (const route of ['upload', 'images', 'migrations', 'aliases', 'api-tokens', 'settings', 'system']) {
      await page.goto(`/admin/${route}`)
      await expect(page.locator('main').getByRole('heading', { level: 1 })).toBeVisible()
      await page.waitForLoadState('networkidle')
      await expect(page.locator('main')).not.toContainText(/加载失败|扫描失败/)
      await expectNoHorizontalOverflow(page)
      if (width === 375 || width === 1440) {
        await page.screenshot({ path: testInfo.outputPath(`${route}-${width}.png`), fullPage: true, animations: 'disabled' })
      }
    }
  })
}

test('mobile navigation contains focus, closes with Escape, and restores the trigger', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await login(page)
  const trigger = page.getByRole('button', { name: /打开菜单/ })
  await trigger.focus()
  await trigger.press('Enter')
  const navigation = page.getByRole('dialog', { name: '页面导航' }).getByRole('navigation', { name: '主要导航' })
  await expect(navigation).toBeVisible()
  await expect.poll(() => page.evaluate(() => Boolean(document.activeElement?.closest('aside, [role="dialog"]')))).toBe(true)

  for (let index = 0; index < 14; index += 1) {
    await page.keyboard.press('Tab')
    const focusInNavigation = await page.evaluate(() => {
      const active = document.activeElement
      return Boolean(active?.closest('aside, [role="dialog"]'))
    })
    expect(focusInNavigation).toBe(true)
  }

  await page.keyboard.press('Escape')
  await expect(navigation).not.toBeVisible()
  await expect(trigger).toBeFocused()
  await trigger.press('Enter')
  await navigation.getByRole('link', { name: /图片库|图片管理/, exact: true }).click()
  await expect(page).toHaveURL(/\/admin\/images$/)
  await expect(navigation).not.toBeVisible()
  await expectNoHorizontalOverflow(page)
})

test('search is reachable by keyboard and reduced motion avoids entrance travel', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await login(page)
  await page.keyboard.press('ControlOrMeta+k')
  const search = page.getByRole('dialog')
  await expect(search).toBeVisible()
  await expect(search.getByRole('textbox')).toBeFocused()
  await search.getByRole('textbox').fill('redesign-search-no-match')
  await search.getByRole('textbox').press('Enter')
  await expect(page).toHaveURL(/\/admin\/images\?q=redesign-search-no-match/)
  await expect(search).not.toBeVisible()

  const animations = await page.evaluate(() => document.getAnimations().filter((animation) => {
    const target = animation.effect instanceof KeyframeEffect ? animation.effect.target : null
    return target instanceof Element && Boolean(target.closest('main'))
  }).map((animation) => Number(animation.effect?.getTiming().duration)))
  expect(animations.every((duration) => duration <= 1)).toBe(true)
  await expectNoHorizontalOverflow(page)
})

test('detail opens at the top and returns to the same filtered library position', async ({ page }) => {
  test.setTimeout(90_000)
  await page.setViewportSize({ width: 375, height: 700 })
  await login(page)
  const csrf = (await page.context().cookies()).find((cookie) => cookie.name === 'imagesilo_csrf')
  expect(csrf).toBeDefined()
  for (let index = 0; index < 12; index += 1) {
    const response = await page.request.post('/api/v1/images', {
      headers: { 'X-CSRF-Token': csrf!.value, Origin: new URL(page.url()).origin },
      multipart: { file: { name: `redesign-position-${String(index).padStart(2, '0')}.webp`, mimeType: 'image/webp', buffer: tinyWebP } },
    })
    expect(response.ok()).toBe(true)
  }
  await page.goto('/admin/images?q=redesign-position-')
  const image = page.locator('.image-card-link').last()
  await image.scrollIntoViewIfNeeded()
  const imageName = await image.getAttribute('aria-label')
  const position = await page.evaluate(() => window.scrollY)
  expect(position).toBeGreaterThan(100)
  let releaseDetail!: () => void
  let previewBounds: { x: number; y: number; width: number; height: number } | null
  const detailResponse = new Promise<void>((resolve) => { releaseDetail = resolve })
  await page.route(/\/api\/v1\/images\/[^/?]+$/, async (route) => {
    await detailResponse
    await route.continue()
  })
  try {
    await image.press('Enter')
    await expect(page).toHaveURL(/\/admin\/images\/[^/?]+$/)
    // The image remains available while full metadata is still loading.
    await expect(page.getByRole('img', { name: imageName!, exact: true })).toBeVisible()
    await expect.poll(() => page.evaluate(() => window.scrollY)).toBeLessThanOrEqual(1)
    previewBounds = await page.locator('.image-detail-stage').boundingBox()
  } finally {
    releaseDetail()
  }
  await expect(page.locator('.detail-core-info')).toBeVisible()
  expect(await page.locator('.image-detail-stage').boundingBox()).toEqual(previewBounds)
  await page.getByRole('button', { name: /返回图片列表|返回图片库/, exact: true }).click()
  await expect(page).toHaveURL(/\/admin\/images\?q=redesign-position-/)
  await expect.poll(async () => Math.abs((await page.evaluate(() => window.scrollY)) - position)).toBeLessThanOrEqual(2)
  await expect(page.locator('.page-transition')).toHaveCSS('animation-name', 'none')
  await expect(page.locator('.image-card-clickable').first()).toHaveCSS('animation-name', 'none')
})
