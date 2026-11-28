import { createFileRoute } from '@tanstack/react-router'
import { iosReaderResponse } from '../features/reader/ios-data.server'

export const Route = createFileRoute('/ios-data/$')({
  server: {
    handlers: {
      GET: ({ params }) => iosReaderResponse(params._splat ?? ''),
      ANY: () => new Response(null, { status: 405, headers: { Allow: 'GET, HEAD' } }),
    },
  },
})
