import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState, type FormEvent, type KeyboardEvent, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'

import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Checkbox } from '../../components/ui/checkbox'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { CopyLinkControl } from '../../components/ui/copy-link-control'
import { DatePicker } from '../../components/ui/date-picker'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { Select } from '../../components/ui/select'
import { apiRequest } from '../../lib/api-client'
import { formatBytes } from '../../lib/image-links'
import type { BatchOperationResult, Image, ImageList, Visibility } from '../../lib/api-types'

type ViewMode = 'grid' | 'list'

export function ImageListPage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const location = useLocation()
  const [searchParams] = useSearchParams()
  const headerQuery = searchParams.get('q')?.trim() ?? ''
  const [filters, setFilters] = useState(() => queryString(headerQuery))
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [createdFrom, setCreatedFrom] = useState('')
  const [createdTo, setCreatedTo] = useState('')
  const [viewMode, setViewMode] = useState<ViewMode>('grid')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
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
    onMutate: ({ id }) => toast.loading(t('common.working'), { id: `image-visibility-${id}` }),
    onSuccess: (_, variables) => {
      toast.success(variables.visibility === 'public' ? t('toast.imagePublic') : t('toast.imagePrivate'), { id: `image-visibility-${variables.id}` })
      void queryClient.invalidateQueries({ queryKey: ['images'] })
    },
    onError: (_, variables) => toast.error(t('toast.operationFailed'), { id: `image-visibility-${variables.id}` }),
  })
  const batchVisibility = useMutation({
    mutationFn: (visibility: Visibility) => apiRequest<BatchOperationResult>('/api/v1/images/batch-visibility', {
      method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ imageIds: [...selected], visibility }),
    }),
    onMutate: () => toast.loading(t('common.working'), { id: 'batch-visibility' }),
    onSuccess: (result) => {
      showBatchToast(result, 'batch-visibility')
      setSelected(new Set())
      void queryClient.invalidateQueries({ queryKey: ['images'] })
    },
    onError: () => toast.error(t('toast.operationFailed'), { id: 'batch-visibility' }),
  })
  const batchDelete = useMutation({
    mutationFn: () => apiRequest<BatchOperationResult>('/api/v1/images/batch-delete', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ imageIds: [...selected] }),
    }),
    onMutate: () => toast.loading(t('common.working'), { id: 'batch-delete' }),
    onSuccess: (result) => {
      setDeleteConfirmOpen(false)
      showBatchToast(result, 'batch-delete')
      setSelected(new Set())
      void queryClient.invalidateQueries({ queryKey: ['images'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
    },
    onError: () => toast.error(t('toast.operationFailed'), { id: 'batch-delete' }),
  })

  useEffect(() => {
    setSelected(new Set())
    setFilters(queryString(headerQuery))
    setCreatedFrom('')
    setCreatedTo('')
  }, [headerQuery])

  function showBatchToast(result: BatchOperationResult, id: string) {
    const failures = result.items.filter((item) => item.status === 'error' || item.status === 'not_found' || item.status === 'cleanup_pending')
    if (failures.length > 0) toast.warning(t('toast.batchPartial', { success: result.items.length - failures.length, failed: failures.length }), { id })
    else toast.success(t('toast.batchSuccess', { count: result.items.length }), { id })
  }

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const parameters = new URLSearchParams({ limit: '24' })
    append(parameters, 'q', form.get('q'))
    append(parameters, 'visibility', form.get('visibility'))
    append(parameters, 'format', form.get('format'))
    append(parameters, 'uploadedVia', form.get('uploadedVia'))
    appendDate(parameters, 'createdFrom', createdFrom, false)
    appendDate(parameters, 'createdTo', createdTo, true)
    appendMiB(parameters, 'minBytes', form.get('minMiB'))
    appendMiB(parameters, 'maxBytes', form.get('maxMiB'))
    for (const name of ['minWidth', 'maxWidth', 'minHeight', 'maxHeight']) append(parameters, name, form.get(name))
    setSelected(new Set())
    setFilters(parameters.toString())
    if (window.matchMedia?.('(max-width: 720px)').matches) setFiltersOpen(false)
  }

  function resetFilters(form: HTMLFormElement) {
    form.reset()
    setSelected(new Set())
    setCreatedFrom('')
    setCreatedTo('')
    setAdvancedOpen(false)
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

  const busy = visibilityMutation.isPending || batchVisibility.isPending || batchDelete.isPending

  return (
    <section className={selected.size > 0 ? 'has-floating-batch' : ''}>
      <div className="page-heading-row">
        <div><h1 className="page-title">{t('images.title')}</h1><p className="page-description">{t('images.description')}</p></div>
        <div className="library-toolbar">
          <Button className="filter-toggle" size="xs" variant="outline" type="button" aria-label={filtersOpen ? t('images.hideFilters') : t('images.showFilters')} onClick={() => setFiltersOpen((current) => !current)}><Icon name="filter" /><span>{filtersOpen ? t('images.hideFilters') : t('images.showFilters')}</span></Button>
          <div className="view-switcher" role="group" aria-label={t('images.viewMode')}>
            <Button size="xs" variant={viewMode === 'grid' ? 'default' : 'ghost'} type="button" aria-label={t('images.grid')} onClick={() => setViewMode('grid')}><Icon name="grid" /><span>{t('images.grid')}</span></Button>
            <Button size="xs" variant={viewMode === 'list' ? 'default' : 'ghost'} type="button" aria-label={t('images.list')} onClick={() => setViewMode('list')}><Icon name="list" /><span>{t('images.list')}</span></Button>
          </div>
        </div>
      </div>

      <form className={`filter-panel mt-6 ${filtersOpen ? 'is-open' : ''}`} key={headerQuery} onSubmit={applyFilters}>
        <div className="mb-4 flex items-center gap-2 text-sm font-semibold"><Icon name="filter" className="h-4 w-4 text-cyan" />{t('images.filters')}</div>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-[minmax(16rem,2fr)_repeat(3,minmax(9rem,1fr))]">
          <FilterField name="q" label={t('images.search')} placeholder={t('images.searchPlaceholder')} defaultValue={headerQuery} />
          <SelectField name="visibility" label={t('images.visibility')} options={[["", t('common.all')], ['public', t('visibility.public')], ['private', t('visibility.private')]]} />
          <SelectField name="format" label={t('images.format')} options={[["", t('common.all')], ['jpeg', 'JPEG'], ['png', 'PNG'], ['webp', 'WebP'], ['gif', 'GIF']]} />
          <SelectField name="uploadedVia" label={t('images.uploadedVia')} options={[["", t('common.all')], ['admin', t('images.source.admin')], ['api_token', t('images.source.api_token')], ['import', t('images.source.import')]]} />
        </div>
        <div className="advanced-filters mt-3">
          <button className="advanced-filter-toggle" data-open={advancedOpen || undefined} type="button" aria-expanded={advancedOpen} onClick={() => setAdvancedOpen((current) => !current)}>
            <Icon name="filter" />{t('images.advancedFilters')}<Icon name="chevronDown" />
          </button>
          <div className={`advanced-filter-content ${advancedOpen ? 'is-open' : ''}`} aria-hidden={!advancedOpen} inert={!advancedOpen || undefined}>
            <div className="advanced-filter-content-inner">
              <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <DateFilterField label={t('images.createdFrom')} name="createdFrom" value={createdFrom} onChange={setCreatedFrom} locale={i18n.resolvedLanguage ?? i18n.language} t={t} />
                <DateFilterField label={t('images.createdTo')} name="createdTo" value={createdTo} onChange={setCreatedTo} min={createdFrom} locale={i18n.resolvedLanguage ?? i18n.language} t={t} />
                <FilterField name="minMiB" label={t('images.minSize')} type="number" min="0" step="0.1" />
                <FilterField name="maxMiB" label={t('images.maxSize')} type="number" min="0" step="0.1" />
                <FilterField name="minWidth" label={t('images.minWidth')} type="number" min="0" />
                <FilterField name="maxWidth" label={t('images.maxWidth')} type="number" min="0" />
                <FilterField name="minHeight" label={t('images.minHeight')} type="number" min="0" />
                <FilterField name="maxHeight" label={t('images.maxHeight')} type="number" min="0" />
              </div>
            </div>
          </div>
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          <Button className="standard-action-button" type="submit"><Icon name="search" />{t('images.applyFilters')}</Button>
          <Button className="standard-action-button" variant="outline" type="button" onClick={(event) => resetFilters(event.currentTarget.form!)}><Icon name="refresh" />{t('images.resetFilters')}</Button>
        </div>
      </form>

      {query.isLoading ? <p className="mt-8 text-muted-foreground">{t('common.loading')}</p> : null}
      {query.isError ? <p className="mt-8 text-danger">{t('images.failed')}</p> : null}
      {!query.isLoading && images.length === 0 ? <div className="empty-state"><div><span className="empty-state-icon"><Icon name="images" /></span><p>{t('images.empty')}</p></div></div> : null}

      <div className={viewMode === 'grid' ? 'image-grid mt-6' : 'image-list mt-6'}>
        {images.map((image) => (
          <ImageCard image={image} key={image.id} list={viewMode === 'list'} selected={selected.has(image.id)} disabled={busy} returnTo={`${location.pathname}${location.search}`} onSelect={() => toggleSelected(image.id)} onVisibility={(visibility) => visibilityMutation.mutate({ id: image.id, visibility })} />
        ))}
      </div>
      {query.hasNextPage ? <Button className="mt-5" size="sm" variant="outline" type="button" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}><Icon name="plus" />{query.isFetchingNextPage ? t('common.loading') : t('images.loadMore')}</Button> : null}

      {selected.size > 0 ? (
        <div className="floating-batch-toolbar" aria-label={t('images.batchActions')}>
          <span className="floating-batch-count"><Icon name="check" />{t('images.selected', { count: selected.size })}</span>
          <div className="floating-batch-actions">
            <Button size="xs" variant="outline" type="button" aria-label={t('images.makePublic')} disabled={busy} onClick={() => batchVisibility.mutate('public')}><Icon name="visibility" /><span>{t('images.makePublic')}</span></Button>
            <Button size="xs" variant="outline" type="button" aria-label={t('images.makePrivate')} disabled={busy} onClick={() => batchVisibility.mutate('private')}><Icon name="visibilityOff" /><span>{t('images.makePrivate')}</span></Button>
            <Button size="xs" variant="destructive" type="button" aria-label={t('common.delete')} disabled={busy} onClick={() => setDeleteConfirmOpen(true)}><Icon name="trash" /><span>{t('common.delete')}</span></Button>
            <Button size="icon-xs" variant="ghost" type="button" aria-label={t('common.clear')} onClick={() => setSelected(new Set())}><Icon name="x" /></Button>
          </div>
        </div>
      ) : null}

      <ConfirmDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen} title={t('images.deleteSelectionTitle')} description={t('images.confirmBatchDelete', { count: selected.size })} confirmLabel={t('images.deletePermanently')} cancelLabel={t('common.cancel')} closeLabel={t('common.close')} destructive pending={batchDelete.isPending} onConfirm={() => batchDelete.mutate()} />
    </section>
  )
}

function ImageCard({ image, list, selected, disabled, returnTo, onSelect, onVisibility }: { image: Image; list: boolean; selected: boolean; disabled: boolean; returnTo: string; onSelect: () => void; onVisibility: (value: Visibility) => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const open = () => navigate(`/admin/images/${image.id}`, { state: { fromImageList: true, returnTo } })
  const stop = (event: MouseEvent) => event.stopPropagation()
  const keyDown = (event: KeyboardEvent<HTMLElement>) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); open() } }
  return (
    <article className="image-card-clickable" role="link" tabIndex={0} onClick={open} onKeyDown={keyDown} aria-label={image.originalName}>
      <Card size="sm" className={list ? 'image-card gap-0 py-0 sm:flex-row' : 'image-card gap-0 py-0'}>
        <div className={list ? 'image-card-media sm:w-56' : 'image-card-media'}>
          <img className={list ? 'aspect-video h-full w-full object-contain' : 'aspect-[4/3] h-full w-full object-cover'} src={image.thumbnailUrl} alt={image.originalName} loading="lazy" />
          <label className="image-select" onClick={stop}><Checkbox checked={selected} onChange={onSelect} aria-label={t('images.selectImage', { name: image.originalName })} /></label>
        </div>
        <div className="min-w-0 flex-1 p-4">
          <div className="image-card-title-row">
            <h2 className="truncate text-sm font-semibold text-ink" title={image.originalName}>{image.originalName}</h2>
            <Badge className="image-visibility-badge border-cyan/25 bg-cyan/8 text-cyan" variant="outline"><Icon name={image.visibility === 'public' ? 'visibility' : 'lock'} />{t(`visibility.${image.visibility}`)}</Badge>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">{image.width} × {image.height} · {image.extension} · {formatBytes(image.storedSize)}</p>
          <p className="mt-1 text-xs text-muted-foreground">{new Date(image.createdAt).toLocaleString()}</p>
          <div className="image-card-actions mt-3" onClick={stop}>
            <Button size="xs" variant="outline" type="button" disabled={disabled} onClick={() => onVisibility(image.visibility === 'public' ? 'private' : 'public')}><Icon name={image.visibility === 'public' ? 'visibilityOff' : 'visibility'} /><span className="image-action-label">{image.visibility === 'public' ? t('images.makePrivate') : t('images.makePublic')}</span></Button>
            <CopyLinkControl image={image} compact />
          </div>
        </div>
      </Card>
    </article>
  )
}

function FilterField({ name, label, type = 'text', placeholder, min, step, defaultValue }: { name: string; label: string; type?: string; placeholder?: string; min?: string; step?: string; defaultValue?: string }) {
  return <label className="text-sm font-medium">{label}<Input className="mt-1.5" name={name} type={type} placeholder={placeholder} min={min} step={step} defaultValue={defaultValue} /></label>
}

function SelectField({ name, label, options }: { name: string; label: string; options: [string, string][] }) {
  return <label className="text-sm font-medium">{label}<Select className="mt-1.5" name={name} ariaLabel={label} options={options.map(([value, text]) => ({ value, label: text }))} /></label>
}

function DateFilterField({ label, name, value, onChange, locale, min, t }: {
  label: string
  name: string
  value: string
  onChange: (value: string) => void
  locale: string
  min?: string
  t: ReturnType<typeof useTranslation>['t']
}) {
  return (
    <div className="text-sm font-medium">
      <span>{label}</span>
      <DatePicker
        className="mt-1.5"
        name={name}
        value={value}
        onChange={onChange}
        min={min}
        locale={locale}
        ariaLabel={label}
        placeholder={t('common.datePicker.select')}
        clearLabel={t('common.clear')}
        todayLabel={t('common.datePicker.today')}
        previousMonthLabel={t('common.datePicker.previousMonth')}
        nextMonthLabel={t('common.datePicker.nextMonth')}
      />
    </div>
  )
}

function append(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null) { const value = String(raw ?? '').trim(); if (value) parameters.set(name, value) }
function appendDate(parameters: URLSearchParams, name: string, raw: string, endOfDay: boolean) { const value = raw.trim(); if (value) parameters.set(name, new Date(`${value}T${endOfDay ? '23:59:59' : '00:00:00'}Z`).toISOString()) }
function appendMiB(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null) { const value = Number(String(raw ?? '').trim()); if (Number.isFinite(value) && value > 0) parameters.set(name, String(Math.round(value * 1024 * 1024))) }
function queryString(query: string) { const parameters = new URLSearchParams({ limit: '24' }); if (query) parameters.set('q', query); return parameters.toString() }
