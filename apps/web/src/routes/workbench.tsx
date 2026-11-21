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
  validateSearch: (search: Record<string, unknown>) => ({
    view: search.view === 'data' ? 'data' : 'editor',
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
  return feed ? <Board feed={feed} /> : <EditorDesk />
}
