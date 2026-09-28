import { expect, test } from '@playwright/test'

import { login } from './helpers'

test('bottom navigation stays fixed and its indicator follows rapid navigation and browser history', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await login(page)
  const dock = page.getByRole('navigation', { name: '主要导航' })
  const before = (await dock.boundingBox())!
  expect(before.y + before.height).toBeLessThanOrEqual(900)
  await expect(dock.getByRole('link')).toHaveCount(7)
  await expect(page.locator('.tail-sidebar')).toHaveCount(0)

  await dock.getByRole('link', { name: '图片库', exact: true }).click()
  await dock.getByRole('link', { name: '路径映射', exact: true }).click()
  await dock.getByRole('link', { name: '设置', exact: true }).click()
  await expect(page).toHaveURL(/\/admin\/settings$/)
  const expectIndicator = async () => {
    await expect.poll(async () => {
      const active = (await dock.locator('[aria-current="page"]').boundingBox())!
      const indicator = (await dock.locator('.workspace-dock-indicator').boundingBox())!
      return Math.abs(indicator.x - active.x) + Math.abs(indicator.width - active.width)
    }).toBeLessThan(3)
  }
  await expectIndicator()
  await page.goBack()
  await expect(page).toHaveURL(/\/admin\/aliases$/)
  await expectIndicator()
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
  expect(await dock.boundingBox()).toEqual(before)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const duration = await dock.locator('.workspace-dock-indicator').evaluate((el) => parseFloat(getComputedStyle(el).transitionDuration))
  expect(duration).toBeLessThanOrEqual(.001)
})

test('menus can reverse an unfinished exit and closing search does not shift the workspace', async ({ page }) => {
  await login(page)
  const language = page.getByRole('button', { name: '语言', exact: true })
  await language.click()
  await expect(page.getByRole('menu')).toBeVisible()
  await language.click()
  await language.click()
  await expect(page.getByRole('menuitem', { name: 'English', exact: true })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('menu')).toHaveCount(0)
  await expect(language).toBeFocused()

  const before = await page.locator('main').boundingBox()
  await page.keyboard.press('ControlOrMeta+k')
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  const after = await page.locator('main').boundingBox()
  expect(after).toEqual(before)
})
