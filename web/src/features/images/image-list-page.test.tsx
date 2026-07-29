import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

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
  beforeEach(() => {
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
  })
})
