import { expect, test } from '@playwright/test'

import { login, readClipboard, uploadTinyImage } from './helpers'

test('mobile administrator can login, upload, manage, and copy a link', async ({ page }, testInfo) => {
  await login(page)
  await page.screenshot({ path: testInfo.outputPath('console-mobile-upload.png'), animations: 'disabled' })
  const imageName = 'mobile-e2e.webp'
  await uploadTinyImage(page, imageName)

  await page.getByRole('button', { name: '打开菜单' }).click()
  await page.getByRole('dialog', { name: '页面导航' }).getByRole('link', { name: '图片库' }).click()
  const card = page.locator('article').filter({ hasText: imageName })
  await expect(card).toBeVisible()
  await card.getByRole('button', { name: '改为私密' }).click()
  await expect(card.getByText('私密', { exact: true })).toBeVisible()
  await card.getByRole('button', { name: '复制直链', exact: true }).click()
  await expect.poll(() => readClipboard(page)).toContain('/image/')
  await card.getByRole('button', { name: '改为公开' }).click()
  await expect(card.getByText('公开', { exact: true })).toBeVisible()

  const imageCheckbox = card.getByRole('checkbox', { name: `选择图片 ${imageName}` })
  await imageCheckbox.check()
  const batchToolbar = page.locator('.floating-batch-toolbar')
  const dockBounds = await batchToolbar.boundingBox()
  expect(dockBounds).not.toBeNull()
  const viewport = page.viewportSize()!
  expect(dockBounds!.x).toBeGreaterThanOrEqual(0)
  expect(dockBounds!.x + dockBounds!.width).toBeLessThanOrEqual(viewport.width)
  expect(dockBounds!.y).toBeGreaterThanOrEqual(0)
  expect(dockBounds!.y + dockBounds!.height).toBeLessThanOrEqual(viewport.height)
  const navigationBounds = (await page.locator('.workspace-dock').boundingBox())!
  expect(dockBounds!.y + dockBounds!.height).toBeLessThan(navigationBounds.y)
  await batchToolbar.locator('.copy-link-caret').click()
  const menuBounds = await batchToolbar.locator('.copy-format-menu').boundingBox()
  expect(menuBounds).not.toBeNull()
  expect(menuBounds!.x).toBeGreaterThanOrEqual(0)
  expect(menuBounds!.x + menuBounds!.width).toBeLessThanOrEqual(viewport.width)
  await page.getByRole('menuitem', { name: '复制 BBCode', exact: true }).click()
  await batchToolbar.getByRole('button', { name: /复制选中 1 张图片的BBCode/ }).click()
  await expect.poll(() => readClipboard(page)).toContain('[img]')
  await imageCheckbox.uncheck()

  const menuButton = page.getByRole('button', { name: '打开菜单' })
  await menuButton.focus()
  await menuButton.press('Enter')
  const migrationsLink = page.getByRole('dialog', { name: '页面导航' }).getByRole('link', { name: '迁移文件' })
  await expect(migrationsLink).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('console-mobile-navigation.png'), animations: 'disabled' })
  await migrationsLink.focus()
  await migrationsLink.press('Enter')
  const migrationPath = '/i/2026/08/migration-mobile.webp'
  const migrationCard = page.locator('article').filter({ hasText: migrationPath })
  await expect(migrationCard).toBeVisible()
  await migrationCard.getByRole('button', { name: '复制BBCode', exact: true }).click()
  await expect.poll(() => readClipboard(page)).toContain(migrationPath)
})

for (const viewport of [{ width: 320, height: 640 }, { width: 844, height: 390 }]) {
  test(`compact navigation works at ${viewport.width}x${viewport.height} including all destinations`, async ({ page }) => {
    await login(page)
    await page.setViewportSize(viewport)
    const dock = page.locator('.workspace-dock')
    for (const [label, path] of [['迁移文件', 'migrations'], ['路径映射', 'aliases'], ['API Token', 'api-tokens'], ['设置', 'settings'], ['系统状态', 'system']]) {
      await page.getByRole('button', { name: '打开菜单', exact: true }).click()
      const dialog = page.getByRole('dialog', { name: '页面导航' })
      await expect(dialog).toBeVisible()
      await dialog.getByRole('link', { name: label, exact: true }).click()
      await expect(page).toHaveURL(new RegExp(`/admin/${path}$`))
      await expect(dialog).not.toBeVisible()
      await expect(dock.getByRole('button', { name: '打开菜单' })).toContainText(label)
      const box = (await dock.boundingBox())!
      expect(box.x).toBeGreaterThanOrEqual(0)
      expect(box.x + box.width).toBeLessThanOrEqual(viewport.width)
      expect(box.y + box.height).toBeLessThanOrEqual(viewport.height)
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
    }
    await page.getByRole('button', { name: '深色', exact: true }).click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
    await page.getByRole('button', { name: '语言', exact: true }).click()
    await page.getByRole('menuitem', { name: 'English', exact: true }).click()
    await expect(dock.getByRole('link', { name: 'Upload images' })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width)
  })
}
