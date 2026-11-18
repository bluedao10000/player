import { createFileRoute } from '@tanstack/react-router'
import { Home } from '../features/post/editorial'

export const Route = createFileRoute('/')({
  head: () => ({
    meta: [
      { title: '潮汐邮局 · 白帆港故事集' },
      {
        name: 'description',
        content:
          '发现原创角色阿澪和白帆港的短篇故事，从一幕海边的来信进入私人剧情。',
      },
    ],
  }),
  component: Home,
})
