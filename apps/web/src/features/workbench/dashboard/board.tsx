import { Button } from '@astryxdesign/core/Button'
import { Theme } from '@astryxdesign/core/theme'
import { neutralTheme } from '@astryxdesign/theme-neutral/built'
import { useRouter } from '@tanstack/react-router'
import {
  Activity,
  Archive,
  BookOpenCheck,
  CirclePause,
  CirclePlay,
  DatabaseBackup,
  Images,
  RefreshCw,
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { siteName } from '../../../site'
import { BackupReport } from '../backup-report'
import { DeskNav } from '../editor-desk'
import type { WorkbenchFeed } from '../feed'
import { MediaGallery } from '../media-gallery'
import { ProductionReport } from '../production-report'
import { ResearchHistory } from '../research-history'
import { ScopeReport } from '../scope-report'
import { formatClock } from '../shared/format'
import { LiveTerminal } from '../terminal/live-terminal'
import { WorldInventory } from '../world-inventory'
import { WorkflowCanvas } from '../workflow/workflow-canvas'
import { ReleaseOperations } from './operations'
import { Overview } from './overview'

const views = [
  { id: 'overview', label: '运行总览', icon: Activity },
  { id: 'media', label: '图片资产', icon: Images },
  { id: 'research', label: '研究与制作', icon: BookOpenCheck },
  { id: 'operations', label: '备份与发布', icon: DatabaseBackup },
  { id: 'inventory', label: '知识库存', icon: Archive },
] as const

type View = (typeof views)[number]['id']

export function Board({ feed }: { feed: WorkbenchFeed }) {
  return (
    <Theme theme={neutralTheme} mode="dark">
      <BoardContent feed={feed} />
    </Theme>
  )
}

function BoardContent({ feed }: { feed: WorkbenchFeed }) {
  const router = useRouter()
  const [view, setView] = useState<View>('overview')
  const [auto, setAuto] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const pending = useRef(false)

  const refresh = useCallback(async () => {
    if (pending.current) return
    pending.current = true
    setRefreshing(true)
    try {
      await router.invalidate()
    } finally {
      pending.current = false
      setRefreshing(false)
    }
  }, [router])

  useEffect(() => {
    if (!auto) return
    const id = setInterval(() => {
      if (document.visibilityState === 'visible') void refresh()
    }, 15000)
    return () => clearInterval(id)
  }, [auto, refresh])

  return (
    <main className="wb">
      <header className="wb-topbar">
        <div className="wb-brand-lockup">
          <a className="wb-brand" href="/">{siteName}</a>
          <span>/</span>
          <strong>数据运行</strong>
          <small>DATA OPERATIONS</small>
        </div>
        <div className="wb-topbar-actions">
          <span className="wb-last-update">最近更新 · {formatClock(feed.checked)} UTC</span>
          <Button
            className="wb-button"
            variant="ghost"
            size="sm"
            label={auto ? '暂停自动刷新' : '开启自动刷新'}
            icon={auto ? <CirclePause size={15} /> : <CirclePlay size={15} />}
            aria-pressed={auto}
            onClick={() => setAuto((value) => !value)}
          />
          <Button
            className="wb-button"
            variant="secondary"
            size="sm"
            label="立即刷新"
            icon={<RefreshCw size={15} />}
            isLoading={refreshing}
            onClick={() => void refresh()}
          />
        </div>
      </header>
      <div className="wb-shell">
        <div className="wb-heading">
          <div>
            <DeskNav data />
            <span className="wb-eyebrow">CONTROL CENTER / 01</span>
            <h1>数据运行控制台</h1>
            <p>统一管理知识同步、图片生产、质量审核与数据备份</p>
          </div>
          <div className="wb-heading-status">
            <span><i className="wb-status-dot" data-status={feed.syncOnline || feed.qualityOnline ? 'success' : 'failed'} />{feed.syncOnline || feed.qualityOnline ? '运行记录已接入' : '记录接口不可用'}</span>
            <code>15 秒轮询 · {auto ? '开启' : '暂停'}</code>
          </div>
        </div>
        <nav className="wb-view-nav" aria-label="数据运行视图">
          {views.map((item) => {
            const Icon = item.icon
            return <button key={item.id} aria-pressed={view === item.id} onClick={() => setView(item.id)}><Icon size={16} />{item.label}</button>
          })}
        </nav>
        <div className="wb-view" key={view}>
          {view === 'overview' && (
            <>
              <Overview feed={feed} />
              <WorkflowCanvas feed={feed} />
              <LiveTerminal runs={feed.runs} syncOnline={feed.syncOnline} qualityOnline={feed.qualityOnline} auto={auto} onAutoChange={setAuto} />
            </>
          )}
          {view === 'media' && <MediaGallery items={feed.media} />}
          {view === 'research' && <div className="wb-report-stack"><ResearchHistory runs={feed.research} /><ProductionReport works={feed.usage} /></div>}
          {view === 'operations' && <div className="wb-report-stack"><BackupReport data={feed.backup} checked={feed.checked} /><ReleaseOperations feed={feed} /></div>}
          {view === 'inventory' && <div className="wb-report-stack"><ScopeReport scopes={feed.scopes} /><WorldInventory worlds={feed.worlds} scopes={feed.scopes} /></div>}
        </div>
      </div>
    </main>
  )
}
