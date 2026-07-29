import { expect, test } from '@playwright/test'

import { login, readClipboard, uploadTinyImage } from './helpers'

test('mobile administrator can login, upload, manage, and copy a link', async ({ page }) => {
  await login(page)
  const imageName = 'mobile-e2e.webp'
  await uploadTinyImage(page, imageName)

  await page.getByRole('link', { name: '图片' }).click()
  const card = page.getByRole('article').filter({ hasText: imageName })
  await expect(card).toBeVisible()
  await card.getByRole('button', { name: '改为私密' }).click()
  await expect(card.getByText('私密', { exact: true })).toBeVisible()
  await card.getByRole('button', { name: '复制直链' }).click()
  await expect.poll(() => readClipboard(page)).toContain('/image/')
  await card.getByRole('button', { name: '改为公开' }).click()
  await expect(card.getByText('公开', { exact: true })).toBeVisible()
})
