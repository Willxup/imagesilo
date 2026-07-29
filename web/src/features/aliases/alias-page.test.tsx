import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { ImageAlias, ImageAliasList } from '../../lib/api-types'
import { AliasPage } from './alias-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))

const alias = {
  id: '019c1234-5678-7abc-8def-0123456789ab',
  path: '/i/2022/05/example.webp',
  imageId: '019c4321-8765-7cba-8fed-ba9876543210',
  source: 'manual',
  createdAt: '2026-07-29T00:00:00Z',
} as ImageAlias

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(<QueryClientProvider client={queryClient}><AliasPage /></QueryClientProvider>)
}

describe('AliasPage', () => {
  afterEach(cleanup)

  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
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
    expect(await screen.findByText(`目标图片：${alias.imageId}`)).toBeInTheDocument()
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
})
