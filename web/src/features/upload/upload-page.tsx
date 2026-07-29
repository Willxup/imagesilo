import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { apiRequest } from '../../lib/api-client'
import type { Image, Visibility } from '../../lib/api-types'

export function UploadPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [file, setFile] = useState<File | null>(null)
  const [visibility, setVisibility] = useState<Visibility | 'default'>('default')
  const [result, setResult] = useState<Image | null>(null)
  const [error, setError] = useState('')
  const [uploading, setUploading] = useState(false)

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!file) return
    setUploading(true)
    setError('')
    setResult(null)
    const body = new FormData()
    body.append('file', file)
    if (visibility !== 'default') body.append('visibility', visibility)
    try {
      const image = await apiRequest<Image>('/api/v1/images', { method: 'POST', body })
      setResult(image)
      await queryClient.invalidateQueries({ queryKey: ['images'] })
    } catch {
      setError(t('upload.failed'))
    } finally {
      setUploading(false)
    }
  }

  return (
    <section>
      <h1 className="page-title">{t('upload.title')}</h1>
      <p className="page-description">{t('upload.description')}</p>
      <form className="mt-8 rounded-3xl border border-dashed border-line bg-panel p-8" onSubmit={(event) => void submit(event)}>
        <input
          aria-label={t('upload.chooseFile')}
          type="file"
          accept="image/jpeg,.jpg,.jpeg"
          required
          onChange={(event) => setFile(event.target.files?.[0] ?? null)}
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
        <button className="button-primary mt-6" type="submit" disabled={!file || uploading}>
          {uploading ? t('common.working') : t('upload.upload')}
        </button>
      </form>
      {error ? <p className="mt-5 rounded-xl bg-danger-soft px-4 py-3 text-danger">{error}</p> : null}
      {result ? (
        <div className="mt-6 rounded-2xl border border-line bg-panel p-5">
          <p className="font-medium">{t('upload.success')}</p>
          <p className="mt-1 text-sm text-muted">{t(`visibility.${result.visibility}`)}</p>
          <a className="mt-2 block break-all text-accent underline" href={result.standardUrl} target="_blank" rel="noreferrer">
            {result.standardUrl}
          </a>
        </div>
      ) : null}
    </section>
  )
}
