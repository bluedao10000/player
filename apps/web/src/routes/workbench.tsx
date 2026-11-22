import { requireLocalWorkbench } from '../features/workbench/local'
import astryxCss from '@astryxdesign/core/astryx.css?url'
import astryxReset from '@astryxdesign/core/reset.css?url'
import neutralFonts from '@astryxdesign/theme-neutral/fonts.css?url'
import neutralCss from '@astryxdesign/theme-neutral/theme.css?url'
import { createFileRoute } from '@tanstack/react-router'
import { siteTitle } from '../site'
import { Board } from '../features/workbench/dashboard/board'
import { EditorDesk } from '../features/workbench/editor-desk'
import { readFeed } from '../features/workbench/feed'
import editorCss from '../features/workbench/editor.css?url'
import css from '../features/workbench/board.css?url'

export const Route = createFileRoute('/workbench')({
  beforeLoad: () => requireLocalWorkbench(),
  validateSearch: (search: Record<string, unknown>) => ({
    section:
      search.section === 'media' ? 'media' as const
      : search.section === 'knowledge' ? 'knowledge' as const
      : search.section === 'history' ? 'history' as const
      : 'data' as const,
    view: search.view === 'data' ? 'data' as const : search.view === 'me' ? 'me' as const : search.view === 'chat' ? 'chat' as const : 'editor' as const,
    world: typeof search.world === 'string' ? search.world : undefined,
    task: typeof search.task === 'string' ? search.task : undefined,
    plan: typeof search.plan === 'string' ? search.plan : undefined,
    candidate: Number.isSafeInteger(Number(search.candidate)) && Number(search.candidate)>0 ? Number(search.candidate) : undefined,
  }),
  loaderDeps: ({ search }) => ({ view: search.view }),
  loader: ({ deps }) => (deps.view === 'data' ? readFeed() : null),
  headers: () => ({ 'X-Robots-Tag': 'noindex, nofollow', 'Cache-Control': 'private, no-store' }),
  head: () => ({
    meta: [
      { title: siteTitle('编辑与数据工作台') },
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
  component: Workbench,
})

function Workbench() {
  const feed = Route.useLoaderData()
  const { section, view, candidate, world, plan, task } = Route.useSearch()
  return feed ? <Board feed={feed} section={section} candidate={candidate} /> : <EditorDesk accountView={view === 'me'} chatView={view === 'chat'} context={{world, plan, task}} />
}
