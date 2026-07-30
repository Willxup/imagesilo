import { Navigate, createBrowserRouter } from 'react-router-dom'

import { AuthGuard } from '../features/auth/auth-guard'
import { LoginPage } from '../features/auth/login-page'
import { SetupPage } from '../features/auth/setup-page'
import { AdminLayout } from '../components/layout/admin-layout'
import { AliasPage, ApiTokenPage, ImageDetailPage, ImageListPage, SettingsPage, SystemPage, UploadPage } from './lazy-pages'

export const router = createBrowserRouter(
  [
    {
      path: '/admin/setup',
      element: <SetupPage />,
    },
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
