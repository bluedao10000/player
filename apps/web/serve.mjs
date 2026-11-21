import { fileURLToPath } from 'node:url'
import { serve } from 'srvx/node'
import { staticMiddleware } from 'srvx/static'
import app from './dist/server/server.js'

serve({
  hostname: process.env.WEB_HOST ?? '127.0.0.1',
  port: Number(process.env.PORT ?? 4310),
  trustProxy: 'loopback',
  gracefulShutdown: true,
  middleware: [
    staticMiddleware({
      dir: fileURLToPath(new URL('./dist/client/', import.meta.url)),
    }),
  ],
  fetch: app.fetch,
})
