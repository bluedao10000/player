import { createFileRoute } from '@tanstack/react-router'
import { billingNotification } from '../features/reader/billing.server'

export const Route = createFileRoute('/api/billing/alipay/notify')({
  server: { handlers: { POST: ({ request }) => billingNotification(request) } },
})
