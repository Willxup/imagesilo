import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { toast } from 'sonner'

import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { ComponentCard } from '../../components/ui/component-card'
import { ConfirmDialog } from '../../components/ui/confirm-dialog'
import { CopyLinkControl } from '../../components/ui/copy-link-control'
import { Icon } from '../../components/ui/icon'
import { apiRequest } from '../../lib/api-client'
import { formatBytes } from '../../lib/image-links'
import type { DeleteImageResult, ImageDetail, Visibility, WebPConversionResult } from '../../lib/api-types'

export function ImageDetailPage() {
  const { t } = useTranslation()
  const { imageId = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const returnTimer = useRef<number | null>(null)
  const [leaving, setLeaving] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [convertOpen, setConvertOpen] = useState(false)

  useEffect(() => () => {
    if (returnTimer.current !== null) window.clearTimeout(returnTimer.current)
  }, [])

  function returnToList() {
    if (leaving) return
    setLeaving(true)
    const state = location.state as { fromImageList?: boolean; returnTo?: string } | null
    const delay = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ? 0 : 220
    returnTimer.current = window.setTimeout(() => {
      if (state?.fromImageList) navigate(-1)
      else navigate(state?.returnTo || '/admin/images')
    }, delay)
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
    onSuccess: (result) => {
      toast.success(result.cleanupPending ? t('toast.imageDeletedCleanup') : t('toast.imageDeleted'), { id: 'detail-delete' })
      void queryClient.invalidateQueries({ queryKey: ['images'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
      navigate('/admin/images', { replace: true })
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

  if (query.isLoading) return <p className="text-muted-foreground">{t('common.loading')}</p>
  if (query.isError || !query.data) return <p className="text-danger">{t('images.detailFailed')}</p>
  const image = query.data
  const canConvert = image.mimeType === 'image/jpeg' || image.mimeType === 'image/png'

  return (
    <section className={`detail-slide-in ${leaving ? 'is-leaving' : ''}`}>
      <Button size="sm" variant="outline" type="button" disabled={leaving} onClick={returnToList}><Icon name="chevronLeft" />{t('images.back')}</Button>
      <div className="image-detail-layout mt-5">
        <Card className="image-detail-preview gap-0 overflow-hidden py-0"><div className="image-detail-stage"><img src={image.standardUrl} alt={image.originalName} /></div></Card>
        <div className="image-detail-summary min-w-0">
          <h1 className="page-title break-words">{image.originalName}</h1>
          <p className="mt-2 text-sm text-muted-foreground">{image.width} × {image.height} · {image.mimeType} · {formatBytes(image.storedSize)}</p>
        </div>
      </div>

      <div className="image-detail-action-bar mt-4">
        <div className="image-detail-actions">
          <Button size="sm" asChild><a href={image.standardUrl} target="_blank" rel="noreferrer"><Icon name="external" />{t('images.openImage')}</a></Button>
          <Button size="sm" variant="outline" type="button" disabled={visibility.isPending} onClick={() => visibility.mutate(image.visibility === 'public' ? 'private' : 'public')}><Icon name={image.visibility === 'public' ? 'visibilityOff' : 'visibility'} />{image.visibility === 'public' ? t('images.makePrivate') : t('images.makePublic')}</Button>
          <CopyLinkControl image={image} />
          {canConvert ? <Button size="sm" variant="outline" type="button" disabled={conversion.isPending} onClick={() => setConvertOpen(true)}><Icon name="wand" />{t('images.convertWebp')}</Button> : null}
          <Button size="sm" variant="destructive" type="button" disabled={deletion.isPending} onClick={() => setDeleteOpen(true)}><Icon name="trash" />{t('common.delete')}</Button>
        </div>
      </div>

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <ComponentCard title={t('images.metadata')}>
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-3 text-sm">
            <Meta label={t('images.visibility')} value={t(`visibility.${image.visibility}`)} />
            <Meta label={t('images.uploadedVia')} value={t(`images.source.${image.uploadedVia}`)} />
            <Meta label={t('images.sourceSize')} value={formatBytes(image.sourceSize)} />
            <Meta label={t('images.storedSize')} value={formatBytes(image.storedSize)} />
            <Meta label={t('images.sha256')} value={image.storedSha256} mono />
            <Meta label={t('images.uploadedAt')} value={new Date(image.createdAt).toLocaleString()} />
            <Meta label={t('images.processing')} value={JSON.stringify(image.processingSummary)} mono />
          </dl>
        </ComponentCard>
        <ComponentCard title={t('images.aliases')}>
          {image.aliases.length === 0 ? <p className="text-muted-foreground">{t('images.noAliases')}</p> : <ul className="grid gap-3">{image.aliases.map((alias) => <li className="rounded-xl bg-canvas p-3" key={alias.id}><code className="break-all text-sm">{alias.path}</code><p className="mt-1 text-xs text-muted-foreground">{alias.source}</p></li>)}</ul>}
        </ComponentCard>
      </div>

      <ConfirmDialog open={deleteOpen} onOpenChange={setDeleteOpen} title={t('images.deleteImageTitle')} description={t('images.confirmDelete', { name: image.originalName })} confirmLabel={t('images.deletePermanently')} cancelLabel={t('common.cancel')} closeLabel={t('common.close')} destructive pending={deletion.isPending} onConfirm={() => deletion.mutate()} />
      <ConfirmDialog open={convertOpen} onOpenChange={setConvertOpen} title={t('images.convertWebpTitle')} description={t('images.confirmConvertWebp', { name: image.originalName })} confirmLabel={t('images.convertWebp')} cancelLabel={t('common.cancel')} closeLabel={t('common.close')} pending={conversion.isPending} onConfirm={() => conversion.mutate()} />
    </section>
  )
}

function Meta({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <><dt className="font-medium text-muted-foreground">{label}</dt><dd className={mono ? 'break-all font-mono text-xs' : 'break-words'}>{value}</dd></>
}
