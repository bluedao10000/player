import { brandMark } from '../../../lib/brand'
import { Activity, Bot, BookOpen, BriefcaseBusiness, ChartNoAxesCombined, Network, ShieldAlert, UserRound } from 'lucide-react'
import { useId, useState } from 'react'

const sections = [
  { id: 'overview', href: '/workbench', label: '运营总览', subtitle: 'OVERVIEW', icon: Activity },
  { id: 'operations', href: '/workbench/operations', label: '运营拓扑', subtitle: 'OPERATIONS', icon: Network },
  { id: 'knowledge', href: '/workbench/knowledge', label: '知识资产', subtitle: 'KNOWLEDGE', icon: BookOpen },
  { id: 'projects', href: '/workbench/projects', label: '项目', subtitle: 'PROJECTS', icon: BriefcaseBusiness },
  { id: 'analytics', href: '/workbench/analytics', label: '运营分析', subtitle: 'ANALYTICS', icon: ChartNoAxesCombined },
  { id: 'exceptions', href: '/workbench/exceptions', label: '异常与决策', subtitle: 'EXCEPTIONS', icon: ShieldAlert },
  { id: 'assistant', href: '/workbench/assistant', label: 'AI 助手', subtitle: 'ASSISTANT', icon: Bot },
] as const

export type WorkbenchSection = (typeof sections)[number]['id'] | 'me'

export function WorkbenchSidebar({ current }: { current: WorkbenchSection }) {
  const menuId = useId()
  const [expanded, setExpanded] = useState(false)
  return (
    <aside className="wb-sidebar" data-expanded={expanded}>
      <div className="wb-sidebar-header">
        <a href="/" className="wb-brand"><img src={brandMark} alt="" width="26" height="26" /><span>Folio 时光</span></a>
        <button
          type="button"
          className="wb-sidebar-toggle"
          aria-label={expanded ? '收起工作台导航' : '展开工作台导航'}
          aria-expanded={expanded}
          aria-controls={menuId}
          onClick={() => setExpanded(!expanded)}
        >菜单</button>
      </div>
      <div className="wb-sidebar-content" id={menuId}>
        <nav className="wb-primary-nav" aria-label="运营中心">
          {sections.map(({ id, href, label, subtitle, icon: Icon }) => (
            <a key={id} href={href} aria-current={current === id ? 'page' : undefined}>
              <Icon size={17} aria-hidden />
              <span className="wb-nav-copy"><strong>{label}</strong><small>{subtitle}</small></span>
            </a>
          ))}
        </nav>
        <footer className="wb-sidebar-footer">
          <a href="/workbench/me" aria-current={current === 'me' ? 'page' : undefined}>
            <UserRound size={17} aria-hidden /><span className="wb-nav-copy"><strong>我的</strong></span>
          </a>
        </footer>
      </div>
    </aside>
  )
}
