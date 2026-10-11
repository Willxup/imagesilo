import { PreviewImage } from '../../components/ui/preview-image'
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation, useNavigate, useSearchParams, useViewTransitionState } from 'react-router-dom'
import { toast } from 'sonner'

import { preloadAdminPage } from '../../app/page-loaders'
import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { Checkbox } from '../../components/ui/checkbox'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { CopyLinkControl, CopyLinksControl } from '../../components/ui/copy-link-control'
import { DatePicker } from '../../components/ui/date-picker'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { Select } from '../../components/ui/select'
import { apiRequest } from '../../lib/api-client'
import { readLocalStorage, writeLocalStorage } from '../../lib/browser-storage'
import { formatBytes } from '../../lib/image-links'
import type { BatchOperationResult, Image, ImageList, Visibility } from '../../lib/api-types'

type ViewMode = 'grid' | 'list'

const viewModeStorageKey = 'imagesilo_image_view_mode'
const filtersOpenStorageKey = 'imagesilo_image_filters_open'
const advancedFiltersOpenStorageKey = 'imagesilo_image_advanced_filters_open'
const scrollPositions = new Map<string, { top: number; count: number }>()

function rememberScroll(key: string, count: number) {
  scrollPositions.delete(key)
  scrollPositions.set(key, { top: window.scrollY, count })
  // Bound abandoned detail visits without changing browser history entries.
  while (scrollPositions.size > 20) scrollPositions.delete(scrollPositions.keys().next().value!)
}

export function ImageListPage() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams, setSearchParams] = useSearchParams()
  const formRef = useRef<HTMLFormElement>(null)
  const filterToggleRef = useRef<HTMLButtonElement>(null)
  const searchKey = searchParams.toString()
  const filters = useMemo(() => normalizedFilterQuery(new URLSearchParams(searchKey)), [searchKey])
  const filterValues = useMemo(() => new URLSearchParams(filters), [filters])
  const activeFilterCount = imageFilterNames.filter((name) => filterValues.has(name)).length
  const [filtersOpen, setFiltersOpen] = useState(() => storedBoolean(filtersOpenStorageKey))
  const [advancedOpen, setAdvancedOpen] = useState(
    () => hasAdvancedParameters(searchParams) || storedBoolean(advancedFiltersOpenStorageKey),
  )
  const [createdFrom, setCreatedFrom] = useState(() => dateParameterValue(searchParams.get('createdFrom')))
  const [createdTo, setCreatedTo] = useState(() => dateParameterValue(searchParams.get('createdTo')))
  const [viewMode, setViewMode] = useState<ViewMode>(storedViewMode)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [deleteSnapshot, setDeleteSnapshot] = useState<string[]>([])
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
  const selectedImages = useMemo(() => images.filter((image) => selected.has(image.id)), [images, selected])
  const visibilityMutation = useMutation({
    mutationFn: ({ id, visibility }: { id: string; visibility: Visibility }) =>
      apiRequest<void>(`/api/v1/images/${id}/visibility`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ visibility }),
      }),
    onMutate: ({ id }) => toast.loading(t('common.working'), { id: `image-visibility-${id}` }),
    onSuccess: (_, variables) => {
      toast.success(variables.visibility === 'public' ? t('toast.imagePublic') : t('toast.imagePrivate'), { id: `image-visibility-${variables.id}` })
      void queryClient.invalidateQueries({ queryKey: ['images'] })
    },
    onError: (_, variables) => toast.error(t('toast.operationFailed'), { id: `image-visibility-${variables.id}` }),
  })
  const batchVisibility = useMutation({
    mutationFn: ({ visibility, imageIds }: { visibility: Visibility; imageIds: string[] }) =>
      apiRequest<BatchOperationResult>('/api/v1/images/batch-visibility', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ imageIds, visibility }),
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
    mutationFn: (imageIds: string[]) =>
      apiRequest<BatchOperationResult>('/api/v1/images/batch-delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ imageIds }),
      }),
    onMutate: () => toast.loading(t('common.working'), { id: 'batch-delete' }),
    onSuccess: (result) => {
      setDeleteConfirmOpen(false)
      setDeleteSnapshot([])
      showBatchToast(result, 'batch-delete')
      setSelected(new Set())
      void queryClient.invalidateQueries({ queryKey: ['images'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
    },
    onError: () => toast.error(t('toast.operationFailed'), { id: 'batch-delete' }),
  })

  useEffect(() => {
    const current = new URLSearchParams(searchKey)
    setSelected(new Set())
    setCreatedFrom(dateParameterValue(current.get('createdFrom')))
    setCreatedTo(dateParameterValue(current.get('createdTo')))
    if (hasAdvancedParameters(current)) setAdvancedOpen(true)
  }, [searchKey])

  useEffect(() => {
    writeLocalStorage(filtersOpenStorageKey, String(filtersOpen))
  }, [filtersOpen])

  useEffect(() => {
    writeLocalStorage(advancedFiltersOpenStorageKey, String(advancedOpen))
  }, [advancedOpen])

  useEffect(() => {
    writeLocalStorage(viewModeStorageKey, viewMode)
  }, [viewMode])

  const { isSuccess, hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = query
  useLayoutEffect(() => {
    if (!isSuccess || !scrollPositions.has(location.key)) return
    // Restore before the browser captures the return transition, not one frame later.
    const target = scrollPositions.get(location.key)
    if (target === undefined) return
    if (images.length < target.count && hasNextPage && !isFetchNextPageError) {
      if (!isFetchingNextPage) void fetchNextPage()
      return
    }
    window.scrollTo({ top: target.top, behavior: 'instant' })
    scrollPositions.delete(location.key)
  }, [isSuccess, hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage, images.length, location.key])

  function setVisibilityFilter(value: string) {
    const next = new URLSearchParams(searchParams)
    if (value) next.set('visibility', value)
    else next.delete('visibility')
    setSearchParams(next, { replace: true })
  }

  function showBatchToast(result: BatchOperationResult, id: string) {
    const failures = result.items.filter((item) => item.status === 'error' || item.status === 'not_found' || item.status === 'cleanup_pending')
    if (failures.length > 0) toast.warning(t('toast.batchPartial', { success: result.items.length - failures.length, failed: failures.length }), { id })
    else toast.success(t('toast.batchSuccess', { count: result.items.length }), { id })
  }

  useEffect(() => { formRef.current?.reset() }, [filters])

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
    setSearchParams(parameters, { replace: true })
    if (window.matchMedia?.('(max-width: 720px)').matches) {
      setFiltersOpen(false)
      filterToggleRef.current?.focus()
    }
  }

  function resetFilters() {
    formRef.current?.reset()
    setSelected(new Set())
    setCreatedFrom('')
    setCreatedTo('')
    setAdvancedOpen(false)
    setSearchParams(new URLSearchParams(), { replace: true })
  }

  function toggleSelected(id: string) {
    if (busy) return
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else if (next.size >= 100) {
        toast.warning(t('images.selectionLimit'))
      } else next.add(id)
      return next
    })
  }

  const busy = visibilityMutation.isPending || batchVisibility.isPending || batchDelete.isPending

  return (
    <section className={selected.size > 0 ? 'has-floating-batch' : ''}>
      <div className="page-heading-row library-heading">
        <div>
          <p className="page-kicker">{t('images.eyebrow')}</p>
          <h1 className="page-title">{t('images.title')}</h1>
          <p className="page-description">{t('images.description')}</p>
        </div>
        <Button type="button" aria-label={t('images.uploadAction')} onClick={() => navigate('/admin/upload')}><Icon name="plus" /><span className="library-upload-label">{t('images.uploadAction')}</span></Button>
      </div>
      <div className="library-toolbar">
        <div className="library-visibility" role="group" aria-label={t('images.visibility')}>
          {([['', t('images.allImages')], ['public', t('visibility.public')], ['private', t('visibility.private')]] as const).map(([value, label]) => <button type="button" key={value} className={filterValues.get('visibility') === (value || null) ? 'active' : ''} aria-pressed={filterValues.get('visibility') === (value || null)} aria-label={label} onClick={() => setVisibilityFilter(value)}><span className="visibility-label-full">{label}</span><span className="visibility-label-compact" aria-hidden="true">{value ? label : t('images.allImagesCompact')}</span></button>)}
        </div>
        <div className="library-tools">
          <Button
            ref={filterToggleRef}
            className="filter-toggle"
            size="xs"
            variant="outline"
            type="button"
            aria-label={filtersOpen ? t('images.hideFilters') : t('images.showFilters')}
            onClick={() => setFiltersOpen((current) => !current)}
          >
            <Icon name="filter" />
            <span>{filtersOpen ? t('images.hideFilters') : t('images.showFilters')}</span>
            {activeFilterCount > 0 ? <span className="filter-count" aria-label={t('images.activeFilterCount', { count: activeFilterCount })}>{activeFilterCount}</span> : null}
          </Button>
          {activeFilterCount > 0 ? <Button className="library-clear-filters" size="xs" variant="ghost" type="button" aria-label={t('images.clearFilters')} onClick={resetFilters}><Icon name="x" /><span>{t('images.clearFilters')}</span></Button> : null}
          <div className="view-switcher" role="group" aria-label={t('images.viewMode')}>
            <Button
              size="xs"
              variant={viewMode === 'grid' ? 'default' : 'ghost'}
              type="button"
              aria-label={t('images.grid')}
              onClick={() => setViewMode('grid')}
            >
              <Icon name="grid" />
              <span>{t('images.grid')}</span>
            </Button>
            <Button
              size="xs"
              variant={viewMode === 'list' ? 'default' : 'ghost'}
              type="button"
              aria-label={t('images.list')}
              onClick={() => setViewMode('list')}
            >
              <Icon name="list" />
              <span>{t('images.list')}</span>
            </Button>
          </div>
        </div>
      </div>

      <form className={`filter-panel ${filtersOpen ? 'is-open' : ''}`} aria-hidden={!filtersOpen} inert={!filtersOpen || undefined} ref={formRef} onSubmit={applyFilters}>
        <div className="filter-panel-inner">
        <div className="mb-4 flex items-center gap-2 text-sm font-semibold">
          <Icon name="filter" className="h-4 w-4 text-cyan" />
          {t('images.filters')}
        </div>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-[minmax(16rem,2fr)_repeat(3,minmax(9rem,1fr))]">
          <FilterField name="q" label={t('images.search')} placeholder={t('images.searchPlaceholder')} defaultValue={filterValues.get('q') ?? ''} />
          <SelectField
            name="visibility"
            label={t('images.visibility')}
            defaultValue={filterValues.get('visibility') ?? ''}
            options={[
              ['', t('common.all')],
              ['public', t('visibility.public')],
              ['private', t('visibility.private')],
            ]}
          />
          <SelectField
            name="format"
            label={t('images.format')}
            defaultValue={filterValues.get('format') ?? ''}
            options={[
              ['', t('common.all')],
              ['jpeg', 'JPEG'],
              ['png', 'PNG'],
              ['webp', 'WebP'],
              ['gif', 'GIF'],
            ]}
          />
          <SelectField
            name="uploadedVia"
            label={t('images.uploadedVia')}
            defaultValue={filterValues.get('uploadedVia') ?? ''}
            options={[
              ['', t('common.all')],
              ['admin', t('images.source.admin')],
              ['api_token', t('images.source.api_token')],
              ['import', t('images.source.import')],
            ]}
          />
        </div>
        <div className="advanced-filters mt-3">
          <button
            className="advanced-filter-toggle"
            data-open={advancedOpen || undefined}
            type="button"
            aria-expanded={advancedOpen}
            onClick={() => setAdvancedOpen((current) => !current)}
          >
            <Icon name="filter" />
            {t('images.advancedFilters')}
            <Icon name="chevronDown" />
          </button>
          <div className={`advanced-filter-content ${advancedOpen ? 'is-open' : ''}`} aria-hidden={!advancedOpen} inert={!advancedOpen || undefined}>
            <div className="advanced-filter-content-inner">
              <div className="mt-3 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <DateFilterField
                  label={t('images.createdFrom')}
                  name="createdFrom"
                  value={createdFrom}
                  onChange={setCreatedFrom}
                  locale={i18n.resolvedLanguage ?? i18n.language}
                  t={t}
                />
                <DateFilterField
                  label={t('images.createdTo')}
                  name="createdTo"
                  value={createdTo}
                  onChange={setCreatedTo}
                  min={createdFrom}
                  locale={i18n.resolvedLanguage ?? i18n.language}
                  t={t}
                />
                <FilterField
                  name="minMiB"
                  label={t('images.minSize')}
                  type="number"
                  min="0"
                  step="0.1"
                  defaultValue={bytesToMiB(filterValues.get('minBytes'))}
                />
                <FilterField
                  name="maxMiB"
                  label={t('images.maxSize')}
                  type="number"
                  min="0"
                  step="0.1"
                  defaultValue={bytesToMiB(filterValues.get('maxBytes'))}
                />
                <FilterField name="minWidth" label={t('images.minWidth')} type="number" min="0" defaultValue={filterValues.get('minWidth') ?? ''} />
                <FilterField name="maxWidth" label={t('images.maxWidth')} type="number" min="0" defaultValue={filterValues.get('maxWidth') ?? ''} />
                <FilterField name="minHeight" label={t('images.minHeight')} type="number" min="0" defaultValue={filterValues.get('minHeight') ?? ''} />
                <FilterField name="maxHeight" label={t('images.maxHeight')} type="number" min="0" defaultValue={filterValues.get('maxHeight') ?? ''} />
              </div>
            </div>
          </div>
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          <Button className="standard-action-button" type="submit">
            <Icon name="search" />
            {t('images.applyFilters')}
          </Button>
          <Button className="standard-action-button" variant="outline" type="button" onClick={resetFilters}>
            <Icon name="refresh" />
            {t('images.resetFilters')}
          </Button>
        </div>
        </div>
      </form>

      {query.isLoading ? <div className="image-grid mt-6" role="status" aria-label={t('common.loading')}>{Array.from({ length: 6 }, (_, index) => <div className="image-skeleton" key={index}><div /><span /><small /></div>)}</div> : null}
      {query.isError ? <div className="empty-state" role="alert"><span className="empty-state-icon"><Icon name="activity" /></span><p>{t('images.failed')}</p><Button variant="outline" type="button" onClick={() => void query.refetch()}>{t('images.retryLoad')}</Button></div> : null}
      {!query.isLoading && !query.isError && images.length === 0 ? (
        <div className="empty-state">
          <span className="empty-state-icon"><Icon name="images" /></span>
          <p>{filters === 'limit=24' ? t('images.emptyLibrary') : t('images.emptyFiltered')}</p>
          <Button variant="outline" type="button" onClick={filters === 'limit=24' ? () => navigate('/admin/upload') : resetFilters}>{filters === 'limit=24' ? t('images.uploadAction') : t('images.clearFilters')}</Button>
        </div>
      ) : null}

      <div className={viewMode === 'grid' ? 'image-grid mt-6' : 'image-list mt-6'}>
        {images.map((image) => (
          <ImageCard
            image={image}
            key={image.id}
            list={viewMode === 'list'}
            selected={selected.has(image.id)}
            disabled={busy}
            returnTo={`${location.pathname}${location.search}`}
            onSelect={() => toggleSelected(image.id)}
            onVisibility={(visibility) => visibilityMutation.mutate({ id: image.id, visibility })}
            onOpen={() => { rememberScroll(location.key, images.length) }}
          />
        ))}
      </div>
      {images.length > 0 ? <p className="library-loaded-count">{t('images.loadedCount', { count: images.length })}</p> : null}
      {query.hasNextPage ? (
        <Button className="mt-5" size="sm" variant="outline" type="button" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
          <Icon name="plus" />
          {query.isFetchingNextPage ? t('common.loading') : t('images.loadMore')}
        </Button>
      ) : null}

      {selected.size > 0 ? (
        <div className="floating-batch-toolbar" aria-label={t('images.batchActions')}>
          <span className="floating-batch-count">
            <Icon name="check" />
            {t('images.selected', { count: selected.size })}
          </span>
          <div className="floating-batch-actions">
            <CopyLinksControl
              images={selectedImages}
              compact
              menuAlign="left"
              label={t('images.copySelectedLinks')}
              ariaLabel={(format) => t('images.copySelectedLinksFormat', { count: selectedImages.length, format })}
            />
            <Button
              size="xs"
              variant="outline"
              type="button"
              aria-label={t('images.makePublic')}
              disabled={busy}
              onClick={() => batchVisibility.mutate({ visibility: 'public', imageIds: [...selected] })}
            >
              <Icon name="visibility" />
              <span>{t('images.makePublic')}</span>
            </Button>
            <Button
              size="xs"
              variant="outline"
              type="button"
              aria-label={t('images.makePrivate')}
              disabled={busy}
              onClick={() => batchVisibility.mutate({ visibility: 'private', imageIds: [...selected] })}
            >
              <Icon name="visibilityOff" />
              <span>{t('images.makePrivate')}</span>
            </Button>
            <Button
              size="xs"
              variant="destructive"
              type="button"
              aria-label={t('common.delete')}
              disabled={busy}
              onClick={() => {
                setDeleteSnapshot([...selected])
                setDeleteConfirmOpen(true)
              }}
            >
              <Icon name="trash" />
              <span>{t('common.delete')}</span>
            </Button>
            <Button size="icon-xs" variant="ghost" type="button" aria-label={t('common.clear')} disabled={busy} onClick={() => setSelected(new Set())}>
              <Icon name="x" />
            </Button>
          </div>
        </div>
      ) : null}

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={(open) => {
          setDeleteConfirmOpen(open)
          if (!open && !batchDelete.isPending) setDeleteSnapshot([])
        }}
        title={t('images.deleteSelectionTitle')}
        description={t('images.confirmBatchDelete', { count: deleteSnapshot.length })}
        confirmLabel={t('images.deletePermanently')}
        cancelLabel={t('common.cancel')}
        closeLabel={t('common.close')}
        destructive
        pending={batchDelete.isPending}
        onConfirm={() => batchDelete.mutate([...deleteSnapshot])}
      />
    </section>
  )
}

function ImageCard({
  image,
  list,
  selected,
  disabled,
  returnTo,
  onSelect,
  onVisibility,
  onOpen,
}: {
  image: Image
  list: boolean
  selected: boolean
  disabled: boolean
  returnTo: string
  onSelect: () => void
  onVisibility: (value: Visibility) => void
  onOpen: () => void
}) {
  const { t, i18n } = useTranslation()
  const transitioning = useViewTransitionState(`/admin/images/${image.id}`)
  const linkState = { fromImageList: true, returnTo, preview: { url: image.standardUrl, name: image.originalName } }
  return (
    <article className={`image-card-clickable ${list ? 'is-list' : ''} ${selected ? 'is-selected' : ''}`} onPointerEnter={() => preloadAdminPage(`/admin/images/${image.id}`)} onFocus={() => preloadAdminPage(`/admin/images/${image.id}`)}>
        <div className={list ? 'image-card-media sm:w-56' : 'image-card-media'} style={{ '--image-ratio': image.width > 0 && image.height > 0 ? `${image.width} / ${image.height}` : '4 / 3' } as CSSProperties}>
          <Link className="image-card-link" to={`/admin/images/${image.id}`} state={linkState} viewTransition onClick={onOpen} aria-label={image.originalName}>
          <PreviewImage
            className={list ? 'aspect-video h-full w-full object-cover' : 'aspect-[4/3] h-full w-full object-cover'}
            src={image.thumbnailUrl}
            alt={image.originalName}
            loading="lazy"
            style={{ viewTransitionName: transitioning ? `image-${image.id}` : 'none' }}
          />
          </Link>
          <label className="image-select">
            <Checkbox checked={selected} disabled={disabled} onChange={onSelect} aria-label={t('images.selectImage', { name: image.originalName })} />
          </label>
        </div>
        <div className="image-card-info">
          <div className="image-card-title-row">
            <h2 className="truncate text-sm font-semibold text-ink" title={image.originalName}>
              <Link to={`/admin/images/${image.id}`} state={linkState} viewTransition onClick={onOpen}>{image.originalName}</Link>
            </h2>
            <Badge className="image-visibility-badge border-cyan/25 bg-cyan/8 text-cyan" variant="outline">
              <Icon name={image.visibility === 'public' ? 'visibility' : 'lock'} />
              {t(`visibility.${image.visibility}`)}
            </Badge>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            {image.width} × {image.height} · {image.extension} · {formatBytes(image.storedSize)}
          </p>
          <p className={`image-card-date mt-1 text-xs text-muted-foreground${list ? '' : ' grid-date'}`}>{new Date(image.createdAt).toLocaleString(i18n.resolvedLanguage ?? i18n.language)}</p>
          <div className="image-card-actions mt-3">
            <Button
              size="xs"
              variant="outline"
              type="button"
              disabled={disabled}
              aria-label={image.visibility === 'public' ? t('images.makePrivate') : t('images.makePublic')}
              onClick={() => onVisibility(image.visibility === 'public' ? 'private' : 'public')}
            >
              <Icon name={image.visibility === 'public' ? 'visibilityOff' : 'visibility'} />
              <span className="image-action-label">{image.visibility === 'public' ? t('images.makePrivate') : t('images.makePublic')}</span>
            </Button>
            <CopyLinkControl image={image} compact />
          </div>
        </div>
    </article>
  )
}

function FilterField({
  name,
  label,
  type = 'text',
  placeholder,
  min,
  step,
  defaultValue,
}: {
  name: string
  label: string
  type?: string
  placeholder?: string
  min?: string
  step?: string
  defaultValue?: string
}) {
  return (
    <label className="text-sm font-medium">
      {label}
      <Input className="mt-1.5" name={name} type={type} placeholder={placeholder} min={min} step={step} defaultValue={defaultValue} />
    </label>
  )
}

function SelectField({ name, label, options, defaultValue }: { name: string; label: string; options: [string, string][]; defaultValue: string }) {
  return (
    <label className="text-sm font-medium">
      {label}
      <Select className="mt-1.5" name={name} ariaLabel={label} defaultValue={defaultValue} options={options.map(([value, text]) => ({ value, label: text }))} />
    </label>
  )
}

function DateFilterField({
  label,
  name,
  value,
  onChange,
  locale,
  min,
  t,
}: {
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

function append(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null) {
  const value = String(raw ?? '').trim()
  if (value) parameters.set(name, value)
}
function appendDate(parameters: URLSearchParams, name: string, raw: string, endOfDay: boolean) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(raw.trim())
  if (!match) return
  const value = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]), endOfDay ? 23 : 0, endOfDay ? 59 : 0, endOfDay ? 59 : 0, endOfDay ? 999 : 0)
  parameters.set(name, value.toISOString())
}
function appendMiB(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null) {
  const value = Number(String(raw ?? '').trim())
  if (Number.isFinite(value) && value > 0) parameters.set(name, String(Math.round(value * 1024 * 1024)))
}

const imageFilterNames = [
  'q',
  'visibility',
  'format',
  'uploadedVia',
  'createdFrom',
  'createdTo',
  'minBytes',
  'maxBytes',
  'minWidth',
  'maxWidth',
  'minHeight',
  'maxHeight',
] as const
const advancedFilterNames = ['createdFrom', 'createdTo', 'minBytes', 'maxBytes', 'minWidth', 'maxWidth', 'minHeight', 'maxHeight'] as const

function normalizedFilterQuery(source: URLSearchParams) {
  const parameters = new URLSearchParams({ limit: '24' })
  for (const name of imageFilterNames) {
    const value = source.get(name)?.trim()
    if (value) parameters.set(name, value)
  }
  return parameters.toString()
}

function hasAdvancedParameters(parameters: URLSearchParams) {
  return advancedFilterNames.some((name) => Boolean(parameters.get(name)?.trim()))
}

function dateParameterValue(raw: string | null) {
  if (!raw) return ''
  const value = new Date(raw)
  if (Number.isNaN(value.getTime())) return ''
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`
}

function bytesToMiB(raw: string | null) {
  const bytes = Number(raw)
  if (!Number.isFinite(bytes) || bytes <= 0) return ''
  return String(Math.round((bytes / 1024 / 1024) * 10) / 10)
}

function storedBoolean(key: string) {
  return readLocalStorage(key) === 'true'
}

function storedViewMode(): ViewMode {
  return readLocalStorage(viewModeStorageKey) === 'list' ? 'list' : 'grid'
}
