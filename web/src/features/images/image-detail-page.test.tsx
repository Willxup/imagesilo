import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createMemoryRouter, MemoryRouter, Route, RouterProvider, Routes } from 'react-router-dom'
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
  afterEach(() => { cleanup(); vi.restoreAllMocks() })

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

  it('invalidates a delayed delete return on popstate before the router commits', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    let releaseRefresh!: () => void
    const refresh = new Promise<void>((resolve) => { releaseRefresh = resolve })
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries').mockImplementation((filters) => filters?.queryKey?.[0] === 'images' ? refresh : Promise.resolve())
    const router = createMemoryRouter([
      { path: '/admin/upload', element: <h1>Upload</h1> },
      { path: '/admin/images', element: <h1>Filtered library</h1> },
      { path: '/admin/images/:imageId', element: <ImageDetailPage /> },
    ], {
      initialEntries: ['/admin/upload', '/admin/images?q=sample', { pathname: `/admin/images/${imageId}`, state: { fromImageList: true } }],
      initialIndex: 2,
    })
    render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)
    await screen.findByRole('img', { name: 'sample.jpg' })
    fireEvent.click(screen.getByRole('button', { name: '删除' }))
    fireEvent.click(screen.getByRole('button', { name: '永久删除' }))
    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ['images'], refetchType: 'all' }))

    // A native POP happens synchronously; leave the memory router uncommitted.
    act(() => { window.dispatchEvent(new PopStateEvent('popstate')) })
    await act(async () => { releaseRefresh() })
    await waitFor(() => expect(queryClient.getMutationCache().getAll()[0]?.state.status).toBe('success'))
    expect(router.state.location.pathname).toBe(`/admin/images/${imageId}`)
    await act(async () => { await router.navigate(-1) })
    expect(router.state.location.pathname).toBe('/admin/images')
    expect(router.state.location.search).toBe('?q=sample')
  })
})
