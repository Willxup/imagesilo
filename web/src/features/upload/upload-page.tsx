import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState, type ClipboardEvent, type DragEvent, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { ComponentCard } from '../../components/ui/component-card'
import { CopyLinkControl } from '../../components/ui/copy-link-control'
import { Icon } from '../../components/ui/icon'
import { Select } from '../../components/ui/select'
import { ApiError, apiRequest, uploadForm } from '../../lib/api-client'
import { formatBytes } from '../../lib/image-links'
import type { Image, SystemInfo, Visibility } from '../../lib/api-types'

type UploadStatus = 'queued' | 'uploading' | 'complete' | 'failed' | 'canceled'

type UploadItem = {
  id: string
  file: File
  status: UploadStatus
  progress: number
  result?: Image
  controller?: AbortController
  error?: string
}

export function UploadPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const systemQuery = useQuery({ queryKey: ['system'], queryFn: () => apiRequest<SystemInfo>('/api/v1/system') })
  const [items, setItems] = useState<UploadItem[]>([])
  const [visibility, setVisibility] = useState<Visibility | 'default'>('default')
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)
  const [dragActive, setDragActive] = useState(false)

  function addFiles(files: File[]) {
    const supported = files.filter(isSupportedImageFile)
    const maxBatchCount = systemQuery.data?.maxBatchCount ?? 20
    if (items.length + supported.length > maxBatchCount) {
      setError(t('upload.tooMany', { count: maxBatchCount }))
      return
    }
    if (supported.length !== files.length) setError(t('upload.unsupported'))
    else setError('')
    setItems((current) => [
      ...current,
      ...supported.map((file) => ({
        id: crypto.randomUUID(),
        file,
        status: 'queued' as UploadStatus,
        progress: 0,
      })),
    ])
  }

  function chooseFiles(files: FileList | null) {
    addFiles(Array.from(files ?? []))
  }

  function drop(event: DragEvent<HTMLElement>) {
    event.preventDefault()
    event.stopPropagation()
    setDragActive(false)
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
    updateItem(source.id, { status: 'uploading', progress: 0, result: undefined, controller, error: undefined })
    const body = new FormData()
    body.append('file', source.file)
    if (visibility !== 'default') body.append('visibility', visibility)
    try {
      const image = await uploadForm<Image>('/api/v1/images', body, (progress) => updateItem(source.id, { progress }), controller.signal)
      updateItem(source.id, { status: 'complete', progress: 100, result: image, controller: undefined, error: undefined })
      return true
    } catch (uploadError) {
      updateItem(source.id, {
        status: uploadError instanceof DOMException && uploadError.name === 'AbortError' ? 'canceled' : 'failed',
        controller: undefined,
        error: uploadError instanceof ApiError ? uploadError.message : uploadError instanceof Error ? uploadError.message : t('toast.uploadFailed'),
      })
      return false
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    const queue = items.filter((item) => item.status !== 'complete')
    if (queue.length === 0) return
    setUploading(true)
    setError('')
    let nextIndex = 0
    let completed = 0
    const concurrency = Math.min(systemQuery.data?.processingConcurrency ?? 1, queue.length)
    async function worker() {
      while (nextIndex < queue.length) {
        const item = queue[nextIndex]
        nextIndex += 1
        if (await uploadOne(item)) completed += 1
      }
    }
    await Promise.all(Array.from({ length: concurrency }, () => worker()))
    await queryClient.invalidateQueries({ queryKey: ['images'] })
    await queryClient.invalidateQueries({ queryKey: ['overview'] })
    setUploading(false)
    if (completed === queue.length) toast.success(t('toast.uploadComplete', { count: completed }))
    else toast.warning(t('toast.uploadPartial', { success: completed, failed: queue.length - completed }))
  }

  async function retry(item: UploadItem) {
    if (uploading) return
    setUploading(true)
    const succeeded = await uploadOne(item)
    await queryClient.invalidateQueries({ queryKey: ['images'] })
    await queryClient.invalidateQueries({ queryKey: ['overview'] })
    setUploading(false)
    if (succeeded) toast.success(t('toast.uploadComplete', { count: 1 }))
    else toast.error(t('toast.uploadFailed'))
  }

  function updateItem(id: string, patch: Partial<UploadItem>) {
    setItems((current) => current.map((item) => (item.id === id ? { ...item, ...patch } : item)))
  }

  return (
    <section onPaste={paste}>
      <div className="page-heading-row">
        <div>
          <h1 className="page-title">{t('upload.title')}</h1>
          <p className="page-description">{t('upload.description')}</p>
        </div>
        <div className="runtime-chips" aria-hidden="true">
          <span className="runtime-chip">
            <Icon name="zap" />
            <span>{t('upload.concurrency')}</span>
            <strong>{systemQuery.data?.processingConcurrency ?? 1}</strong>
          </span>
          <span className="runtime-chip">
            <Icon name="images" />
            <span>{t('upload.batch')}</span>
            <strong>{systemQuery.data?.maxBatchCount ?? 20}</strong>
          </span>
        </div>
      </div>
      <form onSubmit={(event) => void submit(event)}>
        <ComponentCard className="upload-panel mt-8" title={t('upload.chooseFiles')}>
          <label
            className={`upload-dropzone ${dragActive ? 'is-dragging' : ''}`}
            htmlFor="upload-files"
            onDragEnter={(event) => {
              event.preventDefault()
              setDragActive(true)
            }}
            onDragOver={(event) => {
              event.preventDefault()
              event.dataTransfer.dropEffect = 'copy'
              setDragActive(true)
            }}
            onDragLeave={(event) => {
              if (event.relatedTarget instanceof Node && event.currentTarget.contains(event.relatedTarget)) return
              setDragActive(false)
            }}
            onDrop={drop}
          >
            <div className="upload-dropzone-content">
              <div className="upload-icon">
                <Icon name="cloudUpload" className="h-8 w-8" />
              </div>
              <h2 className="text-xl font-semibold tracking-tight">{t('upload.dropOrPaste')}</h2>
              <p className="mt-3 text-sm leading-6 text-muted-foreground">{t('upload.dropHelp')}</p>
              <span className="upload-dropzone-action">
                <Icon name="image" />
                {t('upload.chooseFiles')}
              </span>
            </div>
          </label>
          <input
            className="sr-only"
            id="upload-files"
            aria-label={t('upload.chooseFiles')}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif,.jpg,.jpeg,.png,.webp,.gif"
            multiple
            onChange={(event) => {
              chooseFiles(event.target.files)
              event.target.value = ''
            }}
          />
          <div className="upload-controls">
            <label className="block min-w-0 flex-1 font-medium" htmlFor="upload-visibility">
              <span className="text-sm text-muted-foreground">{t('upload.visibility')}</span>
              <Select
                className="mt-1.5 max-w-sm"
                id="upload-visibility"
                ariaLabel={t('upload.visibility')}
                value={visibility}
                onValueChange={(value) => setVisibility(value as Visibility | 'default')}
                options={[
                  { value: 'default', label: t('upload.visibilityDefault') },
                  { value: 'public', label: t('visibility.public') },
                  { value: 'private', label: t('visibility.private') },
                ]}
              />
            </label>
            <div className="flex flex-wrap gap-3">
              <Button className="standard-action-button" type="submit" disabled={items.length === 0 || uploading}>
                <Icon name="upload" />
                {uploading ? t('common.working') : t('upload.uploadCount', { count: items.filter((item) => item.status !== 'complete').length })}
              </Button>
              <Button
                className="standard-action-button"
                variant="outline"
                type="button"
                disabled={uploading || items.length === 0}
                onClick={() => setItems([])}
              >
                <Icon name="x" />
                {t('common.clear')}
              </Button>
            </div>
          </div>
        </ComponentCard>
      </form>
      {error ? <p className="mt-5 rounded-xl bg-danger-soft px-4 py-3 text-danger">{error}</p> : null}
      {items.length === 0 ? (
        <div className="empty-state">
          <div>
            <span className="empty-state-icon">
              <Icon name="image" />
            </span>
            <p>{t('upload.empty')}</p>
          </div>
        </div>
      ) : null}
      <div className="upload-queue mt-6 grid gap-3">
        {items.map((item) => (
          <UploadRow item={item} key={item.id} uploading={uploading} onCancel={() => item.controller?.abort()} onRetry={() => void retry(item)} />
        ))}
      </div>
    </section>
  )
}

function UploadRow({ item, uploading, onCancel, onRetry }: { item: UploadItem; uploading: boolean; onCancel: () => void; onRetry: () => void }) {
  const { t } = useTranslation()
  return (
    <article>
      <Card size="sm" className="p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <span className="nav-icon-wrap text-cyan">
              <Icon name={item.status === 'complete' ? 'check' : 'image'} />
            </span>
            <div className="min-w-0">
              <p className="truncate font-medium">{item.file.name}</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {formatBytes(item.file.size)} · {t(`upload.status.${item.status}`)}
              </p>
            </div>
          </div>
          <div className="flex gap-2">
            {item.status === 'uploading' ? (
              <Button size="sm" variant="outline" type="button" onClick={onCancel}>
                <Icon name="x" />
                {t('common.cancel')}
              </Button>
            ) : null}
            {item.status === 'failed' || item.status === 'canceled' ? (
              <Button size="sm" variant="outline" type="button" disabled={uploading} onClick={onRetry}>
                <Icon name="refresh" />
                {t('common.retry')}
              </Button>
            ) : null}
          </div>
        </div>
        <div className="mt-3 h-2 overflow-hidden rounded-full bg-canvas" aria-label={t('upload.progress', { percent: item.progress })}>
          <div className="h-full bg-primary transition-all" style={{ width: `${item.progress}%` }} />
        </div>
        {item.result ? <UploadResult image={item.result} /> : null}
        {item.error ? (
          <p className="mt-3 text-sm text-danger" role="alert">
            {item.error}
          </p>
        ) : null}
      </Card>
    </article>
  )
}

function UploadResult({ image }: { image: Image }) {
  const { t } = useTranslation()
  return (
    <div className="mt-4">
      <p className="text-sm text-muted-foreground">
        {t('upload.result', {
          format: image.extension,
          before: formatBytes(image.sourceSize),
          after: formatBytes(image.storedSize),
          percent: savingsPercent(image),
        })}
      </p>
      <div className="mt-3">
        <CopyLinkControl image={image} />
      </div>
    </div>
  )
}

function savingsPercent(image: Image) {
  if (image.sourceSize === 0) return '0.0'
  return (((image.sourceSize - image.storedSize) / image.sourceSize) * 100).toFixed(1)
}

function isSupportedImageFile(file: File) {
  if (['image/jpeg', 'image/png', 'image/webp', 'image/gif'].includes(file.type)) return true
  return file.type === '' && /\.(?:jpe?g|png|webp|gif)$/i.test(file.name)
}
