import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { AppSettings, SystemOverview } from '../../lib/api-types'
import { SettingsPage } from './settings-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))

describe('SettingsPage', () => {
  beforeEach(() => {
    vi.mocked(apiRequest).mockImplementation(async (path) => {
      if (path === '/api/v1/settings') {
        return {
          defaultVisibility: 'public',
          compressionEnabled: false,
          jpegQuality: 82,
          webpQuality: 82,
          pngCompressionLevel: 6,
          conversionEnabled: false,
          conversionWebpQuality: 82,
          conversionWebpLossless: false,
        } as AppSettings
      }
      return {
        imageCount: 0,
        storedBytes: 0,
        aliasCount: 0,
        heapAllocBytes: 1024,
        heapSysBytes: 2048,
        rssBytes: 4096,
        goroutines: 8,
        indexes: { images: 0, aliases: 0, sessions: 1, tokens: 0 },
        indexConsistent: true,
        missingImageCount: 0,
        missingImageIds: [],
        lastInspection: null,
        lastRebuild: null,
        lastDaily: null,
      } as SystemOverview
    })
  })

  it('shows the lightweight overview and keeps byte-changing defaults off', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(<QueryClientProvider client={queryClient}><SettingsPage /></QueryClientProvider>)
    expect(await screen.findByText('系统概览')).toBeInTheDocument()
    expect(await screen.findByLabelText('启用同格式压缩')).not.toBeChecked()
    expect(screen.getByLabelText('将 JPEG 和 PNG 转为 WebP')).not.toBeChecked()
    expect(screen.getByText('数据库与图片/别名索引数量一致。')).toBeInTheDocument()
  })
})
