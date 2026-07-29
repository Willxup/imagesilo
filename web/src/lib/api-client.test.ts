import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiRequest } from './api-client'

afterEach(() => {
  vi.restoreAllMocks()
  document.cookie = 'imagesilo_csrf=; Max-Age=0; Path=/'
})

describe('apiRequest', () => {
  it('adds the session CSRF token to unsafe requests', async () => {
    document.cookie = 'imagesilo_csrf=isc_test_token; Path=/'
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }))

    await apiRequest<void>('/api/v1/auth/logout', { method: 'POST' })

    const headers = new Headers(fetchMock.mock.calls[0][1]?.headers)
    expect(headers.get('X-CSRF-Token')).toBe('isc_test_token')
  })

  it('does not add a CSRF header to safe requests', async () => {
    document.cookie = 'imagesilo_csrf=isc_test_token; Path=/'
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }))

    await apiRequest<void>('/api/v1/auth/session')

    const headers = new Headers(fetchMock.mock.calls[0][1]?.headers)
    expect(headers.has('X-CSRF-Token')).toBe(false)
  })
})
