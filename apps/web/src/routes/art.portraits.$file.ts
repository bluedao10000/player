import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/art/portraits/$file')({
  server: {
    handlers: {
      GET: async ({ params, request }) => {
        if (!/^[a-f0-9]{64}\.webp$/.test(params.file))
          return new Response(null, { status: 404 })
        const url = new URL(
          '/api/v1/knowledge/reader-portraits/' + params.file,
          process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321',
        )
        const response = await fetch(url, { signal: request.signal })
        if (!response.ok) return new Response(null, { status: response.status })
        return new Response(response.body, {
          headers: {
            'Content-Type': 'image/webp',
            'Cache-Control': 'public, max-age=31536000, immutable',
          },
        })
      },
    },
  },
})
