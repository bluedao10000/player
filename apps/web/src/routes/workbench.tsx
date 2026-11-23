import astryxCss from '@astryxdesign/core/astryx.css?url'
import astryxReset from '@astryxdesign/core/reset.css?url'
import neutralFonts from '@astryxdesign/theme-neutral/fonts.css?url'
import neutralCss from '@astryxdesign/theme-neutral/theme.css?url'
import { Theme } from '@astryxdesign/core/theme'
import { neutralTheme } from '@astryxdesign/theme-neutral/built'
import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { EditorDesk } from '../features/workbench/editor-desk'
import editorCss from '../features/workbench/editor.css?url'
import css from '../features/workbench/board.css?url'
import { legacyWorkbenchHref, resolveLegacyWorkbench } from '../features/workbench/legacy-links'
import { requireLocalWorkbench } from '../features/workbench/local'
import { readWorkbenchSession } from '../features/workbench/feed'
import { siteTitle } from '../site'
import type { OpsTopologyWindow } from '../features/workbench/feed'

export const Route = createFileRoute('/workbench')({
  validateSearch: (search: Record<string, unknown>) => ({
    view: typeof search.view === 'string' ? search.view : undefined,
    section: typeof search.section === 'string' ? search.section : undefined,
    world: typeof search.world === 'string' ? search.world : undefined,
    task: typeof search.task === 'string' ? search.task : undefined,
    plan: typeof search.plan === 'string' ? search.plan : undefined,
    candidate: positiveInteger(search.candidate),
    item_key: typeof search.item_key === 'string' ? search.item_key : undefined,
    run_key: typeof search.run_key === 'string' ? search.run_key : undefined,
    source_table: typeof search.source_table === 'string' ? search.source_table : undefined,
    source_id: typeof search.source_id === 'string' ? search.source_id : undefined,
    episode: positiveInteger(search.episode),
    worker: typeof search.worker === 'string' ? search.worker : undefined,
    loop: typeof search.loop === 'string' ? search.loop : undefined,
    stage: typeof search.stage === 'string' ? search.stage : undefined,
    window: topologyWindow(search.window),
  }),
  beforeLoad: async ({ search }) => {
    await requireLocalWorkbench()
    const legacy = resolveLegacyWorkbench(search)
    if (legacy) throw redirect({ href: legacyWorkbenchHref(legacy), replace: true })
    return { hasWorkbenchSession: await readWorkbenchSession() }
  },
  headers: () => ({ 'X-Robots-Tag': 'noindex, nofollow', 'Cache-Control': 'private, no-store' }),
  head: () => ({
    meta: [
      { title: siteTitle('运营中心') },
      { name: 'robots', content: 'noindex,nofollow' },
    ],
    links: [
      { rel: 'stylesheet', href: neutralFonts },
      { rel: 'stylesheet', href: astryxReset },
      { rel: 'stylesheet', href: astryxCss },
      { rel: 'stylesheet', href: neutralCss },
      { rel: 'stylesheet', href: css },
      { rel: 'stylesheet', href: editorCss },
    ],
  }),
  component: WorkbenchLayout,
})

function positiveInteger(value: unknown) {
  const number = Number(value)
  return Number.isSafeInteger(number) && number > 0 ? number : undefined
}

function topologyWindow(value: unknown): OpsTopologyWindow {
  return value === '7d' || value === '30d' ? value : '24h'
}

function WorkbenchLayout() {
  const { hasWorkbenchSession } = Route.useRouteContext()
  return <Theme theme={neutralTheme} mode="dark">{hasWorkbenchSession ? <Outlet /> : <EditorDesk />}</Theme>
}
