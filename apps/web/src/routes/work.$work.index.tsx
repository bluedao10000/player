import { workHead } from '../features/seo/world'
import { createFileRoute } from '@tanstack/react-router'
import { overview } from '../features/worlds/api'
import { WorkPage } from '../features/worlds/pages'

export const Route = createFileRoute('/work/$work/')({
  loader: ({ params }) => overview({ data: params.work }),
  head: ({ loaderData }) => loaderData ? workHead(loaderData) : {},
  component: () => <WorkPage {...Route.useLoaderData()} />,
})
