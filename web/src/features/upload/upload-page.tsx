import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { apiRequest } from '../../lib/api-client'
import type { Image, SystemInfo, Visibility } from '../../lib/api-types'

type UploadStatus = 'queued' | 'uploading' | 'complete' | 'failed'

type UploadItem = {
  file: File
  status: UploadStatus
  result?: Image
}

export function UploadPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const systemQuery = useQuery({
    queryKey: ['system'],
    queryFn: () => apiRequest<SystemInfo>('/api/v1/system'),
  })
  const [items, setItems] = useState<UploadItem[]>([])
  const [visibility, setVisibility] = useState<Visibility | 'default'>('default')
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)

  function chooseFiles(files: FileList | null) {
    const selected = Array.from(files ?? [])
    const maxBatchCount = systemQuery.data?.maxBatchCount ?? 20
    if (selected.length > maxBatchCount) {
      setItems([])
      setError(t('upload.tooMany', { count: maxBatchCount }))
      return
    }
    setError('')
    setItems(selected.map((file) => ({ file, status: 'queued' })))
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (items.length === 0) return
    setUploading(true)
    setError('')
    setItems((current) => current.map((item) => ({ ...item, status: 'queued', result: undefined })))
    const sourceItems = items.map((item) => ({ ...item, status: 'queued' as UploadStatus, result: undefined }))
    let nextIndex = 0
    const concurrency = Math.min(systemQuery.data?.processingConcurrency ?? 1, sourceItems.length)

    async function worker() {
      while (nextIndex < sourceItems.length) {
        const index = nextIndex
        nextIndex += 1
        const item = sourceItems[index]
        setItems((current) => current.map((value, itemIndex) => itemIndex === index ? { ...value, status: 'uploading' } : value))
        const body = new FormData()
        body.append('file', item.file)
        if (visibility !== 'default') body.append('visibility', visibility)
        try {
          const image = await apiRequest<Image>('/api/v1/images', { method: 'POST', body })
          setItems((current) => current.map((value, itemIndex) => itemIndex === index ? { ...value, status: 'complete', result: image } : value))
        } catch {
          setItems((current) => current.map((value, itemIndex) => itemIndex === index ? { ...value, status: 'failed' } : value))
        }
      }
    }

    await Promise.all(Array.from({ length: concurrency }, () => worker()))
    await queryClient.invalidateQueries({ queryKey: ['images'] })
    setUploading(false)
  }

  return (
    <section>
      <h1 className="page-title">{t('upload.title')}</h1>
      <p className="page-description">{t('upload.description')}</p>
      <p className="mt-2 text-sm text-muted">
        {t('upload.runtime', {
          concurrency: systemQuery.data?.processingConcurrency ?? 1,
          count: systemQuery.data?.maxBatchCount ?? 20,
        })}
      </p>
      <form className="mt-8 rounded-3xl border border-dashed border-line bg-panel p-8" onSubmit={(event) => void submit(event)}>
        <input
          aria-label={t('upload.chooseFiles')}
          type="file"
          accept="image/jpeg,image/png,image/webp,image/gif,.jpg,.jpeg,.png,.webp,.gif"
          multiple
          required
          onChange={(event) => chooseFiles(event.target.files)}
        />
        <label className="mt-6 block font-medium" htmlFor="upload-visibility">{t('upload.visibility')}</label>
        <select
          className="field"
          id="upload-visibility"
          value={visibility}
          onChange={(event) => setVisibility(event.target.value as Visibility | 'default')}
        >
          <option value="default">{t('upload.visibilityDefault')}</option>
          <option value="public">{t('visibility.public')}</option>
          <option value="private">{t('visibility.private')}</option>
        </select>
        <button className="button-primary mt-6" type="submit" disabled={items.length === 0 || uploading}>
          {uploading ? t('common.working') : t('upload.uploadCount', { count: items.length })}
        </button>
      </form>
      {error ? <p className="mt-5 rounded-xl bg-danger-soft px-4 py-3 text-danger">{error}</p> : null}
      <div className="mt-6 grid gap-3">
        {items.map((item, index) => (
          <article className="rounded-2xl border border-line bg-panel p-4" key={`${item.file.name}-${item.file.lastModified}-${index}`}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="font-medium">{item.file.name}</p>
                <p className="mt-1 text-sm text-muted">{formatBytes(item.file.size)} · {t(`upload.status.${item.status}`)}</p>
              </div>
              {item.result ? (
                <a className="break-all text-sm text-accent underline" href={item.result.standardUrl} target="_blank" rel="noreferrer">
                  {item.result.standardUrl}
                </a>
              ) : null}
            </div>
            {item.result ? (
              <p className="mt-2 text-sm text-muted">
                {t('upload.result', {
                  format: item.result.extension,
                  before: formatBytes(item.result.sourceSize),
                  after: formatBytes(item.result.storedSize),
                  percent: savingsPercent(item.result),
                })}
              </p>
            ) : null}
          </article>
        ))}
      </div>
    </section>
  )
}

function savingsPercent(image: Image) {
  if (image.sourceSize === 0) return '0.0'
  return (((image.sourceSize - image.storedSize) / image.sourceSize) * 100).toFixed(1)
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  return `${(value / 1024 / 1024).toFixed(1)} MiB`
}
