export type WorkbenchRoute = '/workbench' | '/workbench/operations' | '/workbench/knowledge' | `/workbench/knowledge/${string}` | '/workbench/projects' | '/workbench/analytics' | '/workbench/exceptions' | '/workbench/assistant' | '/workbench/me'
export type WorkbenchRouteSearch = Record<string, string | number>
export type LegacyWorkbenchSearch = Record<string, unknown> & { view?: unknown; section?: unknown }

const forwardedKeys = ['world', 'plan', 'task', 'candidate', 'item_key', 'run_key', 'source_table', 'source_id', 'episode', 'worker', 'loop'] as const

export function resolveLegacyWorkbench(search: LegacyWorkbenchSearch): { pathname: WorkbenchRoute; search: WorkbenchRouteSearch } | null {
  const view = typeof search.view === 'string' ? search.view : undefined
  const section = typeof search.section === 'string' ? search.section : undefined
  if (view === undefined && section === undefined) return null

  let pathname: WorkbenchRoute
  if (view === 'chat') pathname = '/workbench/assistant'
  else if (view === 'me') pathname = '/workbench/me'
  else if (view === 'editor') pathname = '/workbench/exceptions'
  else if (view === 'data') pathname = dataSection(section, search.world)
  else if (section !== undefined) pathname = dataSection(section, search.world)
  else pathname = '/workbench/exceptions'

  const forwarded: WorkbenchRouteSearch = {}
  for (const key of forwardedKeys) {
    const value = search[key]
    if (typeof value === 'string' && value !== '') forwarded[key] = value
    else if (typeof value === 'number' && Number.isFinite(value)) forwarded[key] = value
  }
  return { pathname, search: forwarded }
}

function dataSection(section: string | undefined, world: unknown): WorkbenchRoute {
  switch (section) {
    case 'knowledge':
    case 'media': return typeof world === 'string' && world !== ''
      ? `/workbench/knowledge/${encodeURIComponent(world)}`
      : '/workbench/knowledge'
    case 'history': return '/workbench/operations'
    default: return '/workbench'
  }
}

export function legacyWorkbenchHref(target: { pathname: WorkbenchRoute; search: WorkbenchRouteSearch }) {
  const query = new URLSearchParams()
  Object.entries(target.search).forEach(([key, value]) => query.set(key, String(value)))
  const suffix = query.toString()
  return target.pathname + (suffix ? '?' + suffix : '')
}
