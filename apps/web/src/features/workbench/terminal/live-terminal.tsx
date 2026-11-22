import { Button } from '@astryxdesign/core/Button'
import { CirclePause, CirclePlay, TerminalSquare } from 'lucide-react'
import { useState } from 'react'
import type { WorkbenchRun } from '../feed'
import { formatClock, formatDate, formatDuration, runStatusLabels } from '../shared/format'
import { EmptyState, Panel, SectionHeading, StatusBadge } from '../shared/ui'

export function LiveTerminal({
  runs,
  syncOnline,
  qualityOnline,
  auto,
  onAutoChange,
}: {
  runs: WorkbenchRun[]
  syncOnline: boolean
  qualityOnline: boolean
  auto: boolean
  onAutoChange: (value: boolean) => void
}) {
  const [status, setStatus] = useState('all')
  const [task, setTask] = useState('all')
  const [selected, setSelected] = useState<string | null>(null)
  const tasks = [...new Set(runs.map((run) => run.task))]
  const list = runs.filter((run) => (status === 'all' || run.status === status) && (task === 'all' || run.task === task))
  const active = list.find((run) => run.id === selected) ?? list[0]

  return (
    <Panel className="wb-terminal" labelledby="terminal-title">
      <SectionHeading
        id="terminal-title"
        eyebrow="LIVE TERMINAL"
        title="实时运行日志"
        detail="结构化运行回执 · 非进程 stdout / stderr"
        actions={
          <div className="wb-terminal-controls">
            <select aria-label="筛选任务" value={task} onChange={(event) => setTask(event.target.value)}>
              <option value="all">全部任务</option>
              {tasks.map((name) => <option key={name} value={name}>{name}</option>)}
            </select>
            <select aria-label="筛选运行状态" value={status} onChange={(event) => setStatus(event.target.value)}>
              <option value="all">全部状态</option>
              {Object.entries(runStatusLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
            <Button
              className="wb-button"
              variant="ghost"
              size="sm"
              label={auto ? '暂停 15 秒刷新' : '开启 15 秒刷新'}
              icon={auto ? <CirclePause size={15} /> : <CirclePlay size={15} />}
              onClick={() => onAutoChange(!auto)}
              aria-pressed={auto}
            />
          </div>
        }
      />
      <div className="wb-terminal-status">
        <span><i className="wb-status-dot" data-status={auto ? 'running' : 'pending'} />{auto ? 'LIVE · 15s' : 'PAUSED'}</span>
        <span>{list.length} 条记录</span>
        {(!syncOnline || !qualityOnline) && (
          <strong>{!syncOnline ? '同步记录不可用 ' : ''}{!qualityOnline ? '质检记录不可用' : ''}</strong>
        )}
      </div>
      <div className="wb-terminal-table" role="table" aria-label="任务运行记录">
        <div className="wb-terminal-head" role="row">
          <span role="columnheader">时间</span>
          <span role="columnheader">任务</span>
          <span role="columnheader">状态</span>
          <span role="columnheader">HTTP</span>
          <span role="columnheader">读取</span>
          <span role="columnheader">更新 / 命中</span>
          <span role="columnheader">质量分</span>
        </div>
        <div className="wb-terminal-body">
          {list.map((run) => (
            <button
              className="wb-terminal-row"
              role="row"
              key={run.id}
              aria-pressed={active?.id === run.id}
              onClick={() => setSelected(run.id)}
            >
              <time role="cell" dateTime={run.start}>{formatDate(run.start).slice(5)} {formatClock(run.start)}</time>
              <strong role="cell">{run.task}<small>{run.id.slice(0, 8)}</small></strong>
              <span role="cell"><StatusBadge status={run.status} label={run.skip === 'version_unchanged' ? '版本未变' : undefined} /></span>
              <code role="cell">{run.http ?? '—'}</code>
              <code role="cell">{run.read.toLocaleString('zh-CN')}</code>
              <code role="cell">{(run.issues ?? run.updated).toLocaleString('zh-CN')}</code>
              <code role="cell">{run.score ?? '—'}</code>
              {(run.error || run.failed) && <span className="wb-terminal-error">{run.error ?? `${run.failed} 条记录失败`}</span>}
            </button>
          ))}
        </div>
      </div>
      {!list.length && <EmptyState icon={TerminalSquare} title="当前筛选下没有记录" detail="调整任务或状态筛选，或等待下一次运行回执。" />}
      {active && (
        <footer className="wb-terminal-detail">
          <div><span className="wb-eyebrow">SELECTED RUN</span><code>{active.id}</code></div>
          <span>开始 {formatDate(active.start)} {formatClock(active.start)}</span>
          <span>耗时 {formatDuration(active.start, active.finish)}</span>
          <span>失败条目 {active.failed ?? '—'}</span>
          {active.skip && <span>跳过原因 {active.skip}</span>}
          {active.error && <p>{active.error}</p>}
        </footer>
      )}
    </Panel>
  )
}
