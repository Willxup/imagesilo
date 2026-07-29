import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { apiRequest } from '../../lib/api-client'
import { copyText } from '../../lib/image-links'
import type { ImageAlias, ImageAliasList } from '../../lib/api-types'

export function AliasPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [resolved, setResolved] = useState<ImageAlias | null>(null)
  const [created, setCreated] = useState<ImageAlias | null>(null)
  const query = useQuery({ queryKey: ['aliases'], queryFn: () => apiRequest<ImageAliasList>('/api/v1/aliases?limit=100') })
  const create = useMutation({
    mutationFn: (value: { path: string; imageId: string; source: string }) => apiRequest<ImageAlias>('/api/v1/aliases', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(value),
    }),
    onSuccess: (value) => {
      setCreated(value)
      void queryClient.invalidateQueries({ queryKey: ['aliases'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
    },
  })
  const resolve = useMutation({
    mutationFn: (path: string) => apiRequest<ImageAlias>(`/api/v1/aliases/resolve?path=${encodeURIComponent(path)}`),
    onSuccess: setResolved,
  })
  const remove = useMutation({
    mutationFn: (id: string) => apiRequest<void>(`/api/v1/aliases/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['aliases'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
    },
  })

  function createAlias(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setCreated(null)
    const form = new FormData(event.currentTarget)
    create.mutate({
      path: String(form.get('path') ?? ''),
      imageId: String(form.get('imageId') ?? ''),
      source: String(form.get('source') ?? ''),
    })
  }

  function resolveAlias(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setResolved(null)
    const form = new FormData(event.currentTarget)
    resolve.mutate(String(form.get('resolvePath') ?? ''))
  }

  return (
    <section>
      <h1 className="page-title">{t('aliases.title')}</h1>
      <p className="page-description">{t('aliases.description')}</p>
      <div className="mt-8 grid gap-6 lg:grid-cols-2">
        <form className="rounded-2xl border border-line bg-panel p-6" onSubmit={createAlias}>
          <h2 className="text-xl font-semibold">{t('aliases.create')}</h2>
          <label className="mt-5 block font-medium" htmlFor="alias-path">{t('aliases.path')}</label>
          <input className="field" id="alias-path" name="path" placeholder="/i/2022/05/example.webp" required maxLength={2048} />
          <label className="mt-4 block font-medium" htmlFor="alias-image-id">{t('aliases.imageId')}</label>
          <input className="field" id="alias-image-id" name="imageId" required />
          <label className="mt-4 block font-medium" htmlFor="alias-source">{t('aliases.source')}</label>
          <input className="field" id="alias-source" name="source" defaultValue="manual" required maxLength={100} />
          <button className="button-primary mt-5" type="submit" disabled={create.isPending}>{create.isPending ? t('common.working') : t('aliases.create')}</button>
          {created ? <p className="mt-4 text-accent" aria-live="polite">{t('aliases.created', { path: created.path })}</p> : null}
          {create.isError ? <p className="mt-4 text-danger">{t('aliases.createFailed')}</p> : null}
        </form>
        <form className="rounded-2xl border border-line bg-panel p-6" onSubmit={resolveAlias}>
          <h2 className="text-xl font-semibold">{t('aliases.resolve')}</h2>
          <label className="mt-5 block font-medium" htmlFor="resolve-alias-path">{t('aliases.path')}</label>
          <input className="field" id="resolve-alias-path" name="resolvePath" required maxLength={2048} />
          <button className="button-secondary mt-5" type="submit" disabled={resolve.isPending}>{resolve.isPending ? t('common.working') : t('aliases.resolve')}</button>
          {resolved ? <div className="mt-4 rounded-xl bg-canvas p-4"><code className="break-all">{resolved.path}</code><p className="mt-2 text-sm text-muted">{t('aliases.target', { id: resolved.imageId })}</p></div> : null}
          {resolve.isError ? <p className="mt-4 text-danger">{t('aliases.notFound')}</p> : null}
        </form>
      </div>

      <section className="mt-6 rounded-2xl border border-line bg-panel p-6">
        <h2 className="text-xl font-semibold">{t('aliases.existing')}</h2>
        {query.isLoading ? <p className="mt-4 text-muted">{t('common.loading')}</p> : null}
        {query.isError ? <p className="mt-4 text-danger">{t('aliases.listFailed')}</p> : null}
        {query.data?.items.length === 0 ? <p className="mt-4 text-muted">{t('aliases.empty')}</p> : null}
        <ul className="mt-4 grid gap-3">
          {query.data?.items.map((alias) => (
            <li className="flex flex-col gap-3 rounded-xl bg-canvas p-4 sm:flex-row sm:items-center sm:justify-between" key={alias.id}>
              <div className="min-w-0"><code className="break-all text-sm">{alias.path}</code><p className="mt-1 text-xs text-muted">{alias.imageId} · {alias.source}</p></div>
              <div className="flex shrink-0 gap-2">
                <button className="button-secondary" type="button" onClick={() => void copyText(new URL(alias.path, window.location.origin).toString())}>{t('common.copy')}</button>
                <button className="button-danger" type="button" disabled={remove.isPending} onClick={() => {
                  if (window.confirm(t('aliases.confirmDelete', { path: alias.path }))) remove.mutate(alias.id)
                }}>{t('common.delete')}</button>
              </div>
            </li>
          ))}
        </ul>
        {remove.isError ? <p className="mt-4 text-danger">{t('aliases.deleteFailed')}</p> : null}
      </section>
    </section>
  )
}
