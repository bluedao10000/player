import { localWorkbenchRequest } from "./local";
import { getRequest, getCookie } from '@tanstack/react-start/server'
import { createServerFn } from "@tanstack/react-start";

import type { ScopeAcceptance, MediaCandidate, ResearchRun, WorkKnowledgeStatus, ProductionUsage } from "@anime/shared/knowledge";

type QueueRow = {world_slug:string; number:number; status:string; priority:number; token_budget:number; committed:number; measured_tokens:number|null; capture_failures?:{at:string;reason:string}[];capture_error?:string; capture_started?:string; capture_finished?:string}

type ReaderRelease = {
  topic: string; world_slug: string; number: number; title: string; admitted_at: string; measured_tokens: number;
  started_at: string | null; finished_at: string | null; error: string | null; href: string | null; status: string
}

type BackupRun = {
  attempts?: { at: string; reason: string }[]
  job_uid: string; job_name: string; status: 'running' | 'success' | 'failed'
  started_at: string; finished_at: string | null; archive_bytes: number | null
  verified_tables: number | null; failure_reason: string | null; trigger_kind: "manual" | "scheduled" | "unknown"
}
export type BackupStatus = { observed_at: string | null; latest_verified: BackupRun | null; runs: BackupRun[] }

type Inventory = {checked: string; works: {slug:string; title:string; published:number; missing?:number[]; today?:number[]; source?:string; note?:string}[]}

type Status = "running" | "pending" | "success" | "failed";
type Sync = {
  id: string;
  source: string;
  status: Status;
  started_at: string;
  finished_at: string | null;
  skip_reason: string | null;
  records_read: number;
  records_updated: number;
  records_created: number;
  http_status: number | null;
  records_failed: number;
  error_message: string | null;
};
type Quality = {
  id: string;
  started_at: string;
  finished_at: string | null;
  status: Status;
  report: {
    works: number;
    mean_score: number;
    issues: Record<string, number>;
  } | null;
};

async function getJSON<T>(path: string, adminToken?: string): Promise<T | null> {
  try {
    const base = process.env.KNOWLEDGE_API_URL ?? "http://127.0.0.1:4321";
    const res = await fetch(
      new URL("/api/v1/" + (adminToken ? "admin/" : "") + "knowledge/" + path + "?limit=30", base),
      { signal: AbortSignal.timeout(5000), cache: "no-store", headers: adminToken ? {Authorization: "Bearer " + adminToken} : {} },
    );
    if (res.ok) return (await res.json()) as T;
  } catch {
    /* Show an unavailable feed separately from an empty one. */
  }
  return null;
}
function get<T>(path: string, adminToken?: string) {
  return getJSON<T[]>(path, adminToken)
}
export const readFeed = createServerFn({ method: "GET" }).handler(async () => {
  if (!localWorkbenchRequest(getRequest())) throw new Error('工作台限内部访问')
  const session = getCookie('folio_workbench')
  if (!session) return null
  const check = await fetch(new URL('/api/v1/editor/account',process.env.KNOWLEDGE_API_URL ?? 'http://127.0.0.1:4321'),{headers:{Authorization:'Bearer '+session},cache:'no-store',redirect:'error',signal:AbortSignal.timeout(5000)})
  if(!check.ok || !(await check.json()).editor)return null
  const admin = process.env.KNOWLEDGE_ADMIN_TOKEN
  const [sync, quality, worlds, research, media, scopes, usage, inventory, queue, releases, backup] = await Promise.all([
    get<Sync>("sync-runs"),
    get<Quality>("quality-runs"),
    get<WorkKnowledgeStatus>("world-statuses"),
    get<ResearchRun>("research-runs"),
    get<MediaCandidate>("media-candidates"),
    get<ScopeAcceptance>("scope-acceptances"),
    admin ? get<ProductionUsage>("production-usage", admin) : Promise.resolve(null),
    admin ? get<Inventory>("episode-inventory", admin) : Promise.resolve(null),
    admin ? get<QueueRow>("episode-queue", admin) : Promise.resolve(null),
    admin ? get<ReaderRelease>("reader-releases", admin) : Promise.resolve(null),
    admin ? getJSON<BackupStatus>("backup-status", admin) : Promise.resolve(null),
  ]);
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
      error: r.error_message,
    })),
    ...(quality ?? []).map((r) => ({
      id: r.id,
      task: "质量扫描",
      start: r.started_at,
      finish: r.finished_at,
      status: r.status,
      skip: null,
      read: r.report?.works ?? 0,
      updated: 0,
      http: null,
      failed: null,
      score: r.report?.mean_score ?? null,
      error: null as string | null,
      issues: r.report
        ? Object.values(r.report.issues).reduce((a, b) => a + b, 0)
        : null,
    })),
  ].sort((a, b) => Date.parse(b.start) - Date.parse(a.start));
  return {
    runs,
    releases,
    backup,
    queue,
    inventory: inventory?.[0],
    worlds,
    research,
    scopes,
    usage,
    media,
    syncOnline: sync !== null,
    qualityOnline: quality !== null,
    checked: new Date().toISOString(),
  };
});

export type WorkbenchFeed = NonNullable<Awaited<ReturnType<typeof readFeed>>>
export type WorkbenchRun = WorkbenchFeed['runs'][number]
