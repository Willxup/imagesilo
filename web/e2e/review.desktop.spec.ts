import { expect, test } from '@playwright/test'
import { login, uploadTinyImage } from './helpers'

test('filter submission preserves focus and real image links keep controls independent', async ({ page }) => {
  await login(page)
  await uploadTinyImage(page, 'review-focus.webp')
  await page.goto('/admin/images')
  await page.getByRole('button', { name: '筛选', exact: true }).click()
  await page.locator('input[name="q"]').fill('review-focus')
  const apply = page.getByRole('button', { name: '应用筛选', exact: true })
  await apply.focus()
  await apply.press('Enter')
  await expect(apply).toBeFocused()
  const card = page.locator('.image-grid article').filter({ hasText: 'review-focus.webp' })
  await expect(card).toBeVisible()
  await expect(card).not.toHaveAttribute('role', 'link')
  await card.getByRole('checkbox').check()
  await expect(page).toHaveURL(/images\?/)
  await card.getByRole('checkbox').uncheck()
  const link = card.locator('.image-card-link')
  await expect(link).toHaveAttribute('href', /\/admin\/images\//)
  await link.focus()
  await link.press('Enter')
  await expect(page.locator('.detail-page h1')).toHaveText('review-focus.webp')
  await page.getByRole('button', { name: '返回图片列表', exact: true }).click()
  await expect(page.locator('input[name="q"]')).toHaveValue('review-focus')
  await page.getByRole('button', { name: '重置', exact: true }).click()
  await expect(page.locator('input[name="q"]')).toHaveValue('')
})

test('dark destructive controls keep readable contrast and overview errors retain rebuild', async ({ page }) => {
  await login(page)
  await page.getByRole('button', { name: '深色', exact: true }).click()
  await page.route('**/api/v1/overview', route => route.fulfill({ status: 500, contentType: 'application/json', body: '{}' }))
  await page.goto('/admin/system')
  await page.getByRole('button', { name: '重建缓存', exact: true }).click()
  const dialog = page.getByRole('dialog')
  const action = dialog.getByRole('button', { name: '重建缓存', exact: true })
  const contrast = await action.evaluate(el => {
    const style = getComputedStyle(el)
    const luminance = (color: string) => {
      const rgb = color.match(/[\d.]+/g)!.slice(0, 3).map(Number).map(v => v / 255).map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4)
      return rgb[0] * .2126 + rgb[1] * .7152 + rgb[2] * .0722
    }
    const a = luminance(style.color), b = luminance(style.backgroundColor)
    return (Math.max(a, b) + .05) / (Math.min(a, b) + .05)
  })
  expect(contrast).toBeGreaterThanOrEqual(4.5)
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
})
