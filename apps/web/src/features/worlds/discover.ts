import { createServerFn } from '@tanstack/react-start'
import { discoveryCatalog } from '@anime/shared/knowledge'
import type { DiscoveryWork } from '@anime/shared/knowledge'

export type Directory = 'works' | 'characters' | 'stories'
export function discoverySearch(input: Record<string, unknown>) {
  const clean = (value: unknown) =>
    typeof value === 'string'
      ? value.trim().slice(0, 160) || undefined
      : undefined
  return { q: clean(input.q), work: clean(input.work) }
}
type Search = ReturnType<typeof discoverySearch>

function workCard(w: DiscoveryWork) {
  return {
    title: w.title,
    href: w.href,
    slug: w.slug,
    edition: w.edition_label,
    image: w.hero_asset_key,
    intro: w.intro,
    chapters: w.chapters.length,
  }
}
export type DiscoverCard = ReturnType<typeof workCard>
function portraits(w: DiscoveryWork) {
  const byIntroduction = (name: string) => {
    const index = w.intro.indexOf(name)
    return index < 0 ? Infinity : index
  }
  return [...w.characters]
    .sort((a, b) => byIntroduction(a.name) - byIntroduction(b.name))
    .map((p) => ({
      title: p.name,
      href: p.href,
      subtitle: p.role,
      image: p.portrait,
      work: w.title,
    }))
}
function stories(w: DiscoveryWork) {
  return w.chapters.map((c) => ({
    title: c.title,
    href: c.href,
    subtitle: c.intro,
    label: c.label,
    image: c.image,
    work: w.title,
  }))
}
export type Portrait = ReturnType<typeof portraits>[number]
export type StoryEntry = ReturnType<typeof stories>[number]

function recentEditions(works: DiscoveryWork[]) {
  const year = (work: DiscoveryWork) =>
    Number(work.edition_label.match(/\b(?:19|20)\d{2}\b/)?.[0] ?? 0)
  return [...works].sort((a, b) => year(b) - year(a))
}
export const discoverHome = createServerFn({ method: 'GET' }).handler(
  async () => {
    const { items } = await discoveryCatalog()
    const ordered = recentEditions(items)
    const lead = ordered[0]
    const classic = items.find(
      (w) =>
        w.origin === 'external' &&
        w !== lead &&
        w.edition_label.slice(0, 4) !== lead?.edition_label.slice(0, 4),
    )
    const original = items.find((w) => w.origin === 'original')
    const featured = [
      ...new Set(
        [lead, classic, original, ...ordered].filter(
          (w): w is DiscoveryWork => !!w,
        ),
      ),
    ].slice(0, 3)
    const others = ordered.filter((w) => !featured.includes(w))
    return {
      total: items.length,
      featured: featured.map(workCard),
      covers: others.slice(0, 6).map(workCard),
      portraits: ordered
        .flatMap((w) => {
          const cast = portraits(w)
          const lead = cast.find((p) => p.image) ?? cast[0]
          return lead ? [lead] : []
        })
        .slice(0, 6),
      scenes: [classic, ...others]
        .flatMap((w) => {
          const scene = w?.scene
          if (!w || !scene) return []
          const image =
            scene.image ??
            w.chapters.find((c) => c.slug === scene.chapter_slug)?.image
          return [
            {
              href: scene.href,
              title: scene.title,
              work: w.title,
              place: scene.place,
              image,
            },
          ]
        })
        .slice(0, 2),
      stories: [...others, ...featured]
        .flatMap((w) => stories(w).slice(0, 1))
        .slice(0, 4),
    }
  },
)

export const discoverDirectory = createServerFn({ method: 'GET' })
  .validator((data: Search & { kind: Directory }) => data)
  .handler(async ({ data }) => {
    const { items } = await discoveryCatalog()
    const works = recentEditions(items)
    const query = data.q?.toLocaleLowerCase() ?? ''
    const matches = (...words: string[]) =>
      words.some((word) => word.toLocaleLowerCase().includes(query))
    const scoped = works.filter((w) => !data.work || w.slug === data.work)
    const groups = scoped
      .map((w) => ({
        work: workCard(w),
        portraits:
          data.kind === 'characters'
            ? portraits(w).filter((p) => matches(w.title, p.title, p.subtitle))
            : [],
        stories:
          data.kind === 'stories'
            ? stories(w).filter((c) => matches(w.title, c.title, c.label))
            : [],
      }))
      .filter((g) =>
        data.kind === 'works'
          ? matches(g.work.title, g.work.edition)
          : g.portraits.length + g.stories.length > 0,
      )
    return {
      groups,
      options: works.map((w) => ({
        slug: w.slug,
        title: w.title,
        edition: w.edition_label,
      })),
    }
  })
