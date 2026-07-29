import { createContext, useContext } from 'react'

import type { AdminSession } from '../../lib/api-types'

export type AuthContextValue = {
  session: AdminSession | null
  isLoading: boolean
  refresh: () => Promise<AdminSession | null>
  logout: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) {
    throw new Error('useAuth must be used inside AuthProvider')
  }
  return value
}
