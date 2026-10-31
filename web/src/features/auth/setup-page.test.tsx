import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, expect, it, vi } from 'vitest'
import i18n from '../../i18n/config'
import { SetupPage } from './setup-page'
vi.mock('./auth-context', () => ({ useAuth: () => ({ session: null, setupStatus: { initialized: false }, refresh: vi.fn(), refreshSetup: vi.fn() }) }))
afterEach(async () => { cleanup(); await i18n.changeLanguage('zh-CN'); delete document.documentElement.dataset.theme; localStorage.clear() })
it('keeps setup input while switching language and exposes the language menu state', async () => {
  await i18n.changeLanguage('zh-CN')
  render(<MemoryRouter><SetupPage /></MemoryRouter>)
  fireEvent.change(screen.getByLabelText('管理员邮箱'), { target: { value: 'setup@example.com' } })
  const language = screen.getByRole('button', { name: '中文' })
  expect(language).toHaveAttribute('aria-haspopup', 'menu')
  fireEvent.click(language)
  expect(language).toHaveAttribute('aria-expanded', 'true')
  await act(async () => fireEvent.click(screen.getByRole('menuitem', { name: 'English' })))
  expect(screen.getByDisplayValue('setup@example.com')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'EN' })).toHaveAttribute('aria-expanded', 'false')
})

it('announces setup validation errors', async () => {
  await i18n.changeLanguage('zh-CN')
  const { container } = render(<MemoryRouter><SetupPage /></MemoryRouter>)
  fireEvent.change(container.querySelector('input[name="password"]')!, { target: { value: 'password-one' } })
  fireEvent.change(container.querySelector('input[name="confirmPassword"]')!, { target: { value: 'password-two' } })
  fireEvent.submit(container.querySelector('form')!)
  expect(screen.getByRole('alert')).toHaveTextContent(i18n.t('setup.passwordMismatch'))
})
