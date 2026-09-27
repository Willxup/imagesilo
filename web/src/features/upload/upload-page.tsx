import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState, type ClipboardEvent, type DragEvent, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '../../components/ui/button'
import { Card } from '../../components/ui/card'
import { CopyLinkControl, CopyLinksControl } from '../../components/ui/copy-link-control'
import { Icon } from '../../components/ui/icon'
import { Select } from '../../components/ui/select'
import { ApiError, apiRequest, uploadForm } from '../../lib/api-client'
import { readLocalStorage, writeLocalStorage } from '../../lib/browser-storage'
import { formatBytes } from '../../lib/image-links'
import type { Image, SystemInfo, Visibility } from '../../lib/api-types'

type UploadStatus = 'queued' | 'uploading' | 'complete' | 'failed' | 'canceled'

type UploadItem = {
  id: string
  file: File
  previewUrl: string
  status: UploadStatus
  progress: number
  result?: Image
  controller?: AbortController
  error?: string
}

const visibilityStorageKey = 'imagesilo_upload_visibility'

export function UploadPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const systemQuery = useQuery({ queryKey: ['system'], queryFn: () => apiRequest<SystemInfo>('/api/v1/system') })
  const [items, setItems] = useState<UploadItem[]>([])
  const [visibility, setVisibility] = useState<Visibility | 'default'>(storedVisibility)
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)
  const [dragActive, setDragActive] = useState(false)
  const previewUrls = useRef(new Set<string>())
  const completedImages = useMemo(() => items.flatMap((item) => (item.result ? [item.result] : [])), [items])

  useEffect(() => {
    writeLocalStorage(visibilityStorageKey, visibility)
  }, [visibility])

  useEffect(() => () => {
    for (const url of previewUrls.current) URL.revokeObjectURL(url)
    previewUrls.current.clear()
  }, [])

  function clearQueue() {
    for (const url of previewUrls.current) URL.revokeObjectURL(url)
    previewUrls.current.clear()
    setItems([])
  }

  function addFiles(files: File[]) {
    const supported = files.filter(isSupportedImageFile)
    const maxBatchCount = systemQuery.data?.maxBatchCount ?? 20
    if (items.length + supported.length > maxBatchCount) {
      setError(t('upload.tooMany', { count: maxBatchCount }))
      return
    }
    if (supported.length !== files.length) setError(t('upload.unsupported'))
    else setError('')
    const additions = supported.map((file) => ({
        id: crypto.randomUUID(),
        file,
        previewUrl: makePreviewUrl(file),
        status: 'queued' as UploadStatus,
        progress: 0,
      }))
    setItems((current) => [...current, ...additions])
  }

  function makePreviewUrl(file: File) {
    if (typeof URL.createObjectURL !== 'function') return ''
    const url = URL.createObjectURL(file)
    previewUrls.current.add(url)
    return url
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
          <p className="page-kicker">{t('upload.eyebrow')}</p>
          <h1 className="page-title">{t('upload.title')}</h1>
          <p className="page-description">{t('upload.description')}</p>
        </div>
      </div>
      <form onSubmit={(event) => void submit(event)}>
        <div className="upload-studio mt-8">
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
              <div className="fold-art" aria-hidden="true"><span /><span /><span /></div>
              <h2>{t('upload.dropOrPaste')}</h2>
              <p>{t('upload.dropHelp')}</p>
              <span className="upload-dropzone-action">
                <Icon name="plus" />
                {t('upload.chooseFiles')}
              </span>
              <small className="upload-formats">JPG · PNG · WEBP · GIF</small>
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
          <aside className="upload-options">
            <div><p className="section-kicker">{t('upload.options')}</p><h2>{t('upload.optionsTitle')}</h2></div>
            <label className="field-group" htmlFor="upload-visibility">
              <span>{t('upload.visibility')}</span>
              <Select
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
            <p className="upload-option-hint">{t('upload.visibilityHelp')}</p>
            <dl className="upload-limits"><dt>{t('upload.batch')}</dt><dd>{systemQuery.data?.maxBatchCount ?? 20}</dd><dt>{t('upload.concurrency')}</dt><dd>{systemQuery.data?.processingConcurrency ?? 1}</dd></dl>
            <div className="upload-option-actions">
              <Button className="standard-action-button" type="submit" disabled={items.length === 0 || uploading || items.every((item) => item.status === 'complete')}>
                <Icon name="upload" />
                {uploading ? t('common.working') : t('upload.uploadCount', { count: items.filter((item) => item.status !== 'complete').length })}
              </Button>
            </div>
          </aside>
        </div>
      </form>
      {error ? <p className="mt-5 rounded-xl bg-danger-soft px-4 py-3 text-danger">{error}</p> : null}
      {!uploading && items.length > 1 && completedImages.length > 0 ? (
        <Card size="sm" className="mt-5 flex flex-wrap items-center justify-between gap-3 overflow-visible p-4">
          <p className="text-sm text-muted-foreground">{t('upload.completedLinks', { count: completedImages.length })}</p>
          <CopyLinksControl
            images={completedImages}
            label={t('upload.copyCompleted')}
            ariaLabel={(format) => t('upload.copyCompletedFormat', { count: completedImages.length, format })}
          />
        </Card>
      ) : null}
      <div className="queue-heading"><h2>{t('upload.queueTitle')} {items.length > 0 ? <span>· {items.length}</span> : null}</h2><Button variant="ghost" type="button" disabled={uploading || items.length === 0} onClick={clearQueue}>{t('common.clear')}</Button></div>
      {items.length === 0 ? <p className="queue-empty"><Icon name="image" />{t('upload.empty')}</p> : null}
      <div className="upload-queue">
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
    <article className="upload-row" data-status={item.status}>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            {item.previewUrl ? <img decoding="async" className="upload-row-preview" src={item.previewUrl} alt="" /> : <span className="upload-row-preview upload-row-fallback"><Icon name="image" /></span>}
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
        <div className="upload-progress" role="progressbar" aria-valuenow={item.progress} aria-valuemin={0} aria-valuemax={100} aria-label={t('upload.progress', { percent: item.progress })}>
          <div className="upload-progress-fill" style={{ transform: `scaleX(${Math.max(0, Math.min(100, item.progress)) / 100})` }} />
        </div>
        {item.result ? <UploadResult image={item.result} /> : null}
        {item.error ? (
          <p className="mt-3 text-sm text-danger" role="alert">
            {item.error}
          </p>
        ) : null}
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
        <CopyLinkControl image={image} menuAlign="left" />
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

function storedVisibility(): Visibility | 'default' {
  const value = readLocalStorage(visibilityStorageKey)
  return value === 'public' || value === 'private' ? value : 'default'
}
