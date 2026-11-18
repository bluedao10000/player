// Server-side client used by TanStack Start server functions and the Pi worker.
export type WorkSummary = {
  id: string
  canonical_title: string
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
    license: string | { name: string; url: string }
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
