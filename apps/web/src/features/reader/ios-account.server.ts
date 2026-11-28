import { serverKnowledgeApiUrl } from '@anime/shared/knowledge'
import { readBody } from '../../lib/request-body.server'

const gets = ['state', 'membership', 'native/challenge']
const posts = ['code', 'verify', 'logout', 'email/link', 'bookmark', 'progress', 'merge', 'native/apple', 'native/wechat',
  'native/link/apple', 'native/link/wechat', 'native/store/receipt', 'native/store/notifications']

export async function iosAccountRequest(request: Request, path: string) {
  const headers = {'Cache-Control': 'private, no-store', 'X-Robots-Tag': 'noindex', 'Content-Type': 'application/json'}
  const post = request.method === 'POST'
  if (!(post ? posts : gets).includes(path)) return new Response(null, {status: 404, headers})
  if (post && request.headers.get('content-type')?.split(';')[0] !== 'application/json') return new Response(null,{status:415, headers})
  let body: string | undefined
  try { if (post) body = await readBody(request, path === 'merge' ? 512*1024 : 128*1024) }
  catch { return new Response(null, {status:413, headers}) }
  try {
    const r = await fetch(new URL('/api/v1/reader/' + path, serverKnowledgeApiUrl()), {
      method: request.method, redirect:'error', headers: {'Content-Type': 'application/json',
      Authorization: request.headers.get('authorization') ?? '', 'X-Reader-ID': request.headers.get('x-reader-id') ?? ''},
      body, signal: AbortSignal.timeout(15000),
    })
    return new Response(r.body,{status:r.status, headers})
  } catch {return new Response(null,{status:503, headers})}
}
