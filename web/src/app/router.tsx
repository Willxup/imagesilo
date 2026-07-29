import { Navigate, createBrowserRouter } from 'react-router-dom'

import { AuthGuard } from '../features/auth/auth-guard'
import { LoginPage } from '../features/auth/login-page'
import { ImageListPage } from '../features/images/image-list-page'
import { UploadPage } from '../features/upload/upload-page'
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
      ],
    },
  ],
  { basename: '/' },
)
