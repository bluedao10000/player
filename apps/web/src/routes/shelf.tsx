import { siteTitle } from '../site'
import { createFileRoute } from '@tanstack/react-router'
import { Shelf } from '../features/post/ui'

export const Route = createFileRoute('/shelf')({
  head: () => ({
    meta: [
      { title: siteTitle('我的书架') },
      { name: 'description', content: '在这个浏览器里收藏的故事。' },
      { name: 'robots', content: 'noindex' },
    ],
  }),
  component: Shelf,
})
