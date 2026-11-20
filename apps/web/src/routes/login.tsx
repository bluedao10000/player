import { createFileRoute } from '@tanstack/react-router'
import { ReaderLogin, returnPath } from '../features/reader/login'
import { libraryCatalog } from '../features/reader/library'
import { siteTitle } from '../site'

export const Route = createFileRoute('/login')({
  validateSearch: (s: Record<string, unknown>) => ({
    returnTo: returnPath(s.returnTo),
    wechat:
      typeof s.wechat === 'string' &&
      ['conflict', 'cancelled', 'expired', 'failed'].includes(s.wechat)
        ? s.wechat
        : undefined,
  }),
  loader: () => libraryCatalog(),
  headers: () => ({
    'Cache-Control': 'private, no-store',
    'X-Robots-Tag': 'noindex',
  }),
  head: () => ({
    meta: [
      { title: siteTitle('读者登录') },
      { name: 'robots', content: 'noindex' },
    ],
  }),
  component: () => (
    <ReaderLogin
      wechat={Route.useSearch().wechat}
      returnTo={Route.useSearch().returnTo}
      chapters={Route.useLoaderData() ?? []}
    />
  ),
})
