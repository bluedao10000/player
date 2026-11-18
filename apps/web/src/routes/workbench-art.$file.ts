import { mediaAssetPattern } from '@anime/shared/media'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/workbench-art/$file')({
  server: {
    handlers: {
      GET: async ({ params, request }) => {
        if (!mediaAssetPattern.test(params.file))
          return new Response(null, { status: 404 })
        const url = new URL(
          '/api/v1/knowledge/media-assets/' + params.file,
          process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321',
        )
        const upstream = await fetch(url, { signal: request.signal })
        if (!upstream.ok) return new Response(null, { status: upstream.status })
        return new Response(upstream.body, {
          headers: {
            'Content-Type': params.file.endsWith('.png')
              ? 'image/png'
              : 'image/jpeg',
            'Cache-Control': 'private, max-age=86400, immutable',
            'X-Robots-Tag': 'noindex',
          },
        })
      },
    },
  },
})
