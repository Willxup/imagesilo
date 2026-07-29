import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'

import { useAuth } from '../../features/auth/auth-context'

export function AdminLayout() {
  const { t, i18n } = useTranslation()
  const { session, logout } = useAuth()
  const navigate = useNavigate()
  const [theme, setTheme] = useState<'light' | 'dark'>(() => document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light')

  async function signOut() {
    await logout()
    navigate('/admin/login', { replace: true })
  }

  function toggleTheme() {
    const next = theme === 'light' ? 'dark' : 'light'
    document.documentElement.dataset.theme = next
    localStorage.setItem('imagesilo_theme', next)
    setTheme(next)
  }

  function toggleLanguage() {
    const next = i18n.language === 'zh-CN' ? 'en-US' : 'zh-CN'
    localStorage.setItem('imagesilo_language', next)
    void i18n.changeLanguage(next)
  }

  return (
    <div className="min-h-screen bg-canvas text-ink">
      <header className="border-b border-line bg-panel">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-5 py-4">
          <div>
            <p className="text-lg font-semibold">ImageSilo</p>
            <p className="text-xs text-muted">{session?.email}</p>
          </div>
          <nav className="flex max-w-full flex-wrap items-center gap-2" aria-label={t('nav.primary')}>
            <NavLink className="nav-link" to="/admin/upload">{t('nav.upload')}</NavLink>
            <NavLink className="nav-link" to="/admin/images">{t('nav.images')}</NavLink>
            <NavLink className="nav-link" to="/admin/aliases">{t('nav.aliases')}</NavLink>
            <NavLink className="nav-link" to="/admin/api-tokens">{t('nav.apiTokens')}</NavLink>
            <NavLink className="nav-link" to="/admin/settings">{t('nav.settings')}</NavLink>
            <NavLink className="nav-link" to="/admin/system">{t('nav.system')}</NavLink>
            <button className="button-secondary" type="button" onClick={toggleTheme}>{theme === 'light' ? t('preferences.dark') : t('preferences.light')}</button>
            <button className="button-secondary" type="button" onClick={toggleLanguage}>{i18n.language === 'zh-CN' ? 'EN' : '中文'}</button>
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
