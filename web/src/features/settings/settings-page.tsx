import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { apiRequest } from '../../lib/api-client'
import type { AppSettings, Visibility } from '../../lib/api-types'

export function SettingsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [passwordChanged, setPasswordChanged] = useState(false)
  const settingsQuery = useQuery({
    queryKey: ['settings'],
    queryFn: () => apiRequest<AppSettings>('/api/v1/settings'),
  })
  const visibilityMutation = useMutation({
    mutationFn: (defaultVisibility: Visibility) => apiRequest<AppSettings>('/api/v1/settings/default-visibility', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ defaultVisibility }),
    }),
    onSuccess: (settings) => queryClient.setQueryData(['settings'], settings),
  })
  const passwordMutation = useMutation({
    mutationFn: () => apiRequest<void>('/api/v1/auth/password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ currentPassword, newPassword }),
    }),
    onSuccess: () => {
      setCurrentPassword('')
      setNewPassword('')
      setPasswordChanged(true)
    },
  })

  function changePassword(event: FormEvent) {
    event.preventDefault()
    setPasswordChanged(false)
    passwordMutation.mutate()
  }

  return (
    <section>
      <h1 className="page-title">{t('settings.title')}</h1>
      <p className="page-description">{t('settings.description')}</p>

      <div className="mt-8 rounded-2xl border border-line bg-panel p-6">
        <h2 className="text-xl font-semibold">{t('settings.defaultVisibility')}</h2>
        <p className="mt-2 text-sm text-muted">{t('settings.defaultVisibilityHelp')}</p>
        {settingsQuery.isLoading ? <p className="mt-4 text-muted">{t('common.loading')}</p> : null}
        {settingsQuery.isError ? <p className="mt-4 text-danger">{t('settings.loadFailed')}</p> : null}
        {settingsQuery.data ? (
          <select
            className="field max-w-sm"
            aria-label={t('settings.defaultVisibility')}
            value={settingsQuery.data.defaultVisibility}
            disabled={visibilityMutation.isPending}
            onChange={(event) => visibilityMutation.mutate(event.target.value as Visibility)}
          >
            <option value="public">{t('visibility.public')}</option>
            <option value="private">{t('visibility.private')}</option>
          </select>
        ) : null}
        {visibilityMutation.isError ? <p className="mt-4 text-danger">{t('settings.updateFailed')}</p> : null}
      </div>

      <form className="mt-6 rounded-2xl border border-line bg-panel p-6" onSubmit={(event) => void changePassword(event)}>
        <h2 className="text-xl font-semibold">{t('settings.changePassword')}</h2>
        <label className="mt-5 block font-medium" htmlFor="current-password">{t('settings.currentPassword')}</label>
        <input
          className="field"
          id="current-password"
          type="password"
          autoComplete="current-password"
          required
          value={currentPassword}
          onChange={(event) => setCurrentPassword(event.target.value)}
        />
        <label className="mt-5 block font-medium" htmlFor="new-password">{t('settings.newPassword')}</label>
        <input
          className="field"
          id="new-password"
          type="password"
          autoComplete="new-password"
          minLength={12}
          required
          value={newPassword}
          onChange={(event) => setNewPassword(event.target.value)}
        />
        <button className="button-primary mt-6" type="submit" disabled={passwordMutation.isPending}>
          {passwordMutation.isPending ? t('common.working') : t('settings.savePassword')}
        </button>
        {passwordChanged ? <p className="mt-4 text-accent">{t('settings.passwordChanged')}</p> : null}
        {passwordMutation.isError ? <p className="mt-4 text-danger">{t('settings.passwordFailed')}</p> : null}
      </form>
    </section>
  )
}
