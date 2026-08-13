import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { ApiError, apiRequest, uploadForm } from '../../lib/api-client'
import type { SystemInfo } from '../../lib/api-types'
import { UploadPage } from './upload-page'

vi.mock('../../lib/api-client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib/api-client')>()
  return { ...actual, apiRequest: vi.fn(), uploadForm: vi.fn() }
})

describe('UploadPage', () => {
  afterEach(cleanup)
  beforeEach(() => {
    window.localStorage.clear()
    vi.mocked(apiRequest).mockReset()
    vi.mocked(uploadForm).mockReset()
    vi.mocked(apiRequest).mockResolvedValue({
      processingConcurrency: 1,
      maxBatchCount: 20,
      maxUploadBytes: 20 * 1024 * 1024,
      maxTotalPixels: 16_000_000,
      supportedFormats: ['image/jpeg', 'image/png', 'image/webp', 'image/gif'],
      vipsVersion: '8.18.4',
    } as SystemInfo)
  })

  it('queues files from drag and drop in the browser', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <UploadPage />
      </QueryClientProvider>,
    )
    const file = new File(['jpeg'], 'dropped.jpg', { type: 'image/jpeg' })
    const dropZone = await screen.findByText('拖放图片到这里，或直接粘贴剪贴板图片')
    fireEvent.drop(dropZone.parentElement!, { dataTransfer: { files: [file] } })
    expect(await screen.findByText('dropped.jpg')).toBeInTheDocument()
    expect(screen.getByText(/在浏览器中排队/)).toBeInTheDocument()
  })

  it('accepts a supported extension when the browser provides an empty MIME type', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <UploadPage />
      </QueryClientProvider>,
    )
    const file = new File(['jpeg'], 'camera.JPG', { type: '' })
    const input = await screen.findByLabelText('选择图片文件')
    fireEvent.change(input, { target: { files: [file] } })
    expect(await screen.findByText('camera.JPG')).toBeInTheDocument()
  })

  it('keeps the structured server reason on a failed upload row', async () => {
    vi.mocked(uploadForm).mockRejectedValue(new ApiError(413, 'Image exceeds the configured maximum.', 'file_too_large'))
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <UploadPage />
      </QueryClientProvider>,
    )
    const file = new File(['jpeg'], 'large.jpg', { type: 'image/jpeg' })
    fireEvent.change(await screen.findByLabelText('选择图片文件'), { target: { files: [file] } })
    fireEvent.click(await screen.findByRole('button', { name: /上传 1 个文件/ }))
    expect(await screen.findByText('Image exceeds the configured maximum.')).toBeInTheDocument()
  })

  it('restores upload visibility and copies all successful links', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    window.localStorage.setItem('imagesilo_upload_visibility', 'private')
    vi.mocked(uploadForm).mockImplementation(async (_path, body) => {
      const file = body.get('file') as File
      const suffix = file.name === 'first.jpg' ? 'ab' : 'ac'
      return {
        id: `019c1234-5678-7abc-8def-0123456789${suffix}`,
        originalName: file.name,
        mimeType: 'image/jpeg',
        extension: '.jpg',
        width: 1,
        height: 1,
        sourceSize: 4,
        storedSize: 4,
        sourceSha256: 'a'.repeat(64),
        storedSha256: 'a'.repeat(64),
        processingSummary: {
          action: 'preserve',
          sourceFormat: 'jpeg',
          storedFormat: 'jpeg',
          preserved: true,
          compressionEnabled: false,
          conversionEnabled: false,
        },
        visibility: 'private',
        standardUrl: `/image/019c1234-5678-7abc-8def-0123456789${suffix}`,
        thumbnailUrl: `/api/v1/images/019c1234-5678-7abc-8def-0123456789${suffix}/thumbnail`,
        createdAt: '2026-08-13T00:00:00Z',
      }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <UploadPage />
      </QueryClientProvider>,
    )

    expect(await screen.findByLabelText('可见性')).toHaveTextContent('私密')
    const files = [
      new File(['jpeg'], 'first.jpg', { type: 'image/jpeg' }),
      new File(['jpeg'], 'second.jpg', { type: 'image/jpeg' }),
    ]
    fireEvent.change(screen.getByLabelText('选择图片文件'), { target: { files } })
    fireEvent.click(screen.getByRole('button', { name: /上传 2 个文件/ }))

    const copyAll = await screen.findByRole('button', { name: '复制 2 张成功图片的直链' })
    expect(vi.mocked(uploadForm).mock.calls.every(([, body]) => body.get('visibility') === 'private')).toBe(true)
    expect(copyAll.closest('[data-slot="card"]')).toHaveClass('overflow-visible')
    fireEvent.click(copyAll)
    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(
        'http://localhost:3000/image/019c1234-5678-7abc-8def-0123456789ab\n' +
          'http://localhost:3000/image/019c1234-5678-7abc-8def-0123456789ac',
      )
    })

    fireEvent.click(screen.getAllByRole('button', { name: /选择链接格式/ })[0])
    fireEvent.click(screen.getByRole('button', { name: '复制 Markdown' }))
    fireEvent.click(screen.getByRole('button', { name: '复制 2 张成功图片的MD' }))
    await waitFor(() => {
      expect(writeText).toHaveBeenLastCalledWith(
        '![first.jpg](http://localhost:3000/image/019c1234-5678-7abc-8def-0123456789ab)\n' +
          '![second.jpg](http://localhost:3000/image/019c1234-5678-7abc-8def-0123456789ac)',
      )
    })
  })
})
