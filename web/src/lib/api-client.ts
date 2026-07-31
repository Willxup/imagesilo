export class ApiError extends Error {
  readonly status: number
  readonly code?: string

  constructor(status: number, message: string, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

type UnauthorizedListener = () => void
const unauthorizedListeners = new Set<UnauthorizedListener>()

export function subscribeUnauthorized(listener: UnauthorizedListener) {
  unauthorizedListeners.add(listener)
  return () => {
    unauthorizedListeners.delete(listener)
  }
}

function notifyUnauthorized(path: string) {
  if (path === '/api/v1/auth/login') return
  for (const listener of unauthorizedListeners) listener()
}

export async function apiRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  headers.set('Accept', 'application/json')
  if (!isSafeMethod(init?.method) && !headers.has('X-CSRF-Token')) {
    const csrfToken = readCookie('imagesilo_csrf')
    if (csrfToken) headers.set('X-CSRF-Token', csrfToken)
  }
  const response = await fetch(path, {
    ...init,
    credentials: 'include',
    headers,
  })
  if (response.status === 401) notifyUnauthorized(path)
  if (response.status === 204) {
    return undefined as T
  }

  const contentType = response.headers.get('content-type') ?? ''
  const body = contentType.includes('application/json') ? await response.json() : undefined
  if (!response.ok) {
    const message = typeof body?.message === 'string' ? body.message : `Request failed with status ${response.status}`
    const code = typeof body?.code === 'string' ? body.code : undefined
    throw new ApiError(response.status, message, code)
  }
  return body as T
}

export function uploadForm<T>(path: string, body: FormData, onProgress: (percent: number) => void, signal: AbortSignal): Promise<T> {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest()
    const abort = () => request.abort()
    signal.addEventListener('abort', abort, { once: true })
    request.open('POST', path)
    request.withCredentials = true
    request.setRequestHeader('Accept', 'application/json')
    const csrfToken = readCookie('imagesilo_csrf')
    if (csrfToken) request.setRequestHeader('X-CSRF-Token', csrfToken)
    request.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable) onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)))
    })
    request.addEventListener('load', () => {
      signal.removeEventListener('abort', abort)
      const contentType = request.getResponseHeader('content-type') ?? ''
      let value: unknown
      if (contentType.includes('application/json') && request.responseText) {
        try {
          value = JSON.parse(request.responseText)
        } catch {
          value = undefined
        }
      }
      if (request.status >= 200 && request.status < 300) {
        onProgress(100)
        resolve(value as T)
        return
      }
      if (request.status === 401) notifyUnauthorized(path)
      const error = value as { message?: unknown; code?: unknown } | undefined
      reject(
        new ApiError(
          request.status,
          typeof error?.message === 'string' ? error.message : `Request failed with status ${request.status}`,
          typeof error?.code === 'string' ? error.code : undefined,
        ),
      )
    })
    request.addEventListener('error', () => {
      signal.removeEventListener('abort', abort)
      reject(new ApiError(0, 'Network request failed'))
    })
    request.addEventListener('abort', () => {
      signal.removeEventListener('abort', abort)
      reject(new DOMException('Upload canceled', 'AbortError'))
    })
    request.send(body)
  })
}

function isSafeMethod(method?: string) {
  const normalized = (method ?? 'GET').toUpperCase()
  return normalized === 'GET' || normalized === 'HEAD' || normalized === 'OPTIONS'
}

function readCookie(name: string) {
  const prefix = `${encodeURIComponent(name)}=`
  for (const item of document.cookie.split(';')) {
    const cookie = item.trim()
    if (cookie.startsWith(prefix)) return decodeURIComponent(cookie.slice(prefix.length))
  }
  return ''
}
