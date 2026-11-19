import { createFileRoute } from '@tanstack/react-router'
import { readerRequest } from '../features/billing/proxy.server'

export const Route = createFileRoute('/reader-api/$')({
  server: {
    handlers: {
      GET: ({ request, params }) => readerRequest(request, params._splat ?? ''),
      POST: ({ request, params }) =>
        readerRequest(request, params._splat ?? ''),
    },
  },
})
