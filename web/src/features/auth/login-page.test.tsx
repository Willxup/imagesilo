import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createMemoryRouter, MemoryRouter, RouterProvider } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { AuthProvider } from './auth-provider'
import { AuthGuard } from './auth-guard'
import { LoginPage } from './login-page'

describe('LoginPage', () => {
  afterEach(() => { cleanup(); vi.restoreAllMocks() })

  it('renders the administrator credentials form', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <MemoryRouter>
            <LoginPage />
          </MemoryRouter>
        </AuthProvider>
      </QueryClientProvider>,
    )
    expect(screen.getByRole('img', { name: 'ImageSilo' })).toBeInTheDocument()
    expect(await screen.findByLabelText('管理员邮箱')).toBeInTheDocument()
    expect(screen.getByLabelText('密码')).toBeInTheDocument()
    expect(screen.getByRole('contentinfo')).toHaveTextContent(/© 2026ImageSilo·License/)
    expect(screen.getByRole('link', { name: 'Version: dev' })).toHaveAttribute('href', 'https://github.com/Willxup/imagesilo/releases')
  })

  it.each(['/admin/images?q=legacy&visibility=private#selected-image', '/admin/settings#settings-security'])('restores %s after the auth provider rerenders on login', async (destination) => {
    mockAuthentication(false)
    const router = renderAuthentication(destination)
    fireEvent.change(await screen.findByLabelText('管理员邮箱'), { target: { value: 'admin@example.com' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'imagesilo-test-password' } })
    fireEvent.click(screen.getByRole('button', { name: '登录' }))

    await screen.findByRole('heading', { name: 'Restored destination' })
    await waitFor(() => expect(`${router.state.location.pathname}${router.state.location.search}${router.state.location.hash}`).toBe(destination))
  })

  it.each([
    ['/admin/images?q=legacy#selected-image', '/admin/images?q=legacy#selected-image'],
    ['https://example.com/admin/images', '/admin/upload'],
    ['//example.com/admin/images', '/admin/upload'],
    ['/admin/../../outside', '/admin/upload'],
    ['/admin/login', '/admin/upload'],
    ['/admin/setup', '/admin/upload'],
  ])('uses a safe local destination for an already authenticated visit: %s', async (from, destination) => {
    mockAuthentication(true)
    const router = renderAuthentication({ pathname: '/admin/login', state: { from } })

    await waitFor(() => expect(`${router.state.location.pathname}${router.state.location.search}${router.state.location.hash}`).toBe(destination))
  })
})

function mockAuthentication(initiallyAuthenticated: boolean) {
  let authenticated = initiallyAuthenticated
  const session = { adminId: 'admin', displayName: 'ImageSilo', email: 'admin@example.com', csrfToken: 'test', expiresAt: '2030-01-01T00:00:00Z' }
  vi.spyOn(globalThis, 'fetch').mockImplementation(async (input) => {
    const path = String(input)
    let status = 200
    let body: unknown = session
    if (path === '/api/v1/setup/status') body = { initialized: true }
    else if (path === '/api/v1/auth/login') authenticated = true
    else if (path === '/api/v1/auth/session' && !authenticated) { status = 401; body = { code: 'invalid_session' } }
    return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
  })
}

function renderAuthentication(initialEntry: string | { pathname: string; state: { from: string } }) {
  const router = createMemoryRouter([
    { path: '/admin/login', element: <LoginPage /> },
    { path: '/admin/images', element: <AuthGuard><h1>Restored destination</h1></AuthGuard> },
    { path: '/admin/settings', element: <AuthGuard><h1>Restored destination</h1></AuthGuard> },
    { path: '/admin/upload', element: <AuthGuard><h1>Upload destination</h1></AuthGuard> },
  ], { initialEntries: [initialEntry] })
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(<QueryClientProvider client={queryClient}><AuthProvider><RouterProvider router={router} /></AuthProvider></QueryClientProvider>)
  return router
}
