import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { Image, ImageDetail, ImageList, Visibility } from '../../lib/api-types'
import { ImageDetailPage } from './image-detail-page'
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

const secondImage = {
  ...image,
  id: '019c1234-5678-7abc-8def-0123456789ac',
  originalName: 'second.png',
  mimeType: 'image/png',
  extension: '.png',
  standardUrl: '/image/019c1234-5678-7abc-8def-0123456789ac',
  thumbnailUrl: '/api/v1/images/019c1234-5678-7abc-8def-0123456789ac/thumbnail',
} as Image

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((complete) => { resolve = complete })
  return { promise, resolve }
}

describe('ImageListPage', () => {
  afterEach(() => { cleanup(); vi.restoreAllMocks() })

  beforeEach(() => {
    window.localStorage.clear()
    vi.mocked(apiRequest).mockReset()
    vi.mocked(apiRequest).mockResolvedValue({ items: [image] } as ImageList)
  })

  it('loads thumbnails only and applies server-side search filters', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={createMemoryRouter([{ path: '*', element: <ImageListPage /> }])} />
      </QueryClientProvider>,
    )
    const preview = await screen.findByRole('img', { name: 'sample.jpg' })
    expect(preview).toHaveAttribute('src', image.thumbnailUrl)
    expect(preview).not.toHaveAttribute('src', image.standardUrl)

    fireEvent.click(screen.getByRole('button', { name: '筛选' }))
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
        <RouterProvider router={createMemoryRouter([{ path: '*', element: <ImageListPage /> }])} />
      </QueryClientProvider>,
    )
    await screen.findByRole('img', { name: 'sample.jpg' })

    fireEvent.click(screen.getByRole('button', { name: '筛选' }))
    const advanced = screen.getByRole('button', { name: '高级筛选' })
    fireEvent.click(advanced)
    expect(advanced).toHaveAttribute('aria-expanded', 'true')
    expect(advanced).toHaveAttribute('data-open', 'true')

    fireEvent.click(screen.getByRole('button', { name: /^开始日期:/ }))
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

  it('copies selected image links as one newline-delimited payload', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    vi.mocked(apiRequest).mockResolvedValue({ items: [image, secondImage] } as ImageList)
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={createMemoryRouter([{ path: '*', element: <ImageListPage /> }])} />
      </QueryClientProvider>,
    )

    fireEvent.click(await screen.findByRole('checkbox', { name: '选择图片 sample.jpg' }))
    fireEvent.click(screen.getByRole('checkbox', { name: '选择图片 second.png' }))
    fireEvent.click(screen.getByRole('button', { name: '复制选中 2 张图片的直链' }))

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(
        'http://localhost:3000/image/019c1234-5678-7abc-8def-0123456789ab\n' +
          'http://localhost:3000/image/019c1234-5678-7abc-8def-0123456789ac',
      )
    })
  })

  it('limits batch selection to 100 images across loaded pages while allowing replacements', async () => {
    const items = Array.from({ length: 101 }, (_, index) => ({
      ...image,
      id: `019c1234-5678-7abc-8def-${String(index).padStart(12, '0')}`,
      originalName: `batch-${index}.jpg`,
    }))
    vi.mocked(apiRequest).mockImplementation(async (path, options) => {
      if (String(path).includes('batch-visibility')) {
        const { imageIds } = JSON.parse(String(options?.body)) as { imageIds: string[] }
        return { items: imageIds.map((imageId) => ({ imageId, status: 'updated' })) }
      }
      const offset = Number(new URL(String(path), 'http://imagesilo.test').searchParams.get('cursor') ?? 0)
      return { items: items.slice(offset, offset + 24), nextCursor: offset + 24 < items.length ? String(offset + 24) : undefined }
    })
    const warning = vi.spyOn(toast, 'warning')
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={createMemoryRouter([{ path: '*', element: <ImageListPage /> }])} />
      </QueryClientProvider>,
    )
    await screen.findByRole('checkbox', { name: '选择图片 batch-0.jpg' })
    const loadMore = screen.getByRole('button', { name: '加载更多' })
    for (let page = 1; page <= 4; page += 1) {
      fireEvent.click(loadMore)
      await waitFor(() => expect(document.querySelectorAll('.image-select input')).toHaveLength(Math.min((page + 1) * 24, 101)))
    }
    const checkboxes = Array.from(document.querySelectorAll<HTMLInputElement>('.image-select input'))
    act(() => { for (const checkbox of checkboxes) fireEvent.click(checkbox) })
    expect(checkboxes.filter((checkbox) => checkbox.checked)).toHaveLength(100)
    expect(checkboxes[100]).not.toBeChecked()
    expect(warning).toHaveBeenCalledWith('每批最多选择 100 张图片。')

    act(() => {
      fireEvent.click(checkboxes[0])
      fireEvent.click(checkboxes[100])
    })
    expect(checkboxes[100]).toBeChecked()
    fireEvent.click(within(document.querySelector<HTMLElement>('.floating-batch-toolbar')!).getByRole('button', { name: '改为私密' }))
    await waitFor(() => {
      const request = vi.mocked(apiRequest).mock.calls.find(([path]) => String(path).includes('batch-visibility'))
      expect(request).toBeDefined()
      const { imageIds } = JSON.parse(String(request?.[1]?.body)) as { imageIds: string[] }
      expect(imageIds).toHaveLength(100)
      expect(imageIds).not.toContain(items[0].id)
      expect(imageIds).toContain(items[100].id)
    })
  }, 15_000)

  it('restores and updates local library preferences', async () => {
    window.localStorage.setItem('imagesilo_image_view_mode', 'list')
    window.localStorage.setItem('imagesilo_image_filters_open', 'true')
    window.localStorage.setItem('imagesilo_image_advanced_filters_open', 'true')
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={createMemoryRouter([{ path: '*', element: <ImageListPage /> }])} />
      </QueryClientProvider>,
    )

    await screen.findByRole('img', { name: 'sample.jpg' })
    expect(document.querySelector('.image-list')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '高级筛选' })).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('button', { name: '收起筛选' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '网格' }))
    fireEvent.click(screen.getByRole('button', { name: '收起筛选' }))
    await waitFor(() => {
      expect(window.localStorage.getItem('imagesilo_image_view_mode')).toBe('grid')
      expect(window.localStorage.getItem('imagesilo_image_filters_open')).toBe('false')
    })
  })

  it.each([
    ['public', 'private', '改为私密', '改为公开'],
    ['private', 'public', '改为公开', '改为私密'],
  ] as const)('updates a fresh %s detail after a gallery change to %s', async (initial, next, action, nextAction) => {
    let visibility: Visibility = initial
    let detailReads = 0
    const refreshed = deferred<ImageDetail>()
    vi.mocked(apiRequest).mockImplementation(async (path, options) => {
      if (options?.method === 'PATCH') {
        visibility = next
        return
      }
      if (path === `/api/v1/images/${image.id}`) {
        detailReads += 1
        return detailReads === 1 ? { ...image, visibility, aliases: [] } : refreshed.promise
      }
      return { items: [{ ...image, visibility }] }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 30_000 } } })
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={createMemoryRouter([
          { path: '/admin/images', element: <ImageListPage /> },
          { path: '/admin/images/:imageId', element: <ImageDetailPage /> },
        ], { initialEntries: [`/admin/images/${image.id}`] })} />
      </QueryClientProvider>,
    )
    await screen.findByRole('button', { name: action })
    fireEvent.click(screen.getByRole('button', { name: '返回图片列表' }))
    await screen.findByRole('checkbox', { name: `选择图片 ${image.originalName}` })
    fireEvent.click(screen.getByRole('button', { name: action }))
    await screen.findByRole('button', { name: nextAction })
    fireEvent.click(screen.getAllByRole('link', { name: image.originalName })[0])
    await screen.findByRole('heading', { name: image.originalName })
    // The confirmed value must render even while the refreshed detail request is pending.
    expect(await screen.findByRole('button', { name: nextAction })).toBeInTheDocument()
    await waitFor(() => expect(detailReads).toBe(2))
    await act(async () => refreshed.resolve({ ...image, visibility: next, aliases: [] }))
  })

  it.each(['error', 'not_found'])('only updates successful batch detail caches when another item returns %s', async (failureStatus) => {
    const uncached = { ...image, id: 'uncached', originalName: 'uncached.jpg' }
    const items = [image, secondImage, uncached]
    vi.mocked(apiRequest).mockImplementation(async (path) => {
      if (path === '/api/v1/images/batch-visibility') return {
        items: items.map((item) => ({ imageId: item.id, status: item.id === secondImage.id ? failureStatus : 'updated' })),
      }
      return { items }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 30_000 } } })
    const firstDetail = { ...image, aliases: [] }
    const failedDetail = { ...secondImage, aliases: [] }
    queryClient.setQueryData(['image', image.id], firstDetail)
    queryClient.setQueryData(['image', secondImage.id], failedDetail)
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={createMemoryRouter([{ path: '*', element: <ImageListPage /> }])} />
      </QueryClientProvider>,
    )
    for (const item of items) fireEvent.click(await screen.findByRole('checkbox', { name: `选择图片 ${item.originalName}` }))
    fireEvent.click(within(document.querySelector<HTMLElement>('.floating-batch-toolbar')!).getByRole('button', { name: '改为私密' }))
    await waitFor(() => expect(queryClient.getQueryData(['image', image.id])).toEqual({ ...firstDetail, visibility: 'private' }))
    expect(queryClient.getQueryState(['image', image.id])?.isInvalidated).toBe(true)
    expect(queryClient.getQueryData(['image', secondImage.id])).toEqual(failedDetail)
    expect(queryClient.getQueryState(['image', secondImage.id])?.isInvalidated).toBe(false)
    expect(queryClient.getQueryData(['image', uncached.id])).toBeUndefined()
  })

  it('prevents an older detail response from overwriting a confirmed gallery visibility change', async () => {
    let visibility: Visibility = 'public'
    vi.mocked(apiRequest).mockImplementation(async (_, options) => {
      if (options?.method === 'PATCH') { visibility = 'private'; return }
      return { items: [{ ...image, visibility }] }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: 30_000 } } })
    const oldDetail = { ...image, aliases: [] }
    queryClient.setQueryData(['image', image.id], oldDetail)
    const delayed = deferred<ImageDetail>()
    const pending = queryClient.fetchQuery({ queryKey: ['image', image.id], queryFn: () => delayed.promise, staleTime: 0 }).catch(() => undefined)
    render(
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={createMemoryRouter([{ path: '*', element: <ImageListPage /> }])} />
      </QueryClientProvider>,
    )
    fireEvent.click(await screen.findByRole('button', { name: '改为私密' }))
    await screen.findByRole('button', { name: '改为公开' })
    await act(async () => { delayed.resolve(oldDetail); await pending })
    expect(queryClient.getQueryData(['image', image.id])).toEqual({ ...oldDetail, visibility: 'private' })
  })
})
