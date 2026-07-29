import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { apiRequest } from '../../lib/api-client'
import type { ApiTokenList, ApiTokenScope, CreatedApiToken } from '../../lib/api-types'

const availableScopes: ApiTokenScope[] = [
  'images:upload',
  'images:read_private',
  'images:delete',
  'aliases:write',
]

export function ApiTokenPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState<ApiTokenScope[]>([])
  const [expiresAt, setExpiresAt] = useState('')
  const [created, setCreated] = useState<CreatedApiToken | null>(null)
  const query = useQuery({
    queryKey: ['api-tokens'],
    queryFn: () => apiRequest<ApiTokenList>('/api/v1/api-tokens'),
  })
  const createMutation = useMutation({
    mutationFn: () => apiRequest<CreatedApiToken>('/api/v1/api-tokens', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        scopes,
        expiresAt: expiresAt ? new Date(expiresAt).toISOString() : null,
      }),
    }),
    onSuccess: async (token) => {
      setCreated(token)
      setName('')
      setScopes([])
      setExpiresAt('')
      await queryClient.invalidateQueries({ queryKey: ['api-tokens'] })
    },
  })
  const revokeMutation = useMutation({
    mutationFn: (id: string) => apiRequest<void>(`/api/v1/api-tokens/${id}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['api-tokens'] }),
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    setCreated(null)
    createMutation.mutate()
  }

  function toggleScope(scope: ApiTokenScope) {
    setScopes((current) => current.includes(scope) ? current.filter((item) => item !== scope) : [...current, scope])
  }

  function revoke(id: string, tokenName: string) {
    if (window.confirm(t('apiTokens.confirmRevoke', { name: tokenName }))) revokeMutation.mutate(id)
  }

  return (
    <section>
      <h1 className="page-title">{t('apiTokens.title')}</h1>
      <p className="page-description">{t('apiTokens.description')}</p>

      <form className="mt-8 rounded-2xl border border-line bg-panel p-6" onSubmit={(event) => void submit(event)}>
        <label className="font-medium" htmlFor="token-name">{t('apiTokens.name')}</label>
        <input
          className="field"
          id="token-name"
          maxLength={100}
          required
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
        <fieldset className="mt-6">
          <legend className="font-medium">{t('apiTokens.scopes')}</legend>
          <div className="mt-3 grid gap-3 sm:grid-cols-2">
            {availableScopes.map((scope) => (
              <label className="flex items-start gap-3 rounded-xl border border-line p-3" key={scope}>
                <input
                  className="mt-1"
                  type="checkbox"
                  checked={scopes.includes(scope)}
                  onChange={() => toggleScope(scope)}
                />
                <span>
                  <span className="block font-mono text-sm">{scope}</span>
                  <span className="mt-1 block text-xs text-muted">{t(`apiTokens.scopeHelp.${scope}`)}</span>
                </span>
              </label>
            ))}
          </div>
        </fieldset>
        <label className="mt-6 block font-medium" htmlFor="token-expiry">{t('apiTokens.expiresAt')}</label>
        <input
          className="field"
          id="token-expiry"
          type="datetime-local"
          value={expiresAt}
          onChange={(event) => setExpiresAt(event.target.value)}
        />
        <button className="button-primary mt-6" type="submit" disabled={!name.trim() || scopes.length === 0 || createMutation.isPending}>
          {createMutation.isPending ? t('common.working') : t('apiTokens.create')}
        </button>
        {createMutation.isError ? <p className="mt-4 text-danger">{t('apiTokens.createFailed')}</p> : null}
      </form>

      {created ? (
        <div className="mt-6 rounded-2xl border border-accent bg-accent-soft p-5" role="status">
          <p className="font-semibold text-accent">{t('apiTokens.copyNow')}</p>
          <code className="mt-3 block break-all rounded-xl bg-panel p-4 text-sm text-ink">{created.token}</code>
          <button className="button-secondary mt-3" type="button" onClick={() => void navigator.clipboard.writeText(created.token)}>
            {t('apiTokens.copy')}
          </button>
        </div>
      ) : null}

      <h2 className="mt-10 text-xl font-semibold">{t('apiTokens.existing')}</h2>
      {query.isLoading ? <p className="mt-4 text-muted">{t('common.loading')}</p> : null}
      {query.isError ? <p className="mt-4 text-danger">{t('apiTokens.listFailed')}</p> : null}
      {query.data?.items.length === 0 ? <p className="mt-4 text-muted">{t('apiTokens.empty')}</p> : null}
      <div className="mt-4 grid gap-3">
        {query.data?.items.map((token) => (
          <article className="rounded-2xl border border-line bg-panel p-5" key={token.id}>
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p className="font-semibold">{token.name}</p>
                <p className="mt-1 font-mono text-sm text-muted">{token.tokenPrefix}…</p>
                <p className="mt-2 text-sm text-muted">{token.scopes.join(', ')}</p>
                <p className="mt-1 text-xs text-muted">
                  {token.expiresAt ? t('apiTokens.expires', { value: new Date(token.expiresAt).toLocaleString() }) : t('apiTokens.neverExpires')}
                </p>
              </div>
              <div className="flex items-center gap-3">
                <span className="rounded-full bg-accent-soft px-3 py-1 text-xs font-medium text-accent">{t(`apiTokens.status.${token.status}`)}</span>
                {token.status === 'active' ? (
                  <button className="button-secondary text-danger" type="button" disabled={revokeMutation.isPending} onClick={() => revoke(token.id, token.name)}>
                    {t('apiTokens.revoke')}
                  </button>
                ) : null}
              </div>
            </div>
          </article>
        ))}
      </div>
    </section>
  )
}
