import { Buffer } from 'node:buffer'
import { expect, test, type Locator } from '@playwright/test'

import { login, uploadTinyImage } from './helpers'

test('active filters keep labels and view controls inside narrow Chinese and English layouts', async ({ page }, testInfo) => {
  await login(page)
  for (const language of ['zh', 'en']) {
    if (language === 'en') {
      await page.getByRole('button', { name: '语言', exact: true }).click()
      await page.getByRole('menuitem', { name: 'English', exact: true }).click()
    }
    for (const width of [320, 375, 393]) {
      await page.setViewportSize({ width, height: 851 })
      await page.goto('/admin/images')
      await page.getByRole('button', { name: language === 'zh' ? '公开' : 'Public', exact: true }).click()
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
      for (const button of await page.locator('.library-toolbar button:visible').all()) {
        const box = (await button.boundingBox())!
        expect(box.x).toBeGreaterThanOrEqual(0)
        expect(box.x + box.width).toBeLessThanOrEqual(width)
        if (!(await button.evaluate((el) => el.classList.contains('filter-toggle')))) {
          expect(await button.evaluate((el) => el.scrollWidth <= el.clientWidth + 1)).toBe(true)
        }
      }
      const heading = page.locator('.library-heading')
      const title = heading.locator('h1')
      expect((await title.boundingBox())!.height).toBeLessThanOrEqual(await title.evaluate((el) => parseFloat(getComputedStyle(el).lineHeight) + 1))
      expect((await heading.locator('.page-description').boundingBox())!.width).toBeCloseTo((await heading.boundingBox())!.width, 0)
      const tabs = (await page.locator('.library-visibility').boundingBox())!
      const tools = (await page.locator('.library-toolbar > .library-tools').boundingBox())!
      expect(Math.abs(tabs.y + tabs.height / 2 - tools.y - tools.height / 2)).toBeLessThan(2)
      await page.getByRole('button', { name: language === 'zh' ? '列表' : 'List', exact: true }).click()
      await page.screenshot({ path: testInfo.outputPath(`filtered-${language}-${width}.png`), animations: 'disabled' })
      await page.getByRole('button', { name: language === 'zh' ? '筛选' : 'Filters', exact: true }).click()
      await page.getByRole('button', { name: language === 'zh' ? '重置' : 'Reset', exact: true }).click()
      await page.getByRole('button', { name: language === 'zh' ? '收起筛选' : 'Hide filters', exact: true }).click()
      await expect(page).toHaveURL(/\/admin\/images$/)
    }
  }
})

async function expectInsideViewport(locator: Locator, width: number, height: number) {
  await expect.poll(async () => {
    const box = await locator.boundingBox()
    return Boolean(box && box.x >= 0 && box.y >= 0 && box.x + box.width <= width + 1 && box.y + box.height <= height + 1)
  }).toBe(true)
}

test('touch controls, copy feedback and overlays stay usable on narrow and short screens', async ({ page }, testInfo) => {
  await login(page)
  await uploadTinyImage(page, 'polish-touch.webp')
  await page.goto('/admin/images?q=polish-touch')
  const card = page.locator('article').filter({ hasText: 'polish-touch.webp' })
  await expect(card.locator('.preview-image')).toHaveAttribute('data-ready', 'true')

  for (const viewport of [{ width: 320, height: 640 }, { width: 393, height: 851 }, { width: 844, height: 390 }]) {
    await page.setViewportSize(viewport)
    const copy = card.locator('.copy-link-main')
    await copy.scrollIntoViewIfNeeded()
    for (const button of await card.locator('.image-card-actions button').all()) {
      const box = (await button.boundingBox())!
      expect(box.height).toBeGreaterThanOrEqual(44)
      expect(box.width).toBeGreaterThanOrEqual(44)
    }
    const documentBounds = () => copy.evaluate((el) => {
      const rect = el.getBoundingClientRect()
      return { x: rect.x + window.scrollX, y: rect.y + window.scrollY, width: rect.width, height: rect.height }
    })
    const before = await documentBounds()
    await copy.click()
    await expect(card.locator('.copy-link-control')).toHaveAttribute('data-copied', 'true')
    expect(await documentBounds()).toEqual(before)
    await card.getByRole('checkbox').check()
    const batch = page.locator('.floating-batch-toolbar')
    await expectInsideViewport(batch, viewport.width, viewport.height)
    await batch.locator('.copy-link-caret').click()
    const menu = batch.getByRole('menu')
    await expectInsideViewport(menu, viewport.width, viewport.height)
    await menu.getByRole('menuitem', { name: '复制 HTML', exact: true }).click()
    await card.getByRole('checkbox').uncheck()
    await page.getByRole('button', { name: '打开菜单', exact: true }).click()
    const sheet = page.getByRole('dialog', { name: '页面导航' })
    await expectInsideViewport(sheet, viewport.width, viewport.height)
    const layerOrder = await page.evaluate(() => {
      const toast = document.querySelector('[data-sonner-toaster]')!
      const modal = document.querySelector('.ui-modal-root')!
      return Number(getComputedStyle(modal).zIndex) > Number(getComputedStyle(toast).zIndex)
    })
    expect(layerOrder).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`navigation-${viewport.width}.png`), animations: 'disabled' })
    await page.keyboard.press('Escape')
    await expect(sheet).toHaveCount(0)
  }

  await page.setViewportSize({ width: 320, height: 640 })
  await page.goto('/admin/api-tokens')
  const date = page.locator('.ui-date-trigger')
  await date.click()
  const calendar = page.locator('.ui-date-popover')
  await expectInsideViewport(calendar, 320, 640)
  const horizontalOverflow = await calendar.evaluate((el) => el.scrollWidth - el.clientWidth)
  expect(horizontalOverflow).toBeLessThanOrEqual(1)
  await calendar.getByRole('button', { name: '今天', exact: true }).click()
  await expect(calendar).toHaveCount(0)
  await expect(date).toBeFocused()

  await page.goto('/admin/images?q=polish-touch')
  await page.locator('.image-card-title-row h2').first().click()
  await expect(page.locator('.detail-visibility button')).toBeVisible()
  for (const width of [375, 393]) {
    await page.setViewportSize({ width, height: 851 })
    for (const button of await page.locator('.detail-visibility button, .image-detail-actions > [data-slot="button"]').all()) {
      expect((await button.boundingBox())!.height).toBeGreaterThanOrEqual(44)
    }
  }
})


test('mobile grid shows complete landscape, portrait and square images in one column', async ({ page }) => {
  await login(page)
  const csrf = (await page.context().cookies()).find((cookie) => cookie.name === 'imagesilo_csrf')!
  for (const [width, height] of [[1200, 400], [400, 900], [600, 600]]) {
    const data = await page.evaluate(({ width, height }) => {
      const canvas = document.createElement('canvas')
      canvas.width = width; canvas.height = height
      const ctx = canvas.getContext('2d')!
      ctx.fillStyle = '#dcefea'; ctx.fillRect(0, 0, width, height)
      ctx.strokeStyle = '#1862cb'; ctx.lineWidth = 24; ctx.strokeRect(12, 12, width - 24, height - 24)
      return canvas.toDataURL('image/png').split(',')[1]
    }, { width, height })
    const response = await page.request.post('/api/v1/images', {
      headers: { 'X-CSRF-Token': csrf.value, Origin: new URL(page.url()).origin },
      multipart: { file: { name: `uncropped-${width}-${height}.png`, mimeType: 'image/png', buffer: Buffer.from(data, 'base64') } },
    })
    expect(response.ok()).toBe(true)
  }
  await page.goto('/admin/images?q=uncropped-')
  await expect(page.locator('.image-grid article')).toHaveCount(3)
  for (const width of [320, 375, 393]) {
    await page.setViewportSize({ width, height: 851 })
    const cards = page.locator('.image-grid article')
    const first = (await cards.first().boundingBox())!
    for (const card of await cards.all()) {
      expect((await card.boundingBox())!.x).toBeCloseTo(first.x, 0)
      const media = card.locator('.image-card-media')
      const img = media.locator('img')
      await img.scrollIntoViewIfNeeded()
      await expect(img).toHaveAttribute('data-ready', 'true')
      await expect(img).toHaveCSS('object-fit', 'contain')
      const ratio = await img.evaluate((el: HTMLImageElement) => el.naturalWidth / el.naturalHeight)
      const box = (await media.boundingBox())!
      expect(Math.abs(box.width / box.height - ratio)).toBeLessThan(0.02)
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
  }
})
