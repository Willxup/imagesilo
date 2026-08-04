import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router-dom'
import { toast } from 'sonner'

import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { Checkbox } from '../../components/ui/checkbox'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { CopyLinkControl } from '../../components/ui/copy-link-control'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { Select } from '../../components/ui/select'
import { apiRequest } from '../../lib/api-client'
import { formatBytes } from '../../lib/image-links'
import type { MigrationImage, MigrationImageBatchResult, MigrationImageList } from '../../lib/api-types'

type ViewMode = 'grid' | 'list'

export function MigrationImagePage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const searchKey = searchParams.toString()
  const filters = useMemo(() => normalizedFilterQuery(new URLSearchParams(searchKey)), [searchKey])
  const filterValues = useMemo(() => new URLSearchParams(filters), [filters])
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [viewMode, setViewMode] = useState<ViewMode>('grid')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [deleteSnapshot, setDeleteSnapshot] = useState<string[]>([])

  const query = useInfiniteQuery({
    queryKey: ['migration-images', filters],
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const parameters = new URLSearchParams(filters)
      if (pageParam) parameters.set('cursor', pageParam)
      return apiRequest<MigrationImageList>(`/api/v1/migration-images?${parameters.toString()}`)
    },
    getNextPageParam: (page) => page.nextCursor || undefined,
  })
  const images = useMemo(() => query.data?.pages.flatMap((page) => page.items) ?? [], [query.data])
  const mutationsEnabled = query.data?.pages[0]?.mutationsEnabled ?? false
  const skippedFiles = query.data?.pages[0]?.skippedFiles ?? 0

  const refresh = useMutation({
    mutationFn: () => apiRequest<void>('/api/v1/migration-images/refresh', { method: 'POST' }),
    onSuccess: async () => {
      setSelected(new Set())
      await queryClient.invalidateQueries({ queryKey: ['migration-images'] })
      toast.success(t('migrations.refreshSuccess'), { id: 'migration-refresh' })
    },
    onError: () => toast.error(t('migrations.refreshFailed'), { id: 'migration-refresh' }),
  })

  const batchDelete = useMutation({
    mutationFn: (paths: string[]) =>
      apiRequest<MigrationImageBatchResult>('/api/v1/migration-images/batch-delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ paths }),
      }),
    onMutate: () => toast.loading(t('common.working'), { id: 'migration-batch-delete' }),
    onSuccess: (result) => {
      const failures = result.items.filter((item) => item.status !== 'deleted')
      if (failures.length > 0) {
        toast.warning(t('migrations.batchPartial', { success: result.items.length - failures.length, failed: failures.length }), {
          id: 'migration-batch-delete',
        })
      } else {
        toast.success(t('migrations.batchSuccess', { count: result.items.length }), { id: 'migration-batch-delete' })
      }
      setDeleteConfirmOpen(false)
      setDeleteSnapshot([])
      setSelected(new Set())
      void queryClient.invalidateQueries({ queryKey: ['migration-images'] })
    },
    onError: () => toast.error(t('migrations.deleteFailed'), { id: 'migration-batch-delete' }),
  })

  useEffect(() => {
    setSelected(new Set())
  }, [searchKey])

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const parameters = new URLSearchParams({ limit: '24' })
    append(parameters, 'q', form.get('q'))
    append(parameters, 'format', form.get('format'))
    appendMiB(parameters, 'minBytes', form.get('minMiB'))
    appendMiB(parameters, 'maxBytes', form.get('maxMiB'))
    setSelected(new Set())
    setSearchParams(parameters, { replace: true })
    if (window.matchMedia?.('(max-width: 720px)').matches) setFiltersOpen(false)
  }

  function resetFilters() {
    setSelected(new Set())
    setSearchParams(new URLSearchParams(), { replace: true })
  }

  function toggleSelected(path: string) {
    if (batchDelete.isPending || !mutationsEnabled) return
    setSelected((current) => {
      const next = new Set(current)
      if (next.has(path)) next.delete(path)
      else if (next.size >= 100) {
        toast.warning(t('migrations.selectionLimit'))
      } else next.add(path)
      return next
    })
  }

  function confirmDelete(paths: string[]) {
    if (!mutationsEnabled || batchDelete.isPending) return
    setDeleteSnapshot(paths)
    setDeleteConfirmOpen(true)
  }

  return (
    <section className={selected.size > 0 ? 'has-floating-batch' : ''}>
      <div className="page-heading-row">
        <div>
          <h1 className="page-title">{t('migrations.title')}</h1>
          <p className="page-description">{t('migrations.description')}</p>
        </div>
        <div className="library-toolbar">
          <Button
            className="filter-toggle"
            size="xs"
            variant="outline"
            type="button"
            aria-label={filtersOpen ? t('images.hideFilters') : t('images.showFilters')}
            onClick={() => setFiltersOpen((current) => !current)}
          >
            <Icon name="filter" />
            <span>{filtersOpen ? t('images.hideFilters') : t('images.showFilters')}</span>
          </Button>
          <Button
            size="xs"
            variant="outline"
            type="button"
            aria-label={t('migrations.refresh')}
            disabled={refresh.isPending}
            onClick={() => refresh.mutate()}
          >
            <Icon name={refresh.isPending ? 'loader' : 'refresh'} className={refresh.isPending ? 'animate-spin' : ''} />
            <span>{refresh.isPending ? t('migrations.scanning') : t('migrations.refresh')}</span>
          </Button>
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

      {!query.isLoading && !query.isError ? (
        <div className={`migration-mode-notice mt-5 ${mutationsEnabled ? 'is-writable' : ''}`} role="status">
          <Icon name={mutationsEnabled ? 'shield' : 'lock'} />
          <div>
            <strong>{mutationsEnabled ? t('migrations.writableTitle') : t('migrations.readOnlyTitle')}</strong>
            <p>{mutationsEnabled ? t('migrations.writableHelp') : t('migrations.readOnlyHelp')}</p>
          </div>
        </div>
      ) : null}

      {skippedFiles > 0 ? (
        <p className="mt-3 text-xs text-muted-foreground" role="status">
          {t('migrations.skippedFiles', { count: skippedFiles })}
        </p>
      ) : null}

      <form className={`filter-panel mt-5 ${filtersOpen ? 'is-open' : ''}`} key={filters} onSubmit={applyFilters}>
        <div className="mb-4 flex items-center gap-2 text-sm font-semibold">
          <Icon name="filter" className="h-4 w-4 text-cyan" />
          {t('images.filters')}
        </div>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <FilterField name="q" label={t('images.search')} placeholder={t('migrations.searchPlaceholder')} defaultValue={filterValues.get('q') ?? ''} />
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
          <FilterField name="minMiB" label={t('images.minSize')} type="number" min="0" step="0.1" defaultValue={bytesToMiB(filterValues.get('minBytes'))} />
          <FilterField name="maxMiB" label={t('images.maxSize')} type="number" min="0" step="0.1" defaultValue={bytesToMiB(filterValues.get('maxBytes'))} />
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
      </form>

      {query.isLoading ? (
        <p className="mt-8 flex items-center gap-2 text-muted-foreground" role="status">
          <Icon name="loader" className="animate-spin" />
          {t('migrations.scanning')}
        </p>
      ) : null}
      {query.isError ? <p className="mt-8 text-danger">{t('migrations.failed')}</p> : null}
      {!query.isLoading && !query.isError && images.length === 0 ? (
        <div className="empty-state">
          <div>
            <span className="empty-state-icon">
              <Icon name="images" />
            </span>
            <p>{t('migrations.empty')}</p>
          </div>
        </div>
      ) : null}

      <div className={viewMode === 'grid' ? 'image-grid mt-6' : 'image-list mt-6'}>
        {images.map((image) => (
          <MigrationImageCard
            image={image}
            key={image.path}
            list={viewMode === 'list'}
            selected={selected.has(image.path)}
            mutationsEnabled={mutationsEnabled}
            disabled={batchDelete.isPending}
            onSelect={() => toggleSelected(image.path)}
            onDelete={() => confirmDelete([image.path])}
          />
        ))}
      </div>

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
            <Button size="xs" variant="destructive" type="button" aria-label={t('common.delete')} disabled={batchDelete.isPending} onClick={() => confirmDelete([...selected])}>
              <Icon name="trash" />
              <span>{t('common.delete')}</span>
            </Button>
            <Button size="icon-xs" variant="ghost" type="button" aria-label={t('common.clear')} disabled={batchDelete.isPending} onClick={() => setSelected(new Set())}>
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
        title={deleteSnapshot.length === 1 ? t('migrations.deleteImageTitle') : t('migrations.deleteSelectionTitle')}
        description={
          deleteSnapshot.length === 1
            ? t('migrations.confirmDelete', { path: displayPath(deleteSnapshot[0] ?? '') })
            : t('migrations.confirmBatchDelete', { count: deleteSnapshot.length })
        }
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

function MigrationImageCard({
  image,
  list,
  selected,
  mutationsEnabled,
  disabled,
  onSelect,
  onDelete,
}: {
  image: MigrationImage
  list: boolean
  selected: boolean
  mutationsEnabled: boolean
  disabled: boolean
  onSelect: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  return (
    <article>
      <Card size="sm" className={list ? 'image-card gap-0 py-0 sm:flex-row' : 'image-card gap-0 py-0'}>
        <div className={list ? 'image-card-media sm:w-56' : 'image-card-media'}>
          <img
            className={list ? 'aspect-video h-full w-full object-contain' : 'aspect-[4/3] h-full w-full object-cover'}
            src={image.standardUrl}
            alt={image.originalName}
            loading="lazy"
          />
          {mutationsEnabled ? (
            <label className="image-select">
              <Checkbox checked={selected} disabled={disabled} onChange={onSelect} aria-label={t('migrations.selectImage', { path: displayPath(image.path) })} />
            </label>
          ) : null}
        </div>
        <div className="min-w-0 flex-1 p-4">
          <div className="image-card-title-row">
            <h2 className="truncate text-sm font-semibold text-ink" title={image.originalName}>
              {image.originalName}
            </h2>
            <Badge className="image-visibility-badge border-cyan/25 bg-cyan/8 text-cyan" variant="outline">
              <Icon name="server" />
              {t('migrations.badge')}
            </Badge>
          </div>
          <p className="migration-image-path mt-1 text-xs text-muted-foreground" title={displayPath(image.path)}>
            {displayPath(image.path)}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            {image.extension.replace('.', '').toUpperCase()} · {formatBytes(image.storedSize)}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">{new Date(image.modifiedAt).toLocaleString()}</p>
          <div className="image-card-actions migration-image-actions mt-3">
            <CopyLinkControl image={image} compact />
            <Button asChild size="xs" variant="outline">
              <a href={image.standardUrl} target="_blank" rel="noreferrer" aria-label={t('migrations.openImage', { path: displayPath(image.path) })}>
                <Icon name="external" />
                <span className="image-action-label">{t('images.openImage')}</span>
              </a>
            </Button>
            {mutationsEnabled ? (
              <Button size="icon-xs" variant="destructive" type="button" disabled={disabled} aria-label={t('migrations.deleteImage', { path: displayPath(image.path) })} onClick={onDelete}>
                <Icon name="trash" />
              </Button>
            ) : null}
          </div>
        </div>
      </Card>
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

function normalizedFilterQuery(input: URLSearchParams) {
  const output = new URLSearchParams({ limit: '24' })
  for (const name of ['q', 'format', 'minBytes', 'maxBytes']) {
    const value = input.get(name)?.trim()
    if (value) output.set(name, value)
  }
  return output.toString()
}

function append(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null) {
  const value = String(raw ?? '').trim()
  if (value) parameters.set(name, value)
}

function appendMiB(parameters: URLSearchParams, name: string, raw: FormDataEntryValue | null) {
  const value = Number(String(raw ?? '').trim())
  if (Number.isFinite(value) && value > 0) parameters.set(name, String(Math.round(value * 1024 * 1024)))
}

function bytesToMiB(value: string | null) {
  if (!value) return ''
  const bytes = Number(value)
  return Number.isFinite(bytes) && bytes > 0 ? String(Math.round((bytes / 1024 / 1024) * 10) / 10) : ''
}

function displayPath(value: string) {
  try {
    return decodeURI(value)
  } catch {
    return value
  }
}
