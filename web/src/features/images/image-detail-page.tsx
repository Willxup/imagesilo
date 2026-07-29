import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { apiRequest } from '../../lib/api-client'
import { copyText, formatBytes, imageLinks, type LinkFormat } from '../../lib/image-links'
import type { DeleteImageResult, ImageDetail, Visibility } from '../../lib/api-types'

export function ImageDetailPage() {
  const { t } = useTranslation()
  const { imageId = '' } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [copied, setCopied] = useState<LinkFormat | ''>('')
  const query = useQuery({ queryKey: ['image', imageId], queryFn: () => apiRequest<ImageDetail>(`/api/v1/images/${imageId}`), enabled: Boolean(imageId) })
  const visibility = useMutation({
    mutationFn: (value: Visibility) => apiRequest<void>(`/api/v1/images/${imageId}/visibility`, {
      method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ visibility: value }),
    }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['image', imageId] })
      void queryClient.invalidateQueries({ queryKey: ['images'] })
    },
  })
  const deletion = useMutation({
    mutationFn: () => apiRequest<DeleteImageResult>(`/api/v1/images/${imageId}`, { method: 'DELETE' }),
    onSuccess: (result) => {
      void queryClient.invalidateQueries({ queryKey: ['images'] })
      void queryClient.invalidateQueries({ queryKey: ['overview'] })
      navigate('/admin/images', { replace: true, state: { cleanupPending: result.cleanupPending } })
    },
  })

  if (query.isLoading) return <p className="text-muted">{t('common.loading')}</p>
  if (query.isError || !query.data) return <p className="text-danger">{t('images.detailFailed')}</p>
  const image = query.data
  const links = imageLinks(image)

  async function copy(format: LinkFormat) {
    await copyText(links[format])
    setCopied(format)
  }

  function remove() {
    if (window.confirm(t('images.confirmDelete', { name: image.originalName }))) deletion.mutate()
  }

  return (
    <section>
      <Link className="text-accent hover:underline" to="/admin/images">← {t('images.back')}</Link>
      <div className="mt-5 grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.8fr)]">
        <div className="overflow-hidden rounded-2xl border border-line bg-panel">
          <img className="max-h-[70vh] w-full bg-canvas object-contain" src={image.thumbnailUrl} alt={image.originalName} />
        </div>
        <div>
          <h1 className="page-title break-words">{image.originalName}</h1>
          <p className="mt-2 text-muted">{image.width} × {image.height} · {image.mimeType} · {formatBytes(image.storedSize)}</p>
          <div className="mt-5 flex flex-wrap gap-2">
            <a className="button-primary" href={image.standardUrl} target="_blank" rel="noreferrer">{t('images.openImage')}</a>
            <button className="button-secondary" type="button" disabled={visibility.isPending} onClick={() => visibility.mutate(image.visibility === 'public' ? 'private' : 'public')}>
              {image.visibility === 'public' ? t('images.makePrivate') : t('images.makePublic')}
            </button>
            <button className="button-danger" type="button" disabled={deletion.isPending} onClick={remove}>{t('images.deletePermanently')}</button>
          </div>

          <section className="mt-6 rounded-2xl border border-line bg-panel p-5">
            <h2 className="text-lg font-semibold">{t('images.copyLinks')}</h2>
            <div className="mt-4 grid gap-3">
              {(Object.keys(links) as LinkFormat[]).map((format) => (
                <div className="flex flex-col gap-2 sm:flex-row" key={format}>
                  <code className="min-w-0 flex-1 overflow-x-auto rounded-lg bg-canvas p-3 text-xs">{links[format]}</code>
                  <button className="button-secondary" type="button" onClick={() => void copy(format)}>{copied === format ? t('common.copied') : t(`images.linkFormat.${format}`)}</button>
                </div>
              ))}
            </div>
          </section>
        </div>
      </div>

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <section className="rounded-2xl border border-line bg-panel p-5">
          <h2 className="text-lg font-semibold">{t('images.metadata')}</h2>
          <dl className="mt-4 grid grid-cols-[auto_1fr] gap-x-4 gap-y-3 text-sm">
            <Meta label={t('images.visibility')} value={t(`visibility.${image.visibility}`)} />
            <Meta label={t('images.uploadedVia')} value={t(`images.source.${image.uploadedVia}`)} />
            <Meta label={t('images.sourceSize')} value={formatBytes(image.sourceSize)} />
            <Meta label={t('images.storedSize')} value={formatBytes(image.storedSize)} />
            <Meta label={t('images.sha256')} value={image.storedSha256} mono />
            <Meta label={t('images.uploadedAt')} value={new Date(image.createdAt).toLocaleString()} />
            <Meta label={t('images.processing')} value={JSON.stringify(image.processingSummary)} mono />
          </dl>
        </section>
        <section className="rounded-2xl border border-line bg-panel p-5">
          <h2 className="text-lg font-semibold">{t('images.aliases')}</h2>
          {image.aliases.length === 0 ? <p className="mt-4 text-muted">{t('images.noAliases')}</p> : (
            <ul className="mt-4 grid gap-3">
              {image.aliases.map((alias) => <li className="rounded-xl bg-canvas p-3" key={alias.id}><code className="break-all text-sm">{alias.path}</code><p className="mt-1 text-xs text-muted">{alias.source}</p></li>)}
            </ul>
          )}
        </section>
      </div>
      {(visibility.isError || deletion.isError) ? <p className="mt-5 text-danger">{t('images.operationFailed')}</p> : null}
    </section>
  )
}

function Meta({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return <><dt className="font-medium text-muted">{label}</dt><dd className={mono ? 'break-all font-mono text-xs' : 'break-words'}>{value}</dd></>
}
