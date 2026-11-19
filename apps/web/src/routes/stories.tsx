import { createFileRoute } from '@tanstack/react-router'
import { siteOrigin, siteTitle } from '../site'
import { DirectoryPage } from '../features/worlds/discover-pages'
import { discoverDirectory, discoverySearch } from '../features/worlds/discover'

export const Route = createFileRoute('/stories')({
  validateSearch: discoverySearch,
  loaderDeps: ({ search }) => search,
  loader: ({ deps }) =>
    discoverDirectory({ data: { ...deps, kind: 'stories' } }),
  head: () => ({
    links: [{ rel: 'canonical', href: siteOrigin + '/stories' }],
    meta: [
      { title: siteTitle('故事书库') },
      { name: 'description', content: '按作品浏览公开章节、短篇与分集故事。' },
    ],
  }),
  component: () => (
    <DirectoryPage
      kind="stories"
      page={Route.useLoaderData()}
      search={Route.useSearch()}
    />
  ),
})
