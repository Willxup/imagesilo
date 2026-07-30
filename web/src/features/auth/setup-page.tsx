import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useNavigate } from 'react-router-dom'

import { BrandLogo } from '../../components/brand-logo'
import { Button } from '../../components/ui/button'
import { Checkbox } from '../../components/ui/checkbox'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { Select } from '../../components/ui/select'
import { apiRequest } from '../../lib/api-client'
import type { AdminSession, SetupRequest, Visibility } from '../../lib/api-types'
import { useAuth } from './auth-context'

export function SetupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { session, setupStatus, refresh, refreshSetup } = useAuth()
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [visibility, setVisibility] = useState<Visibility>('public')
  const [compressionEnabled, setCompressionEnabled] = useState(false)
  const [conversionEnabled, setConversionEnabled] = useState(false)

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
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request),
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
    <main className="setup-page">
      <section className="setup-card">
        <div className="setup-brand"><BrandLogo imageClassName="h-10 w-auto" /></div>
        <div className="setup-heading">
          <span className="setup-icon"><Icon name="wand" /></span>
          <div><h1>{t('setup.title')}</h1><p>{t('setup.description')}</p></div>
        </div>
        <form className="setup-form" onSubmit={submit}>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="field-group">{t('setup.displayName')}<Input name="displayName" autoComplete="name" maxLength={80} required defaultValue="ImageSilo" /></label>
            <label className="field-group">{t('auth.email')}<Input name="email" type="email" autoComplete="username" required /></label>
            <label className="field-group">{t('auth.password')}<Input name="password" type="password" autoComplete="new-password" minLength={12} required /></label>
            <label className="field-group">{t('setup.confirmPassword')}<Input name="confirmPassword" type="password" autoComplete="new-password" minLength={12} required /></label>
          </div>
          <div className="setup-options">
            <label className="field-group">{t('settings.defaultVisibility')}
              <Select ariaLabel={t('settings.defaultVisibility')} value={visibility} onValueChange={(value) => setVisibility(value as Visibility)} options={[
                { value: 'public', label: t('visibility.public') }, { value: 'private', label: t('visibility.private') },
              ]} />
            </label>
            <label className="setup-check"><Checkbox checked={compressionEnabled} onChange={(event) => setCompressionEnabled(event.target.checked)} />
              <span><strong>{t('settings.compressionEnabled')}</strong><small>{t('setup.compressionHelp')}</small></span>
            </label>
            <label className="setup-check"><Checkbox checked={conversionEnabled} onChange={(event) => setConversionEnabled(event.target.checked)} />
              <span><strong>{t('settings.conversionEnabled')}</strong><small>{t('setup.conversionHelp')}</small></span>
            </label>
          </div>
          {error ? <p className="setup-error">{error}</p> : null}
          <Button className="w-full" type="submit" disabled={pending}>
            <Icon name={pending ? 'loader' : 'arrowRight'} className={pending ? 'animate-spin' : ''} />
            {pending ? t('common.working') : t('setup.finish')}
          </Button>
        </form>
      </section>
    </main>
  )
}
