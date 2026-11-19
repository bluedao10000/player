import { siteName } from '../site'
import { createFileRoute } from '@tanstack/react-router'
import { Home } from '../features/post/editorial'

import { discovery } from '../features/worlds/api'

export const Route = createFileRoute('/')({
  loader: () => discovery(),
  head: () => ({
    meta: [
      { title: siteName + ' · 二次元作品世界' },
      {
        name: 'description',
        content:
          '从《潮汐邮局》《星际牛仔》和《火影忍者》进入作品世界，认识角色，阅读章节，重访故事里的场景。',
      },
    ],
  }),
  component: () => <Home works={Route.useLoaderData().items} />,
})
