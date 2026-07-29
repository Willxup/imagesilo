import { Navigate, createBrowserRouter } from 'react-router-dom'

import { AuthGuard } from '../features/auth/auth-guard'
import { LoginPage } from '../features/auth/login-page'
import { ImageListPage } from '../features/images/image-list-page'
import { ImageDetailPage } from '../features/images/image-detail-page'
import { UploadPage } from '../features/upload/upload-page'
import { ApiTokenPage } from '../features/api-tokens/api-token-page'
import { SettingsPage } from '../features/settings/settings-page'
import { AliasPage } from '../features/aliases/alias-page'
import { SystemPage } from '../features/system/system-page'
import { AdminLayout } from '../components/layout/admin-layout'

export const router = createBrowserRouter(
  [
    {
      path: '/admin/login',
      element: <LoginPage />,
    },
    {
      path: '/admin',
      element: (
        <AuthGuard>
          <AdminLayout />
        </AuthGuard>
      ),
      children: [
        { index: true, element: <Navigate to="upload" replace /> },
        { path: 'upload', element: <UploadPage /> },
        { path: 'images', element: <ImageListPage /> },
        { path: 'images/:imageId', element: <ImageDetailPage /> },
        { path: 'aliases', element: <AliasPage /> },
        { path: 'api-tokens', element: <ApiTokenPage /> },
        { path: 'settings', element: <SettingsPage /> },
        { path: 'system', element: <SystemPage /> },
      ],
    },
  ],
  { basename: '/' },
)
