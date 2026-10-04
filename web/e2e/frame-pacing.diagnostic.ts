// Opt-in profiling, not a machine-independent performance gate.
import { expect, test } from '@playwright/test'
import { writeFile } from 'node:fs/promises'
import { login } from './helpers'

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
