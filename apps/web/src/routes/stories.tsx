import { createFileRoute } from '@tanstack/react-router'
import { Library } from '../features/post/ui'

export const Route = createFileRoute('/stories')({
  head: () => ({
    meta: [
      { title: '故事来信 · 潮汐邮局' },
      {
        name: 'description',
        content: '阅读白帆港的短篇故事、角色手记和独立剧情片段。',
      },
    ],
  }),
  component: Library,
})
