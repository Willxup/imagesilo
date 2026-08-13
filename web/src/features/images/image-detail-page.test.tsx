import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { ImageAlias, ImageDetail } from '../../lib/api-types'
import { ImageDetailPage } from './image-detail-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))

const imageId = '019c1234-5678-7abc-8def-0123456789ab'
const image = {
  id: imageId,
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
  uploadedVia: 'admin',
  standardUrl: `/image/${imageId}`,
  thumbnailUrl: `/api/v1/images/${imageId}/thumbnail`,
  createdAt: '2026-07-29T00:00:00Z',
  aliases: [],
} as ImageDetail

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/admin/images/${imageId}`]}>
        <Routes>
          <Route path="/admin/images/:imageId" element={<ImageDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ImageDetailPage', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
    vi.mocked(apiRequest).mockImplementation(async (path) => {
      if (String(path) === '/api/v1/aliases') {
        return {
          id: '019c1234-5678-7abc-8def-0123456789ac',
          path: '/i/2026/08/sample.jpg',
          imageId,
          source: 'admin',
          createdAt: '2026-08-13T00:00:00Z',
        } as ImageAlias
      }
      return image
    })
  })

  it('creates a historical path directly for the current image', async () => {
    renderPage()
    await screen.findByRole('img', { name: 'sample.jpg' })

    fireEvent.change(screen.getByLabelText('新历史路径'), { target: { value: ' /i/2026/08/sample.jpg ' } })
    fireEvent.click(screen.getByRole('button', { name: '添加历史路径' }))

    await waitFor(() => {
      const creation = vi.mocked(apiRequest).mock.calls.find(([path]) => path === '/api/v1/aliases')
      expect(creation?.[1]?.method).toBe('POST')
      expect(JSON.parse(String(creation?.[1]?.body))).toEqual({
        path: '/i/2026/08/sample.jpg',
        imageId,
        source: 'admin',
      })
    })
    await waitFor(() => expect(screen.getByLabelText('新历史路径')).toHaveValue(''))
  })
})
