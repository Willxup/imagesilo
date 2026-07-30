import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import '../../i18n/config'
import { apiRequest } from '../../lib/api-client'
import type { AppSettings } from '../../lib/api-types'
import { AuthContext } from '../auth/auth-context'
import { SettingsPage } from './settings-page'

vi.mock('../../lib/api-client', () => ({ apiRequest: vi.fn() }))

describe('SettingsPage', () => {
  beforeEach(() => {
    vi.mocked(apiRequest).mockImplementation(async (path) => {
      if (path === '/api/v1/settings') {
        return {
          defaultVisibility: 'public',
          compressionEnabled: false,
          jpegQuality: 82,
          webpQuality: 82,
          pngCompressionLevel: 6,
          conversionEnabled: false,
          conversionWebpQuality: 82,
          conversionWebpLossless: false,
        } as AppSettings
      }
      throw new Error(`unexpected request: ${path}`)
    })
  })

  it('shows account settings without duplicating the system overview', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <AuthContext.Provider value={{
          session: { adminId: '019c1234-5678-7abc-8def-0123456789ab', displayName: 'ImageSilo', email: 'admin@example.com', csrfToken: 'isc_test', expiresAt: '2026-07-30T00:00:00Z' },
          setupStatus: { initialized: true }, isLoading: false,
          refresh: vi.fn(), refreshSetup: vi.fn(), logout: vi.fn(),
        }}>
          <SettingsPage />
        </AuthContext.Provider>
      </QueryClientProvider>,
    )
    expect(await screen.findByText('管理员资料')).toBeInTheDocument()
    expect(screen.queryByText('系统概览')).not.toBeInTheDocument()
    expect(await screen.findByLabelText('启用同格式压缩')).not.toBeChecked()
    expect(screen.getByLabelText('将 JPEG 和 PNG 转为 WebP')).not.toBeChecked()
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }))
    expect(screen.getAllByRole('alert')).toHaveLength(2)
    expect(screen.getAllByText('此项为必填项。')).toHaveLength(2)
  })
})
