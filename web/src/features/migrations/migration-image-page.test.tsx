import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { MigrationImage, MigrationImageBatchResult, MigrationImageList } from '../../lib/api-types'
import { MigrationImagePage } from './migration-image-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))

const image = {
  path: '/i/2026/08/%E6%97%A7%E5%9B%BE.jpg',
  originalName: '旧图.jpg',
  mimeType: 'image/jpeg',
  extension: '.jpg',
  storedSize: 2048,
  standardUrl: '/i/2026/08/%E6%97%A7%E5%9B%BE.jpg',
  modifiedAt: '2026-08-03T00:00:00Z',
} as MigrationImage

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <MigrationImagePage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('MigrationImagePage', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
  })

  it('shows an explicit scanning state while the initial snapshot is loading', () => {
    vi.mocked(apiRequest).mockReturnValue(new Promise(() => undefined))
    renderPage()

    expect(screen.getByRole('status')).toHaveTextContent('正在扫描迁移文件…')
  })

  it('shows the flat public path in read-only mode and applies filters', async () => {
    vi.mocked(apiRequest).mockResolvedValue({ items: [image], skippedFiles: 1, mutationsEnabled: false } as MigrationImageList)
    renderPage()

    const preview = await screen.findByRole('img', { name: '旧图.jpg' })
    expect(preview).toHaveAttribute('src', image.standardUrl)
    expect(screen.getByText('/i/2026/08/旧图.jpg')).toBeInTheDocument()
    expect(screen.getByText('迁移目录当前为只读模式')).toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: '打开迁移图片 /i/2026/08/旧图.jpg' })).toHaveAttribute('href', image.standardUrl)

    fireEvent.change(screen.getByLabelText('搜索'), { target: { value: '2026/08' } })
    fireEvent.click(screen.getByRole('button', { name: '应用筛选' }))
    await waitFor(() => {
      expect(vi.mocked(apiRequest).mock.calls.some(([path]) => String(path).includes('q=2026%2F08'))).toBe(true)
    })
  })

  it('selects and permanently deletes migration paths when enabled', async () => {
    vi.mocked(apiRequest).mockImplementation(async (path) => {
      if (String(path).includes('batch-delete')) {
        return { items: [{ path: image.path, status: 'deleted', removedDirectories: 2 }] } as MigrationImageBatchResult
      }
      return { items: [image], skippedFiles: 0, mutationsEnabled: true } as MigrationImageList
    })
    renderPage()

    const checkbox = await screen.findByRole('checkbox', { name: '选择迁移图片 /i/2026/08/旧图.jpg' })
    fireEvent.click(checkbox)
    fireEvent.click(screen.getByRole('button', { name: /^删除$/ }))
    expect(screen.getByText(/确定永久删除“\/i\/2026\/08\/旧图.jpg”/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '永久删除' }))

    await waitFor(() => {
      const deletion = vi.mocked(apiRequest).mock.calls.find(([path]) => String(path).includes('batch-delete'))
      expect(deletion?.[1]?.method).toBe('POST')
      expect(JSON.parse(String(deletion?.[1]?.body))).toEqual({ paths: [image.path] })
    })
  })

  it('manually refreshes the migration snapshot', async () => {
    vi.mocked(apiRequest).mockImplementation(async (path) => {
      if (String(path).endsWith('/refresh')) return undefined
      return { items: [image], skippedFiles: 0, mutationsEnabled: false } as MigrationImageList
    })
    renderPage()

    await screen.findByRole('img', { name: '旧图.jpg' })
    fireEvent.click(screen.getByRole('button', { name: '刷新' }))

    await waitFor(() => {
      const refresh = vi.mocked(apiRequest).mock.calls.find(([path]) => String(path).endsWith('/refresh'))
      expect(refresh?.[1]?.method).toBe('POST')
    })
  })
})
