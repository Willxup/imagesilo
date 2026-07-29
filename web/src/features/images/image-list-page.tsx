import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { apiRequest } from '../../lib/api-client'
import type { ImageList } from '../../lib/api-types'

export function ImageListPage() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['images'],
    queryFn: () => apiRequest<ImageList>('/api/v1/images?limit=50'),
  })

  return (
    <section>
      <h1 className="page-title">{t('images.title')}</h1>
      <p className="page-description">{t('images.description')}</p>
      {query.isLoading ? <p className="mt-8 text-muted">{t('common.loading')}</p> : null}
      {query.isError ? <p className="mt-8 text-danger">{t('images.failed')}</p> : null}
      {query.data?.items.length === 0 ? <p className="mt-8 text-muted">{t('images.empty')}</p> : null}
      <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {query.data?.items.map((image) => (
          <article className="overflow-hidden rounded-2xl border border-line bg-panel" key={image.id}>
            <a href={image.standardUrl} target="_blank" rel="noreferrer">
              <img className="aspect-video w-full bg-canvas object-contain" src={image.standardUrl} alt={image.originalName} loading="lazy" />
            </a>
            <div className="p-4">
              <p className="truncate font-medium" title={image.originalName}>{image.originalName}</p>
              <p className="mt-1 text-sm text-muted">{image.width} × {image.height} · {formatBytes(image.storedSize)}</p>
            </div>
          </article>
        ))}
      </div>
    </section>
  )
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  return `${(value / 1024 / 1024).toFixed(1)} MiB`
}
