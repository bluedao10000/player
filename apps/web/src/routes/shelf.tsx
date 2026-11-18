import { createFileRoute } from '@tanstack/react-router'
import { Shelf } from '../features/post/ui'

export const Route = createFileRoute('/shelf')({
  head: () => ({
    meta: [
      { title: '我的书架 · 潮汐邮局' },
      { name: 'description', content: '在这个浏览器里收藏的故事。' },
      { name: 'robots', content: 'noindex' },
    ],
  }),
  component: Shelf,
})
