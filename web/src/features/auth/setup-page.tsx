import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useNavigate } from 'react-router-dom'

import { BrandLogo } from '../../components/brand-logo'
import { AppFooter } from '../../components/app-footer'
import { Button } from '../../components/ui/button'
import { Checkbox } from '../../components/ui/checkbox'
import { DropdownItem, DropdownMenu } from '../../components/ui/dropdown-menu'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { Select } from '../../components/ui/select'
import { apiRequest } from '../../lib/api-client'
import type { AdminSession, SetupRequest, Visibility } from '../../lib/api-types'
import { writeLocalStorage } from '../../lib/browser-storage'
import { useAuth } from './auth-context'

export function SetupPage() {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const { session, setupStatus, refresh, refreshSetup } = useAuth()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [visibility, setVisibility] = useState<Visibility>('public')
  const [compressionEnabled, setCompressionEnabled] = useState(false)
  const [conversionEnabled, setConversionEnabled] = useState(false)
  const [theme, setTheme] = useState<'light' | 'dark'>(() => (document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light'))
  const [languageOpen, setLanguageOpen] = useState(false)

  if (session) return <Navigate to="/admin/upload" replace />
  if (setupStatus?.initialized) return <Navigate to="/admin/login" replace />

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    if (form.get('password') !== form.get('confirmPassword')) {
      setError(t('setup.passwordMismatch'))
      return
    }
    setPending(true)
    setError('')
    const request: SetupRequest = {
      bootstrapToken: String(form.get('bootstrapToken') ?? ''),
      displayName: String(form.get('displayName') ?? ''),
      email: String(form.get('email') ?? ''),
      password: String(form.get('password') ?? ''),
      defaultVisibility: visibility,
      compressionEnabled,
      jpegQuality: 85,
      webpQuality: 82,
      pngCompressionLevel: 6,
      conversionEnabled,
      conversionWebpQuality: 82,
      conversionWebpLossless: false,
    }
    try {
      await apiRequest<AdminSession>('/api/v1/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      })
      await Promise.all([refresh(), refreshSetup()])
      navigate('/admin/upload', { replace: true })
    } catch {
      setError(t('setup.failed'))
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="setup-page login-page">
      <div className="login-layout setup-layout">
      <section className="login-brand-panel">
        <BrandLogo imageClassName="h-11 w-auto" />
        <div className="login-story"><p className="page-kicker">{t('auth.storyEyebrow')}</p><h2>{t('auth.storyTitleFirst')}<br /><span>{t('auth.storyTitleSecond')}</span></h2><p>{t('auth.storyDescription')}</p></div>
        <div className="login-art" aria-hidden="true"><div className="login-plane plane-green" /><div className="login-plane plane-cyan" /><div className="login-plane plane-blue"><div className="plane-window" /></div></div>
        <p className="login-brand-footer">{t('auth.brandFooter')}</p>
      </section>
      <div className="login-panel">
        <div className="login-toolbar">
          <Button size="icon-sm" variant="outline" type="button" aria-label={theme === 'light' ? t('preferences.dark') : t('preferences.light')} onClick={() => { const next = theme === 'light' ? 'dark' : 'light'; document.documentElement.dataset.theme = next; writeLocalStorage('imagesilo_theme', next); setTheme(next) }}><Icon name={theme === 'light' ? 'moon' : 'sun'} /></Button>
          <DropdownMenu open={languageOpen} onOpenChange={setLanguageOpen} trigger={<Button size="sm" variant="outline" type="button" aria-haspopup="menu" aria-expanded={languageOpen} onClick={() => setLanguageOpen((value) => !value)}><Icon name="languages" />{i18n.language === 'zh-CN' ? '中文' : 'EN'}<Icon name="chevronDown" /></Button>}>
            <DropdownItem active={i18n.language === 'en-US'} onClick={() => { writeLocalStorage('imagesilo_language', 'en-US'); void i18n.changeLanguage('en-US'); setLanguageOpen(false) }}>English</DropdownItem>
            <DropdownItem active={i18n.language === 'zh-CN'} onClick={() => { writeLocalStorage('imagesilo_language', 'zh-CN'); void i18n.changeLanguage('zh-CN'); setLanguageOpen(false) }}>简体中文</DropdownItem>
          </DropdownMenu>
        </div>
      <main className="setup-card">
        <div className="setup-heading">
          <span className="setup-icon">
            <Icon name="wand" />
          </span>
          <div>
            <h1>{t('setup.title')}</h1>
            <p>{t('setup.description')}</p>
          </div>
        </div>
        <form className="setup-form" onSubmit={submit}>
          <label className="field-group">
            {t('setup.bootstrapToken')}
            <Input name="bootstrapToken" autoComplete="off" spellCheck={false} maxLength={47} pattern="isb_[A-Za-z0-9_-]{43}" required />
          </label>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="field-group">
              {t('setup.displayName')}
              <Input name="displayName" autoComplete="name" maxLength={80} required defaultValue="ImageSilo" />
            </label>
            <label className="field-group">
              {t('auth.email')}
              <Input name="email" type="email" autoComplete="username" maxLength={254} required />
            </label>
            <label className="field-group">
              {t('auth.password')}
              <Input name="password" type="password" autoComplete="new-password" minLength={12} maxLength={1024} required />
            </label>
            <label className="field-group">
              {t('setup.confirmPassword')}
              <Input name="confirmPassword" type="password" autoComplete="new-password" minLength={12} maxLength={1024} required />
            </label>
          </div>
          <div className="setup-options">
            <label className="field-group">
              {t('settings.defaultVisibility')}
              <Select
                ariaLabel={t('settings.defaultVisibility')}
                value={visibility}
                onValueChange={(value) => setVisibility(value as Visibility)}
                options={[
                  { value: 'public', label: t('visibility.public') },
                  { value: 'private', label: t('visibility.private') },
                ]}
              />
            </label>
            <label className="setup-check">
              <Checkbox checked={compressionEnabled} onChange={(event) => setCompressionEnabled(event.target.checked)} />
              <span>
                <strong>{t('settings.compressionEnabled')}</strong>
                <small>{t('setup.compressionHelp')}</small>
              </span>
            </label>
            <label className="setup-check">
              <Checkbox checked={conversionEnabled} onChange={(event) => setConversionEnabled(event.target.checked)} />
              <span>
                <strong>{t('settings.conversionEnabled')}</strong>
                <small>{t('setup.conversionHelp')}</small>
              </span>
            </label>
          </div>
          {error ? <p className="setup-error" role="alert">{error}</p> : null}
          <Button className="w-full" variant="brandSolid" type="submit" disabled={pending}>
            <Icon name={pending ? 'loader' : 'arrowRight'} className={pending ? 'animate-spin' : ''} />
            {pending ? t('common.working') : t('setup.finish')}
          </Button>
        </form>
      </main>
      <AppFooter />
      </div>
      </div>
    </div>
  )
}
