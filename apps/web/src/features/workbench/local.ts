import { notFound } from '@tanstack/react-router'
import { createServerFn } from '@tanstack/react-start'
import { getRequest } from '@tanstack/react-start/server'

export function localWorkbenchRequest(request: Request) {
  return process.env.PLAYER_PUBLIC_ONLY !== 'true' &&
    ['127.0.0.1', 'localhost', '[::1]'].includes(new URL(request.url).hostname)
}

export const requireLocalWorkbench = createServerFn({ method: 'GET' }).handler(() => {
  if (!localWorkbenchRequest(getRequest())) throw notFound()
})
