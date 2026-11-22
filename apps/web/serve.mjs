import { isIP } from 'node:net'
import { fileURLToPath } from 'node:url'
import { serve } from 'srvx/node'
import { staticMiddleware } from 'srvx/static'
import app from './dist/server/server.js'

const proxyIP = process.env.WEB_TRUSTED_PROXY_IP
if (proxyIP && !isIP(proxyIP)) throw new Error('WEB_TRUSTED_PROXY_IP must be a single IP address')

serve({
  hostname: process.env.WEB_HOST ?? '127.0.0.1',
  port: Number(process.env.PORT ?? 4310),
  trustProxy: proxyIP ? [proxyIP] : 'loopback',
  gracefulShutdown: true,
  middleware: [
    staticMiddleware({
      dir: fileURLToPath(new URL('./dist/client/', import.meta.url)),
    }),
  ],
  fetch: app.fetch,
})
