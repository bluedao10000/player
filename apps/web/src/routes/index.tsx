import { siteName, siteOrigin } from '../site'
import { createFileRoute } from '@tanstack/react-router'
import { DiscoverHome } from '../features/worlds/discover-pages'

import { discoverHome } from '../features/worlds/discover'

export const Route = createFileRoute('/')({
  loader: () => discoverHome(),
  head: () => ({
    links: [{ rel: 'canonical', href: siteOrigin + '/' }],
    meta: [
      { title: siteName + ' · 二次元作品世界' },
      {
        name: 'description',
        content:
          '发现动画与原创作品，翻开角色图鉴，阅读每部作品的故事和场景。',
      },
    ],
  }),
  component: () => <DiscoverHome page={Route.useLoaderData()} />,
})
