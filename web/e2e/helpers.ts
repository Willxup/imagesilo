import { expect, type Page } from '@playwright/test'
import { Buffer } from 'node:buffer'

export const tinyWebP = Buffer.from('UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA', 'base64')

export async function login(page: Page) {
  await page.goto('/admin/login')
  await expect(page.getByRole('contentinfo')).toBeVisible()
  const viewport = await page.evaluate(() => ({
    clientHeight: document.documentElement.clientHeight,
    scrollHeight: document.documentElement.scrollHeight,
  }))
  expect(viewport.scrollHeight).toBeLessThanOrEqual(viewport.clientHeight)
  await page.getByLabel('管理员邮箱').fill('e2e@example.com')
  await page.getByLabel('密码').fill('imagesilo-e2e-password')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page.getByRole('heading', { name: '上传图片' })).toBeVisible()
}

export async function uploadTinyImage(page: Page, name: string) {
  await page.goto('/admin/upload')
  await page.getByLabel('选择图片文件').setInputFiles({ name, mimeType: 'image/webp', buffer: tinyWebP })
  await page.getByRole('button', { name: '上传 1 个文件' }).click()
  await expect(page.getByText(/已完成/)).toBeVisible()
}

export async function readClipboard(page: Page) {
  return page.evaluate(() => navigator.clipboard.readText())
}
