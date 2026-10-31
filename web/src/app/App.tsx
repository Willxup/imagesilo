import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createPortal } from 'react-dom'
import { RouterProvider } from 'react-router-dom'
import { Toaster } from 'sonner'

import { AuthProvider } from '../features/auth/auth-provider'
import { router } from './router'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
    },
  },
})

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <RouterProvider router={router} />
        {createPortal(<div id="imagesilo-notifications" data-modal-notifications>
          <Toaster closeButton richColors duration={4000} position="top-right" visibleToasts={2} toastOptions={{ className: 'imagesilo-toast' }} />
        </div>, document.body)}
      </AuthProvider>
    </QueryClientProvider>
  )
}
