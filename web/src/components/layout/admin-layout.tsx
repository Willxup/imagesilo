import { Suspense, useEffect, useLayoutEffect, useMemo, useState, type CSSProperties, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { NavLink, Outlet, useLocation, useNavigate, useNavigationType } from 'react-router-dom'

import { preloadAdminPage } from '../../app/page-loaders'
import { useAuth } from '../../features/auth/auth-context'
import { writeLocalStorage } from '../../lib/browser-storage'
import { AppFooter } from '../app-footer'
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
  { icon: 'server', key: 'migrations', to: '/admin/migrations' },
  { icon: 'history', key: 'aliases', to: '/admin/aliases' },
]

const systemNavigation: { icon: IconName; key: string; to: string }[] = [
  { icon: 'key', key: 'apiTokens', to: '/admin/api-tokens' },
  { icon: 'settings', key: 'settings', to: '/admin/settings' },
  { icon: 'activity', key: 'system', to: '/admin/system' },
]

export function AdminLayout() {
  const { t, i18n } = useTranslation()
  const { session, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const navigationType = useNavigationType()
  const [theme, setTheme] = useState<'light' | 'dark'>(() => (document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'))
  const [mobileOpen, setMobileOpen] = useState(false)
  const [mobile, setMobile] = useState(() => window.matchMedia('(max-width: 1023px)').matches)
  const [searchOpen, setSearchOpen] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const [languageOpen, setLanguageOpen] = useState(false)
  const [profileOpen, setProfileOpen] = useState(false)
  const navigationItems = [...workspaceNavigation, ...systemNavigation]
  const currentIndex = navigationItems.findIndex((item) => location.pathname.startsWith(item.to))
  const currentItem = navigationItems[currentIndex]
  const dockIndex = mobile ? Math.min(Math.max(currentIndex, 0), 2) : Math.max(currentIndex, 0)
  const initials = useMemo(() => (session?.displayName || session?.email || 'IS').slice(0, 2).toUpperCase(), [session?.displayName, session?.email])

  useEffect(() => {
    const query = window.matchMedia('(max-width: 1023px)')
    const update = () => {
      setMobile(query.matches)
      if (!query.matches) setMobileOpen(false)
    }
    query.addEventListener('change', update)
    return () => query.removeEventListener('change', update)
  }, [])

  useEffect(() => {
    function openSearch(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setMobileOpen(false)
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

  // A fresh page starts at its heading; POP keeps the library's keyed restoration.
  useLayoutEffect(() => {
    if (navigationType !== 'POP') window.scrollTo({ top: 0, left: 0, behavior: 'instant' })
  }, [location.pathname, navigationType])

  async function signOut() {
    await logout()
    navigate('/admin/login', { replace: true })
  }

  function toggleTheme() {
    const next = theme === 'light' ? 'dark' : 'light'
    document.documentElement.dataset.theme = next
    writeLocalStorage('imagesilo_theme', next)
    setTheme(next)
  }

  function changeLanguage(next: 'zh-CN' | 'en-US') {
    writeLocalStorage('imagesilo_language', next)
    void i18n.changeLanguage(next)
    setLanguageOpen(false)
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

  return (
    <div className="tail-admin-shell">
      <div className="tail-main-frame">
        <header className="tail-header">
          <div className="tail-header-inner">
            <div className="tail-header-primary">
              <a className="tail-shell-logo" href={repositoryURL} target="_blank" rel="noreferrer" aria-label={t('nav.openRepository')}>
                <BrandLogo imageClassName="h-8 w-auto" />
              </a>
              <span className="tail-header-context">{currentItem ? t(`nav.${currentItem.key}`) : 'ImageSilo'}</span>
            </div>

            <div className="tail-header-actions">
              <button className="tail-search-trigger" type="button" aria-label={t('nav.globalSearch')} onClick={() => setSearchOpen(true)}>
                <Icon name="search" /><span>{t('images.search')}</span><kbd>⌘ K</kbd>
              </button>
              <button
                className="tail-header-circle"
                type="button"
                aria-label={theme === 'light' ? t('preferences.dark') : t('preferences.light')}
                onClick={toggleTheme}
              >
                <Icon name={theme === 'light' ? 'moon' : 'sun'} className="h-5 w-5" />
              </button>
              <DropdownMenu
                open={languageOpen}
                onOpenChange={setLanguageOpen}
                rootClassName="tail-language-menu"
                className="tail-language-dropdown"
                trigger={
                  <button
                    className="tail-language-trigger"
                    type="button"
                    aria-label={t('preferences.language')}
                    aria-haspopup="menu" aria-expanded={languageOpen} onClick={() => setLanguageOpen((value) => !value)}
                  >
                    <Icon name="languages" />
                    <span>{i18n.language === 'zh-CN' ? '中文' : 'EN'}</span>
                    <Icon name="chevronDown" />
                  </button>
                }
              >
                <DropdownItem active={i18n.language === 'en-US'} onClick={() => changeLanguage('en-US')}>
                  <span>English</span>
                  {i18n.language === 'en-US' ? <Icon name="check" className="ml-auto" /> : null}
                </DropdownItem>
                <DropdownItem active={i18n.language === 'zh-CN'} onClick={() => changeLanguage('zh-CN')}>
                  <span>简体中文</span>
                  {i18n.language === 'zh-CN' ? <Icon name="check" className="ml-auto" /> : null}
                </DropdownItem>
              </DropdownMenu>
              <DropdownMenu
                open={profileOpen}
                onOpenChange={setProfileOpen}
                rootClassName="tail-account-menu"
                className="tail-account-dropdown"
                trigger={
                  <button className="tail-profile-trigger" type="button" aria-label={t('nav.account')} aria-haspopup="menu" aria-expanded={profileOpen} onClick={() => setProfileOpen((value) => !value)}>
                    <span className="tail-profile-avatar">{initials}</span>
                    <span className="tail-profile-text">
                      <strong>{session?.displayName || 'ImageSilo'}</strong>
                      <small>{session?.email}</small>
                    </span>
                    <Icon name="chevronDown" className="h-4 w-4" />
                  </button>
                }
              >
                <div className="tail-account-summary">
                  <strong>{session?.displayName || 'ImageSilo'}</strong>
                  <small>{session?.email}</small>
                </div>
                <div className="tail-account-divider" role="separator" />
                <DropdownItem
                  onClick={() => {
                    setProfileOpen(false)
                    navigate('/admin/settings')
                  }}
                >
                  <Icon name="settings" />
                  {t('nav.settings')}
                </DropdownItem>
                <DropdownItem destructive onClick={() => void signOut()}>
                  <Icon name="logOut" />
                  {t('auth.signOut')}
                </DropdownItem>
              </DropdownMenu>
            </div>
          </div>
        </header>

        <main className="tail-content">
          <Suspense
            fallback={
              <div className="route-placeholder" role="status" aria-busy="true">
                <div className="route-placeholder-label"><Icon name="loader" className="animate-spin" />{location.pathname === '/admin/migrations' ? t('migrations.scanning') : t('common.loading')}</div>
                <div className="route-placeholder-content" aria-hidden="true"><div /><span /><span /><span /></div>
              </div>
            }
          >
            <div className={`page-transition${location.pathname.startsWith('/admin/images/') ? ' is-detail' : ''}`} data-navigation={navigationType} key={location.pathname}>
              <Outlet />
            </div>
          </Suspense>
        </main>
        <AppFooter />
      </div>

      <nav
        className="workspace-dock"
        aria-label={t('nav.primary')}
        style={{ '--dock-count': mobile ? 3 : navigationItems.length, '--dock-index': dockIndex } as CSSProperties}
      >
        <span className="workspace-dock-indicator" aria-hidden="true" style={{ opacity: currentIndex < 0 ? 0 : 1 }} />
        {(mobile ? navigationItems.slice(0, 2) : navigationItems).map((item) => (
          <NavigationLink key={item.to} item={item} className="workspace-dock-link" compactLabel={mobile} />
        ))}
        {mobile ? (
          <button
            type="button"
            className={`workspace-dock-link${currentIndex > 1 ? ' active' : ''}`}
            aria-label={`${currentIndex > 1 ? t(`nav.compact.${currentItem.key}`) : t('nav.more')}, ${t('common.menu')}`}
            aria-haspopup="dialog"
            aria-expanded={mobileOpen}
            aria-current={currentIndex > 1 ? 'page' : undefined}
            onClick={() => setMobileOpen(true)}
          >
            <Icon name={currentIndex > 1 ? currentItem.icon : 'menu'} />
            <span>{currentIndex > 1 ? t(`nav.compact.${currentItem.key}`) : t('nav.more')}</span>
            <Icon name="chevronDown" className="dock-more-chevron" />
          </button>
        ) : null}
      </nav>

      <Modal
        open={mobileOpen}
        onClose={() => setMobileOpen(false)}
        title={t('nav.allPages')}
        closeLabel={t('common.close')}
        rootClassName="navigation-sheet"
        size="md"
      >
        <nav className="navigation-sheet-grid" aria-label={t('nav.primary')}>
          {navigationItems.map((item) => (
            <NavigationLink key={item.to} item={item} className="navigation-sheet-link" onNavigate={() => setMobileOpen(false)} />
          ))}
        </nav>
      </Modal>

      <Modal
        open={searchOpen}
        onClose={() => setSearchOpen(false)}
        title={t('nav.searchTitle')}
        description={t('nav.searchDescription')}
        closeLabel={t('common.close')}
        size="md"
      >
        <form className="command-search" onSubmit={searchImages}>
          <Icon name="search" />
          <Input
            autoFocus
            aria-label={t('nav.globalSearch')}
            placeholder={t('images.searchPlaceholder')}
            value={searchTerm}
            onChange={(event) => setSearchTerm(event.target.value)}
          />
          <Button type="submit">
            <Icon name="arrowRight" />
            {t('images.search')}
          </Button>
        </form>
        <p className="command-search-hint">
          <Icon name="command" />
          {t('nav.searchHint')}
        </p>
      </Modal>
    </div>
  )
}

function NavigationLink({ item, className, onNavigate, compactLabel = false }: {
  item: { icon: IconName; key: string; to: string }
  className: string
  onNavigate?: () => void
  compactLabel?: boolean
}) {
  const { t } = useTranslation()
  return (
    <NavLink
      className={({ isActive }) => `${className}${isActive ? ' active' : ''}`}
      to={item.to}
      onPointerEnter={() => preloadAdminPage(item.to)}
      onFocus={() => preloadAdminPage(item.to)}
      onPointerDown={() => preloadAdminPage(item.to)}
      onClick={onNavigate}
      aria-label={t(`nav.${item.key}`)}
    >
      <Icon name={item.icon} />
      <span>{t(`nav.${compactLabel ? 'compact.' : ''}${item.key}`)}</span>
    </NavLink>
  )
}
