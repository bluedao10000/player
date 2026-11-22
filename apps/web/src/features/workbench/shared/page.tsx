import { Button } from '@astryxdesign/core/Button'
import { useRouter } from '@tanstack/react-router'
import { CirclePause, CirclePlay, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { WorkbenchSidebar, type WorkbenchSection } from './shell'
import { formatDateTime } from './format'

export function useWorkbenchRefresh() {
  const router = useRouter()
  const [auto, setAuto] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const pending = useRef(false)
  const refresh = useCallback(async () => {
    if (pending.current) return
    pending.current = true
    setRefreshing(true)
    try { await router.invalidate() }
    finally { pending.current = false; setRefreshing(false) }
  }, [router])
  useEffect(() => {
    if (!auto) return
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') void refresh()
    }, 15000)
    return () => clearInterval(timer)
  }, [auto, refresh])
  return { auto, setAuto, refreshing, refresh }
}

export function WorkbenchPageShell({ current, title, english, description, updatedAt, auto, onAutoChange, refreshing, onRefresh, children }: {
  current: WorkbenchSection
  title: string
  english: string
  description: string
  updatedAt?: string | null
  auto: boolean
  onAutoChange: (value: boolean) => void
  refreshing: boolean
  onRefresh: () => void
  children: ReactNode
}) {
  return (
    <main className="wb wb-layout">
      <WorkbenchSidebar current={current} />
      <div className="wb-shell">
        <header className="wb-heading">
          <div>
            <span className="wb-eyebrow">{english}</span>
            <h1>{title}</h1>
            <p>{description}</p>
          </div>
          <div className="wb-heading-status">
            <span>更新于 {updatedAt ? formatDateTime(updatedAt) : '—'}</span>
            <Button className="wb-button" variant="ghost" size="sm" label={auto ? '暂停自动刷新' : '开启自动刷新'}
              icon={auto ? <CirclePause size={15} /> : <CirclePlay size={15} />} aria-pressed={auto} onClick={() => onAutoChange(!auto)} />
            <Button className="wb-button" variant="secondary" size="sm" label="立即刷新" icon={<RefreshCw size={15} />}
              isLoading={refreshing} onClick={onRefresh} />
          </div>
        </header>
        {children}
      </div>
    </main>
  )
}
