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
