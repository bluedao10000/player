import { createFileRoute, notFound, redirect } from '@tanstack/react-router'
import { character, chapter, scene } from '../features/worlds/api'
import { ChapterPage, PersonPage, ScenePage } from '../features/worlds/pages'

export const Route = createFileRoute('/work/$work/$kind/$slug')({
  loader: async ({ params }) => {
    const data = { work: params.work, slug: params.slug }
    switch (params.kind) {
      case 'character': {
        const page = await character({ data })
        if (page.character.href.startsWith('/character/'))
          throw redirect({ href: page.character.href })
        return {
          kind: 'character' as const,
          page,
          title: page.character.name,
          summary: page.character.role,
        }
      }
      case 'chapter': {
        const page = await chapter({ data })
        if (page.chapter.href.startsWith('/story/'))
          throw redirect({ href: page.chapter.href })
        return {
          kind: 'chapter' as const,
          page,
          title: page.chapter.title,
          summary: page.chapter.intro,
        }
      }
      case 'scene': {
        const page = await scene({ data })
        return {
          kind: 'scene' as const,
          page,
          title: page.scene.title,
          summary: page.scene.text,
        }
      }
      default:
        throw notFound()
    }
  },
  head: ({ loaderData }) => ({
    meta: [
      {
        title: loaderData
          ? loaderData.title + ' · ' + loaderData.page.work.title
          : '未找到内容',
      },
      { name: 'description', content: loaderData?.summary },
    ],
  }),
  component: WorldDetail,
})
function WorldDetail() {
  const data = Route.useLoaderData()
  switch (data.kind) {
    case 'character':
      return <PersonPage {...data.page} />
    case 'chapter':
      return <ChapterPage {...data.page} />
    case 'scene':
      return <ScenePage {...data.page} />
  }
}
