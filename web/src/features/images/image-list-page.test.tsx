import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { Image, ImageList } from '../../lib/api-types'
import { ImageListPage } from './image-list-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))

const image = {
  id: '019c1234-5678-7abc-8def-0123456789ab',
  originalName: 'sample.jpg',
  mimeType: 'image/jpeg',
  extension: '.jpg',
  width: 800,
  height: 600,
  sourceSize: 1000,
  storedSize: 900,
  sourceSha256: 'a'.repeat(64),
  storedSha256: 'b'.repeat(64),
  processingSummary: {
    action: 'preserve',
    sourceFormat: 'jpeg',
    storedFormat: 'jpeg',
    preserved: true,
    compressionEnabled: false,
    conversionEnabled: false,
  },
  visibility: 'public',
  standardUrl: '/image/019c1234-5678-7abc-8def-0123456789ab',
  thumbnailUrl: '/api/v1/images/019c1234-5678-7abc-8def-0123456789ab/thumbnail',
  createdAt: '2026-07-29T00:00:00Z',
} as Image

describe('ImageListPage', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
    vi.mocked(apiRequest).mockResolvedValue({ items: [image] } as ImageList)
  })

  it('loads thumbnails only and applies server-side search filters', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ImageListPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    const preview = await screen.findByRole('img', { name: 'sample.jpg' })
    expect(preview).toHaveAttribute('src', image.thumbnailUrl)
    expect(preview).not.toHaveAttribute('src', image.standardUrl)

    fireEvent.change(screen.getByLabelText('搜索'), { target: { value: 'legacy/sample' } })
    fireEvent.click(screen.getByRole('button', { name: '应用筛选' }))
    await waitFor(() => {
      expect(vi.mocked(apiRequest).mock.calls.some(([path]) => String(path).includes('q=legacy%2Fsample'))).toBe(true)
    })
    fireEvent.click(screen.getByRole('button', { name: '重置' }))
    await waitFor(() => expect(screen.getByLabelText('搜索')).toHaveValue(''))
    await waitFor(() => {
      const lastPath = String(vi.mocked(apiRequest).mock.calls.at(-1)?.[0])
      expect(new URL(lastPath, 'http://imagesilo.test').searchParams.has('q')).toBe(false)
    })
  })

  it('animates and highlights advanced filters and submits the shared date picker value', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <ImageListPage />
        </MemoryRouter>
      </QueryClientProvider>,
    )
    await screen.findByRole('img', { name: 'sample.jpg' })

    const advanced = screen.getByRole('button', { name: '高级筛选' })
    fireEvent.click(advanced)
    expect(advanced).toHaveAttribute('aria-expanded', 'true')
    expect(advanced).toHaveAttribute('data-open', 'true')

    fireEvent.click(screen.getByRole('button', { name: '开始日期' }))
    fireEvent.click(screen.getByRole('button', { name: '今天' }))
    fireEvent.click(screen.getByRole('button', { name: '应用筛选' }))
    await waitFor(() => {
      const matching = vi.mocked(apiRequest).mock.calls.find(([path]) => String(path).includes('createdFrom='))
      expect(matching).toBeDefined()
      const today = new Date()
      const expected = new Date(today.getFullYear(), today.getMonth(), today.getDate()).toISOString()
      expect(new URL(String(matching?.[0]), 'http://imagesilo.test').searchParams.get('createdFrom')).toBe(expected)
    })
  })
})
