import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router-dom'

import { apiRequest } from '../../lib/api-client'
import { copyText, formatBytes, imageLinks } from '../../lib/image-links'
import type { BatchOperationResult, Image, ImageList, Visibility } from '../../lib/api-types'

type ViewMode = 'grid' | 'list'

export function ImageListPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState('limit=24')
  const [viewMode, setViewMode] = useState<ViewMode>('grid')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [batchResult, setBatchResult] = useState<BatchOperationResult | null>(null)
  const query = useInfiniteQuery({
    queryKey: ['images', filters],
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const parameters = new URLSearchParams(filters)
      if (pageParam) parameters.set('cursor', pageParam)
      return apiRequest<ImageList>(`/api/v1/images?${parameters.toString()}`)
    },
    getNextPageParam: (page) => page.nextCursor || undefined,
  })
  const images = useMemo(() => query.data?.pages.flatMap((page) => page.items) ?? [], [query.data])
  const visibilityMutation = useMutation({
    mutationFn: ({ id, visibility }: { id: string; visibility: Visibility }) => apiRequest<void>(
      `/api/v1/images/${id}/visibility`,
      { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ visibility }) },
    ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['images'] }),
  })
  const batchVisibility = useMutation({
    mutationFn: (visibility: Visibility) => apiRequest<BatchOperationResult>('/api/v1/images/batch-visibility', {
      method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ imageIds: [...selected], visibility }),
    }),
    onSuccess: (result) => {
      setBatchResult(result)
      setSelected(new Set())
      void queryClient.invalidateQueries({ queryKey: ['images'] })
    },
  })
  const batchDelete = useMutation({
    mutationFn: () => apiRequest<BatchOperationResult>('/api/v1/images/batch-delete', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ imageIds: [...selected] }),
    }),
    onSuccess: (result) => {
      setBatchResult(result)
      setSelected(new Set())
      void queryClient.invalidateQueries({ queryKey: ['images'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
    },
  })

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const parameters = new URLSearchParams({ limit: '24' })
    append(parameters, 'q', form.get('q'))
    append(parameters, 'visibility', form.get('visibility'))
    append(parameters, 'format', form.get('format'))
    append(parameters, 'uploadedVia', form.get('uploadedVia'))
    appendDate(parameters, 'createdFrom', form.get('createdFrom'), false)
    appendDate(parameters, 'createdTo', form.get('createdTo'), true)
    appendMiB(parameters, 'minBytes', form.get('minMiB'))
    appendMiB(parameters, 'maxBytes', form.get('maxMiB'))
    for (const name of ['minWidth', 'maxWidth', 'minHeight', 'maxHeight']) append(parameters, name, form.get(name))
    setSelected(new Set())
    setBatchResult(null)
    setFilters(parameters.toString())
  }

  function resetFilters(form: HTMLFormElement) {
    form.reset()
    setSelected(new Set())
    setBatchResult(null)
    setFilters('limit=24')
  }

  function toggleSelected(id: string) {
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function deleteSelection() {
    if (selected.size === 0) return
    if (window.confirm(t('images.confirmBatchDelete', { count: selected.size }))) batchDelete.mutate()
  }

  const busy = visibilityMutation.isPending || batchVisibility.isPending || batchDelete.isPending

  return (
    <section>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="page-title">{t('images.title')}</h1>
          <p className="page-description">{t('images.description')}</p>
        </div>
        <div className="flex gap-2" role="group" aria-label={t('images.viewMode')}>
          <button className={viewMode === 'grid' ? 'button-primary' : 'button-secondary'} type="button" onClick={() => setViewMode('grid')}>{t('images.grid')}</button>
          <button className={viewMode === 'list' ? 'button-primary' : 'button-secondary'} type="button" onClick={() => setViewMode('list')}>{t('images.list')}</button>
        </div>
      </div>

      <form className="mt-6 rounded-2xl border border-line bg-panel p-5" onSubmit={applyFilters}>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <FilterField name="q" label={t('images.search')} placeholder={t('images.searchPlaceholder')} />
          <SelectField name="visibility" label={t('images.visibility')} options={[
            ['', t('common.all')], ['public', t('visibility.public')], ['private', t('visibility.private')],
          ]} />
          <SelectField name="format" label={t('images.format')} options={[
            ['', t('common.all')], ['jpeg', 'JPEG'], ['png', 'PNG'], ['webp', 'WebP'], ['gif', 'GIF'],
          ]} />
          <SelectField name="uploadedVia" label={t('images.uploadedVia')} options={[
            ['', t('common.all')], ['admin', t('images.source.admin')], ['api_token', t('images.source.api_token')], ['import', t('images.source.import')],
          ]} />
          <FilterField name="createdFrom" label={t('images.createdFrom')} type="date" />
          <FilterField name="createdTo" label={t('images.createdTo')} type="date" />
          <FilterField name="minMiB" label={t('images.minSize')} type="number" min="0" step="0.1" />
          <FilterField name="maxMiB" label={t('images.maxSize')} type="number" min="0" step="0.1" />
          <FilterField name="minWidth" label={t('images.minWidth')} type="number" min="0" />
          <FilterField name="maxWidth" label={t('images.maxWidth')} type="number" min="0" />
          <FilterField name="minHeight" label={t('images.minHeight')} type="number" min="0" />
          <FilterField name="maxHeight" label={t('images.maxHeight')} type="number" min="0" />
        </div>
        <div className="mt-5 flex flex-wrap gap-3">
          <button className="button-primary" type="submit">{t('images.applyFilters')}</button>
          <button className="button-secondary" type="button" onClick={(event) => resetFilters(event.currentTarget.form!)}>{t('images.resetFilters')}</button>
        </div>
      </form>

      {selected.size > 0 ? (
        <div className="sticky top-3 z-10 mt-5 flex flex-wrap items-center gap-3 rounded-2xl border border-line bg-panel p-4 shadow-lg">
          <span className="font-medium">{t('images.selected', { count: selected.size })}</span>
          <button className="button-secondary" type="button" disabled={busy} onClick={() => batchVisibility.mutate('public')}>{t('images.makePublic')}</button>
          <button className="button-secondary" type="button" disabled={busy} onClick={() => batchVisibility.mutate('private')}>{t('images.makePrivate')}</button>
          <button className="button-danger" type="button" disabled={busy} onClick={deleteSelection}>{t('images.deletePermanently')}</button>
          <button className="button-secondary" type="button" onClick={() => setSelected(new Set())}>{t('common.clear')}</button>
        </div>
      ) : null}

      {query.isLoading ? <p className="mt-8 text-muted">{t('common.loading')}</p> : null}
      {query.isError ? <p className="mt-8 text-danger">{t('images.failed')}</p> : null}
      {!query.isLoading && images.length === 0 ? <p className="mt-8 text-muted">{t('images.empty')}</p> : null}
      {batchResult ? <BatchResult result={batchResult} /> : null}

      <div className={viewMode === 'grid' ? 'mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3' : 'mt-8 grid gap-3'}>
        {images.map((image) => (
          <ImageCard
            image={image}
            key={image.id}
            list={viewMode === 'list'}
            selected={selected.has(image.id)}
            disabled={busy}
            onSelect={() => toggleSelected(image.id)}
            onVisibility={(visibility) => visibilityMutation.mutate({ id: image.id, visibility })}
          />
        ))}
      </div>
      {query.hasNextPage ? (
        <button className="button-secondary mt-6" type="button" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
          {query.isFetchingNextPage ? t('common.loading') : t('images.loadMore')}
        </button>
      ) : null}
      {(visibilityMutation.isError || batchVisibility.isError || batchDelete.isError) ? <p className="mt-5 text-danger">{t('images.operationFailed')}</p> : null}
    </section>
  )
}

function ImageCard({ image, list, selected, disabled, onSelect, onVisibility }: {
  image: Image
  list: boolean
  selected: boolean
  disabled: boolean
  onSelect: () => void
  onVisibility: (value: Visibility) => void
}) {
  const { t } = useTranslation()
  const links = imageLinks(image)
  return (
    <article className={list ? 'flex flex-col overflow-hidden rounded-2xl border border-line bg-panel sm:flex-row' : 'overflow-hidden rounded-2xl border border-line bg-panel'}>
      <div className={list ? 'relative sm:w-56' : 'relative'}>
        <img className="aspect-video h-full w-full bg-canvas object-contain" src={image.thumbnailUrl} alt={image.originalName} loading="lazy" />
        <label className="absolute left-3 top-3 rounded-lg bg-panel/90 p-2 shadow">
          <input type="checkbox" checked={selected} onChange={onSelect} aria-label={t('images.selectImage', { name: image.originalName })} />
        </label>
      </div>
      <div className="min-w-0 flex-1 p-4">
        <Link className="font-medium text-accent hover:underline" to={`/admin/images/${image.id}`}>{image.originalName}</Link>
        <p className="mt-1 text-sm text-muted">{image.width} × {image.height} · {image.extension} · {formatBytes(image.storedSize)}</p>
        <p className="mt-1 text-xs text-muted">{new Date(image.createdAt).toLocaleString()}</p>
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <span className="rounded-full bg-accent-soft px-3 py-1 text-xs font-medium text-accent">{t(`visibility.${image.visibility}`)}</span>
          <button className="button-secondary" type="button" disabled={disabled} onClick={() => onVisibility(image.visibility === 'public' ? 'private' : 'public')}>
            {image.visibility === 'public' ? t('images.makePrivate') : t('images.makePublic')}
          </button>
          <button className="button-secondary" type="button" onClick={() => void copyText(links.direct)}>{t('images.copyDirect')}</button>
          <Link className="button-secondary" to={`/admin/images/${image.id}`}>{t('images.details')}</Link>
        </div>
      </div>
    </article>
  )
}

function BatchResult({ result }: { result: BatchOperationResult }) {
  const { t } = useTranslation()
  const failures = result.items.filter((item) => item.status === 'error' || item.status === 'not_found' || item.status === 'cleanup_pending')
  return (
    <div className="mt-5 rounded-2xl border border-line bg-panel p-4" aria-live="polite">
      <p className="font-medium">{t('images.batchCompleted', { count: result.items.length })}</p>
      {failures.length > 0 ? (
        <ul className="mt-2 list-disc pl-5 text-sm text-danger">
          {failures.map((item) => <li key={item.imageId}>{item.imageId}: {t(`images.batchStatus.${item.status}`)}</li>)}
        </ul>
      ) : <p className="mt-2 text-sm text-accent">{t('images.batchSuccess')}</p>}
    </div>
  )
}

function FilterField({ name, label, type = 'text', placeholder, min, step }: {
  name: string
  label: string
  type?: string
  placeholder?: string
  min?: string
  step?: string
}) {
  return <label className="text-sm font-medium">{label}<input className="field" name={name} type={type} placeholder={placeholder} min={min} step={step} /></label>
}

function SelectField({ name, label, options }: { name: string; label: string; options: [string, string][] }) {
  return (
    <label className="text-sm font-medium">{label}
      <select className="field" name={name}>{options.map(([value, text]) => <option key={value} value={value}>{text}</option>)}</select>
    </label>
  )
}

function append(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null) {
  const value = String(raw ?? '').trim()
  if (value) parameters.set(name, value)
}

function appendDate(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null, endOfDay: boolean) {
  const value = String(raw ?? '').trim()
  if (!value) return
  parameters.set(name, new Date(`${value}T${endOfDay ? '23:59:59' : '00:00:00'}Z`).toISOString())
}

function appendMiB(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null) {
  const value = Number(String(raw ?? '').trim())
  if (Number.isFinite(value) && value > 0) parameters.set(name, String(Math.round(value * 1024 * 1024)))
}
