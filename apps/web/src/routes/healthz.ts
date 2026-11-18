import { createFileRoute } from '@tanstack/react-router'
import { health } from '@anime/shared'

export const Route = createFileRoute('/healthz')({
  server: {
    handlers: { GET: () => Response.json(health('web')) },
  },
})
