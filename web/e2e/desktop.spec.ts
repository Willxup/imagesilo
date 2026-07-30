import { expect, test } from '@playwright/test'

import { login, readClipboard, tinyWebP, uploadTinyImage } from './helpers'

test('desktop administrator completes upload, management, alias, settings, theme, and language flows', async ({ page, request }) => {
  await login(page)
  const brandLogo = page.getByRole('img', { name: 'ImageSilo' })
  await expect(brandLogo).toBeVisible()
  await expect.poll(() => brandLogo.locator('img:visible').evaluate((image: HTMLImageElement) => image.naturalWidth)).toBeGreaterThan(0)
  expect((await request.get('/brand/favicon-64.png')).status()).toBe(200)
  const imageName = 'desktop-e2e.webp'
  await uploadTinyImage(page, imageName)
  await page.getByRole('button', { name: '复制直链' }).click()
  await expect.poll(() => readClipboard(page)).toContain('/image/')

  await page.getByRole('link', { name: '图片管理' }).click()
  const thumbnail = page.getByRole('img', { name: imageName })
  await expect(thumbnail).toBeVisible()
  await expect(thumbnail).toHaveAttribute('src', /\/api\/v1\/images\/.+\/thumbnail/)
  await page.locator('article').filter({ hasText: imageName }).click()
  await expect(page.getByRole('heading', { name: imageName })).toBeVisible()
  await expect(page).toHaveURL(/\/admin\/images\/[^/]+$/)

  await page.getByRole('button', { name: '选择链接格式' }).click()
  await page.getByRole('button', { name: '复制 Markdown' }).click()
  await page.getByRole('button', { name: /复制.*MD/ }).click()
  await expect.poll(() => readClipboard(page)).toContain('![')

  await page.getByRole('link', { name: '路径映射' }).click()
  const aliasPath = '/legacy/desktop-e2e.webp'
  await page.getByLabel('历史路径').first().fill(aliasPath)
  await page.getByLabel('图片文件').setInputFiles({ name: 'legacy-desktop-e2e.webp', mimeType: 'image/webp', buffer: tinyWebP })
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

  await page.getByRole('link', { name: '图片管理' }).click()
  await page.getByLabel('搜索', { exact: true }).fill(tokenImageName)
  await page.getByLabel('可见性').click()
  await page.getByRole('option', { name: '私密' }).click()
  await page.getByLabel('上传来源').click()
  await page.getByRole('option', { name: 'API Token' }).click()
  await page.getByRole('button', { name: '应用筛选' }).click()
  await expect(page.getByRole('img', { name: tokenImageName })).toBeVisible()
  await page.locator('article').filter({ hasText: tokenImageName }).click()
  await expect(page.getByRole('heading', { name: tokenImageName })).toBeVisible()
  const metadataCard = page.getByRole('heading', { name: '图片信息' }).locator('..').locator('..')
  await expect(metadataCard.getByText('API Token', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '删除', exact: true }).click()
  await page.getByRole('dialog').getByRole('button', { name: '永久删除' }).click()
  await expect(page).toHaveURL(/\/admin\/images$/)
  expect((await request.get(tokenImage.standardUrl, { headers: { Authorization: `Bearer ${plaintextToken}` } })).status()).toBe(404)

  await page.getByRole('link', { name: '设置' }).click()
  await expect(page.getByLabel('启用同格式压缩')).not.toBeChecked()
  await expect(page.getByLabel('将 JPEG 和 PNG 转为 WebP')).not.toBeChecked()
  await page.getByRole('link', { name: '系统' }).click()
  await expect(page.getByText('数据库与图片/别名索引数量一致。')).toBeVisible()

  await page.getByRole('button', { name: '深色' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await page.getByRole('button', { name: '语言' }).click()
  await page.getByRole('button', { name: 'English' }).click()
  await expect(page.getByRole('link', { name: 'Upload' })).toBeVisible()
  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.getByRole('link', { name: 'Upload' })).toBeVisible()
})
