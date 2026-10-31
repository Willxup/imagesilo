import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import { SystemOverviewPanel } from './system-overview-panel'
vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))
afterEach(cleanup)
it('keeps cache rebuild available when overview fails and requires confirmation', async () => {
  vi.mocked(apiRequest).mockRejectedValue(new Error('overview unavailable'))
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}><SystemOverviewPanel /></QueryClientProvider>)
  await screen.findByText('系统概览加载失败。')
  fireEvent.click(screen.getByRole('button', { name: '重建缓存' }))
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByRole('button', { name: '重建缓存' })).toBeEnabled()
  expect(vi.mocked(apiRequest).mock.calls.every(([path]) => path === '/api/v1/overview')).toBe(true)
})
