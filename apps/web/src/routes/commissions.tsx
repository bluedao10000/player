import { siteTitle } from '../site'
import { createFileRoute } from '@tanstack/react-router'
import { Commissions } from '../features/billing/commissions'
import css from '../features/billing/commissions.css?url'

export const Route = createFileRoute('/commissions')({
  head: () => ({
    meta: [
      { title: siteTitle('我的约稿') },
      { name: 'robots', content: 'noindex, nofollow' },
    ],
    links: [{ rel: 'stylesheet', href: css }],
  }),
  component: Commissions,
})
