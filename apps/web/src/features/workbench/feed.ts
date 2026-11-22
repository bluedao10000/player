import { localWorkbenchRequest } from './local'
import { getCookie, getRequest } from '@tanstack/react-start/server'
import { createServerFn } from '@tanstack/react-start'
import type { ScopeAcceptance, MediaCandidate, ResearchRun, WorkKnowledgeStatus, ProductionUsage } from '@anime/shared/knowledge'

type QueueRow = {
  world_slug: string; number: number; status: string; priority: number; token_budget: number; committed: number; measured_tokens: number | null
  capture_failures?: { at: string; reason: string }[]; capture_error?: string; capture_started?: string; capture_finished?: string
}
type ReaderRelease = {
  topic: string; world_slug: string; number: number; title: string; admitted_at: string; measured_tokens: number
  started_at: string | null; finished_at: string | null; error: string | null; href: string | null; status: string
}
export type BackupRun = {
  attempts?: { at: string; reason: string }[]
  job_uid: string; job_name: string; status: 'running' | 'success' | 'failed'
  started_at: string; finished_at: string | null; archive_bytes: number | null
  verified_tables: number | null; failure_reason: string | null; trigger_kind: 'manual' | 'scheduled' | 'unknown'
}
export type BackupStatus = { observed_at: string | null; latest_verified: BackupRun | null; runs: BackupRun[] }
type Inventory = { checked: string; works: { slug: string; title: string; published: number; missing?: number[]; today?: number[]; source?: string; note?: string }[] }
type Status = 'running' | 'pending' | 'success' | 'failed'
type Sync = {
  id: string; source: string; status: Status; started_at: string; finished_at: string | null; skip_reason: string | null
  records_read: number; records_updated: number; records_created: number; http_status: number | null; records_failed: number; error_message: string | null
}
type Quality = {
  id: string; started_at: string; finished_at: string | null; status: Status
  report: { works: number; mean_score: number; issues: Record<string, number> } | null
}

export type WorkbenchRun = {
  id: string; task: string; start: string; finish: string | null; status: Status; skip: string | null
  read: number; updated: number; issues: number | null; http: number | null; failed: number | null; score: number | null; error: string | null
}
export type WorkbenchFeed = {
  runs: WorkbenchRun[]; releases: ReaderRelease[] | null; backup: BackupStatus | null; queue: QueueRow[] | null
  inventory: Inventory | null; worlds: WorkKnowledgeStatus[] | null; research: ResearchRun[] | null; scopes: ScopeAcceptance[] | null
  usage: ProductionUsage[] | null; media: MediaCandidate[] | null; syncOnline: boolean; qualityOnline: boolean; checked: string
}

export type OverviewWorkType = { type: string | null; count: number }
export type OverviewMetricValue = number | OverviewWorkType[]
export type OverviewMetric<T extends OverviewMetricValue = OverviewMetricValue> = { value: T | null; definition: number | null; updated_at: string | null }
export type OverviewAttention = {
  id: string; kind: string; severity: 'critical' | 'high' | 'medium' | 'info'; stage: string; work: string | null
  world_slug: string | null; subject: string | null; reason_code: string; since: string | null
  link: { page: string; params: Record<string, string | number | null> }
}
export type OpsOverview = {
  as_of: string
  metrics: Record<string, OverviewMetric>
  attention: OverviewAttention[]
}
export type OpsAttentionFeed = { as_of: string; attention: OverviewAttention[] }

async function getJSON<T>(path: string, adminToken?: string, withLimit = true): Promise<T | null> {
  try {
    const base = process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321'
    const url = new URL('/api/v1/' + (adminToken ? 'admin/' : '') + 'knowledge/' + path, base)
    if (withLimit) url.searchParams.set('limit', '30')
    const response = await fetch(url, {
      signal: AbortSignal.timeout(5000),
      cache: 'no-store',
      headers: adminToken ? { Authorization: 'Bearer ' + adminToken } : {},
    })
    if (response.ok) return (await response.json()) as T
  } catch {
    // Keep unavailable feeds distinct from empty results.
  }
  return null
}

async function validEditorSession() {
  const request = getRequest()
  if (!localWorkbenchRequest(request)) return false
  const session = getCookie('folio_workbench')
  if (!session) return false
  try {
    const response = await fetch(new URL('/api/v1/editor/account', process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321'), {
      headers: { Authorization: 'Bearer ' + session }, cache: 'no-store', redirect: 'error', signal: AbortSignal.timeout(5000),
    })
    if (!response.ok) return false
    return Boolean((await response.json()).editor)
  } catch {
    return false
  }
}

export const readWorkbenchSession = createServerFn({ method: 'GET' }).handler(validEditorSession)

function normalizeRuns(sync: Sync[] | null, quality: Quality[] | null): WorkbenchRun[] {
  return [
    ...(sync ?? []).map((row) => ({
      id: 'source_sync:' + row.id, task: row.source, start: row.started_at, finish: row.finished_at, status: row.status,
      skip: row.skip_reason, read: row.records_read, updated: row.records_updated + row.records_created, issues: null,
      http: row.http_status, failed: row.records_failed, score: null, error: row.error_message,
    })),
    ...(quality ?? []).map((row) => ({
      id: 'quality:' + row.id, task: '质量扫描', start: row.started_at, finish: row.finished_at, status: row.status,
      skip: null, read: row.report?.works ?? 0, updated: 0, http: null, failed: null,
      score: row.report?.mean_score ?? null, error: null,
      issues: row.report ? Object.values(row.report.issues).reduce((sum, value) => sum + value, 0) : null,
    })),
  ].sort((a, b) => Date.parse(b.start) - Date.parse(a.start))
}

function emptyFeed(patch: Partial<WorkbenchFeed> = {}): WorkbenchFeed {
  return {
    runs: [], releases: null, backup: null, queue: null, inventory: null, worlds: null, research: null, scopes: null,
    usage: null, media: null, syncOnline: false, qualityOnline: false, checked: new Date().toISOString(), ...patch,
  }
}

export const readOpsOverview = createServerFn({ method: 'GET' }).handler(async () => {
  if (!(await validEditorSession())) return null
  const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
  return admin ? getJSON<OpsOverview>('ops/overview', admin, false) : null
})

export const readOpsAttention = createServerFn({ method: 'GET' }).handler(async () => {
  if (!(await validEditorSession())) return null
  const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
  return admin ? getJSON<OpsAttentionFeed>('ops/attention', admin, false) : null
})

export const readOperationsFeed = createServerFn({ method: 'GET' }).handler(async () => {
  if (!(await validEditorSession())) return null
  const [sync, quality, research, media] = await Promise.all([
    getJSON<Sync[]>('sync-runs'), getJSON<Quality[]>('quality-runs'),
    getJSON<ResearchRun[]>('research-runs'), getJSON<MediaCandidate[]>('media-candidates'),
  ])
  return emptyFeed({ runs: normalizeRuns(sync, quality), research, media, syncOnline: sync !== null, qualityOnline: quality !== null })
})

export const readKnowledgeFeed = createServerFn({ method: 'GET' }).handler(async () => {
  if (!(await validEditorSession())) return null
  const [worlds, scopes, media] = await Promise.all([
    getJSON<WorkKnowledgeStatus[]>('world-statuses'), getJSON<ScopeAcceptance[]>('scope-acceptances'), getJSON<MediaCandidate[]>('media-candidates'),
  ])
  return emptyFeed({ worlds, scopes, media })
})

export const readProjectFeed = createServerFn({ method: 'GET' }).handler(async () => {
  if (!(await validEditorSession())) return null
  const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
  const [releases, queue, inventoryRows, usage] = await Promise.all([
    admin ? getJSON<ReaderRelease[]>('reader-releases', admin) : Promise.resolve(null),
    admin ? getJSON<QueueRow[]>('episode-queue', admin) : Promise.resolve(null),
    admin ? getJSON<Inventory[]>('episode-inventory', admin) : Promise.resolve(null),
    admin ? getJSON<ProductionUsage[]>('production-usage', admin) : Promise.resolve(null),
  ])
  return emptyFeed({ releases, queue, inventory: inventoryRows?.[0] ?? null, usage })
})

export const readAnalyticsFeed = createServerFn({ method: 'GET' }).handler(async () => {
  if (!(await validEditorSession())) return null
  const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
  const [usage, backup] = await Promise.all([
    admin ? getJSON<ProductionUsage[]>('production-usage', admin) : Promise.resolve(null),
    admin ? getJSON<BackupStatus>('backup-status', admin) : Promise.resolve(null),
  ])
  return emptyFeed({ usage, backup })
})
