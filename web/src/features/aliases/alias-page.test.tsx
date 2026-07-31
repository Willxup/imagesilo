import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest, uploadForm } from '../../lib/api-client'
import type { ImageAlias, ImageAliasList, ImportResult } from '../../lib/api-types'
import { AliasPage } from './alias-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn(), uploadForm: vi.fn() }))

const alias = {
  id: '019c1234-5678-7abc-8def-0123456789ab',
  path: '/i/2022/05/example.webp',
  imageId: '019c4321-8765-7cba-8fed-ba9876543210',
  source: 'manual',
  createdAt: '2026-07-29T00:00:00Z',
} as ImageAlias

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <AliasPage />
    </QueryClientProvider>,
  )
}

describe('AliasPage', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
    vi.mocked(uploadForm).mockReset()
  })

  it('shows an existing alias and reports a successful resolve', async () => {
    vi.mocked(apiRequest).mockImplementation(async (path) => {
      if (String(path).startsWith('/api/v1/aliases/resolve')) return alias
      return { items: [alias] } as ImageAliasList
    })

    renderPage()
    expect(await screen.findByText(alias.path)).toBeInTheDocument()

    fireEvent.change(screen.getAllByLabelText('历史路径')[1], { target: { value: alias.path } })
    fireEvent.click(screen.getByRole('button', { name: '解析路径' }))
    expect(await screen.findByRole('link', { name: '已关联图片' })).toHaveAttribute('href', `/image/${alias.imageId}`)
  })

  it('uploads one image and historical path through the atomic import endpoint', async () => {
    vi.mocked(apiRequest).mockResolvedValue({ items: [] } as ImageAliasList)
    vi.mocked(uploadForm).mockResolvedValue({ imageId: alias.imageId, standardUrl: `/image/${alias.imageId}`, sha256: 'a'.repeat(64), alias } as ImportResult)
    renderPage()
    await screen.findByText('还没有历史路径映射。')

    const file = new File(['image'], 'legacy.webp', { type: 'image/webp' })
    fireEvent.change(screen.getByLabelText('图片文件'), { target: { files: [file] } })
    fireEvent.change(screen.getAllByLabelText('历史路径')[0], { target: { value: alias.path } })
    fireEvent.submit(screen.getByRole('button', { name: '创建映射' }).closest('form')!)

    await waitFor(() => expect(uploadForm).toHaveBeenCalledOnce())
    const [path, body] = vi.mocked(uploadForm).mock.calls[0]
    expect(path).toBe('/api/v1/imports')
    expect(body.get('file')).toBe(file)
    expect(body.get('alias')).toBe(alias.path)
    expect(await screen.findByText(`已创建 ${alias.path}`)).toBeInTheDocument()
  })

  it('renders empty and error states without hiding the management forms', async () => {
    vi.mocked(apiRequest).mockResolvedValueOnce({ items: [] } as ImageAliasList)
    renderPage()
    expect(await screen.findByText('还没有历史路径映射。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '创建映射' })).toBeInTheDocument()

    vi.mocked(apiRequest).mockRejectedValueOnce(new Error('offline'))
    fireEvent.change(screen.getAllByLabelText('历史路径')[1], { target: { value: '/missing.webp' } })
    fireEvent.click(screen.getByRole('button', { name: '解析路径' }))
    await waitFor(() => expect(screen.getByText('没有找到该映射。')).toBeInTheDocument())
  })

  it('loads aliases beyond the first 100-item cursor page', async () => {
    const secondAlias = { ...alias, id: '019c1234-5678-7abc-8def-0123456789ac', path: '/i/2022/05/second.webp' }
    vi.mocked(apiRequest).mockImplementation(async (path) =>
      String(path).includes('cursor=next-page')
        ? ({ items: [secondAlias] } as ImageAliasList)
        : ({ items: [alias], nextCursor: 'next-page' } as ImageAliasList),
    )
    renderPage()
    expect(await screen.findByText(alias.path)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '加载更多' }))
    expect(await screen.findByText(secondAlias.path)).toBeInTheDocument()
    expect(vi.mocked(apiRequest).mock.calls.some(([path]) => String(path).includes('cursor=next-page'))).toBe(true)
  })
})
