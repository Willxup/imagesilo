import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { ComponentCard } from '../../components/ui/component-card'
import { Checkbox } from '../../components/ui/checkbox'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { DatePicker } from '../../components/ui/date-picker'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { Select } from '../../components/ui/select'
import { apiRequest } from '../../lib/api-client'
import { copyText } from '../../lib/image-links'
import type { ApiTokenList, ApiTokenScope, CreatedApiToken } from '../../lib/api-types'

const availableScopes: ApiTokenScope[] = [
  'images:upload',
  'images:read_private',
  'images:delete',
  'aliases:write',
]

export function ApiTokenPage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState<ApiTokenScope[]>([])
  const [expiresDate, setExpiresDate] = useState('')
  const [expiresHour, setExpiresHour] = useState('23')
  const [expiresMinute, setExpiresMinute] = useState('59')
  const [created, setCreated] = useState<CreatedApiToken | null>(null)
  const [revokeTarget, setRevokeTarget] = useState<{ id: string; name: string } | null>(null)
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
        expiresAt: expiresDate ? expirationISO(expiresDate, expiresHour, expiresMinute) : null,
      }),
    }),
    onSuccess: async (token) => {
      setCreated(token)
      setName('')
      setScopes([])
      setExpiresDate('')
      setExpiresHour('23')
      setExpiresMinute('59')
      await queryClient.invalidateQueries({ queryKey: ['api-tokens'] })
	  toast.success(t('toast.tokenCreated'))
    },
	onError: () => toast.error(t('apiTokens.createFailed')),
  })
  const revokeMutation = useMutation({
    mutationFn: (id: string) => apiRequest<void>(`/api/v1/api-tokens/${id}`, { method: 'DELETE' }),
    onSuccess: () => { setRevokeTarget(null); toast.success(t('toast.tokenRevoked')); return queryClient.invalidateQueries({ queryKey: ['api-tokens'] }) },
	onError: () => toast.error(t('toast.operationFailed')),
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    setCreated(null)
    createMutation.mutate()
  }

  function toggleScope(scope: ApiTokenScope) {
    setScopes((current) => current.includes(scope) ? current.filter((item) => item !== scope) : [...current, scope])
  }

  return (
    <section>
      <h1 className="page-title">{t('apiTokens.title')}</h1>
      <p className="page-description">{t('apiTokens.description')}</p>

      <form onSubmit={(event) => void submit(event)}>
        <ComponentCard className="mt-8" title={t('apiTokens.create')}>
        <label className="font-medium" htmlFor="token-name">{t('apiTokens.name')}</label>
        <Input
          className="mt-1.5"
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
                <Checkbox
                  className="mt-1"
                  checked={scopes.includes(scope)}
                  onChange={() => toggleScope(scope)}
                />
                <span>
                  <span className="block font-mono text-sm">{scope}</span>
                  <span className="mt-1 block text-xs text-muted-foreground">{t(`apiTokens.scopeHelp.${scope}`)}</span>
                </span>
              </label>
            ))}
          </div>
        </fieldset>
        <div className="mt-6 font-medium">
          <span>{t('apiTokens.expiresAt')}</span>
          <div className="token-expiry-controls mt-1.5">
            <DatePicker
              id="token-expiry"
              value={expiresDate}
              onChange={setExpiresDate}
              min={todayValue()}
              locale={i18n.resolvedLanguage ?? i18n.language}
              ariaLabel={t('apiTokens.expiresAt')}
              placeholder={t('common.datePicker.select')}
              clearLabel={t('common.clear')}
              todayLabel={t('common.datePicker.today')}
              previousMonthLabel={t('common.datePicker.previousMonth')}
              nextMonthLabel={t('common.datePicker.nextMonth')}
            />
            <Select
              ariaLabel={t('common.datePicker.hour')}
              value={expiresHour}
              disabled={!expiresDate}
              onValueChange={setExpiresHour}
              options={Array.from({ length: 24 }, (_, hour) => ({ value: String(hour).padStart(2, '0'), label: t('common.datePicker.hourValue', { value: String(hour).padStart(2, '0') }) }))}
            />
            <Select
              ariaLabel={t('common.datePicker.minute')}
              value={expiresMinute}
              disabled={!expiresDate}
              onValueChange={setExpiresMinute}
              options={['00', '15', '30', '45', '59'].map((minute) => ({ value: minute, label: t('common.datePicker.minuteValue', { value: minute }) }))}
            />
          </div>
        </div>
        <Button className="standard-action-button mt-5" type="submit" disabled={!name.trim() || scopes.length === 0 || createMutation.isPending}>
          <Icon name="key" />{createMutation.isPending ? t('common.working') : t('apiTokens.create')}
        </Button>
        {createMutation.isError ? <p className="mt-4 text-danger">{t('apiTokens.createFailed')}</p> : null}
        </ComponentCard>
      </form>

      {created ? (
        <div className="mt-6 rounded-2xl border border-primary/25 bg-accent-soft p-5" role="status">
          <p className="font-semibold text-primary">{t('apiTokens.copyNow')}</p>
          <code className="mt-3 block break-all rounded-xl bg-panel p-4 text-sm text-ink">{created.token}</code>
          <Button className="mt-3" size="sm" variant="outline" type="button" onClick={() => void copyText(created.token).then(() => toast.success(t('toast.tokenCopied'))).catch(() => toast.error(t('toast.copyFailed')))}>
            <Icon name="copy" />{t('apiTokens.copy')}
          </Button>
        </div>
      ) : null}

      <ComponentCard className="mt-6" title={t('apiTokens.existing')}>
      {query.isLoading ? <p className="text-muted-foreground">{t('common.loading')}</p> : null}
      {query.isError ? <p className="text-danger">{t('apiTokens.listFailed')}</p> : null}
      {query.data?.items.length === 0 ? <p className="text-muted-foreground">{t('apiTokens.empty')}</p> : null}
      <div className="grid gap-3">
        {query.data?.items.map((token) => (
          <article className="surface-panel p-5" key={token.id}>
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p className="font-semibold">{token.name}</p>
                <p className="mt-1 font-mono text-sm text-muted-foreground">{token.tokenPrefix}…</p>
                <p className="mt-2 text-sm text-muted-foreground">{token.scopes.join(', ')}</p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {token.expiresAt ? t('apiTokens.expires', { value: new Date(token.expiresAt).toLocaleString() }) : t('apiTokens.neverExpires')}
                </p>
              </div>
              <div className="flex items-center gap-3">
                <Badge variant="secondary">{t(`apiTokens.status.${token.status}`)}</Badge>
                {token.status === 'active' ? (
                  <Button size="xs" variant="destructive" type="button" disabled={revokeMutation.isPending} onClick={() => setRevokeTarget({ id: token.id, name: token.name })}>
                    <Icon name="trash" />{t('apiTokens.revoke')}
                  </Button>
                ) : null}
              </div>
            </div>
          </article>
        ))}
      </div>
      </ComponentCard>
      <ConfirmDialog open={Boolean(revokeTarget)} onOpenChange={(open) => !open && setRevokeTarget(null)} title={t('apiTokens.revokeTitle')} description={t('apiTokens.confirmRevoke', { name: revokeTarget?.name ?? '' })} confirmLabel={t('apiTokens.revoke')} cancelLabel={t('common.cancel')} closeLabel={t('common.close')} destructive pending={revokeMutation.isPending} onConfirm={() => revokeTarget && revokeMutation.mutate(revokeTarget.id)} />
    </section>
  )
}

function expirationISO(date: string, hour: string, minute: string) {
  const [year, month, day] = date.split('-').map(Number)
  return new Date(year, month - 1, day, Number(hour), Number(minute), 0, 0).toISOString()
}

function todayValue() {
  const today = new Date()
  return `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
}
