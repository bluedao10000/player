import { Activity, BookOpen, Clock3, NotebookPen, UserRound } from 'lucide-react'
import { useId, useState } from 'react'

const sections = [
  { id: 'data', href: '/workbench?view=data', label: '数据运行', icon: Activity },
  { id: 'knowledge', href: '/workbench?view=data&section=knowledge', label: '知识库', icon: BookOpen },
  { id: 'editor', href: '/workbench?view=editor', label: '编辑审稿', icon: NotebookPen },
  { id: 'history', href: '/workbench?view=data&section=history', label: '运行历史', icon: Clock3 },
] as const

export type WorkbenchSection = (typeof sections)[number]['id'] | 'me'

export function WorkbenchSidebar({ current }: { current: WorkbenchSection }) {
  const menuId = useId()
  const [expanded, setExpanded] = useState(false)
  return (
    <aside className="wb-sidebar" data-expanded={expanded}>
      <div className="wb-sidebar-header">
        <a href="/" className="wb-brand"><img src="/brand/folio-plane.svg" alt="" width="26" height="26" /><span>Folio 时光</span></a>
        <button
          type="button"
          className="wb-sidebar-toggle"
          aria-label={expanded ? '收起工作台导航' : '展开工作台导航'}
          aria-expanded={expanded}
          aria-controls={menuId}
          onClick={() => setExpanded(!expanded)}
        >
          菜单
        </button>
      </div>
      <div className="wb-sidebar-content" id={menuId}>
        <nav className="wb-primary-nav" aria-label="工作台">
          {sections.map(({ id, href, label, icon: Icon }) => (
            <a key={id} href={href} aria-current={current === id ? 'page' : undefined}>
              <Icon size={17} aria-hidden />{label}
            </a>
          ))}
        </nav>
        <footer className="wb-sidebar-footer">
          <a href="/workbench?view=me" aria-current={current === 'me' ? 'page' : undefined}>
            <UserRound size={17} aria-hidden />我的
          </a>
        </footer>
      </div>
    </aside>
  )
}
