import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useLayoutEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

import { Button } from '../../components/ui/button'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { CopyLinkControl } from '../../components/ui/copy-link-control'
import { Icon } from '../../components/ui/icon'
import { Input } from '../../components/ui/input'
import { apiRequest } from '../../lib/api-client'
import { formatBytes } from '../../lib/image-links'
import type { DeleteImageResult, ImageAlias, ImageDetail, Visibility, WebPConversionResult } from '../../lib/api-types'

export function ImageDetailPage() {
  const { t, i18n } = useTranslation()
  const { imageId = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const currentEntry = useRef<string | null>(location.key)
  useLayoutEffect(() => {
    currentEntry.current = location.key
    // Browser history changes before a concurrent route transition commits.
    const invalidateReturn = () => { currentEntry.current = null }
    window.addEventListener('popstate', invalidateReturn)
    return () => {
      currentEntry.current = null
      window.removeEventListener('popstate', invalidateReturn)
    }
  }, [location.key])
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [convertOpen, setConvertOpen] = useState(false)
  const [aliasPath, setAliasPath] = useState('')

  useLayoutEffect(() => { window.scrollTo({ top: 0, behavior: 'instant' }) }, [imageId])

  function returnToList() {
    const state = location.state as { fromImageList?: boolean; returnTo?: string } | null
    if (state?.fromImageList) navigate(-1)
    else navigate(state?.returnTo || '/admin/images', { viewTransition: true })
  }
  const query = useQuery({ queryKey: ['image', imageId], queryFn: () => apiRequest<ImageDetail>(`/api/v1/images/${imageId}`), enabled: Boolean(imageId) })
  const visibility = useMutation({
    mutationFn: (value: Visibility) => apiRequest<void>(`/api/v1/images/${imageId}/visibility`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ visibility: value }) }),
    onMutate: () => toast.loading(t('common.working'), { id: 'detail-visibility' }),
    onSuccess: (_, value) => {
      toast.success(value === 'public' ? t('toast.imagePublic') : t('toast.imagePrivate'), { id: 'detail-visibility' })
      void queryClient.invalidateQueries({ queryKey: ['image', imageId] })
      void queryClient.invalidateQueries({ queryKey: ['images'] })
    },
    onError: () => toast.error(t('toast.operationFailed'), { id: 'detail-visibility' }),
  })
  const deletion = useMutation({
    mutationFn: () => apiRequest<DeleteImageResult>(`/api/v1/images/${imageId}`, { method: 'DELETE' }),
    onMutate: () => toast.loading(t('common.working'), { id: 'detail-delete' }),
    onSuccess: async (result) => {
      toast.success(result.cleanupPending ? t('toast.imageDeletedCleanup') : t('toast.imageDeleted'), { id: 'detail-delete' })
      // Refresh cached lists before restoring their position, including shorter last pages.
      await queryClient.invalidateQueries({ queryKey: ['images'], refetchType: 'all' })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
      if (currentEntry.current !== location.key) return
      const state = location.state as { fromImageList?: boolean; returnTo?: string } | null
      if (state?.fromImageList) navigate(-1)
      else navigate(state?.returnTo || '/admin/images', { replace: true })
    },
    onError: () => toast.error(t('toast.operationFailed'), { id: 'detail-delete' }),
  })
  const conversion = useMutation({
    mutationFn: () => apiRequest<WebPConversionResult>(`/api/v1/images/${imageId}/convert-webp`, { method: 'POST' }),
    onMutate: () => toast.loading(t('common.working'), { id: 'detail-conversion' }),
    onSuccess: (result) => {
      setConvertOpen(false)
      toast.success(result.cleanupPending ? t('toast.webpConvertedCleanup') : t('toast.webpConverted'), { id: 'detail-conversion' })
      void queryClient.invalidateQueries({ queryKey: ['image', imageId] })
      void queryClient.invalidateQueries({ queryKey: ['images'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
    },
    onError: () => toast.error(t('toast.webpFailed'), { id: 'detail-conversion' }),
  })
  const aliasCreation = useMutation({
    mutationFn: (path: string) =>
      apiRequest<ImageAlias>('/api/v1/aliases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, imageId, source: 'admin' }),
      }),
    onMutate: () => toast.loading(t('images.aliasSaving'), { id: 'detail-alias' }),
    onSuccess: async () => {
      setAliasPath('')
      await queryClient.invalidateQueries({ queryKey: ['image', imageId] })
      await queryClient.invalidateQueries({ queryKey: ['aliases'] })
      toast.success(t('toast.aliasCreated'), { id: 'detail-alias' })
    },
    onError: () => toast.error(t('images.aliasCreateFailed'), { id: 'detail-alias' }),
  })

  function createAlias(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const path = aliasPath.trim()
    if (path) aliasCreation.mutate(path)
  }

  if (query.isLoading) {
    const preview = (location.state as { preview?: { url?: string; name?: string } } | null)?.preview
    return <section className="detail-page detail-loading">
      <Button size="sm" variant="ghost" type="button" onClick={returnToList}><Icon name="chevronLeft" />{t('images.back')}</Button>
      <div className="image-detail-layout mt-5" role="status" aria-label={t('common.loading')}>
        <div className="image-detail-preview"><div className="image-detail-stage">{preview?.url ? <img decoding="async" src={preview.url} alt={preview.name ?? ''} style={{ viewTransitionName: `image-${imageId}` }} /> : <Icon name="loader" className="animate-spin" />}</div></div>
        <aside className="image-detail-summary"><p className="page-kicker">{t('images.detailEyebrow')}</p><h1 className="page-title">{preview?.name ?? t('images.detailEyebrow')}</h1><p className="page-description">{t('common.loading')}</p></aside>
      </div>
    </section>
  }
  if (query.isError || !query.data) return <div className="empty-state" role="alert"><p>{t('images.detailFailed')}</p><Button variant="outline" type="button" onClick={() => void query.refetch()}>{t('images.retryLoad')}</Button><Button variant="ghost" type="button" onClick={returnToList}>{t('images.back')}</Button></div>
  const image = query.data
  const canConvert = image.mimeType === 'image/jpeg' || image.mimeType === 'image/png'

  return (
    <section className="detail-page">
      <Button size="sm" variant="ghost" type="button" onClick={returnToList}><Icon name="chevronLeft" />{t('images.back')}</Button>
      <div className="image-detail-layout mt-5">
        <div className="image-detail-preview"><div className="image-detail-stage"><img decoding="async" src={image.standardUrl} alt={image.originalName} style={{ viewTransitionName: `image-${imageId}` }} /></div></div>
        <aside className="image-detail-summary min-w-0">
          <p className="page-kicker">{t('images.detailEyebrow')}</p>
          <h1 className="page-title break-words">{image.originalName}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{image.width} × {image.height} · {image.mimeType} · {formatBytes(image.storedSize)}</p>
          <div className="detail-visibility"><span>{t('images.visibility')} <strong>{t(`visibility.${image.visibility}`)}</strong></span><Button variant="ghost" type="button" disabled={visibility.isPending} onClick={() => visibility.mutate(image.visibility === 'public' ? 'private' : 'public')}><Icon name={image.visibility === 'public' ? 'visibilityOff' : 'visibility'} />{image.visibility === 'public' ? t('images.makePrivate') : t('images.makePublic')}</Button></div>
          <div className="image-detail-actions">
          <CopyLinkControl image={image} prominent />
          <Button size="sm" asChild variant="outline"><a href={image.standardUrl} target="_blank" rel="noreferrer"><Icon name="external" />{t('images.openImage')}</a></Button>
          </div>
          <dl className="detail-core-info"><dt>{t('images.uploadedAt')}</dt><dd>{new Date(image.createdAt).toLocaleString(i18n.resolvedLanguage ?? i18n.language)}</dd><dt>{t('images.uploadedVia')}</dt><dd>{t(`images.source.${image.uploadedVia}`)}</dd><dt>{t('images.storedSize')}</dt><dd>{formatBytes(image.storedSize)}</dd><dt>{t('images.format')}</dt><dd>{image.extension.toUpperCase()}</dd></dl>
        </aside>
      </div>

      <div className="detail-secondary-grid">
        <section className="detail-secondary">
          <h2>{t('images.metadata')}</h2>
          <dl>
            <Meta label={t('images.sourceSize')} value={formatBytes(image.sourceSize)} />
            <Meta label={t('images.sha256')} value={image.storedSha256} mono />
            <Meta label={t('images.processing')} value={JSON.stringify(image.processingSummary)} mono />
          </dl>
        </section>
        <section className="detail-secondary">
          <h2>{t('images.aliases')}</h2>
          <form className="mb-4 flex flex-col gap-2 sm:flex-row" onSubmit={createAlias}>
            <label className="sr-only" htmlFor="detail-alias-path">{t('images.aliasPath')}</label>
            <Input className="min-w-0 flex-1" id="detail-alias-path" value={aliasPath} onChange={(event) => setAliasPath(event.target.value)} placeholder="/i/2026/08/example.jpg" maxLength={2048} required />
            <Button type="submit" variant="outline" disabled={!aliasPath.trim() || aliasCreation.isPending}><Icon name={aliasCreation.isPending ? 'loader' : 'plus'} className={aliasCreation.isPending ? 'animate-spin' : ''} />{aliasCreation.isPending ? t('images.aliasSaving') : t('images.addAlias')}</Button>
          </form>
          {image.aliases.length === 0 ? <p className="text-muted-foreground">{t('images.noAliases')}</p> : <ul className="grid gap-3">{image.aliases.map((alias) => <li className="rounded-xl bg-canvas p-3" key={alias.id}><code className="break-all text-sm">{alias.path}</code><p className="mt-1 text-xs text-muted-foreground">{alias.source}</p></li>)}</ul>}
        </section>
      </div>
      <div className="detail-danger-actions">
          {canConvert ? <Button size="sm" variant="outline" type="button" disabled={conversion.isPending} onClick={() => setConvertOpen(true)}><Icon name="wand" />{t('images.convertWebp')}</Button> : null}
          <Button size="sm" variant="destructive" type="button" disabled={deletion.isPending} onClick={() => setDeleteOpen(true)}><Icon name="trash" />{t('common.delete')}</Button>
      </div>

      <ConfirmDialog open={deleteOpen} onOpenChange={setDeleteOpen} title={t('images.deleteImageTitle')} description={t('images.confirmDelete', { name: image.originalName })} confirmLabel={t('images.deletePermanently')} cancelLabel={t('common.cancel')} closeLabel={t('common.close')} destructive pending={deletion.isPending} onConfirm={() => deletion.mutate()} />
      <ConfirmDialog open={convertOpen} onOpenChange={setConvertOpen} title={t('images.convertWebpTitle')} description={t('images.confirmConvertWebp', { name: image.originalName })} confirmLabel={t('images.convertWebp')} cancelLabel={t('common.cancel')} closeLabel={t('common.close')} pending={conversion.isPending} onConfirm={() => conversion.mutate()} />
    </section>
  )
}

function Meta({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <><dt className="font-medium text-muted-foreground">{label}</dt><dd className={mono ? 'break-all font-mono text-xs' : 'break-words'}>{value}</dd></>
}
