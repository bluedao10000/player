import { isIP } from 'node:net'
import { serve } from 'srvx/node'
import { iosReaderResponse } from './features/reader/ios-data.server'
import { authRequest } from './features/reader/proxy.server'
import { billingNotification } from './features/reader/billing.server'
import { iosAccountRequest } from './features/reader/ios-account.server'
import { sitemap } from './features/seo/sitemap.server'
import { siteOrigin } from './site'

const proxyIP = process.env.READER_API_TRUSTED_PROXY_IP
if (proxyIP && !isIP(proxyIP)) throw new Error('READER_API_TRUSTED_PROXY_IP must be a single IP address')

async function dispatch(request: Request): Promise<Response> {
  const { pathname } = new URL(request.url)
  const read = request.method === 'GET' || request.method === 'HEAD'
  const post = request.method === 'POST'
  const allowed = (methods: string) => new Response(null, { status: 405, headers: { Allow: methods } })

  if (pathname === '/healthz') return read ? new Response('ok\n') : allowed('GET, HEAD')
  if (pathname === '/robots.txt')
    return read ? new Response(`User-agent: *\nAllow: /\n\nSitemap: ${siteOrigin}/sitemap.xml\n`) : allowed('GET, HEAD')
  if (pathname === '/sitemap.xml') return read ? sitemap() : allowed('GET, HEAD')
  if (pathname === '/api/billing/alipay/notify')
    return post ? billingNotification(request) : allowed('POST')
  if (pathname.startsWith('/reader-auth/'))
    return request.method === 'GET' || post
      ? authRequest(request, pathname.slice('/reader-auth/'.length))
      : allowed('GET, POST')
  if (pathname.startsWith('/ios-account/'))
    return request.method === 'GET' || post
      ? iosAccountRequest(request, pathname.slice('/ios-account/'.length))
      : allowed('GET, POST')
  if (pathname.startsWith('/ios-data/'))
    return read ? iosReaderResponse(pathname.slice('/ios-data/'.length)) : allowed('GET, HEAD')
  return new Response(null, { status: 404 })
}

serve({
  hostname: process.env.READER_API_HOST ?? '127.0.0.1',
  port: Number(process.env.READER_API_PORT ?? 4310),
  trustProxy: proxyIP ? [proxyIP] : 'loopback',
  gracefulShutdown: true,
  async fetch(request) {
    const response = await dispatch(request)
    if (request.method !== 'HEAD') return response
    await response.body?.cancel()
    return new Response(null, { status: response.status, headers: response.headers })
  },
})
