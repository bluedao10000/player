// Public knowledge client used by the reader apps.
export type WorkSummary = {
  id: string
  canonical_title: string
  world_href?: string | null
  cover_image?: string | null
  original_title: string | null
  type: string
  episodes: number | null
  status: string | null
  season_year: number | null
}
export type WorkPage = { items: WorkSummary[]; next_cursor: string | null }
export type WorkDetail = WorkSummary & {
  titles: { lang: string; title: string }[]
  tags: string[]
  studios: string[]
  producers: string[]
  broadcasts: Record<string, unknown>[]
  relations: { url: string; related_work_id: string | null }[]
  sources: { provider: string; external_id: string; url: string }[]
  provenance: {
    source: string
    repository: string
    version: string
    license: string | { name: string; url: string } | null
  }[]
}
export class KnowledgeError extends Error {
  status: number
  constructor(status: number) {
    super('Knowledge API returned HTTP ' + status)
    this.status = status
  }
}
export function serverKnowledgeApiUrl(): string {
  return process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321'
}

async function request<T>(path: string): Promise<T> {
  const base = typeof window === 'undefined' ? serverKnowledgeApiUrl() : window.location.origin
  const response = await fetch(new URL('/api/v1/knowledge/' + path, base), {
    signal: AbortSignal.timeout(12000),
  })
  if (!response.ok) throw new KnowledgeError(response.status)
  return (await response.json()) as T
}
export function searchWorks(q: string, after = '', limit = 20) {
  return request<WorkPage>(
    'works?' + new URLSearchParams({ q, after, limit: String(limit) }),
  )
}
export function getWork(id: string) {
  if (!/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i.test(id))
    throw new KnowledgeError(400)
  return request<WorkDetail>('works/' + encodeURIComponent(id))
}

export type WorldCard = {
  work_id: string
  slug: string
  title: string
  edition_label: string
  intro: string
  cast_title: string
  hero_asset_key: string | null
  href: string
  revision: number
  origin: 'original' | 'external'
  // From the canonical work type. Branch on this, never on edition_label text.
  format?: 'film' | 'series'
}
export type CharacterCard = {
  slug: string
  name: string
  role: string
  portrait?: string
  href: string
}
export type ChapterCard = {
  slug: string
  title: string
  label: string
  episode?: number
  intro: string
  image?: string
  image_caption?: string
  image_credit?: string
  href: string
}
export type SceneCard = {
  slug: string
  title: string
  place: string
  image?: string
  image_caption?: string
  image_credit?: string
  chapter_slug: string
  href: string
}
export type ReadingBlock = { key: string; text: string; anchors: string[] }
type Attribution = { uri: string; locator: string }
export type EpisodeIllustration = {
  name: string; chapter: string; block: string; anchor: string; image: string
}
export type WorldOverview = {
  world_setting?: string
  completed_scope: {first: number; last: number; picks: EpisodeIllustration[]} | null
  reader_intro: { premise: string; setting: string } | null
  work: WorldCard
  characters: CharacterCard[]
  scenes: SceneCard[]
  storylines: {
    slug: string
    title: string
    intro: string
    chapters: ChapterCard[]
  }[]
}
export type HomeWorld = WorldCard & { scene_previews: SceneCard[] }
export type CharacterDetail = {
  intro: string | null
  scene_previews: SceneCard[]
  work: WorldCard
  character: CharacterCard
  appearances: ChapterCard[]
}
export type ResearchParagraph = {
  episode: number
  kind: 'motive' | 'turn' | 'cause'
  text: string
  cites: { id: string; url: string; locator: string; quote: string }[]
}
export type ChapterDetail = {
  accepted_art: EpisodeIllustration[] | null
  reader_passages: ReadingBlock[] | null
  research: { run_id: string; paragraphs: ResearchParagraph[] } | null
  next_chapter: ChapterCard | null
  work: WorldCard
  chapter: ChapterCard
  blocks: ReadingBlock[]
  scenes: SceneCard[]
  content_kind: 'original' | 'editorial_guide'
  attribution: Attribution[]
}
export type SceneDetail = {
  work: WorldCard
  chapter: ChapterCard
  scene: SceneCard & { text: string }
  scene_cast: CharacterCard[]
  return_to: { block_key: string; href: string }
  attribution: Attribution[]
}

function worldKey(key: string) {
  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(key)) throw new KnowledgeError(404)
  return key
}
export type DiscoveryWork = WorldCard & { chapters: ChapterCard[]; characters: CharacterCard[]; scene: SceneCard | null }
export const discoveryCatalog = () => request<{ items: DiscoveryWork[] }>('discovery')
export const homeWorlds = () =>
  request<{ items: HomeWorld[] }>('worlds?placement=home')
export const getWorld = (work: string) =>
  request<WorldOverview>('worlds/' + worldKey(work))
export const worldCharacter = (work: string, slug: string) =>
  request<CharacterDetail>(
    `worlds/${worldKey(work)}/characters/${worldKey(slug)}`,
  )
export const worldChapter = (work: string, slug: string) =>
  request<ChapterDetail>(`worlds/${worldKey(work)}/chapters/${worldKey(slug)}`)
export const worldScene = (work: string, slug: string) =>
  request<SceneDetail>(`worlds/${worldKey(work)}/scenes/${worldKey(slug)}`)
