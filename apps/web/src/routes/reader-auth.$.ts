import { createFileRoute } from '@tanstack/react-router'
import { authRequest } from '../features/reader/proxy.server'

export const Route = createFileRoute('/reader-auth/$')({
  server: {
    handlers: {
      GET: ({ request, params }) => authRequest(request, params._splat ?? ''),
      POST: ({ request, params }) => authRequest(request, params._splat ?? ''),
    },
  },
})
