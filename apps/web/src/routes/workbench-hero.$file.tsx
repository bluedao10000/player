import { createFileRoute, notFound } from '@tanstack/react-router'
import { CharacterHero } from '../features/post/character'
import { bebopArt, casinoArt } from '@anime/shared/media'
import { Hero } from '../features/worlds/pages'
import { overview, character } from '../features/worlds/api'
import css from '../features/post/character.css?url'

export const Route = createFileRoute('/workbench-hero/$file')({
  validateSearch: (s: Record<string, unknown>) => {
    const focus = Number(s.focus ?? 40)
    return {
      focus: Number.isInteger(focus) && focus >= 0 && focus <= 100 ? focus : 40,
    }
  },
  loader: async ({ params }) => {
    const match = /^(mio|bebop|bebop-e3)-([1-9])-r[12]\.png$/.exec(params.file)
    if (!match) throw notFound()
    if (match[1] !== 'mio') {
      const page = await overview({ data: 'cowboy-bebop-tv' })
      const scene =
        match[1] === 'bebop-e3'
          ? casinoArt
          : bebopArt[Math.floor((Number(match[2]) - 1) / 3)]
      return { kind: 'scene' as const, work: page.work, scene }
    }
    const page = await character({ data: { work: 'tidepost', slug: 'mio' } })
    return { kind: 'character' as const, character: page.character }
  },
  head: () => ({
    links: [{ rel: 'stylesheet', href: css }],
    meta: [
      { name: 'robots', content: 'noindex' },
      { title: '场景试稿 · 手机预览' },
    ],
  }),
  component: HeroPreview,
})
function HeroPreview() {
  const data = Route.useLoaderData()
  const src = '/workbench-art/' + Route.useParams().file
  const focus = Route.useSearch().focus
  if (data.kind === 'scene')
    return (
      <div style={{ marginInline: 16 }}>
        <Hero
          work={data.work}
          image={src}
          focus={focus}
          label={`星际牛仔 · 第 ${data.scene.episode} 集`}
          title={data.scene.title}
        />
      </div>
    )
  return (
    <CharacterHero
      name={data.character.name}
      role={data.character.role}
      preview={{ src, focus }}
    />
  )
}
