class DeskError extends Error {
  constructor(readonly code: number, message: string) {
    super(message)
  }
}

export async function api<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch('/editor-api/' + path, {
    cache: 'no-store',
    ...(body === undefined ? {} : {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),
  })
  if (!response.ok) {
    let message = '请求失败，请重试'
    try { message = (await response.json()).error || message } catch {}
    throw new DeskError(response.status, response.status === 401 && path !== 'password-login' ? '登录已过期，请重新登录。' : message)
  }
  return response.status === 204 ? (undefined as T) : response.json()
}
