import { createFileRoute } from '@tanstack/react-router'
import { sitemap } from '../features/seo/sitemap.server'

export const Route = createFileRoute('/sitemap.xml')({
  server: { handlers: { GET: sitemap } },
})
