import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'

import { BrandLogo } from '../../components/brand-logo'
import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { DropdownItem, DropdownMenu } from '../../components/ui/dropdown-menu'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { apiRequest } from '../../lib/api-client'
import type { AdminSession } from '../../lib/api-types'
import { useAuth } from './auth-context'

type LoginFields = {
  email: string
  password: string
}

export function LoginPage() {
  const { t, i18n } = useTranslation()
  const { session, setupStatus, refresh } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [error, setError] = useState('')
  const [theme, setTheme] = useState<'light' | 'dark'>(() => document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light')
  const [languageOpen, setLanguageOpen] = useState(false)
  const { register, handleSubmit, formState } = useForm<LoginFields>()

  if (session) {
    return <Navigate to="/admin/upload" replace />
  }
  if (setupStatus && !setupStatus.initialized) {
    return <Navigate to="/admin/setup" replace />
  }

  const submit = handleSubmit(async (values) => {
    setError('')
    try {
      await apiRequest<AdminSession>('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(values),
      })
      await refresh()
      const destination = typeof location.state?.from === 'string' ? location.state.from : '/admin/upload'
      navigate(destination, { replace: true })
    } catch {
      setError(t('auth.invalidCredentials'))
    }
  })

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

  return (
    <main className="login-page text-ink">
      <div className="login-toolbar">
        <Button size="icon-sm" variant="outline" type="button" aria-label={theme === 'light' ? t('preferences.dark') : t('preferences.light')} onClick={toggleTheme}>
          <Icon name={theme === 'light' ? 'moon' : 'sun'} />
        </Button>
        <DropdownMenu open={languageOpen} onOpenChange={setLanguageOpen} trigger={<Button size="sm" variant="outline" type="button" onClick={() => setLanguageOpen((value) => !value)}><Icon name="languages" />{i18n.language === 'zh-CN' ? '中文' : 'EN'}<Icon name="chevronDown" /></Button>}>
          <DropdownItem active={i18n.language === 'en-US'} onClick={() => changeLanguage('en-US')}>English{i18n.language === 'en-US' ? <Icon name="check" className="ml-auto" /> : null}</DropdownItem>
          <DropdownItem active={i18n.language === 'zh-CN'} onClick={() => changeLanguage('zh-CN')}>简体中文{i18n.language === 'zh-CN' ? <Icon name="check" className="ml-auto" /> : null}</DropdownItem>
        </DropdownMenu>
      </div>

      <div className="login-shell">
        <section className="login-hero" aria-labelledby="login-hero-title">
          <div className="login-brand-row">
            <div className="login-signal"><img src="/brand/imagesilo-mark.png" alt="" /></div>
            <p className="page-kicker">{t('auth.heroEyebrow')}</p>
          </div>
          <div className="login-copy">
            <h1 className="login-hero-title" id="login-hero-title">
              {t('auth.heroTitle')}
              <span>{t('auth.heroAccent')}</span>
            </h1>
            <p className="login-hero-copy">{t('auth.heroDescription')}</p>
          </div>
          <div className="login-features" aria-label={t('auth.features')}>
            <Badge className="login-feature" variant="outline"><Icon name="zap" />{t('auth.lightweight')}</Badge>
            <Badge className="login-feature" variant="outline"><Icon name="shield" />{t('auth.selfHosted')}</Badge>
            <Badge className="login-feature" variant="outline"><Icon name="server" />{t('auth.directDelivery')}</Badge>
          </div>
        </section>

        <div className="login-form-wrap">
          <form className="login-form" onSubmit={submit}>
            <BrandLogo imageClassName="h-10 w-auto" />
            <h2 className="login-form-title">{t('auth.signIn')}</h2>
            <p className="login-form-copy">{t('auth.signInDescription')}</p>

            <label className="field-label" htmlFor="email">{t('auth.email')}</label>
            <div className="field-shell">
              <Icon name="mail" />
              <Input id="email" type="email" autoComplete="username" required {...register('email')} />
            </div>

            <label className="field-label" htmlFor="password">{t('auth.password')}</label>
            <div className="field-shell">
              <Icon name="lock" />
              <Input id="password" type="password" autoComplete="current-password" required {...register('password')} />
            </div>

            {error ? <p className="mt-4 rounded-xl border border-danger/20 bg-danger-soft px-4 py-3 text-sm text-danger">{error}</p> : null}
            <Button type="submit" disabled={formState.isSubmitting} className="mt-5 w-full bg-[var(--silo-gradient)] hover:opacity-90">
              {formState.isSubmitting ? t('common.working') : t('auth.signIn')}
              <Icon name="arrowRight" />
            </Button>
          </form>
        </div>
      </div>
    </main>
  )
}
