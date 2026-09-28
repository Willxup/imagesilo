import { expect, test } from '@playwright/test'
import { writeFile } from 'node:fs/promises'

import { login, uploadTinyImage } from './helpers'

test('switching navigation to search releases the background only after the last dialog closes', async ({ page }) => {
  await page.setViewportSize({ width: 800, height: 900 })
  await login(page)
  const trigger = page.getByRole('button', { name: '打开菜单', exact: true })
  await trigger.click()
  await expect(page.getByRole('dialog', { name: '页面导航' })).toBeVisible()
  await page.keyboard.press('ControlOrMeta+k')
  const search = page.locator('.command-search input')
  await expect(search).toBeFocused()
  await expect(page.locator('.navigation-sheet')).toHaveCount(0)
  await expect(page.locator('#root')).toHaveAttribute('inert', '')
  await expect(page.locator('body')).toHaveCSS('overflow', 'hidden')
  await expect(search).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.locator('#root')).not.toHaveAttribute('inert')
  await expect(page.locator('#root')).not.toHaveAttribute('aria-hidden')
  await expect(page.locator('body')).not.toHaveCSS('overflow', 'hidden')
  await expect(trigger).toBeFocused()
  await page.getByRole('link', { name: '图片库', exact: true }).click()
  await expect(page.locator('main h1')).toHaveText('图片库')
})

test('delayed thumbnails retain an opacity transition and reveal after loading', async ({ page }) => {
  await login(page)
  let release!: () => void
  const response = new Promise<void>((resolve) => { release = resolve })
  await page.route('**/i/2026/08/migration-mobile.webp', async (route) => { await response; await route.continue() })
  await page.goto('/admin/migrations')
  const image = page.getByRole('img', { name: 'migration-mobile.webp', exact: true })
  try {
    await expect(image).toBeAttached()
    await expect(image).not.toHaveAttribute('data-ready')
    await expect(image).toHaveCSS('opacity', '0')
    await expect(image).toHaveCSS('transition-property', 'opacity')
  } finally { release() }
  await expect(image).toHaveAttribute('data-ready', 'true')
  await expect(image).toHaveCSS('opacity', '1')
})

test('all pages retain readable layout in dark English mode', async ({ page }, testInfo) => {
  await login(page)
  await page.getByRole('button', { name: '深色', exact: true }).click()
  await page.getByRole('button', { name: '语言', exact: true }).click()
  await page.getByRole('menuitem', { name: 'English', exact: true }).click()
  for (const width of [393, 1440]) {
    await page.setViewportSize({ width, height: 900 })
    for (const route of ['upload', 'images', 'migrations', 'aliases', 'api-tokens', 'settings', 'system']) {
      await page.goto(`/admin/${route}`)
      await expect(page.locator('main h1')).toBeVisible()
      await page.waitForLoadState('networkidle')
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
      await page.screenshot({ path: testInfo.outputPath(`dark-${route}-${width}.png`), fullPage: true, animations: 'disabled' })
    }
  }
})

test('slow and failed library loading preserves retry and navigation', async ({ page }) => {
  await login(page)
  let failure = true
  await page.route('**/api/v1/images?*', async (route) => {
    if (failure) {
      await new Promise((resolve) => setTimeout(resolve, 500))
      await route.fulfill({ status: 503, contentType: 'application/json', body: '{"error":"temporarily unavailable"}' })
    } else await route.continue()
  })
  await page.getByRole('link', { name: '图片库', exact: true }).click()
  await expect(page.locator('.image-skeleton').first()).toBeVisible()
  await expect(page.getByText('图片列表加载失败。')).toBeVisible({ timeout: 20_000 })
  failure = false
  await page.getByRole('button', { name: '重试加载', exact: true }).click()
  await expect(page.getByText('图片列表加载失败。')).toHaveCount(0)
  await expect(page.locator('.image-skeleton')).toHaveCount(0)
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight))
  await page.getByRole('link', { name: '设置', exact: true }).click()
  await expect(page.locator('main h1')).toHaveText('设置')
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)
})

test('records frame pacing during repeated navigation with CPU throttling', async ({ page }, testInfo) => {
  await login(page)
  const cdp = await page.context().newCDPSession(page)
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 4 })
  try {
    const frames = page.evaluate(async () => {
      const times: number[] = []
      const longTasks: number[] = []
      const observer = new PerformanceObserver((entries) => longTasks.push(...entries.getEntries().map((entry) => entry.duration)))
      observer.observe({ type: 'longtask' })
      await new Promise<void>((resolve) => {
        const frame = (time: number) => { times.push(time); if (times.length < 240) requestAnimationFrame(frame); else resolve() }
        requestAnimationFrame(frame)
      })
      observer.disconnect()
      const intervals = times.slice(1).map((time, index) => time - times[index]).sort((a, b) => a - b)
      return { samples: intervals.length, medianMs: intervals[Math.floor(intervals.length * .5)], p95Ms: intervals[Math.floor(intervals.length * .95)], maxMs: intervals.at(-1), framesOver50Ms: intervals.filter((value) => value > 50).length, longTasks }
    })
    for (const name of ['图片库', '路径映射', '设置', '上传图片', '系统状态', '上传图片']) {
      await page.getByRole('navigation', { name: '主要导航' }).getByRole('link', { name, exact: true }).click()
      await expect(page.locator('main h1')).toBeVisible()
    }
    const reportPath = testInfo.outputPath('frame-pacing-4x-cpu.json')
    await writeFile(reportPath, JSON.stringify(await frames, null, 2))
    await testInfo.attach('frame-pacing-4x-cpu.json', { path: reportPath, contentType: 'application/json' })
  } finally {
    await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 })
    await cdp.detach()
  }
})


test('background copy menu does not consume search input navigation keys', async ({ page }) => {
  await login(page)
  await uploadTinyImage(page, 'keyboard-menu.webp')
  await page.goto('/admin/images?q=keyboard-menu')
  await page.locator('.image-card-actions .copy-link-caret').first().click()
  await page.keyboard.press('ControlOrMeta+k')
  const input = page.locator('.command-search input')
  await expect(input).toBeFocused()
  await input.fill('abcdef')
  await input.press('Home')
  expect(await input.evaluate((el: HTMLInputElement) => el.selectionStart)).toBe(0)
  await input.press('End')
  expect(await input.evaluate((el: HTMLInputElement) => el.selectionStart)).toBe(6)
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
})
