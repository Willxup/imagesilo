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


test('short calendars reveal keyboard focus without scrolling the background page', async ({ page }) => {
  await login(page)
  await page.setViewportSize({ width: 844, height: 390 })
  await page.goto('/admin/api-tokens')
  await page.locator('.ui-date-trigger').click()
  const calendar = page.locator('.ui-date-popover')
  await expect(calendar).toBeVisible()
  const pageScroll = await page.evaluate(() => window.scrollY)
  for (const key of ['ArrowDown', 'ArrowDown', 'ArrowDown', 'ArrowDown', 'ArrowUp', 'ArrowUp', 'ArrowUp']) {
    await page.keyboard.press(key)
    await expect.poll(() => calendar.evaluate(panel => {
      const day = document.activeElement
      if (!(day instanceof HTMLElement) || !day.dataset.date) return false
      const rect = day.getBoundingClientRect()
      const top = panel.getBoundingClientRect().top + panel.clientTop
      return rect.top >= top - 1 && rect.bottom <= top + panel.clientHeight + 1
    })).toBe(true)
    expect(await page.evaluate(() => window.scrollY)).toBe(pageScroll)
  }
})

test('select entrance animates and respects reduced motion', async ({ page }) => {
  await login(page)
  const trigger = page.getByRole('combobox')
  await trigger.click()
  const panel = page.locator('.ui-select-content')
  await expect(panel).toHaveCSS('animation-name', 'ui-menu-in')
  await expect(panel).toHaveCSS('animation-duration', '0.14s')
  await page.keyboard.press('Escape')
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await trigger.click()
  expect(await panel.evaluate(el => parseFloat(getComputedStyle(el).animationDuration))).toBeLessThanOrEqual(.001)
})


test('token controls and long names fit narrow layouts and confirmation dialogs', async ({ page }) => {
  const name = 'production_k8s_image_sync_worker_token'.repeat(2)
  await login(page)
  await page.route('**/api/v1/api-tokens', route => route.fulfill({ json: { items: [{ id: 'layout-test', name, tokenPrefix: 'test', scopes: ['images:upload'], status: 'active', expiresAt: null }] } }))
  await page.setViewportSize({ width: 1024, height: 900 })
  await page.goto('/admin/api-tokens')
  const controls = page.locator('.token-expiry-controls')
  await expect(controls).toBeVisible()
  expect(await controls.evaluate(el => {
    const bounds = el.getBoundingClientRect()
    return Array.from(el.children).every(child => child.getBoundingClientRect().right <= bounds.right + 1)
  })).toBe(true)
  await page.setViewportSize({ width: 320, height: 800 })
  await expect(page.getByText(name, { exact: true })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320)
  await page.getByRole('button', { name: '吊销', exact: true }).click()
  const description = page.locator('.ui-modal-header p')
  await expect(description).toContainText(name)
  expect(await description.evaluate(el => el.scrollWidth <= el.clientWidth + 1)).toBe(true)
})

test('reduced motion preserves the upload illustration on hover and drag', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await login(page)
  const sheets = page.locator('.fold-art span')
  const transforms = () => sheets.evaluateAll(elements => elements.map(el => getComputedStyle(el).transform))
  const initial = await transforms()
  await page.locator('.upload-dropzone').hover()
  await expect.poll(transforms).toEqual(initial)
  await page.mouse.move(0, 0)
  const dataTransfer = await page.evaluateHandle(() => {
    const transfer = new DataTransfer()
    transfer.items.add(new File(['test'], 'motion-test.png', { type: 'image/png' }))
    return transfer
  })
  await page.locator('.upload-dropzone').dispatchEvent('dragenter', { dataTransfer })
  await dataTransfer.dispose()
  await expect(page.locator('.upload-dropzone')).toHaveClass(/is-dragging/)
  await expect.poll(transforms).toEqual(initial)
})


for (const theme of ['light', 'dark'] as const) {
  test(`failed modal actions keep error notifications above the overlay in ${theme} mode`, async ({ page }, testInfo) => {
    await login(page)
    if (theme === 'dark') await page.getByRole('button', { name: '深色', exact: true }).click()
    await page.route('**/api/v1/maintenance/rebuild', route => route.fulfill({ status: 500, json: {} }))
    await page.goto('/admin/system')
    for (const viewport of [{ width: 393, height: 600 }, { width: 844, height: 390 }]) {
      await page.setViewportSize(viewport)
      await page.getByRole('button', { name: '重建缓存', exact: true }).click()
      const dialog = page.getByRole('dialog')
      const action = dialog.getByRole('button', { name: '重建缓存', exact: true })
      await action.click()
      const toast = page.locator('[data-sonner-toast][data-type="error"]')
      await expect(toast).toContainText('维护操作失败')
      await expect(action).toBeEnabled()
      await expect(dialog).toBeVisible()
      const toastLayer = await toast.evaluate(el => Number(getComputedStyle(el.closest('[data-sonner-toaster]')!).zIndex))
      const modalLayer = await page.locator('.ui-modal-root').evaluate(el => Number(getComputedStyle(el).zIndex))
      expect(toastLayer).toBeGreaterThan(modalLayer)
      // Wait for the entrance to settle before retaining visual evidence.
      await expect.poll(() => toast.evaluate(el => el.getBoundingClientRect().top)).toBeGreaterThanOrEqual(0)
      await page.screenshot({ path: testInfo.outputPath(`modal-error-${theme}-${viewport.width}.png`) })
      expect(await toast.evaluate(el => Boolean(el.closest('[inert], [aria-hidden="true"]')))).toBe(false)
      expect(await page.locator('body').ariaSnapshot()).toContain('维护操作失败')
      await expect(page.locator('#root')).toHaveAttribute('inert', '')
      const closeToast = toast.locator('[data-close-button]')
      if (viewport.width === 393) {
        await closeToast.click()
      } else {
        await action.focus()
        await page.keyboard.press('Tab')
        await expect(closeToast).toBeFocused()
        await page.keyboard.press('Enter')
      }
      await expect(toast).toHaveCount(0)
      await expect(dialog).toBeVisible()
      await page.keyboard.press('Escape')
      await expect(dialog).toHaveCount(0)
    }
  })
}
