import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, type ReactNode } from 'react'

import { ApiError, apiRequest, subscribeUnauthorized } from '../../lib/api-client'
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

  const clearAuthenticatedCache = useCallback(() => {
    void queryClient.cancelQueries({ predicate: (cachedQuery) => !['auth', 'setup'].includes(String(cachedQuery.queryKey[0])) })
    queryClient.removeQueries({ predicate: (cachedQuery) => !['auth', 'setup'].includes(String(cachedQuery.queryKey[0])) })
    queryClient.setQueryData(sessionQueryKey, null)
  }, [queryClient])

  useEffect(() => subscribeUnauthorized(clearAuthenticatedCache), [clearAuthenticatedCache])

  const refresh = useCallback(async () => {
    return await queryClient.fetchQuery({ queryKey: sessionQueryKey, queryFn: loadSession, staleTime: 0 })
  }, [queryClient])

  const refreshSetup = useCallback(async () => {
    return await queryClient.fetchQuery({ queryKey: setupQueryKey, queryFn: () => apiRequest<SetupStatus>('/api/v1/setup/status'), staleTime: 0 })
  }, [queryClient])

  const logout = useCallback(async () => {
    try {
      await apiRequest<void>('/api/v1/auth/logout', { method: 'POST' })
    } catch (error) {
      if (!(error instanceof ApiError) || error.status !== 401) throw error
    } finally {
      clearAuthenticatedCache()
    }
  }, [clearAuthenticatedCache])

  return (
    <AuthContext.Provider
      value={{
        session: query.data ?? null,
        setupStatus: setupQuery.data ?? null,
        isLoading: query.isLoading || setupQuery.isLoading,
        refresh,
        refreshSetup,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}
