import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type ClipboardEvent, type DragEvent, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { uploadForm, apiRequest } from '../../lib/api-client'
import { copyText, formatBytes, imageLinks, type LinkFormat } from '../../lib/image-links'
import type { Image, SystemInfo, Visibility } from '../../lib/api-types'

type UploadStatus = 'queued' | 'uploading' | 'complete' | 'failed' | 'canceled'

type UploadItem = {
  id: string
  file: File
  status: UploadStatus
  progress: number
  result?: Image
  controller?: AbortController
}

export function UploadPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const systemQuery = useQuery({ queryKey: ['system'], queryFn: () => apiRequest<SystemInfo>('/api/v1/system') })
  const [items, setItems] = useState<UploadItem[]>([])
  const [visibility, setVisibility] = useState<Visibility | 'default'>('default')
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)

  function addFiles(files: File[]) {
    const supported = files.filter((file) => ['image/jpeg', 'image/png', 'image/webp', 'image/gif'].includes(file.type))
    const maxBatchCount = systemQuery.data?.maxBatchCount ?? 20
    if (items.length + supported.length > maxBatchCount) {
      setError(t('upload.tooMany', { count: maxBatchCount }))
      return
    }
    if (supported.length !== files.length) setError(t('upload.unsupported'))
    else setError('')
    setItems((current) => [...current, ...supported.map((file) => ({
      id: crypto.randomUUID(), file, status: 'queued' as UploadStatus, progress: 0,
    }))])
  }

  function chooseFiles(files: FileList | null) {
    addFiles(Array.from(files ?? []))
  }

  function drop(event: DragEvent<HTMLElement>) {
    event.preventDefault()
    addFiles(Array.from(event.dataTransfer.files))
  }

  function paste(event: ClipboardEvent<HTMLElement>) {
    const files = Array.from(event.clipboardData.files)
    if (files.length > 0) {
      event.preventDefault()
      addFiles(files)
    }
  }

  async function uploadOne(source: UploadItem) {
    const controller = new AbortController()
    updateItem(source.id, { status: 'uploading', progress: 0, result: undefined, controller })
    const body = new FormData()
    body.append('file', source.file)
    if (visibility !== 'default') body.append('visibility', visibility)
    try {
      const image = await uploadForm<Image>('/api/v1/images', body, (progress) => updateItem(source.id, { progress }), controller.signal)
      updateItem(source.id, { status: 'complete', progress: 100, result: image, controller: undefined })
    } catch (uploadError) {
      updateItem(source.id, {
        status: uploadError instanceof DOMException && uploadError.name === 'AbortError' ? 'canceled' : 'failed',
        controller: undefined,
      })
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    const queue = items.filter((item) => item.status !== 'complete')
    if (queue.length === 0) return
    setUploading(true)
    setError('')
    let nextIndex = 0
    const concurrency = Math.min(systemQuery.data?.processingConcurrency ?? 1, queue.length)
    async function worker() {
      while (nextIndex < queue.length) {
        const item = queue[nextIndex]
        nextIndex += 1
        await uploadOne(item)
      }
    }
    await Promise.all(Array.from({ length: concurrency }, () => worker()))
    await queryClient.invalidateQueries({ queryKey: ['images'] })
    await queryClient.invalidateQueries({ queryKey: ['overview'] })
    setUploading(false)
  }

  async function retry(item: UploadItem) {
    if (uploading) return
    setUploading(true)
    await uploadOne(item)
    await queryClient.invalidateQueries({ queryKey: ['images'] })
    await queryClient.invalidateQueries({ queryKey: ['overview'] })
    setUploading(false)
  }

  function updateItem(id: string, patch: Partial<UploadItem>) {
    setItems((current) => current.map((item) => item.id === id ? { ...item, ...patch } : item))
  }

  return (
    <section onPaste={paste}>
      <h1 className="page-title">{t('upload.title')}</h1>
      <p className="page-description">{t('upload.description')}</p>
      <p className="mt-2 text-sm text-muted">{t('upload.runtime', {
        concurrency: systemQuery.data?.processingConcurrency ?? 1,
        count: systemQuery.data?.maxBatchCount ?? 20,
      })}</p>
      <form className="mt-8" onSubmit={(event) => void submit(event)}>
        <div
          className="rounded-3xl border border-dashed border-line bg-panel p-8 text-center"
          onDragOver={(event) => event.preventDefault()}
          onDrop={drop}
        >
          <p className="font-medium">{t('upload.dropOrPaste')}</p>
          <p className="mt-2 text-sm text-muted">{t('upload.dropHelp')}</p>
          <input
            className="mt-5 max-w-full"
            aria-label={t('upload.chooseFiles')}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif,.jpg,.jpeg,.png,.webp,.gif"
            multiple
            onChange={(event) => {
              chooseFiles(event.target.files)
              event.target.value = ''
            }}
          />
        </div>
        <label className="mt-6 block font-medium" htmlFor="upload-visibility">{t('upload.visibility')}</label>
        <select className="field max-w-sm" id="upload-visibility" value={visibility} onChange={(event) => setVisibility(event.target.value as Visibility | 'default')}>
          <option value="default">{t('upload.visibilityDefault')}</option>
          <option value="public">{t('visibility.public')}</option>
          <option value="private">{t('visibility.private')}</option>
        </select>
        <div className="mt-5 flex flex-wrap gap-3">
          <button className="button-primary" type="submit" disabled={items.length === 0 || uploading}>
            {uploading ? t('common.working') : t('upload.uploadCount', { count: items.filter((item) => item.status !== 'complete').length })}
          </button>
          <button className="button-secondary" type="button" disabled={uploading || items.length === 0} onClick={() => setItems([])}>{t('common.clear')}</button>
        </div>
      </form>
      {error ? <p className="mt-5 rounded-xl bg-danger-soft px-4 py-3 text-danger">{error}</p> : null}
      {items.length === 0 ? <p className="mt-6 text-muted">{t('upload.empty')}</p> : null}
      <div className="mt-6 grid gap-3">
        {items.map((item) => <UploadRow item={item} key={item.id} uploading={uploading} onCancel={() => item.controller?.abort()} onRetry={() => void retry(item)} />)}
      </div>
    </section>
  )
}

function UploadRow({ item, uploading, onCancel, onRetry }: {
  item: UploadItem
  uploading: boolean
  onCancel: () => void
  onRetry: () => void
}) {
  const { t } = useTranslation()
  return (
    <article className="rounded-2xl border border-line bg-panel p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate font-medium">{item.file.name}</p>
          <p className="mt-1 text-sm text-muted">{formatBytes(item.file.size)} · {t(`upload.status.${item.status}`)}</p>
        </div>
        <div className="flex gap-2">
          {item.status === 'uploading' ? <button className="button-secondary" type="button" onClick={onCancel}>{t('common.cancel')}</button> : null}
          {(item.status === 'failed' || item.status === 'canceled') ? <button className="button-secondary" type="button" disabled={uploading} onClick={onRetry}>{t('common.retry')}</button> : null}
        </div>
      </div>
      <div className="mt-3 h-2 overflow-hidden rounded-full bg-canvas" aria-label={t('upload.progress', { percent: item.progress })}>
        <div className="h-full bg-accent transition-all" style={{ width: `${item.progress}%` }} />
      </div>
      {item.result ? <UploadResult image={item.result} /> : null}
    </article>
  )
}

function UploadResult({ image }: { image: Image }) {
  const { t } = useTranslation()
  const links = imageLinks(image)
  return (
    <div className="mt-4">
      <p className="text-sm text-muted">{t('upload.result', {
        format: image.extension,
        before: formatBytes(image.sourceSize),
        after: formatBytes(image.storedSize),
        percent: savingsPercent(image),
      })}</p>
      <div className="mt-3 flex flex-wrap gap-2">
        {(Object.keys(links) as LinkFormat[]).map((format) => (
          <button className="button-secondary" type="button" key={format} onClick={() => void copyText(links[format])}>{t(`images.linkFormat.${format}`)}</button>
        ))}
      </div>
    </div>
  )
}

function savingsPercent(image: Image) {
  if (image.sourceSize === 0) return '0.0'
  return (((image.sourceSize - image.storedSize) / image.sourceSize) * 100).toFixed(1)
}
