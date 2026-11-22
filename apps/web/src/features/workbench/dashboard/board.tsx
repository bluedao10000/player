import { Button } from '@astryxdesign/core/Button'
import { Theme } from '@astryxdesign/core/theme'
import { neutralTheme } from '@astryxdesign/theme-neutral/built'
import { useRouter } from '@tanstack/react-router'
import {
  CirclePause,
  CirclePlay,
  RefreshCw,
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { BackupReport } from '../backup-report'
import { WorkbenchSidebar, type WorkbenchSection } from '../shared/shell'
import type { WorkbenchFeed } from '../feed'
import { MediaGallery, RecentMedia } from '../media-gallery'
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
  { id: 'overview', label: '工作流总览' },
  { id: 'sync', label: '知识数据同步' },
  { id: 'production', label: '图片生产流水线' },
  { id: 'backup', label: '备份与恢复' },
] as const

type View = (typeof views)[number]['id']
type DataSection = Exclude<WorkbenchSection, 'editor' | 'me' | 'chat'> | 'media'

export function Board({ feed, section, candidate }: { feed: WorkbenchFeed; section: DataSection; candidate?: number }) {
  return (
    <Theme theme={neutralTheme} mode="dark">
      <BoardContent feed={feed} section={section} candidate={candidate} />
    </Theme>
  )
}

function BoardContent({ feed, section, candidate }: { feed: WorkbenchFeed; section: DataSection; candidate?: number }) {
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
    <main className="wb wb-layout">
      <WorkbenchSidebar current={section === 'media' ? 'knowledge' : section} />
      <div className="wb-shell">
        <div className="wb-heading">
          <div>
            <span className="wb-eyebrow">CONTROL CENTER / 01</span>
            <h1>{section === 'data' ? '数据运行控制台' : section === 'history' ? '运行历史' : '知识库'}</h1>
            <p>{section === 'data' ? '统一管理知识同步、图片生产、质量审核与数据备份' : section === 'history' ? '查看同步与质检的运行回执' : '查看作品库存、分集验收与图片资产'}</p>
          </div>
          <div className="wb-heading-status">
            <span><i className="wb-status-dot" data-status={feed.syncOnline || feed.qualityOnline ? 'success' : 'failed'} />{feed.syncOnline || feed.qualityOnline ? '运行记录已接入' : '记录接口不可用'}</span>
            <code>UTC {formatClock(feed.checked)}</code>
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
        </div>
        {section === 'data' && <Overview feed={feed} />}
        {(section === 'knowledge' || section === 'media') && (
          <nav className="wb-view-nav wb-knowledge-nav" aria-label="知识库视图">
            <a href="/workbench?view=data&section=knowledge" aria-current={section === 'knowledge' ? 'page' : undefined}>作品库存</a>
            <a href="/workbench?view=data&section=media" aria-current={section === 'media' ? 'page' : undefined}>图片资产</a>
          </nav>
        )}
        {section === 'data' && (
          <nav className="wb-view-nav" aria-label="数据运行视图">
            {views.map(({id,label}) => (
              <button key={id} aria-pressed={view === id} onClick={() => setView(id)}>{label}</button>
            ))}
          </nav>
        )}
        <div className="wb-view" key={section + view}>
          {section === 'knowledge' && (
            <div className="wb-report-stack">
              <WorldInventory worlds={feed.worlds} scopes={feed.scopes} />
              <ScopeReport scopes={feed.scopes} />
            </div>
          )}
          {section === 'media' && <MediaGallery key={candidate} items={feed.media} candidate={candidate} />}
          {section === 'history' && <LiveTerminal runs={feed.runs} syncOnline={feed.syncOnline} qualityOnline={feed.qualityOnline} auto={auto} onAutoChange={setAuto} />}
          {section === 'data' && view === 'overview' && (
            <>
              <WorkflowCanvas feed={feed} />
              <RecentMedia items={feed.media} />
              <LiveTerminal runs={feed.runs} syncOnline={feed.syncOnline} qualityOnline={feed.qualityOnline} auto={auto} onAutoChange={setAuto} />
            </>
          )}
          {section === 'data' && view === 'sync' && <div className="wb-report-stack"><WorkflowCanvas feed={feed} /><WorldInventory worlds={feed.worlds} scopes={feed.scopes} /></div>}
          {section === 'data' && view === 'production' && <div className="wb-report-stack"><WorkflowCanvas feed={feed} initialFlow="media" /><ResearchHistory runs={feed.research} /><ProductionReport works={feed.usage} /><ScopeReport scopes={feed.scopes} /></div>}
          {section === 'data' && view === 'backup' && <div className="wb-report-stack"><BackupReport data={feed.backup} checked={feed.checked} /><ReleaseOperations feed={feed} /></div>}
        </div>
      </div>
    </main>
  )
}
