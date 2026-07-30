import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'

import { BrandLogo } from '../../components/brand-logo'
import { apiRequest } from '../../lib/api-client'
import type { AdminSession } from '../../lib/api-types'
import { useAuth } from './auth-context'

type LoginFields = {
  email: string
  password: string
}

export function LoginPage() {
  const { t } = useTranslation()
  const { session, refresh } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [error, setError] = useState('')
  const { register, handleSubmit, formState } = useForm<LoginFields>()

  if (session) {
    return <Navigate to="/admin/upload" replace />
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

  return (
    <main className="grid min-h-screen place-items-center bg-canvas px-6 text-ink">
      <form className="w-full max-w-md rounded-3xl border border-line bg-panel p-8 shadow-xl shadow-black/5" onSubmit={submit}>
        <BrandLogo imageClassName="h-12 w-auto" />
        <h1 className="mt-6 text-3xl font-semibold">{t('auth.signIn')}</h1>
        <label className="mt-8 block text-sm font-medium" htmlFor="email">{t('auth.email')}</label>
        <input id="email" type="email" autoComplete="username" required className="field" {...register('email')} />
        <label className="mt-5 block text-sm font-medium" htmlFor="password">{t('auth.password')}</label>
        <input id="password" type="password" autoComplete="current-password" required className="field" {...register('password')} />
        {error ? <p className="mt-4 rounded-xl bg-danger-soft px-4 py-3 text-sm text-danger">{error}</p> : null}
        <button type="submit" disabled={formState.isSubmitting} className="button-primary mt-6 w-full">
          {formState.isSubmitting ? t('common.working') : t('auth.signIn')}
        </button>
      </form>
    </main>
  )
}
