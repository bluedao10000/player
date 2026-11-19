const cookieName = 'tidepost_reader'
const uuid = '[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}'
const orderRead = new RegExp('^orders/' + uuid + '(/image)?$')
const orderCancel = new RegExp('^orders/' + uuid + '/cancel$')
const privateHeaders = {
  'Cache-Control': 'private, no-store',
  'X-Content-Type-Options': 'nosniff',
}

async function readBody(request: Request) {
  const reader = request.body?.getReader()
  if (!reader) return ''
  const chunks: Uint8Array[] = []
  let size = 0
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    size += value.length
    if (size > 16_384) {
      await reader.cancel()
      throw new Error('body too large')
    }
    chunks.push(value)
  }
  return Buffer.concat(chunks).toString('utf8')
}

export async function readerRequest(request: Request, path: string) {
  const fail = (status: number) =>
    new Response(null, { status, headers: privateHeaders })
  const url = new URL(request.url)
  const post = request.method === 'POST'
  const allowed = post
    ? ['login', 'logout', 'orders'].includes(path) || orderCancel.test(path)
    : ['account', 'quote', 'orders', 'entries'].includes(path) ||
      orderRead.test(path)
  if (!allowed) return fail(404)
  if (
    post &&
    (request.headers.get('origin') !== url.origin ||
      request.headers.get('content-type')?.split(';')[0] !== 'application/json')
  )
    return fail(403)
  const secure =
    url.protocol === 'https:' ||
    !['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)
  const cookie = (value: string, ttl: number) =>
    `${cookieName}=${value}; Path=/reader-api; Max-Age=${ttl}; HttpOnly; SameSite=Strict${secure ? '; Secure' : ''}`
  const session = request.headers
    .get('cookie')
    ?.match(/(?:^|;\s*)tidepost_reader=([a-f0-9]{64})(?:;|$)/)?.[1]
  let token = session
  let body: string | undefined
  if (post) {
    try {
      body = await readBody(request)
    } catch {
      return fail(413)
    }
  }
  if (path === 'login') {
    try {
      token = JSON.parse(body ?? '{}').token
    } catch {
      return fail(400)
    }
    if (typeof token !== 'string' || !/^[!-~]{32,512}$/.test(token))
      return fail(400)
  }
  if (!token) return fail(401)
  const authAction = path === 'login' || path === 'logout'
  const target = new URL(
    '/api/v1/billing/' + (authAction ? 'session' : path),
    process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321',
  )
  if (
    !post &&
    ['entries', 'orders'].includes(path) &&
    url.searchParams.has('before')
  )
    target.searchParams.set('before', url.searchParams.get('before')!)
  try {
    const upstream = await fetch(target, {
      method: path === 'logout' ? 'DELETE' : request.method,
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: post && !authAction ? body : undefined,
      redirect: 'error',
      cache: 'no-store',
      signal: AbortSignal.any([request.signal, AbortSignal.timeout(20000)]),
    })
    const headers = new Headers(privateHeaders)
    if (path === 'login' && upstream.ok) {
      const sessionToken: unknown = (await upstream.json()).token
      if (
        typeof sessionToken !== 'string' ||
        !/^[a-f0-9]{64}$/.test(sessionToken)
      )
        return fail(502)
      headers.set('Set-Cookie', cookie(sessionToken, 43200))
      return new Response(null, { status: 204, headers })
    }
    if (path === 'logout' || upstream.status === 401)
      headers.set('Set-Cookie', cookie('', 0))
    if (upstream.ok && path.endsWith('/image')) {
      headers.set('Content-Type', 'image/png')
      headers.set(
        'Content-Disposition',
        'attachment; filename="tidepost-preview.png"',
      )
    } else headers.set('Content-Type', 'application/json')
    return new Response(upstream.body, { status: upstream.status, headers })
  } catch {
    return fail(503)
  }
}
