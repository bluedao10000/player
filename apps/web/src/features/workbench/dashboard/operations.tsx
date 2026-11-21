import { ArchiveRestore, ListPlus, Rocket } from 'lucide-react'
import type { WorkbenchFeed } from '../feed'
import { formatDateTime, formatDuration } from '../shared/format'
import { EmptyState, Panel, SectionHeading, StatusBadge } from '../shared/ui'

export function ReleaseOperations({ feed }: { feed: WorkbenchFeed }) {
  const title = (slug: string) => feed.inventory?.works.find((work) => work.slug === slug)?.title ?? slug
  return (
    <div className="wb-report-grid">
      <Panel className="wb-report-panel" labelledby="release-title">
        <SectionHeading id="release-title" eyebrow="PRODUCTION RELEASE" title="正式站发布记录" detail="准入、执行、计量与线上验证回执" />
        {feed.releases === null && <EmptyState icon={Rocket} title="发布记录暂不可用" />}
        {feed.releases?.length === 0 && <EmptyState icon={Rocket} title="暂无发布记录" />}
        {feed.releases?.map((row) => (
          <details className="wb-report-row" key={row.topic + (row.started_at ?? '')}>
            <summary><span>{title(row.world_slug)} · 第 {row.number} 集</span><StatusBadge status={row.error ? 'failed' : row.status === '线上已验证' ? 'success' : 'running'} label={row.status} /></summary>
            <div className="wb-report-detail">
              <p>{row.title} · 本集已计量 {row.measured_tokens.toLocaleString('zh-CN')} token</p>
              <p>准入 {formatDateTime(row.admitted_at)}</p>
              {row.started_at && <p>发布开始 {formatDateTime(row.started_at)}</p>}
              {row.started_at && <p>本次执行 {formatDuration(row.started_at, row.finished_at)}{row.finished_at && ` · 自准入起 ${((Date.parse(row.finished_at) - Date.parse(row.admitted_at)) / 60000).toFixed(1)} 分钟`}</p>}
              {row.error && <p className="wb-error-text">{row.error}</p>}
              {row.status === '线上已验证' && row.href && <a href={new URL(row.href, 'https://www.codeiy.com').href}>打开正式章节</a>}
            </div>
          </details>
        ))}
      </Panel>
      <Panel className="wb-report-panel" labelledby="queue-title">
        <SectionHeading id="queue-title" eyebrow="EPISODE QUEUE" title="自动补集" detail="采集队列、预算与失败原因" />
        {feed.queue === null && <EmptyState icon={ListPlus} title="补集队列暂不可用" />}
        {feed.queue?.length === 0 && <EmptyState icon={ListPlus} title="当前没有补集任务" />}
        {feed.queue?.map((row) => (
          <details className="wb-report-row" key={row.world_slug + row.number}>
            <summary><span>{title(row.world_slug)} · 第 {row.number} 集</span><StatusBadge status={row.capture_error ? 'failed' : 'pending'} label={row.status} /></summary>
            <div className="wb-report-detail">
              <p>优先级 {row.priority} · 作品已占用 {row.committed.toLocaleString('zh-CN')} / {row.token_budget.toLocaleString('zh-CN')} token</p>
              <p>本集已计量 {(row.measured_tokens ?? 0).toLocaleString('zh-CN')} token</p>
              {row.capture_started && <p>采集耗时 {formatDuration(row.capture_started, row.capture_finished ?? null)}</p>}
              {row.capture_error && <p className="wb-error-text">{row.capture_error}</p>}
              {row.capture_failures?.map((failure) => <p key={failure.at}>{formatDateTime(failure.at)} · {failure.reason}</p>)}
            </div>
          </details>
        ))}
      </Panel>
      <Panel className="wb-report-panel wb-report-wide" labelledby="gaps-title">
        <SectionHeading id="gaps-title" eyebrow="EPISODE INVENTORY" title="分集缺口" detail={feed.inventory ? `盘点于 ${formatDateTime(feed.inventory.checked)} · 当天排期单列` : '最新分集盘点快照'} />
        {!feed.inventory && <EmptyState icon={ArchiveRestore} title="分集盘点暂不可用" />}
        <div className="wb-gap-grid">
          {feed.inventory?.works.map((work) => (
            <details className="wb-report-row" key={work.slug}>
              <summary><span>{work.title}</span><strong>已发布 {work.published}</strong></summary>
              <div className="wb-report-detail">
                <p>{work.note ?? `应补 ${work.missing?.length ?? 0} 集`}</p>
                {!!work.missing?.length && <p>待补集号 · {work.missing.join('、')}</p>}
                {!!work.today?.length && <p>当天待确认 · {work.today.join('、')}</p>}
                {work.source && <a href={work.source}>查看版本分集表</a>}
              </div>
            </details>
          ))}
        </div>
      </Panel>
    </div>
  )
}
