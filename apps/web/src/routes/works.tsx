import { createFileRoute } from '@tanstack/react-router'
import { siteOrigin, siteTitle } from '../site'
import { DirectoryPage } from '../features/worlds/discover-pages'
import { discoverDirectory, discoverySearch } from '../features/worlds/discover'

export const Route = createFileRoute('/works')({
  validateSearch: discoverySearch,
  loaderDeps: ({ search }) => search,
  loader: ({ deps }) => discoverDirectory({ data: { ...deps, kind: 'works' } }),
  head: () => ({
    links: [{ rel: 'canonical', href: siteOrigin + '/works' }],
    meta: [
      { title: siteTitle('作品目录') },
      {
        name: 'description',
        content: '发现已发布的动画和原创作品，按名称或版本查找作品世界。',
      },
    ],
  }),
  component: () => (
    <DirectoryPage
      kind="works"
      page={Route.useLoaderData()}
      search={Route.useSearch()}
    />
  ),
})
