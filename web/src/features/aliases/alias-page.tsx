import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useMemo, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '../../components/ui/button'
import { ComponentCard } from '../../components/ui/component-card'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { apiRequest, uploadForm } from '../../lib/api-client'
import { copyText } from '../../lib/image-links'
import type { ImageAlias, ImageAliasList, ImportResult } from '../../lib/api-types'

export function AliasPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const createFormRef = useRef<HTMLFormElement>(null)
  const [resolved, setResolved] = useState<ImageAlias | null>(null)
  const [created, setCreated] = useState<ImageAlias | null>(null)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [deleteTarget, setDeleteTarget] = useState<ImageAlias | null>(null)
  const query = useInfiniteQuery({
    queryKey: ['aliases'],
    initialPageParam: '',
    queryFn: ({ pageParam }) => apiRequest<ImageAliasList>(`/api/v1/aliases?limit=100${pageParam ? `&cursor=${encodeURIComponent(pageParam)}` : ''}`),
    getNextPageParam: (page) => page.nextCursor || undefined,
  })
  const aliases = useMemo(() => query.data?.pages.flatMap((page) => page.items) ?? [], [query.data])
  const create = useMutation({
    mutationFn: async (value: { path: string; file: File }) => {
      const upload = new FormData()
      upload.append('file', value.file)
      upload.append('alias', value.path)
      const controller = new AbortController()
      const result = await uploadForm<ImportResult>('/api/v1/imports', upload, setUploadProgress, controller.signal)
      return result.alias
    },
    onMutate: () => {
      setUploadProgress(0)
      toast.loading(t('aliases.saving'), { id: 'alias-create' })
    },
    onSuccess: (value) => {
      setCreated(value)
      setUploadProgress(0)
      setSelectedFile(null)
      createFormRef.current?.reset()
      void queryClient.invalidateQueries({ queryKey: ['aliases'] })
      void queryClient.invalidateQueries({ queryKey: ['images'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
      toast.success(t('toast.aliasCreated'), { id: 'alias-create' })
    },
    onError: () => {
      setUploadProgress(0)
      toast.error(t('aliases.createFailed'), { id: 'alias-create' })
    },
  })
  const resolve = useMutation({
    mutationFn: (path: string) => apiRequest<ImageAlias>(`/api/v1/aliases/resolve?path=${encodeURIComponent(path)}`),
    onSuccess: setResolved,
    onError: () => toast.error(t('aliases.notFound')),
  })
  const remove = useMutation({
    mutationFn: (id: string) => apiRequest<void>(`/api/v1/aliases/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      setDeleteTarget(null)
      void queryClient.invalidateQueries({ queryKey: ['aliases'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
      toast.success(t('toast.aliasDeleted'))
    },
    onError: () => toast.error(t('aliases.deleteFailed')),
  })

  function createAlias(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setCreated(null)
    const form = new FormData(event.currentTarget)
    if (!selectedFile || selectedFile.size === 0) {
      toast.error(t('aliases.createFailed'))
      return
    }
    create.mutate({
      path: String(form.get('path') ?? ''),
      file: selectedFile,
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
        <form ref={createFormRef} onSubmit={createAlias}>
          <ComponentCard className="h-full" title={t('aliases.create')}>
            <label className="block font-medium" htmlFor="alias-path">
              {t('aliases.path')}
            </label>
            <Input className="mt-1.5" id="alias-path" name="path" placeholder="/i/2022/05/example.webp" required maxLength={2048} />
            <span className="mt-5 block font-medium">{t('aliases.imageFile')}</span>
            <label className="alias-file-picker mt-1.5">
              <span className="alias-file-picker-icon">
                <Icon name="image" />
              </span>
              <span className="min-w-0 flex-1">
                <strong>{selectedFile?.name ?? t('aliases.imageFile')}</strong>
                <small>{t('aliases.imageHelp')}</small>
              </span>
              <span className="alias-file-picker-action">{t('upload.chooseFiles')}</span>
              <Input
                className="sr-only"
                id="alias-image"
                name="image"
                type="file"
                aria-label={t('aliases.imageFile')}
                accept="image/jpeg,image/png,image/webp,image/gif,.jpg,.jpeg,.png,.webp,.gif"
                required
                onChange={(event) => setSelectedFile(event.target.files?.[0] ?? null)}
              />
            </label>
            <Button className="standard-action-button mt-6" type="submit" disabled={create.isPending || !selectedFile}>
              <Icon name={create.isPending ? 'loader' : 'plus'} className={create.isPending ? 'animate-spin' : ''} />
              {create.isPending ? `${t('aliases.saving')} ${uploadProgress}%` : t('aliases.create')}
            </Button>
            {created ? (
              <p className="mt-4 text-green" aria-live="polite">
                {t('aliases.created', { path: created.path })}
              </p>
            ) : null}
            {create.isError ? <p className="mt-4 text-danger">{t('aliases.createFailed')}</p> : null}
          </ComponentCard>
        </form>
        <form onSubmit={resolveAlias}>
          <ComponentCard className="h-full" title={t('aliases.resolve')}>
            <label className="block font-medium" htmlFor="resolve-alias-path">
              {t('aliases.path')}
            </label>
            <Input className="mt-1.5" id="resolve-alias-path" name="resolvePath" required maxLength={2048} />
            <Button className="mt-6" variant="outline" type="submit" disabled={resolve.isPending}>
              <Icon name="search" />
              {resolve.isPending ? t('common.working') : t('aliases.resolve')}
            </Button>
            {resolved ? (
              <div className="mt-5 rounded-xl bg-canvas p-4">
                <code className="break-all">{resolved.path}</code>
                <p className="mt-2 text-sm text-muted-foreground">
                  <a className="text-primary hover:underline" href={`/image/${resolved.imageId}`} target="_blank" rel="noreferrer">
                    {t('aliases.target')}
                  </a>
                </p>
              </div>
            ) : null}
            {resolve.isError ? <p className="mt-4 text-danger">{t('aliases.notFound')}</p> : null}
          </ComponentCard>
        </form>
      </div>

      <ComponentCard className="mt-6" title={t('aliases.existing')}>
        {query.isLoading ? <p className="mt-4 text-muted-foreground">{t('common.loading')}</p> : null}
        {query.isError ? <p className="mt-4 text-danger">{t('aliases.listFailed')}</p> : null}
        {aliases.length === 0 && !query.isLoading && !query.isError ? <p className="mt-4 text-muted-foreground">{t('aliases.empty')}</p> : null}
        <ul className="mt-4 grid gap-3">
          {aliases.map((alias) => (
            <li className="flex flex-col gap-3 rounded-xl border border-line bg-canvas/60 p-4 sm:flex-row sm:items-center sm:justify-between" key={alias.id}>
              <div className="min-w-0">
                <code className="break-all text-sm">{alias.path}</code>
                <p className="mt-1 text-xs text-muted-foreground">{alias.source}</p>
              </div>
              <div className="flex shrink-0 gap-2">
                <Button
                  size="xs"
                  variant="outline"
                  type="button"
                  onClick={() =>
                    void copyText(new URL(alias.path, window.location.origin).toString())
                      .then(() => toast.success(t('toast.linkCopiedSimple')))
                      .catch(() => toast.error(t('toast.copyFailed')))
                  }
                >
                  <Icon name="copy" />
                  {t('common.copy')}
                </Button>
                <Button size="xs" variant="destructive" type="button" disabled={remove.isPending} onClick={() => setDeleteTarget(alias)}>
                  <Icon name="trash" />
                  {t('common.delete')}
                </Button>
              </div>
            </li>
          ))}
        </ul>
        {query.hasNextPage ? (
          <Button className="mt-4" size="sm" variant="outline" type="button" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
            <Icon name="plus" />
            {query.isFetchingNextPage ? t('common.loading') : t('images.loadMore')}
          </Button>
        ) : null}
        {remove.isError ? <p className="mt-4 text-danger">{t('aliases.deleteFailed')}</p> : null}
      </ComponentCard>
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('aliases.deleteTitle')}
        description={t('aliases.confirmDelete', { path: deleteTarget?.path ?? '' })}
        confirmLabel={t('common.delete')}
        cancelLabel={t('common.cancel')}
        closeLabel={t('common.close')}
        destructive
        pending={remove.isPending}
        onConfirm={() => deleteTarget && remove.mutate(deleteTarget.id)}
      />
    </section>
  )
}
