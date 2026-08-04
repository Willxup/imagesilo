import { expect, test } from '@playwright/test'

import { login, readClipboard, uploadTinyImage } from './helpers'

test('mobile administrator can login, upload, manage, and copy a link', async ({ page }) => {
  await login(page)
  const imageName = 'mobile-e2e.webp'
  await uploadTinyImage(page, imageName)

  await page.getByRole('button', { name: '打开菜单' }).click()
  await page.getByRole('link', { name: '图片管理' }).click()
  const card = page.locator('article').filter({ hasText: imageName })
  await expect(card).toBeVisible()
  await card.getByRole('button', { name: '改为私密' }).click()
  await expect(card.getByText('私密', { exact: true })).toBeVisible()
  await card.getByRole('button', { name: '复制直链', exact: true }).click()
  await expect.poll(() => readClipboard(page)).toContain('/image/')
  await card.getByRole('button', { name: '改为公开' }).click()
  await expect(card.getByText('公开', { exact: true })).toBeVisible()

  const menuButton = page.getByRole('button', { name: '打开菜单' })
  await menuButton.focus()
  await menuButton.press('Enter')
  const migrationsLink = page.getByRole('link', { name: '迁移管理' })
  await migrationsLink.focus()
  await migrationsLink.press('Enter')
  const migrationPath = '/i/2026/08/migration-mobile.webp'
  const migrationCard = page.locator('article').filter({ hasText: migrationPath })
  await expect(migrationCard).toBeVisible()
  await migrationCard.getByRole('button', { name: '复制直链', exact: true }).click()
  await expect.poll(() => readClipboard(page)).toContain(migrationPath)
})
