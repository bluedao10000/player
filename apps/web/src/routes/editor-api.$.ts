import { createFileRoute } from '@tanstack/react-router'
import { editorRequest } from '../features/workbench/editor-proxy.server'

export const Route = createFileRoute('/editor-api/$')({
  server: {
    handlers: {
      GET: ({ request, params }) => editorRequest(request, params._splat ?? ''),
      POST: ({ request, params }) =>
        editorRequest(request, params._splat ?? ''),
    },
  },
})
