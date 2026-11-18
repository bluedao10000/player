import { createFileRoute, notFound } from '@tanstack/react-router'
import { catalog } from '../features/post/catalog'
import { Reader } from '../features/post/editorial'

export const Route = createFileRoute('/story/$slug')({
  loader: ({ params }) => {
    const story = catalog.find((item) => item.slug === params.slug)
    if (!story) throw notFound()
    return story
  },
  head: ({ loaderData }) => ({
    meta: [
      { title: (loaderData?.title ?? '未找到故事') + ' · 潮汐邮局' },
      { name: 'description', content: loaderData?.intro },
    ],
  }),
  component: () => <Reader story={Route.useLoaderData()} />,
})
