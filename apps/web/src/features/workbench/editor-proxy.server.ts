import { readBody } from '../../lib/request-body.server'
const headers = {
  'Cache-Control': 'private, no-store',
  'X-Content-Type-Options': 'nosniff',
}
const id = '[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}'
const read = new RegExp('^tasks/' + id + '(?:/preview)?$'),
  write = new RegExp('^tasks/' + id + '/(approve|return)$')
export async function editorRequest(req: Request, path: string) {
  const reply = (code: number) => new Response(null, { status: code, headers })
  if(process.env.PLAYER_PUBLIC_ONLY==="true")return reply(404)
  const emailAuth=path==="code" || path==="verify"
  const url = new URL(req.url),
    post = req.method === 'POST'
  if (
    !(post
      ? ['editor-access','code','verify','login', 'logout', 'tasks','plans'].includes(path) || write.test(path) || new RegExp("^plans/"+id+"/messages$").test(path)
      : path === 'account' || path === 'plans' || path === 'tasks' || read.test(path))
  )
    return reply(404)
  if (
    post &&
    (url.origin !== req.headers.get('origin') ||
      req.headers.get('content-type')?.split(';')[0] !== 'application/json')
  )
    return reply(403)
  const cookie = (value: string, ttl: number) =>
    'folio_workbench=' +
    value +
    '; HttpOnly; SameSite=Strict; Path=/; Max-Age=' +
    ttl +
    (url.protocol === 'https:' ||
    !['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)
      ? '; Secure'
      : '')
  let token = req.headers
    .get('cookie')
    ?.match(/(?:^|;\s*)folio_workbench=([a-f0-9]{64})(?:;|$)/)?.[1]
  let body: string | undefined
  if (post) {
    try {
      body = await readBody(req)
    } catch {
      return reply(413)
    }
  }
  if (path === 'login') {
    try {
      token = JSON.parse(body ?? '{}').token
    } catch {
      return reply(400)
    }
    if (typeof token !== 'string' || !/^[!-~]{32,512}$/.test(token))
      return reply(400)
  }
  if (!token && !emailAuth) return reply(401)
  const target = new URL(
    '/api/v1/' +
      (emailAuth ? 'reader/workbench/'+path : path === 'login' ? 'admin/knowledge/editor/session' : 'editor/' + path),
    process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321',
  )
  if (path.endsWith('/preview'))
    for (const k of ['round', 'attempt'])
      target.searchParams.set(k, url.searchParams.get(k) ?? '')
  try {
    const resp = await fetch(target, {
      method: req.method,
      headers: {
        Authorization: token&&!emailAuth?'Bearer '+token:'',
        'Content-Type': 'application/json',
      },
      body: path === 'login' ? undefined : body,
      cache: 'no-store',
      redirect: 'error',
      signal: AbortSignal.timeout(15000),
    })
    const h = new Headers(headers)
    if ((path === 'login'||path==='verify') && resp.ok) {
      const { token: session } = await resp.json()
      if (!/^[a-f0-9]{64}$/.test(session)) return reply(502)
      h.set('Set-Cookie', cookie(session, 28800))
      return new Response(null, { status: 204, headers: h })
    }
    if (resp.status === 401 || path === 'logout')
      h.set('Set-Cookie', cookie('', 0))
    h.set('Content-Type', 'application/json')
    return new Response(resp.body, { status: resp.status, headers: h })
  } catch {
    return reply(503)
  }
}
