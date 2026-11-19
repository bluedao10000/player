import { createFileRoute } from '@tanstack/react-router'
import { siteTitle } from '../site'
import { GuestLibrary } from '../features/reader/guest'
import { libraryCatalog } from '../features/reader/library'

export const Route = createFileRoute('/me')({
  loader: () => libraryCatalog(),
  headers: () => ({
    'X-Robots-Tag': 'noindex',
    'Cache-Control': 'private, no-store',
  }),
  head: () => ({
    meta: [
      { title: siteTitle('我的') },
      { name: 'robots', content: 'noindex' },
    ],
  }),
  component: () => <GuestLibrary catalog={Route.useLoaderData()} />,
})
