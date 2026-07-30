import { expect, test } from '@playwright/test'

import { login, readClipboard, tinyWebP, uploadTinyImage } from './helpers'

test('desktop administrator completes upload, management, alias, settings, theme, and language flows', async ({ page, request }) => {
  await login(page)
  const brandLogo = page.getByRole('img', { name: 'ImageSilo' })
  await expect(brandLogo).toBeVisible()
  await expect.poll(() => brandLogo.evaluate((image: HTMLImageElement) => image.naturalWidth)).toBeGreaterThan(0)
  expect((await request.get('/brand/favicon-64.png')).status()).toBe(200)
  await expect(page.getByText('最多同时处理 1 个文件；每批最多选择 20 个文件。')).toBeVisible()

  const imageName = 'desktop-e2e.webp'
  await uploadTinyImage(page, imageName)
  await page.getByRole('button', { name: '复制直链' }).click()
  await expect.poll(() => readClipboard(page)).toContain('/image/')

  await page.getByRole('link', { name: '图片' }).click()
  const thumbnail = page.getByRole('img', { name: imageName })
  await expect(thumbnail).toBeVisible()
  await expect(thumbnail).toHaveAttribute('src', /\/api\/v1\/images\/.+\/thumbnail/)
  await page.getByRole('link', { name: imageName }).click()
  await expect(page.getByRole('heading', { name: imageName })).toBeVisible()
  const match = page.url().match(/\/admin\/images\/([^/]+)$/)
  expect(match).not.toBeNull()
  const imageID = match![1]

  await page.getByRole('button', { name: '复制 Markdown' }).click()
  await expect(page.getByRole('button', { name: '已复制' })).toBeVisible()
  await expect.poll(() => readClipboard(page)).toContain('![')

  await page.getByRole('link', { name: '历史路径' }).click()
  const aliasPath = '/legacy/desktop-e2e.webp'
  await page.getByLabel('历史路径').first().fill(aliasPath)
  await page.getByLabel('目标图片 UUID').fill(imageID)
  await page.getByLabel('来源标记').fill('e2e')
  await page.getByRole('button', { name: '创建映射' }).click()
  await expect(page.getByText(`已创建 ${aliasPath}`)).toBeVisible()
  const aliasResponse = await page.request.get(aliasPath)
  expect(aliasResponse.status()).toBe(200)
  expect(aliasResponse.headers()['content-type']).toContain('image/webp')

  await page.getByRole('link', { name: 'API Token' }).click()
  await page.getByLabel('Token 名称').fill('desktop private upload')
  await page.getByRole('checkbox', { name: /images:upload/ }).check()
  await page.getByRole('checkbox', { name: /images:read_private/ }).check()
  await page.getByRole('button', { name: '创建 Token' }).click()
  const plaintextToken = (await page.getByRole('status').locator('code').textContent())?.trim()
  expect(plaintextToken).toBeTruthy()

  const tokenImageName = 'desktop-token-private.webp'
  const tokenUpload = await request.post('/api/v1/images', {
    headers: { Authorization: `Bearer ${plaintextToken}` },
    multipart: {
      file: { name: tokenImageName, mimeType: 'image/webp', buffer: tinyWebP },
      visibility: 'private',
    },
  })
  expect(tokenUpload.status()).toBe(201)
  const tokenImage = await tokenUpload.json() as { standardUrl: string }
  expect((await request.get(tokenImage.standardUrl)).status()).toBe(401)
  expect((await request.get(tokenImage.standardUrl, { headers: { Authorization: `Bearer ${plaintextToken}` } })).status()).toBe(200)

  await page.getByRole('link', { name: '图片' }).click()
  await page.getByLabel('搜索').fill(tokenImageName)
  await page.getByLabel('可见性').selectOption('private')
  await page.getByLabel('上传来源').selectOption('api_token')
  await page.getByRole('button', { name: '应用筛选' }).click()
  await expect(page.getByRole('img', { name: tokenImageName })).toBeVisible()
  await page.getByRole('link', { name: tokenImageName }).click()
  await expect(page.getByRole('heading', { name: tokenImageName })).toBeVisible()
  await expect(page.getByRole('heading', { name: '图片信息' }).locator('..').getByText('API Token', { exact: true })).toBeVisible()
  page.once('dialog', (dialog) => dialog.accept())
  await page.getByRole('button', { name: '永久删除' }).click()
  await expect(page).toHaveURL(/\/admin\/images$/)
  expect((await request.get(tokenImage.standardUrl, { headers: { Authorization: `Bearer ${plaintextToken}` } })).status()).toBe(404)

  await page.getByRole('link', { name: '设置' }).click()
  await expect(page.getByLabel('启用同格式压缩')).not.toBeChecked()
  await expect(page.getByLabel('将 JPEG 和 PNG 转为 WebP')).not.toBeChecked()
  await expect(page.getByText('数据库与图片/别名索引数量一致。')).toBeVisible()

  await page.getByRole('button', { name: '深色' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await page.getByRole('button', { name: 'EN' }).click()
  await expect(page.getByRole('link', { name: 'Upload' })).toBeVisible()
  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.getByRole('link', { name: 'Upload' })).toBeVisible()
})
