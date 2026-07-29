import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { SystemInfo } from '../../lib/api-types'
import { UploadPage } from './upload-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn(), uploadForm: vi.fn() }))

describe('UploadPage', () => {
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
    render(<QueryClientProvider client={queryClient}><UploadPage /></QueryClientProvider>)
    const file = new File(['jpeg'], 'dropped.jpg', { type: 'image/jpeg' })
    const dropZone = await screen.findByText('拖放图片到这里，或直接粘贴剪贴板图片')
    fireEvent.drop(dropZone.parentElement!, { dataTransfer: { files: [file] } })
    expect(await screen.findByText('dropped.jpg')).toBeInTheDocument()
    expect(screen.getByText(/在浏览器中排队/)).toBeInTheDocument()
  })
})
