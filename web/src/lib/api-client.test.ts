import { afterEach, describe, expect, it, vi } from 'vitest'

import { ApiError, apiRequest, subscribeUnauthorized } from './api-client'

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

  it('notifies the auth boundary for global 401 responses', async () => {
    const unauthorized = vi.fn()
    const unsubscribe = subscribeUnauthorized(unauthorized)
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ code: 'invalid_session', message: 'Expired', requestId: 'test' }), {
        status: 401,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    await expect(apiRequest('/api/v1/images')).rejects.toBeInstanceOf(ApiError)
    expect(unauthorized).toHaveBeenCalledOnce()
    unsubscribe()
  })
})
