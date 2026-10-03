import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, expect, it, vi } from 'vitest'
import '../../i18n/config'
import { AdminLayout } from './admin-layout'
vi.mock('../../features/auth/auth-context', () => ({ useAuth: () => ({ session: { displayName: 'Test', email: 'test@example.com' }, logout: vi.fn() }) }))
afterEach(() => { cleanup(); vi.unstubAllGlobals() })
it('names the compact current destination and exposes menu state', () => {
  vi.stubGlobal('matchMedia', vi.fn((media: string) => ({ media, matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })))
  render(<MemoryRouter initialEntries={['/admin/system']}><AdminLayout /></MemoryRouter>)
  const menu = screen.getByRole('button', { name: '系统, 打开菜单' })
  expect(menu).toHaveAttribute('aria-expanded', 'false')
  fireEvent.click(menu)
  expect(menu).toHaveAttribute('aria-expanded', 'true')
  expect(screen.getByRole('dialog', { name: '页面导航' })).toBeInTheDocument()
})
