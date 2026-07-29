import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { InspectionResult, SystemOverview } from '../../lib/api-types'
import { SystemPage } from './system-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))

const overview = {
  imageCount: 12,
  storedBytes: 1024,
  aliasCount: 3,
  heapAllocBytes: 2 * 1024,
  heapSysBytes: 4 * 1024,
  rssBytes: 8 * 1024,
  goroutines: 9,
  indexes: { images: 12, aliases: 3, sessions: 1, tokens: 2 },
  indexConsistent: true,
  missingImageCount: 0,
  missingImageIds: [],
  lastInspection: null,
  lastRebuild: null,
  lastDaily: null,
} as SystemOverview

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(<QueryClientProvider client={queryClient}><SystemPage /></QueryClientProvider>)
}

describe('SystemPage', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
  })

  it('shows the lightweight overview and completes a read-only inspection', async () => {
    const inspection = {
      checkedAt: '2026-07-29T00:00:00Z',
      databaseImages: 12,
      imageFiles: 12,
      thumbnailFiles: 12,
      temporaryFiles: 0,
      missingImageCount: 0,
      missingImageIds: [],
      orphanImageCount: 0,
      orphanThumbnailCount: 0,
    } as InspectionResult
    vi.mocked(apiRequest).mockImplementation(async (path) => path === '/api/v1/maintenance/inspect' ? inspection : overview)

    renderPage()
    expect(await screen.findByText('系统状态')).toBeInTheDocument()
    expect(await screen.findByText('12')).toBeInTheDocument()
    expect(screen.getByText('数据库与图片/别名索引数量一致。')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '立即巡检' }))
    await waitFor(() => expect(vi.mocked(apiRequest)).toHaveBeenCalledWith('/api/v1/maintenance/inspect', { method: 'POST' }))
  })

  it('shows a traceable error when the overview cannot be loaded', async () => {
    vi.mocked(apiRequest).mockRejectedValue(new Error('offline'))
    renderPage()
    expect(await screen.findByText('系统概览加载失败。')).toBeInTheDocument()
  })

  it('makes missing formal files explicit without implying database deletion', async () => {
    vi.mocked(apiRequest).mockResolvedValue({
      ...overview,
      indexConsistent: false,
      missingImageCount: 1,
      missingImageIds: ['019c1234-5678-7abc-8def-0123456789ab'],
    } as SystemOverview)
    renderPage()
    expect(await screen.findByText(/数据库中有 1 张图片缺少正式文件/)).toBeInTheDocument()
    expect(screen.getByText('019c1234-5678-7abc-8def-0123456789ab')).toBeInTheDocument()
  })
})
