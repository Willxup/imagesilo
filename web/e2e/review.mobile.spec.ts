import { expect, test } from '@playwright/test'
import { login } from './helpers'

test('header, labels and single-column media remain consistent across themes and languages', async ({ page }, testInfo) => {
  test.setTimeout(90_000)
  await login(page)
  await page.getByLabel('选择图片文件').setInputFiles('../docs/design/assets/sample-12.jpg')
  await page.getByRole('button', { name: '上传 1 个文件' }).click()
  await expect(page.getByText(/已完成/)).toBeVisible()
  for (const language of ['zh-CN', 'en-US']) {
    for (const theme of ['light', 'dark']) {
      await page.evaluate(({ language, theme }) => {
        localStorage.setItem('imagesilo_language', language)
        localStorage.setItem('imagesilo_theme', theme)
        localStorage.setItem('imagesilo_image_view_mode', 'grid')
      }, { language, theme })
      for (const width of [320, 375, 393]) {
        await page.setViewportSize({ width, height: 851 })
        await page.goto('/admin/images?q=sample-12')
        const card = page.locator('.image-grid article').first()
        await expect(card).toBeVisible()
        const logo = (await page.locator('.tail-shell-logo').boundingBox())!
        const tools = (await page.locator('.tail-header-actions').boundingBox())!
        expect(Math.abs(logo.y + logo.height / 2 - tools.y - tools.height / 2)).toBeLessThan(2)
        expect(logo.x + logo.width).toBeLessThan(tools.x)
        expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
        await expect(card.locator('img')).toHaveAttribute('data-ready', 'true')
        await expect(card.locator('img')).toHaveCSS('object-fit', 'contain')
        expect(await page.locator('.image-grid').evaluate(el => getComputedStyle(el).gridTemplateColumns.split(' ').length)).toBe(1)
        await expect(card.locator('.image-card-actions > button')).toHaveCSS('border-radius', '10px')
        await page.screenshot({ path: testInfo.outputPath(`review-${language}-${theme}-${width}.png`), animations: 'disabled' })
        await page.goto('/admin/system')
        const more = page.locator('.workspace-dock > button')
        const label = await more.locator('span').innerText()
        await expect(more).toHaveAccessibleName(new RegExp(label))
        expect(await more.locator('span').evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true)
      }
    }
  }
})

test('mobile auth toolbar stays in flow in both languages and themes', async ({ page }, testInfo) => {
  for (const language of ['zh-CN', 'en-US']) {
    for (const theme of ['light', 'dark']) {
      await page.goto('/admin/login')
      await page.evaluate(({ language, theme }) => {
        localStorage.setItem('imagesilo_language', language)
        localStorage.setItem('imagesilo_theme', theme)
      }, { language, theme })
      await page.setViewportSize({ width: 320, height: 640 })
      await page.reload()
      const story = (await page.locator('.login-story').boundingBox())!
      const toolbar = (await page.locator('.login-toolbar').boundingBox())!
      expect(toolbar.y).toBeGreaterThanOrEqual(story.y + story.height)
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(320)
      await page.screenshot({ path: testInfo.outputPath(`auth-${language}-${theme}.png`), fullPage: true, animations: 'disabled' })
    }
  }
})
