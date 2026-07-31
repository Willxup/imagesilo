import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
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
})
