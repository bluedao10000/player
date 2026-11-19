// Server-side client used by TanStack Start server functions and the Pi worker.
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
async function request<T>(path: string): Promise<T> {
  const base = process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321'
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
export type ProductionUsage = {
  world: string
  title: string
  edition: string
  registered_at: string
  started_at: string | null
  published_at: string | null
  last_media_at: string | null
  failed_research: number
  episodes?: {
    number: number
    source_number: number | null
    started_at: string | null
    published_at: string | null
    failed_research: number
  }[]
  operations: {
    number?: number | null
    purpose: string
    operation: string
    model: string
    calls: number
    errors: number
    unmetered: number
    tokens: number | null
    images: number | null
    seconds: number | null
  }[]
}
export type ScopeAcceptance = {
  id: number; world: string; edition_title: string; first: number; last: number; version: number; rule: number; checked_at: string
  previous_gaps?: {id:number; items:string[]} | null
  published: boolean; passed: boolean; picks: (EpisodeIllustration & {variant: number; round: number})[]
  checks: {name: string; items: {label: string; evidence?: string; source?: string; gap?: string}[]}[]
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
export type ResearchRun = {
  edition_title?: string
  id: string
  started_at: string
  finished_at: string | null
  stage: 'collect' | 'write' | 'ready' | 'failed'
  attempt: number
  policy: number
  current_policy: boolean
  model: string | null
  error: string | null
  world_slug: string
  first_episode: number
  last_episode: number
  version_id: number
  paragraphs: number | null
  documents: {id: string; url: string; locator: string; sha256: string; fetched_at: string}[] | null
  events: {at: string; message: string}[] | null
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

/** Inventory of the current public version; counts are independent of quality acceptance. */
export type WorkKnowledgeStatus = {
  work_id: string
  title: string
  release_status: string | null
  total_episodes: number | null
  slug: string | null
  edition: string | null
  revision: number | null
  version_id: number | null
  published_at: string | null
  maturity: 'basic' | 'linked'
  visibility: 'catalog' | 'sample'
  characters: number
  characters_in_scenes: number
  chapters: number
  readable_episodes: number[]
  storylines: number
  scenes: number
  settings: number
  has_hero: boolean
  portraits: number
  chapter_images: number
}

export type MediaStage = 'reference'|'queued'|'generate'|'review'|'recrop'|'recheck'|'passed'|'held'|'failed'
export type MediaStep = {
  attempt:number
  round:number
  step:MediaStage
  started_at:string
  finished_at:string|null
  error:string|null
  result:{
    source?:string;firstOpinion?:string;secondModel?:string;reference?:string;image?:string;phone?:string;sha?:string;reference_sha?:string;model?:string
    decision?:'pass'|'recrop'|'redraw';reason?:string;focus?:number
    checks?:Record<'identity'|'scene'|'anatomy'|'clean'|'crop',boolean>
  }|null
}
export type MediaCandidate = {
  reader_href?: string | null;
  character_slug?: string | null;
  art?: {kind:"scene"|"portrait";series:string;title:string;episode:number}; batch:string;position:number;reference_sha:string;
  slot:number;scene:string;variant:number;stage:MediaStage;round:number;reason:string;steps:MediaStep[]
}
