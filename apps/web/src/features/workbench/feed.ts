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

export type CompletenessItem = {
  data_state: 'available' | 'no_data'
  completed_required_items: number | null
  applicable_required_items: number | null
  status: 'completed' | 'partial' | 'missing' | 'not_applicable' | 'pending_verification' | null
  details: { item_key: string; status: string; evidence_ref?: string; reason?: string }[]
}
export type CompletenessDimension = { key: string; status: string; items: Record<string, CompletenessItem> }
export type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue }
export type KnowledgeCompleteness = {
  available: boolean; work_id: string; world_slug: string | null; version_id: number | null
  rule_version: number; calculated_at: string | null; dimensions: CompletenessDimension[]
}
export type KnowledgeSummary = {
  as_of: string; definition: number
  works: { active_total: number; by_type: { type: string | null; count: number }[]; published_worlds: number; release_versions: number }
  people: { total: number; introductions_complete: number | null; relationships: { availability: string } }
  plot: { storylines: number; chapters: number; episodes_expected: number; episodes_covered: number; scenes: number }
  world: { locations: number; organizations: { availability: string }; typed_settings: { availability: string } }
  timeline: { availability: string; events: number | null; chapter_links: number | null; order_checks: number | null }
  images: { main_visuals: number; portraits: number; chapter_images: number }
  sources: { records_by_source: { name: string; type: string; records: number; works: number }[]; external_ids_by_provider: { provider: string; external_ids: number; works: number }[] }
  trend: { range_days: number; points: { day: string; definition: number; completeness: JsonValue; [key: string]: JsonValue }[] }
}
export type KnowledgeWorkRow = {
  work_id: string; world_slug: string | null; title: string; type: string | null; release_versions: number
  edition: string | null; episodes_total: number | null; episodes_covered: number | null; characters: number | null
  scenes: number | null; images: number | null; completeness: { dimensions: CompletenessDimension[] } | null
  acceptance_pass_rate: number | null; production_status: string; last_updated_at: string | null; has_knowledge_version: boolean
}
export type KnowledgeWorksPage = { as_of: string; definition: number; items: KnowledgeWorkRow[]; next_cursor: string | null }
export type KnowledgeWorkDetail = {
  overview: {
    work_id: string; world_slug: string | null; title: string; original_title: string | null; type: string | null
    episodes_total: number | null; release_versions: number; version_id: number | null; edition: string | null
    world_name: string | null; world_intro: string | null; origin: string | null; published_at: string | null
    has_knowledge_version: boolean; completeness: { dimensions: CompletenessDimension[] } | null
    sources: { external_ids: { provider: string; external_id: string; url: string }[]; records: { source: string; type: string; record_key: string; version_id: number }[] }
  }
  characters: { slug: string; name: string | null; role: string | null; introduction: string | null; portrait: string | null }[]
  storylines: { slug: string; title: string; intro: string; chapters: { slug: string; title: string }[] }[]
  episodes: { total: number | null; readable: number[]; scopes: { topic: string; number: number; first: number; last: number; passed: boolean | null; checked_at: string | null }[] }
  scenes: { slug: string; name: string; chapter: string; place: string; text: string | null; image: string | null; missing_text: boolean }[]
  world_settings: { locations: { name: string }[]; organizations: { availability: string }; typed_settings: { availability: string } }
  timeline: { availability: string }
  images: { main_visual: string | null; portraits: { slug: string; name: string | null; image: string | null }[]; chapters: { slug: string; title: string | null; image: string | null }[] }
  quality_report: { score: { score: number; components: Record<string, JsonValue>; checked_at: string } | null; open_issues: { rule_id: string; field: string; severity: string; reason: string; checked_at: string }[] }
  acceptance_pass_rate: number | null
  completeness_calculated_at: string | null
}
export type OpsTopologyWindow = '24h' | '7d' | '30d'
export type OpsTopologyCurrent = { total: number; running: number; pending: number; blocked: number; waiting_human: number }
export type OpsTopologyWindowStats = {
  success: number; failed: number; skipped: number; completed: number; success_rate: number | null
  outcome_distribution: Record<string, number>
  duration: { sample_count: number; average_seconds: number | null; median_seconds: number | null; p90_seconds: number | null }
  duration_primary: 'median'; throughput_per_hour: number; sample_quality: 'low' | 'sufficient'; recent_execution_at: string | null
}
export type OpsTopologyAgents = {
  mapped: boolean; online_workers: number | null; workers_with_error: number | null; worker_label?: string
  freshness?: 'online' | 'offline' | null; last_heartbeat_at?: string | null
  running_executions: number | null; execution_coverage: 'not_applicable' | 'legacy_aggregate' | 'stage_specific'; legacy_unclassified: number
}
export type OpsTopologyStage = {
  id: string; title: string; description: string; grp: string; topology_group: string | null
  position: number; upstream: string[]; tracked: boolean; global_scope: boolean; has_records: boolean | null
  bottleneck: boolean; current: OpsTopologyCurrent | null; window_stats: OpsTopologyWindowStats | null; agents: OpsTopologyAgents | null
}
export type OpsTopologyGroup = { id: string; label: string; eyebrow: string; position: number; stages: OpsTopologyStage[] }
export type OpsTopology = {
  as_of: string; timezone: 'Asia/Shanghai'; window: OpsTopologyWindow; window_start: string; window_end: string; definition: number
  selected_world: null | {
    slug: string; title: string; edition: string; work_id: string; has_records: boolean
    progress: { scope_total: number; unregistered: number; registered: number; planned: number; in_production: number; accepted: number; published: number }
  }
  groups: OpsTopologyGroup[]
  infrastructure: { id: string; label: string; eyebrow: string; stages: OpsTopologyStage[] }
}
export type OpsTopologyDetail = {
  as_of: string; stage: OpsTopologyStage
  workers: { role: string; loop: string; worker_id: string | null; node: string | null; version: string | null; state: string | null; last_error: string | null; last_seen: string | null }[]
  current_queue: { item_key: string; stage: string; world_slug: string | null; episode: number | null; subject: string; state: string; reason: string; since: string | null; link: { page: string; params: Record<string, string | number | null> } }[]
  recent_runs: { run_key: string; status: string; outcome: string | null; started_at: string; finished_at: string | null; duration_seconds: number | null; error: string | null; world_slug: string | null; episode_first: number | null; episode_last: number | null; source_table: string; source_id: string; link: { page: string; params: Record<string, string | number | null> } }[]
  failure_reasons: { reason: string; count: number }[]
  pagination: { limit: number; queue_after: string | null; runs_after: string | null }
}
export type KnowledgeCenterFeed = { summary: KnowledgeSummary | null; works: KnowledgeWorksPage | null; checked: string }
export type KnowledgeWorkFeed = { detail: KnowledgeWorkDetail | null; scopes: ScopeAcceptance[] | null; media: MediaCandidate[] | null; checked: string }

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

export type OpsTopologyQuery = { window: OpsTopologyWindow; world: string }
export const readOpsTopology = createServerFn({ method: 'GET' })
  .validator((query: OpsTopologyQuery) => query)
  .handler(async ({ data }): Promise<OpsTopology | null> => {
    if (!(await validEditorSession())) return null
    const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
    if (!admin) return null
    const params = new URLSearchParams({ window: data.window })
    if (data.world) params.set('world', data.world)
    return getJSON<OpsTopology>('ops/topology?' + params.toString(), admin, false)
  })

export type OpsTopologyDetailQuery = {
  stage: string; window: OpsTopologyWindow; world: string; limit: number; queue_after?: string; runs_after?: string
}
export const readOpsTopologyDetail = createServerFn({ method: 'GET' })
  .validator((query: OpsTopologyDetailQuery) => query)
  .handler(async ({ data }): Promise<OpsTopologyDetail | null> => {
    if (!(await validEditorSession())) return null
    const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
    if (!admin) return null
    const params = new URLSearchParams({ window: data.window, limit: String(data.limit) })
    if (data.world) params.set('world', data.world)
    if (data.queue_after) params.set('queue_after', data.queue_after)
    if (data.runs_after) params.set('runs_after', data.runs_after)
    return getJSON<OpsTopologyDetail>('ops/topology/stages/' + encodeURIComponent(data.stage) + '?' + params.toString(), admin, false)
  })

export const readOperationsFeed = createServerFn({ method: 'GET' }).handler(async () => {
  if (!(await validEditorSession())) return null
  const [sync, quality, worlds] = await Promise.all([
    getJSON<Sync[]>('sync-runs'), getJSON<Quality[]>('quality-runs'), getJSON<WorkKnowledgeStatus[]>('world-statuses'),
  ])
  return emptyFeed({ runs: normalizeRuns(sync, quality), worlds, syncOnline: sync !== null, qualityOnline: quality !== null })
})

export const readKnowledgeFeed = createServerFn({ method: 'GET' }).handler(async () => {
  if (!(await validEditorSession())) return null
  const [worlds, scopes, media] = await Promise.all([
    getJSON<WorkKnowledgeStatus[]>('world-statuses'), getJSON<ScopeAcceptance[]>('scope-acceptances'), getJSON<MediaCandidate[]>('media-candidates'),
  ])
  return emptyFeed({ worlds, scopes, media })
})

export const readKnowledgeCenterFeed = createServerFn({ method: 'GET' }).handler(async (): Promise<KnowledgeCenterFeed | null> => {
  if (!(await validEditorSession())) return null
  const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
  if (!admin) return { summary: null, works: null, checked: new Date().toISOString() }
  const [summary, works] = await Promise.all([
    getJSON<KnowledgeSummary>('ops/knowledge/summary?range=30d', admin, false),
    getJSON<KnowledgeWorksPage>('ops/knowledge/works?has_knowledge_version=true&limit=50&sort=last_updated_at', admin, false),
  ])
  return { summary, works, checked: summary?.as_of ?? works?.as_of ?? new Date().toISOString() }
})

export type KnowledgeWorkFilters = {
  q: string; has_knowledge_version: 'true' | 'false' | 'all'; type: string; completeness_status: string
  production_status: string; sort: string; after: string
}

export const readKnowledgeWorksPage = createServerFn({ method: 'GET' })
  .validator((filters: KnowledgeWorkFilters) => filters)
  .handler(async ({ data }): Promise<KnowledgeWorksPage | null> => {
    if (!(await validEditorSession())) return null
    const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
    if (!admin) return null
    const params = new URLSearchParams({
      q: data.q, has_knowledge_version: data.has_knowledge_version, sort: data.sort, limit: '50',
    })
    if (data.type) params.set('type', data.type)
    if (data.completeness_status) params.set('completeness_status', data.completeness_status)
    if (data.production_status) params.set('production_status', data.production_status)
    if (data.after) params.set('after', data.after)
    return getJSON<KnowledgeWorksPage>('ops/knowledge/works?' + params.toString(), admin, false)
  })

export const readKnowledgeWorkFeed = createServerFn({ method: 'GET' })
  .validator((work: string) => work)
  .handler(async ({ data: work }): Promise<KnowledgeWorkFeed | null> => {
    if (!(await validEditorSession())) return null
    const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
    if (!admin) return { detail: null, scopes: null, media: null, checked: new Date().toISOString() }
    const [detail, scopes, media] = await Promise.all([
      getJSON<KnowledgeWorkDetail>('ops/knowledge/works/' + encodeURIComponent(work), admin, false),
      getJSON<ScopeAcceptance[]>('scope-acceptances'),
      getJSON<MediaCandidate[]>('media-candidates'),
    ])
    const slug = detail?.overview.world_slug
    return {
      detail,
      scopes: slug ? scopes?.filter((scope) => scope.world === slug) ?? null : [],
      media: slug ? media?.filter((item) => item.world_slug === slug) ?? null : [],
      checked: detail?.completeness_calculated_at ?? new Date().toISOString(),
    }
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
