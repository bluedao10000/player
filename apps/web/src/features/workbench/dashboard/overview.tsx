import { Activity, AlertTriangle, BookOpen, CheckCircle2, Clock3, Images, Layers3, Users } from 'lucide-react'
import type { OverviewAttention, OverviewMetric, OpsOverview } from '../feed'
import { formatDateTime } from '../shared/format'
import { Panel, SectionHeading } from '../shared/ui'

const definitions: Record<string, { title: string; detail: string; tone: string; icon: typeof Activity }> = {
  active_works: { title: '作品总数', detail: 'works.active = true', tone: 'cyan', icon: BookOpen },
  published_worlds: { title: '已发布作品世界数', detail: 'published_worlds 视图行数', tone: 'mint', icon: Layers3 },
  characters: { title: '人物', detail: '当前作品世界的人物条目', tone: 'violet', icon: Users },
  portraits: { title: '立绘', detail: '当前作品世界的立绘引用', tone: 'violet', icon: Images },
  chapters: { title: '章节', detail: '当前作品世界的章节条目', tone: 'cyan', icon: BookOpen },
  chapter_images: { title: '章节配图', detail: '当前作品世界的章节配图引用', tone: 'cyan', icon: Images },
  storylines: { title: '故事线', detail: '当前作品世界的故事线条目', tone: 'mint', icon: Layers3 },
  scenes: { title: '场景', detail: '当前作品世界的场景条目', tone: 'mint', icon: Layers3 },
  locations: { title: '地点', detail: '当前作品世界的地点条目', tone: 'mint', icon: Layers3 },
  online_workers: { title: '在线工作进程', detail: '最近成功心跳在在线阈值内', tone: 'cyan', icon: Activity },
  running_executions: { title: '运行中执行实例', detail: 'Agent 心跳上报的非终态执行数', tone: 'violet', icon: Activity },
  running_tasks: { title: '当前执行任务', detail: 'ops_task_runs.status = running', tone: 'cyan', icon: Activity },
  pending_work: { title: '等待队列', detail: 'ops_work_items.state = pending', tone: 'mint', icon: Clock3 },
  blocked_work: { title: '当前阻塞', detail: 'ops_work_items.state = blocked', tone: 'red', icon: AlertTriangle },
  waiting_human: { title: '等待人工', detail: 'ops_work_items.state = waiting_human', tone: 'violet', icon: Users },
  completed_today: { title: '今日自动完成', detail: '北京时间当天 finished_at 的成功执行', tone: 'mint', icon: CheckCircle2 },
  failed_today: { title: '今日失败', detail: '北京时间当天 finished_at 的失败执行', tone: 'red', icon: AlertTriangle },
}
const metricOrder = [
  'active_works', 'published_worlds', 'characters', 'portraits', 'chapters', 'chapter_images', 'storylines', 'scenes', 'locations',
  'online_workers', 'running_executions', 'running_tasks', 'pending_work', 'blocked_work', 'waiting_human', 'completed_today', 'failed_today',
]
const workTypeLabels: Record<string, string> = { TV: '电视动画', Movie: '电影', OVA: 'OVA', ONA: '网络动画', Special: '特别篇' }

export function Overview({ data }: { data: OpsOverview | null }) {
  if (!data) return <section className="wb-panel" role="status"><h2>运营总览暂不可用</h2><p>无法读取运营数据，请检查管理员接口和工作台会话。</p></section>
  return (
    <div className="wb-overview" aria-label="运营总览">
      <section className="wb-metrics" aria-label="运营指标">
        {metricOrder.map((name) => {
          const metric = data.metrics[name]
          const definition = definitions[name]
          if (!metric || !definition) return null
          const Icon = definition.icon
          return (
            <Panel className={`wb-metric wb-tone-${definition.tone}`} key={name}>
              <div className="wb-metric-top"><span className="wb-eyebrow">{definition.title}</span><Icon size={18} aria-hidden /></div>
              <strong>{formatMetricValue(metric.value)}</strong>
              <p>{definition.detail}</p>
              <small>口径 v{metric.definition ?? '—'} · 更新于 {metric.updated_at ? formatDateTime(metric.updated_at) : '—'}</small>
            </Panel>
          )
        })}
      </section>
      <section className="wb-panel wb-overview-types" aria-labelledby="overview-types-title">
        <SectionHeading id="overview-types-title" eyebrow="WORK TYPES" title="作品类型分布" detail="按 works.type 原值分组；未登记单独列出，不与知识资产分项相加。" />
        <WorkTypeDistribution metric={data.metrics.work_types} />
      </section>
      <AttentionList items={data.attention} title="需要关注" />
    </div>
  )
}

export function AttentionList({ items, title = '需要关注' }: { items: OverviewAttention[]; title?: string }) {
  return (
    <section className="wb-panel wb-attention-list" aria-labelledby="attention-title">
      <SectionHeading id="attention-title" eyebrow="ATTENTION" title={title} detail={`${items.length} 项，按严重程度排序`} />
      {!items.length && <p>当前没有需要关注的事项。</p>}
      <ol>
        {items.map((item) => (
          <li key={item.id} data-severity={item.severity}>
            <div className="wb-attention-primary">
              <strong>{item.work || item.subject || item.id}</strong>
              <span>{item.stage} · {item.reason_code}</span>
            </div>
            <div className="wb-attention-meta">
              <span className="wb-attention-severity">{severityLabels[item.severity]}</span>
              <time dateTime={item.since ?? undefined}>{item.since ? formatDateTime(item.since) : '时间未知'}</time>
              <a href={attentionHref(item)}>查看详情</a>
            </div>
          </li>
        ))}
      </ol>
    </section>
  )
}

function WorkTypeDistribution({ metric }: { metric?: OverviewMetric }) {
  const rows = Array.isArray(metric?.value) ? metric.value as { type: string | null; count: number }[] : []
  if (!metric || !Array.isArray(metric.value)) return <p>类型分布暂不可用。</p>
  if (!rows.length) return <p>暂无已登记作品。</p>
  return <ul className="wb-type-distribution">
    {rows.map((row) => <li key={row.type ?? 'unregistered'}><span>{row.type === null ? '未登记' : workTypeLabels[row.type] ? `${workTypeLabels[row.type]}（${row.type}）` : row.type}</span><strong>{row.count.toLocaleString('zh-CN')}</strong></li>)}
    <li className="wb-metric-meta">口径 v{metric.definition ?? '—'} · 更新于 {metric.updated_at ? formatDateTime(metric.updated_at) : '—'}</li>
  </ul>
}

function formatMetricValue(value: unknown) {
  return typeof value === 'number' ? value.toLocaleString('zh-CN') : value == null ? '—' : String(value)
}

const severityLabels: Record<OverviewAttention['severity'], string> = { critical: '严重', high: '高', medium: '中', info: '待处理' }
function attentionHref(item: OverviewAttention) {
  const page = item.link.page === 'overview' ? '' : '/' + item.link.page
  const params = new URLSearchParams()
  Object.entries(item.link.params).forEach(([key, value]) => {
    if (value !== null) params.set(key, String(value))
  })
  const query = params.toString()
  return `/workbench${page}${query ? '?' + query : ''}`
}
