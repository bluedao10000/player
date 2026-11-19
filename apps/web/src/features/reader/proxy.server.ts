import { readBody } from '../billing/proxy.server'

export async function authRequest(request: Request, path: string) {
  const headers = new Headers({
    'Cache-Control': 'private, no-store',
    'X-Robots-Tag': 'noindex',
    'Content-Type': 'application/json',
    'X-Content-Type-Options': 'nosniff',
  })
  const reply = (status: number, body: BodyInit | null = null) =>
    new Response(body, { status, headers })
  const url = new URL(request.url)
  const post = request.method === 'POST'
  if (
    post
      ? !['code', 'verify', 'logout', 'merge', 'bookmark', 'progress'].includes(
          path,
        )
      : path !== 'state'
  )
    return reply(404)
  if (
    post &&
    (request.headers.get('origin') !== url.origin ||
      request.headers.get('content-type')?.split(';')[0] !== 'application/json')
  )
    return reply(403)
  const token = request.headers
    .get('cookie')
    ?.match(/(?:^|;\s*)folio_reader=([a-f0-9]{64})(?:;|$)/)?.[1]
  if (!token && !['code', 'verify'].includes(path))
    return path === 'state' ? reply(200, 'null') : reply(401)
  const secure =
    url.protocol === 'https:' ||
    !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)
  const cookie = (value: string, ttl: number) =>
    `folio_reader=${value}; Path=/reader-auth; Max-Age=${ttl}; HttpOnly; SameSite=Lax${secure ? '; Secure' : ''}`
  let body: string | undefined
  try {
    if (post)
      body = await readBody(request, path === 'merge' ? 512 * 1024 : 8192)
  } catch {
    return reply(413)
  }
  try {
    const upstream = await fetch(
      new URL(
        '/api/v1/reader/' + path,
        process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321',
      ),
      {
        method: request.method,
        body,
        headers: {
          'Content-Type': 'application/json',
          Authorization: token ? 'Bearer ' + token : '',
          'X-Reader-ID': request.headers.get('x-reader-id') ?? '',
        },
        redirect: 'error',
        cache: 'no-store',
        signal: AbortSignal.timeout(20000),
      },
    )
    if (path === 'verify' && upstream.ok) {
      const { token } = await upstream.json()
      if (typeof token !== 'string' || !/^[a-f0-9]{64}$/.test(token))
        return reply(502)
      headers.set('Set-Cookie', cookie(token, 2592000))
      return reply(204)
    }
    if (path === 'logout' && upstream.ok)
      headers.set('Set-Cookie', cookie('', 0))
    if (upstream.status === 401 && path === 'state') return reply(200, 'null')
    return reply(upstream.status, upstream.body)
  } catch {
    return reply(503)
  }
}
