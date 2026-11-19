import { createFileRoute } from '@tanstack/react-router'
import { readBody } from '../features/billing/proxy.server'

export const Route = createFileRoute('/api/billing/alipay/notify')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        const reply = (body: string, status: number) =>
          new Response(body, {
            status,
            headers: {
              'Cache-Control': 'no-store',
              'Content-Type': 'text/plain',
            },
          })
        if (
          request.headers.get('content-type')?.split(';')[0] !==
          'application/x-www-form-urlencoded'
        )
          return reply('failure', 400)
        let body: string
        try {
          body = await readBody(request)
        } catch {
          return reply('failure', 413)
        }
        try {
          // Folio issued 32-hex order numbers on this origin before the domain move.
          const orders = new URLSearchParams(body).getAll('out_trade_no')
          const legacy =
            new URL(request.url).hostname === 'www.codeiy.com' &&
            orders.length === 1 &&
            /^[0-9a-f]{32}$/.test(orders[0])
          const response = await fetch(
            new URL(
              '/api/billing/alipay/notify',
              legacy
                ? 'https://r.codeiy.com'
                : (process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321'),
            ),
            {
              method: 'POST',
              headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
              body,
              redirect: 'error',
              signal: AbortSignal.timeout(10000),
            },
          )
          return response.ok && (await response.text()) === 'success'
            ? reply('success', 200)
            : reply('failure', response.status >= 400 ? response.status : 502)
        } catch {
          return reply('failure', 503)
        }
      },
    },
  },
})
