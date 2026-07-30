import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'

import { useAuth } from './auth-context'

export function AuthGuard({ children }: { children: ReactNode }) {
  const { session, setupStatus, isLoading } = useAuth()
  const location = useLocation()

  if (isLoading) {
    return <div className="grid min-h-screen place-items-center text-muted-foreground">Loading…</div>
  }
  if (!session) {
    if (setupStatus && !setupStatus.initialized) {
      return <Navigate to="/admin/setup" replace />
    }
    return <Navigate to="/admin/login" state={{ from: location.pathname }} replace />
  }
  return children
}
