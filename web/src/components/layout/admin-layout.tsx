import { useTranslation } from 'react-i18next'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'

import { useAuth } from '../../features/auth/auth-context'

export function AdminLayout() {
  const { t } = useTranslation()
  const { session, logout } = useAuth()
  const navigate = useNavigate()

  async function signOut() {
    await logout()
    navigate('/admin/login', { replace: true })
  }

  return (
    <div className="min-h-screen bg-canvas text-ink">
      <header className="border-b border-line bg-panel">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-5 py-4">
          <div>
            <p className="text-lg font-semibold">ImageSilo</p>
            <p className="text-xs text-muted">{session?.email}</p>
          </div>
          <nav className="flex items-center gap-2" aria-label={t('nav.primary')}>
            <NavLink className="nav-link" to="/admin/upload">{t('nav.upload')}</NavLink>
            <NavLink className="nav-link" to="/admin/images">{t('nav.images')}</NavLink>
            <button className="button-secondary" type="button" onClick={() => void signOut()}>{t('auth.signOut')}</button>
          </nav>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-5 py-8">
        <Outlet />
      </main>
    </div>
  )
}
