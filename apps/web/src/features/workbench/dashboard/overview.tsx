import { Activity, CheckCircle2, Images, Server, TriangleAlert } from 'lucide-react'
import type { WorkbenchFeed } from '../feed'
import { formatDateTime, isShanghaiToday } from '../shared/format'
import { Panel, StatusBadge } from '../shared/ui'

export function Overview({ feed }: { feed: WorkbenchFeed }) {
  const online = feed.syncOnline && feed.qualityOnline
  const partial = feed.syncOnline !== feed.qualityOnline
  const today = feed.runs.filter((run) => isShanghaiToday(run.start, feed.checked))
  const metrics = [
    {
      label: 'ACTIVE RUNS',
      title: '当前执行任务',
      value: feed.runs.filter((run) => run.status === 'running').length,
      detail: '同步与质量扫描记录窗口',
      icon: Activity,
      tone: 'cyan',
    },
    {
      label: 'SUCCESS TODAY',
      title: '今日成功任务',
      value: today.filter((run) => run.status === 'success').length,
      detail: '按任务开始时间 · 北京时间',
      icon: CheckCircle2,
      tone: 'mint',
    },
    {
      label: 'FAILED TODAY',
      title: '今日失败任务',
      value: today.filter((run) => run.status === 'failed').length,
      detail: '保留失败回执与错误详情',
      icon: TriangleAlert,
      tone: 'red',
    },
    {
      label: 'MEDIA CANDIDATES',
      title: '图片候选',
      value: feed.media?.length ?? '—',
      detail: feed.media
        ? `${feed.media.filter((item) => item.stage === 'passed').length} 个通过 · ${feed.media.filter((item) => item.stage === 'held' || item.stage === 'failed').length} 个待处理`
        : '媒体记录接口暂不可用',
      icon: Images,
      tone: 'violet',
    },
  ]

  return (
    <section className="wb-overview" aria-label="运行概览">
      <div className="wb-service-line">
        <div>
          <span className="wb-eyebrow">DATA SERVICE</span>
          <strong>数据服务状态</strong>
        </div>
        <StatusBadge
          status={online ? 'success' : partial ? 'held' : 'failed'}
          label={online ? '记录接口正常' : partial ? '部分记录不可用' : '记录接口不可用'}
        />
        <span>最近更新 · {formatDateTime(feed.checked)}</span>
      </div>
      <div className="wb-metrics">
        {metrics.map((metric) => {
          const Icon = metric.icon
          return (
            <Panel className={`wb-metric wb-tone-${metric.tone}`} key={metric.label}>
              <div className="wb-metric-top">
                <span className="wb-eyebrow">{metric.label}</span>
                <Icon size={18} />
              </div>
              <strong>{typeof metric.value === 'number' ? metric.value.toLocaleString('zh-CN') : metric.value}</strong>
              <h2>{metric.title}</h2>
              <p>{metric.detail}</p>
            </Panel>
          )
        })}
        <Panel className={`wb-metric wb-tone-blue wb-service-metric`}>
          <div className="wb-metric-top">
            <span className="wb-eyebrow">OBSERVABILITY</span>
            <Server size={18} />
          </div>
          <strong>{feed.runs.length}</strong>
          <h2>最近运行回执</h2>
          <p>同步与质检各读取最近 30 条</p>
        </Panel>
      </div>
    </section>
  )
}
