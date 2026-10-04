import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import i18n from '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import { ApiTokenPage } from './api-token-page'
vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))
afterEach(async () => { cleanup(); await i18n.changeLanguage('zh-CN') })
it('submits selected scopes and an optional date through the translated form', async () => {
  await i18n.changeLanguage('en-US')
  vi.mocked(apiRequest).mockImplementation(async (_path, options) => options?.method === 'POST' ? { token: 'test-only-placeholder' } : { items: [] })
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={client}><ApiTokenPage /></QueryClientProvider>)
  fireEvent.change(screen.getByRole('textbox'), { target: { value: 'Test integration' } })
  fireEvent.click(screen.getByRole('checkbox', { name: /images:upload/ }))
  const date = document.querySelector<HTMLButtonElement>('.ui-date-trigger')!
  fireEvent.click(date)
  fireEvent.click(screen.getByRole('button', { name: 'Today' }))
  expect(date).toHaveFocus()
  fireEvent.submit(document.querySelector('form')!)
  await waitFor(() => expect(vi.mocked(apiRequest).mock.calls.some(([, options]) => options?.method === 'POST')).toBe(true))
  const options = vi.mocked(apiRequest).mock.calls.find(([, options]) => options?.method === 'POST')![1]!
  const body = JSON.parse(options.body as string)
  expect(body.name).toBe('Test integration')
  expect(body.scopes).toEqual(['images:upload'])
  expect(Number.isNaN(Date.parse(body.expiresAt))).toBe(false)
})
