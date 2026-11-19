import { siteTitle } from '../site'
import { createFileRoute } from '@tanstack/react-router'
import { overview } from '../features/worlds/api'
import { WorkPage } from '../features/worlds/pages'

export const Route = createFileRoute('/work/$work/')({
  loader: ({ params }) => overview({ data: params.work }),
  head: ({ loaderData }) => ({
    meta: [
      { title: siteTitle((loaderData?.work.title ?? '作品') + ' · 作品世界') },
      { name: 'description', content: loaderData?.work.intro },
    ],
  }),
  component: () => <WorkPage {...Route.useLoaderData()} />,
})
