import { chapterHead, characterHead, sceneHead } from '../features/seo/world'
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
        }
      }
      case 'chapter': {
        const page = await chapter({ data })
        if (page.chapter.href.startsWith('/story/'))
          throw redirect({ href: page.chapter.href })
        return {
          kind: 'chapter' as const,
          page,
        }
      }
      case 'scene': {
        const page = await scene({ data })
        return {
          kind: 'scene' as const,
          page,
        }
      }
      default:
        throw notFound()
    }
  },
  head: ({ loaderData }) => {
    switch (loaderData?.kind) {
      case 'character': return characterHead(loaderData.page)
      case 'chapter': return chapterHead(loaderData.page)
      case 'scene': return sceneHead(loaderData.page)
      default: return {}
    }
  },
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
