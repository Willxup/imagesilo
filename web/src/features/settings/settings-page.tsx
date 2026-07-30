import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '../../components/ui/button'
import { Checkbox } from '../../components/ui/checkbox'
import { ComponentCard } from '../../components/ui/component-card'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { Select } from '../../components/ui/select'
import { apiRequest } from '../../lib/api-client'
import type { AdminSession, AppSettings, Visibility } from '../../lib/api-types'
import { useAuth } from '../auth/auth-context'

type FormErrors = Record<string, string>

export function SettingsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { session, refresh } = useAuth()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [profileErrors, setProfileErrors] = useState<FormErrors>({})
  const [passwordErrors, setPasswordErrors] = useState<FormErrors>({})
  const [processingErrors, setProcessingErrors] = useState<FormErrors>({})
  const settingsQuery = useQuery({ queryKey: ['settings'], queryFn: () => apiRequest<AppSettings>('/api/v1/settings') })
  const profileMutation = useMutation({
    mutationFn: (value: { displayName: string; email: string }) => apiRequest<AdminSession>('/api/v1/auth/profile', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value) }),
    onMutate: () => toast.loading(t('toast.profileSaving'), { id: 'settings-profile' }),
    onSuccess: async () => { await refresh(); toast.success(t('toast.profileSaved'), { id: 'settings-profile' }) },
    onError: () => toast.error(t('toast.profileFailed'), { id: 'settings-profile' }),
  })
  const visibilityMutation = useMutation({
    mutationFn: (defaultVisibility: Visibility) => apiRequest<AppSettings>('/api/v1/settings/default-visibility', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ defaultVisibility }) }),
    onMutate: () => toast.loading(t('toast.settingsSaving'), { id: 'settings-visibility' }),
    onSuccess: (settings) => { queryClient.setQueryData(['settings'], settings); toast.success(t('toast.settingsSaved'), { id: 'settings-visibility' }) },
    onError: () => toast.error(t('toast.settingsFailed'), { id: 'settings-visibility' }),
  })
  const passwordMutation = useMutation({
    mutationFn: () => apiRequest<void>('/api/v1/auth/password', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ currentPassword, newPassword }) }),
    onMutate: () => toast.loading(t('toast.passwordChanging'), { id: 'settings-password' }),
    onSuccess: () => { setCurrentPassword(''); setNewPassword(''); toast.success(t('toast.passwordChanged'), { id: 'settings-password' }) },
    onError: () => toast.error(t('settings.passwordFailed'), { id: 'settings-password' }),
  })
  const processingMutation = useMutation({
    mutationFn: (value: Omit<AppSettings, 'defaultVisibility'>) => apiRequest<AppSettings>('/api/v1/settings/processing', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value) }),
    onMutate: () => toast.loading(t('toast.settingsSaving'), { id: 'settings-processing' }),
    onSuccess: (settings) => { queryClient.setQueryData(['settings'], settings); toast.success(t('toast.settingsSaved'), { id: 'settings-processing' }) },
    onError: () => toast.error(t('settings.processingFailed'), { id: 'settings-processing' }),
  })

  function updateProfile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const displayName = String(form.get('displayName') ?? '').trim()
    const email = String(form.get('email') ?? '').trim()
    const errors: FormErrors = {}
    if (!displayName) errors.displayName = t('validation.required')
    if (!email) errors.email = t('validation.required')
    else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) errors.email = t('validation.email')
    setProfileErrors(errors)
    if (Object.keys(errors).length > 0) return
    profileMutation.mutate({ displayName, email })
  }

  function changePassword(event: FormEvent) {
    event.preventDefault()
    const errors: FormErrors = {}
    if (!currentPassword) errors.currentPassword = t('validation.required')
    if (!newPassword) errors.newPassword = t('validation.required')
    else if (newPassword.length < 12) errors.newPassword = t('validation.minLength', { count: 12 })
    setPasswordErrors(errors)
    if (Object.keys(errors).length > 0) return
    passwordMutation.mutate()
  }

  function updateProcessing(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const fields = {
      jpegQuality: numberValue(form, 'jpegQuality', 1, 100),
      webpQuality: numberValue(form, 'webpQuality', 1, 100),
      pngCompressionLevel: numberValue(form, 'pngCompressionLevel', 0, 9),
      conversionWebpQuality: numberValue(form, 'conversionWebpQuality', 1, 100),
    }
    const errors = Object.fromEntries(Object.entries(fields).filter(([, value]) => value === null).map(([name]) => [name, t('validation.numberRange')]))
    setProcessingErrors(errors)
    if (Object.keys(errors).length > 0) return
    processingMutation.mutate({
      compressionEnabled: form.has('compressionEnabled'), jpegQuality: fields.jpegQuality!, webpQuality: fields.webpQuality!,
      pngCompressionLevel: fields.pngCompressionLevel!, conversionEnabled: form.has('conversionEnabled'),
      conversionWebpQuality: fields.conversionWebpQuality!, conversionWebpLossless: form.has('conversionWebpLossless'),
    })
  }

  return (
    <section>
      <h1 className="page-title">{t('settings.title')}</h1>
      <p className="page-description">{t('settings.description')}</p>

      {session ? (
        <form key={`${session.displayName}:${session.email}`} noValidate onSubmit={updateProfile}>
          <ComponentCard className="mt-6" title={t('settings.profile')} description={t('settings.profileHelp')}>
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="field-group">{t('setup.displayName')}<Input name="displayName" maxLength={80} required aria-invalid={Boolean(profileErrors.displayName)} defaultValue={session.displayName} />{profileErrors.displayName ? <FieldError message={profileErrors.displayName} /> : null}</label>
              <label className="field-group">{t('auth.email')}<Input name="email" type="email" required aria-invalid={Boolean(profileErrors.email)} defaultValue={session.email} />{profileErrors.email ? <FieldError message={profileErrors.email} /> : null}</label>
            </div>
            <Button className="standard-action-button mt-5" type="submit" disabled={profileMutation.isPending}><Icon name={profileMutation.isPending ? 'loader' : 'user'} className={profileMutation.isPending ? 'animate-spin' : ''} />{t('settings.saveProfile')}</Button>
          </ComponentCard>
        </form>
      ) : null}

      <ComponentCard className="mt-6" title={t('settings.defaultVisibility')} description={t('settings.defaultVisibilityHelp')}>
        {settingsQuery.isLoading ? <p className="text-muted-foreground">{t('common.loading')}</p> : null}
        {settingsQuery.data ? <Select className="mt-3 max-w-sm" ariaLabel={t('settings.defaultVisibility')} value={settingsQuery.data.defaultVisibility} disabled={visibilityMutation.isPending} onValueChange={(value) => visibilityMutation.mutate(value as Visibility)} options={[{ value: 'public', label: t('visibility.public') }, { value: 'private', label: t('visibility.private') }]} /> : null}
      </ComponentCard>

      {settingsQuery.data ? (
        <form key={JSON.stringify(settingsQuery.data)} noValidate onSubmit={updateProcessing}>
          <ComponentCard className="mt-6" title={t('settings.processing')} description={t('settings.processingHelp')}>
            <label className="settings-check"><Checkbox name="compressionEnabled" defaultChecked={settingsQuery.data.compressionEnabled} /><span>{t('settings.compressionEnabled')}</span></label>
            <div className="mt-4 grid gap-4 sm:grid-cols-3">
              <NumberField name="jpegQuality" label={t('settings.jpegQuality')} min={1} max={100} value={settingsQuery.data.jpegQuality} error={processingErrors.jpegQuality} />
              <NumberField name="webpQuality" label={t('settings.webpQuality')} min={1} max={100} value={settingsQuery.data.webpQuality} error={processingErrors.webpQuality} />
              <NumberField name="pngCompressionLevel" label={t('settings.pngCompression')} min={0} max={9} value={settingsQuery.data.pngCompressionLevel} error={processingErrors.pngCompressionLevel} />
            </div>
            <label className="settings-check mt-6"><Checkbox name="conversionEnabled" defaultChecked={settingsQuery.data.conversionEnabled} /><span>{t('settings.conversionEnabled')}</span></label>
            <div className="mt-4 grid gap-4 sm:grid-cols-2">
              <NumberField name="conversionWebpQuality" label={t('settings.conversionWebpQuality')} min={1} max={100} value={settingsQuery.data.conversionWebpQuality} error={processingErrors.conversionWebpQuality} />
              <label className="settings-check self-end pb-2"><Checkbox name="conversionWebpLossless" defaultChecked={settingsQuery.data.conversionWebpLossless} /><span>{t('settings.conversionWebpLossless')}</span></label>
            </div>
            <Button className="standard-action-button mt-5" type="submit" disabled={processingMutation.isPending}><Icon name={processingMutation.isPending ? 'loader' : 'settings'} className={processingMutation.isPending ? 'animate-spin' : ''} />{processingMutation.isPending ? t('common.working') : t('settings.saveProcessing')}</Button>
          </ComponentCard>
        </form>
      ) : null}

      <form noValidate onSubmit={changePassword}>
        <ComponentCard className="mt-6" title={t('settings.changePassword')}>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="field-group">{t('settings.currentPassword')}<Input type="password" autoComplete="current-password" required aria-invalid={Boolean(passwordErrors.currentPassword)} value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} />{passwordErrors.currentPassword ? <FieldError message={passwordErrors.currentPassword} /> : null}</label>
            <label className="field-group">{t('settings.newPassword')}<Input type="password" autoComplete="new-password" minLength={12} required aria-invalid={Boolean(passwordErrors.newPassword)} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} />{passwordErrors.newPassword ? <FieldError message={passwordErrors.newPassword} /> : null}</label>
          </div>
          <Button className="standard-action-button mt-5" type="submit" disabled={passwordMutation.isPending}><Icon name={passwordMutation.isPending ? 'loader' : 'shield'} className={passwordMutation.isPending ? 'animate-spin' : ''} />{passwordMutation.isPending ? t('common.working') : t('settings.savePassword')}</Button>
        </ComponentCard>
      </form>
    </section>
  )
}

function NumberField({ name, label, min, max, value, error }: { name: string; label: string; min: number; max: number; value: number; error?: string }) {
  return <label className="field-group">{label}<Input name={name} type="number" min={min} max={max} required aria-invalid={Boolean(error)} defaultValue={value} />{error ? <FieldError message={error} /> : null}</label>
}

function FieldError({ message }: { message: string }) {
  return <span className="field-error" role="alert"><Icon name="activity" />{message}</span>
}

function numberValue(form: FormData, name: string, min: number, max: number) {
  const value = Number(form.get(name))
  return Number.isInteger(value) && value >= min && value <= max ? value : null
}
