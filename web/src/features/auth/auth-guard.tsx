import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'

import { useAuth } from './auth-context'

export function AuthGuard({ children }: { children: ReactNode }) {
  const { session, isLoading } = useAuth()
  const location = useLocation()

  if (isLoading) {
    return <div className="grid min-h-screen place-items-center text-muted">Loading…</div>
  }
  if (!session) {
    return <Navigate to="/admin/login" state={{ from: location.pathname }} replace />
  }
  return children
}
