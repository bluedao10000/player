import { siteTitle } from '../site'
import { createFileRoute, notFound } from '@tanstack/react-router'
import { catalog } from '../features/post/catalog'
import { Play } from '../features/post/ui'

export const Route = createFileRoute('/play/$slug')({
  loader: ({ params }) => {
    const story = catalog.find((item) => item.slug === params.slug)
    if (!story) throw notFound()
    return story
  },
  head: ({ loaderData }) => ({
    meta: [
      { title: siteTitle((loaderData?.title ?? '未找到故事') + ' · 剧情试玩') },
      { name: 'description', content: loaderData?.intro },
      { name: 'robots', content: 'noindex' },
    ],
  }),
  component: () => <Play story={Route.useLoaderData()} />,
})
