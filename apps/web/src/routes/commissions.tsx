import { createFileRoute } from '@tanstack/react-router'
import { Commissions } from '../features/billing/commissions'
import css from '../features/billing/commissions.css?url'

export const Route = createFileRoute('/commissions')({
  head: () => ({
    meta: [
      { title: '我的约稿 · 潮汐邮局' },
      { name: 'robots', content: 'noindex, nofollow' },
    ],
    links: [{ rel: 'stylesheet', href: css }],
  }),
  component: Commissions,
})
