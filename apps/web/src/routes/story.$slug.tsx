import { createFileRoute } from '@tanstack/react-router'
import { chapter } from '../features/worlds/api'
import { Reader } from '../features/post/editorial'

export const Route = createFileRoute('/story/$slug')({
  loader: ({ params }) =>
    chapter({ data: { work: 'tidepost', slug: params.slug } }),
  head: ({ loaderData }) => ({
    meta: [
      { title: (loaderData?.chapter.title ?? '未找到故事') + ' · 潮汐邮局' },
      { name: 'description', content: loaderData?.chapter.intro },
    ],
  }),
  component: () => <Reader page={Route.useLoaderData()} />,
})
