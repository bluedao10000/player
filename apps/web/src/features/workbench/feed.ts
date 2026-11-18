import { createServerFn } from '@tanstack/react-start'

type Status = 'running' | 'pending' | 'success' | 'failed'
type Sync = {
  id: string
  source: string
  status: Status
  started_at: string
  finished_at: string | null
  skip_reason: string | null
  records_read: number
  records_updated: number
  records_created: number
  http_status: number | null
  records_failed: number
}
type Quality = {
  id: string
  started_at: string
  finished_at: string | null
  status: Status
  report: {
    works: number
    mean_score: number
    issues: Record<string, number>
  } | null
}

async function get<T>(path: string): Promise<T[] | null> {
  try {
    const base = process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321'
    const res = await fetch(
      new URL('/api/v1/knowledge/' + path + '?limit=30', base),
      { signal: AbortSignal.timeout(5000), cache: 'no-store' },
    )
    if (res.ok) return (await res.json()) as T[]
  } catch {
    /* Show an unavailable feed separately from an empty one. */
  }
  return null
}
export const readFeed = createServerFn({ method: 'GET' }).handler(async () => {
  const [sync, quality] = await Promise.all([
    get<Sync>('sync-runs'),
    get<Quality>('quality-runs'),
  ])
  const runs = [
    ...(sync ?? []).map((r) => ({
      id: r.id,
      task: r.source,
      start: r.started_at,
      finish: r.finished_at,
      status: r.status,
      skip: r.skip_reason,
      read: r.records_read,
      updated: r.records_updated + r.records_created,
      issues: null as number | null,
      http: r.http_status,
      failed: r.records_failed,
      score: null as number | null,
    })),
    ...(quality ?? []).map((r) => ({
      id: r.id,
      task: '质量扫描',
      start: r.started_at,
      finish: r.finished_at,
      status: r.status,
      skip: null,
      read: r.report?.works ?? 0,
      updated: 0,
      http: null,
      failed: null,
      score: r.report?.mean_score ?? null,
      issues: r.report
        ? Object.values(r.report.issues).reduce((a, b) => a + b, 0)
        : null,
    })),
  ].sort((a, b) => Date.parse(b.start) - Date.parse(a.start))
  return {
    runs,
    syncOnline: sync !== null,
    qualityOnline: quality !== null,
    checked: new Date().toISOString(),
  }
})
