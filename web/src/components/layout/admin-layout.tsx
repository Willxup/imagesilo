import { Suspense, useEffect, useMemo, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'

import { useAuth } from '../../features/auth/auth-context'
import { BrandLogo } from '../brand-logo'
import { Button } from '../ui/button'
import { DropdownItem, DropdownMenu } from '../ui/dropdown-menu'
import { Icon, type IconName } from '../ui/icon'
import { Input } from '../ui/input'
import { Modal } from '../ui/modal'

const repositoryURL = 'https://github.com/Willxup/imagesilo'

const workspaceNavigation: { icon: IconName; key: string; to: string }[] = [
  { icon: 'upload', key: 'upload', to: '/admin/upload' },
  { icon: 'images', key: 'images', to: '/admin/images' },
  { icon: 'history', key: 'aliases', to: '/admin/aliases' },
  { icon: 'key', key: 'apiTokens', to: '/admin/api-tokens' },
]

const systemNavigation: { icon: IconName; key: string; to: string }[] = [
  { icon: 'settings', key: 'settings', to: '/admin/settings' },
  { icon: 'activity', key: 'system', to: '/admin/system' },
]

export function AdminLayout() {
  const { t, i18n } = useTranslation()
  const { session, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [theme, setTheme] = useState<'light' | 'dark'>(() => document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light')
  const [mobileOpen, setMobileOpen] = useState(false)
  const [sidebarExpanded, setSidebarExpanded] = useState(true)
  const [sidebarHovered, setSidebarHovered] = useState(false)
  const [searchOpen, setSearchOpen] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [headerSearchTerm, setHeaderSearchTerm] = useState('')
  const [languageOpen, setLanguageOpen] = useState(false)
  const [profileOpen, setProfileOpen] = useState(false)
  const sidebarWide = sidebarExpanded || sidebarHovered
  const initials = useMemo(() => (session?.displayName || session?.email || 'IS').slice(0, 2).toUpperCase(), [session?.displayName, session?.email])

  useEffect(() => {
    function openSearch(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setSearchOpen(true)
      }
    }
    document.addEventListener('keydown', openSearch)
    return () => document.removeEventListener('keydown', openSearch)
  }, [])

  useEffect(() => {
    setMobileOpen(false)
    setProfileOpen(false)
    setLanguageOpen(false)
  }, [location.pathname])

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

  function changeLanguage(next: 'zh-CN' | 'en-US') {
    localStorage.setItem('imagesilo_language', next)
    void i18n.changeLanguage(next)
    setLanguageOpen(false)
  }

  function toggleSidebar() {
    if (window.matchMedia('(max-width: 1023px)').matches) setMobileOpen((current) => !current)
    else setSidebarExpanded((current) => !current)
  }

  function navigateToSearch(query: string) {
    navigate(query ? `/admin/images?q=${encodeURIComponent(query)}` : '/admin/images')
  }

  function searchImages(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const query = searchTerm.trim()
    navigateToSearch(query)
    setSearchOpen(false)
    setSearchTerm('')
  }

  function searchFromHeader(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    navigateToSearch(headerSearchTerm.trim())
  }

  return (
    <div className="tail-admin-shell" data-sidebar-wide={sidebarWide ? 'true' : 'false'}>
      <button aria-label={t('common.close')} className={`tail-sidebar-backdrop ${mobileOpen ? 'is-open' : ''}`} type="button" onClick={() => setMobileOpen(false)} />

      <aside
        className={`tail-sidebar ${mobileOpen ? 'is-mobile-open' : ''}`}
        data-wide={sidebarWide ? 'true' : 'false'}
        onMouseEnter={() => !sidebarExpanded && setSidebarHovered(true)}
        onMouseLeave={() => setSidebarHovered(false)}
      >
        <div className="tail-sidebar-brand">
          <a href={repositoryURL} target="_blank" rel="noreferrer" aria-label={t('nav.openRepository')}>
            {sidebarWide || mobileOpen ? <BrandLogo imageClassName="h-10 w-auto" /> : <img className="h-9 w-9 object-contain" src="/brand/imagesilo-mark.png" alt="ImageSilo" />}
          </a>
          <button className="tail-sidebar-mobile-close" type="button" aria-label={t('common.close')} onClick={() => setMobileOpen(false)}><Icon name="x" className="h-5 w-5" /></button>
        </div>
        <nav className="tail-sidebar-nav" aria-label={t('nav.primary')}>
          <SidebarGroup label={t('nav.workspace')} items={workspaceNavigation} wide={sidebarWide || mobileOpen} onNavigate={() => setMobileOpen(false)} />
          <SidebarGroup label={t('nav.systemGroup')} items={systemNavigation} wide={sidebarWide || mobileOpen} onNavigate={() => setMobileOpen(false)} />
        </nav>
      </aside>

      <div className="tail-main-frame">
        <header className="tail-header">
          <div className="tail-header-inner">
            <div className="tail-header-primary">
              <button className="tail-header-square" type="button" aria-label={t('common.menu')} onClick={toggleSidebar}><Icon name={mobileOpen ? 'x' : 'menu'} className="h-5 w-5" /></button>
              <a className="tail-mobile-logo" href={repositoryURL} target="_blank" rel="noreferrer" aria-label={t('nav.openRepository')}><BrandLogo imageClassName="h-8 w-auto" /></a>
              <form className="tail-header-search" role="search" onSubmit={searchFromHeader}>
                <button className="tail-header-search-submit" type="submit" aria-label={t('images.search')}><Icon name="search" className="tail-header-search-icon" /></button>
                <input
                  aria-label={t('nav.globalSearch')}
                  placeholder={t('images.searchPlaceholder')}
                  value={headerSearchTerm}
                  onChange={(event) => setHeaderSearchTerm(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key !== 'Enter') return
                    event.preventDefault()
                    navigateToSearch(event.currentTarget.value.trim())
                  }}
                />
                <kbd>⌘ K</kbd>
              </form>
            </div>

            <div className="tail-header-actions">
              <button className="tail-header-circle" type="button" aria-label={theme === 'light' ? t('preferences.dark') : t('preferences.light')} onClick={toggleTheme}><Icon name={theme === 'light' ? 'moon' : 'sun'} className="h-5 w-5" /></button>
              <DropdownMenu
                open={languageOpen}
                onOpenChange={setLanguageOpen}
                rootClassName="tail-language-menu"
                className="tail-language-dropdown"
                trigger={<button className="tail-language-trigger" type="button" aria-label={t('preferences.language')} onClick={() => setLanguageOpen((value) => !value)}><Icon name="languages" /><span>{i18n.language === 'zh-CN' ? '中文' : 'EN'}</span><Icon name="chevronDown" /></button>}
              >
                <DropdownItem active={i18n.language === 'en-US'} onClick={() => changeLanguage('en-US')}><span>English</span>{i18n.language === 'en-US' ? <Icon name="check" className="ml-auto" /> : null}</DropdownItem>
                <DropdownItem active={i18n.language === 'zh-CN'} onClick={() => changeLanguage('zh-CN')}><span>简体中文</span>{i18n.language === 'zh-CN' ? <Icon name="check" className="ml-auto" /> : null}</DropdownItem>
              </DropdownMenu>
              <DropdownMenu
                open={profileOpen}
                onOpenChange={setProfileOpen}
                rootClassName="tail-account-menu"
                className="tail-account-dropdown"
                trigger={(
                  <button className="tail-profile-trigger" type="button" aria-expanded={profileOpen} onClick={() => setProfileOpen((value) => !value)}>
                    <span className="tail-profile-avatar">{initials}</span>
                    <span className="tail-profile-text"><strong>{session?.displayName || 'ImageSilo'}</strong><small>{session?.email}</small></span>
                    <Icon name="chevronDown" className="h-4 w-4" />
                  </button>
                )}
              >
                <div className="tail-account-summary">
                  <strong>{session?.displayName || 'ImageSilo'}</strong>
                  <small>{session?.email}</small>
                </div>
                <div className="tail-account-divider" role="separator" />
                <DropdownItem onClick={() => { setProfileOpen(false); navigate('/admin/settings') }}><Icon name="settings" />{t('nav.settings')}</DropdownItem>
                <DropdownItem destructive onClick={() => void signOut()}><Icon name="logOut" />{t('auth.signOut')}</DropdownItem>
              </DropdownMenu>
            </div>
          </div>
        </header>

        <main className="tail-content">
          <Suspense fallback={<div className="tail-loading">{t('common.loading')}</div>}>
            <div className={`page-transition${location.pathname.startsWith('/admin/images/') ? ' is-detail' : ''}`} key={location.pathname}><Outlet /></div>
          </Suspense>
        </main>
      </div>

      <Modal open={searchOpen} onClose={() => setSearchOpen(false)} title={t('nav.searchTitle')} description={t('nav.searchDescription')} closeLabel={t('common.close')} size="md">
        <form className="command-search" onSubmit={searchImages}>
          <Icon name="search" />
          <Input autoFocus aria-label={t('nav.globalSearch')} placeholder={t('images.searchPlaceholder')} value={searchTerm} onChange={(event) => setSearchTerm(event.target.value)} />
          <Button type="submit"><Icon name="arrowRight" />{t('images.search')}</Button>
        </form>
        <p className="command-search-hint"><Icon name="command" />{t('nav.searchHint')}</p>
      </Modal>
    </div>
  )
}

function SidebarGroup({ label, items, wide, onNavigate }: {
  label: string
  items: { icon: IconName; key: string; to: string }[]
  wide: boolean
  onNavigate: () => void
}) {
  const { t } = useTranslation()
  return (
    <section className="tail-nav-group">
      <h2>{wide ? label : '•••'}</h2>
      <div className="tail-nav-list">
        {items.map((item) => (
          <NavLink className={({ isActive }) => `tail-nav-link${isActive ? ' active' : ''}`} key={item.to} to={item.to} onClick={onNavigate} title={!wide ? t(`nav.${item.key}`) : undefined}>
            <Icon name={item.icon} className="h-6 w-6" />
            {wide ? <span>{t(`nav.${item.key}`)}</span> : null}
          </NavLink>
        ))}
      </div>
    </section>
  )
}
