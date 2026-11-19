import { createFileRoute } from '@tanstack/react-router'
import { siteOrigin, siteTitle } from '../site'
import { DirectoryPage } from '../features/worlds/discover-pages'
import { discoverDirectory, discoverySearch } from '../features/worlds/discover'

export const Route = createFileRoute('/characters')({
  validateSearch: discoverySearch,
  loaderDeps: ({ search }) => search,
  loader: ({ deps }) =>
    discoverDirectory({ data: { ...deps, kind: 'characters' } }),
  head: () => ({
    links: [{ rel: 'canonical', href: siteOrigin + '/characters' }],
    meta: [
      { title: siteTitle('角色图鉴') },
      {
        name: 'description',
        content: '跨作品认识角色，阅读人物介绍，找到他们出场的章节。',
      },
    ],
  }),
  component: () => (
    <DirectoryPage
      kind="characters"
      page={Route.useLoaderData()}
      search={Route.useSearch()}
    />
  ),
})
