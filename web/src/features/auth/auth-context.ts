import { createContext, useContext } from 'react'

import type { AdminSession, SetupStatus } from '../../lib/api-types'

export type AuthContextValue = {
  session: AdminSession | null
  setupStatus: SetupStatus | null
  isLoading: boolean
  refresh: () => Promise<AdminSession | null>
  refreshSetup: () => Promise<SetupStatus>
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
