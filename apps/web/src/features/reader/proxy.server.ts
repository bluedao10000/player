import { serverKnowledgeApiUrl } from '@anime/shared/knowledge'
import { readBody } from '../../lib/request-body.server'

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
  const callback = !post && path === 'callback/wechat'
  const start = post && ['wechat/start', 'wechat/link'].includes(path)
  if (
    post
      ? ![
          'code',
          'verify',
          'logout',
          'merge',
          'bookmark',
          'progress',
          'email/link',
          'membership/checkout',
          'wechat/start',
          'wechat/link',
        ].includes(path)
      : !callback && !['state', 'methods', 'membership'].includes(path)
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
  if (
    !token &&
    !callback &&
    !['methods', 'wechat/start', 'code', 'verify'].includes(path)
  )
    return path === 'state' ? reply(200, 'null') : reply(401)
  const secure =
    url.protocol === 'https:' ||
    !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)
  const cookie = (value: string, ttl: number) =>
    `folio_reader=${value}; Path=/reader-auth; Max-Age=${ttl}; HttpOnly; SameSite=Lax${secure ? '; Secure' : ''}`
  const flowCookie = (v: string, ttl: number) =>
    `folio_flow=${v}; Path=/reader-auth; Max-Age=${ttl}; HttpOnly; SameSite=Lax${secure ? '; Secure' : ''}`
  const back = (path: string) => {
    headers.set('Location', path)
    headers.set('Referrer-Policy', 'no-referrer')
    headers.append('Set-Cookie', flowCookie('', 0))
    return reply(303)
  }
  let body: string | undefined
  try {
    if (callback) {
      const proof = request.headers
        .get('cookie')
        ?.match(/(?:^|;\s*)folio_flow=([a-f0-9]{64})(?:;|$)/)?.[1]
      if (!proof) return back('/login?wechat=expired')
      body = JSON.stringify({
        proof,
        state: url.searchParams.get('state'),
        code: url.searchParams.get('code') || '',
      })
    }
    if (post)
      body = await readBody(request, path === 'merge' ? 512 * 1024 : 8192)
  } catch {
    return reply(413)
  }
  try {
    const upstream = await fetch(
      new URL(
        '/api/v1/reader/' + (callback ? 'wechat/finish' : path),
        serverKnowledgeApiUrl(),
      ),
      {
        method: callback ? 'POST' : request.method,
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
    if (start && upstream.ok) {
      const { url, proof } = await upstream.json()
      if (
        !/^[a-f0-9]{64}$/.test(proof) ||
        new URL(url).origin !== 'https://open.weixin.qq.com'
      )
        return reply(502)
      headers.set('Set-Cookie', flowCookie(proof, 600))
      return reply(200, JSON.stringify({ url }))
    }
    if (callback) {
      if (!upstream.ok)
        return back(
          '/login?wechat=' +
            (upstream.status === 409
              ? 'conflict'
              : url.searchParams.has('code')
                ? 'failed'
                : 'cancelled'),
        )
      const { token, returnTo } = await upstream.json()
      if (token) {
        if (!/^[a-f0-9]{64}$/.test(token)) return back('/login?wechat=failed')
        headers.set('Set-Cookie', cookie(token, 2592000))
      }
      return back('/login?returnTo=' + encodeURIComponent(returnTo))
    }
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
    return callback ? back('/login?wechat=failed') : reply(503)
  }
}
