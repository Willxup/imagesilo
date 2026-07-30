import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, type ReactNode } from 'react'

import { ApiError, apiRequest } from '../../lib/api-client'
import type { AdminSession, SetupStatus } from '../../lib/api-types'
import { AuthContext } from './auth-context'

const sessionQueryKey = ['auth', 'session'] as const
const setupQueryKey = ['setup', 'status'] as const

async function loadSession(): Promise<AdminSession | null> {
  try {
    return await apiRequest<AdminSession>('/api/v1/auth/session')
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      return null
    }
    throw error
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: sessionQueryKey, queryFn: loadSession, retry: false })
  const setupQuery = useQuery({ queryKey: setupQueryKey, queryFn: () => apiRequest<SetupStatus>('/api/v1/setup/status'), retry: false })

  const refresh = useCallback(async () => {
    return await queryClient.fetchQuery({ queryKey: sessionQueryKey, queryFn: loadSession, staleTime: 0 })
  }, [queryClient])

  const refreshSetup = useCallback(async () => {
    return await queryClient.fetchQuery({ queryKey: setupQueryKey, queryFn: () => apiRequest<SetupStatus>('/api/v1/setup/status'), staleTime: 0 })
  }, [queryClient])

  const logout = useCallback(async () => {
    await apiRequest<void>('/api/v1/auth/logout', { method: 'POST' })
    queryClient.setQueryData(sessionQueryKey, null)
  }, [queryClient])

  return (
    <AuthContext.Provider value={{
      session: query.data ?? null,
      setupStatus: setupQuery.data ?? null,
      isLoading: query.isLoading || setupQuery.isLoading,
      refresh,
      refreshSetup,
      logout,
    }}>
      {children}
    </AuthContext.Provider>
  )
}
