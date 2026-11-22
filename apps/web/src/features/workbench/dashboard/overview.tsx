import { Activity, CheckCircle2, Images, TriangleAlert } from 'lucide-react'
import type { WorkbenchFeed } from '../feed'
import { isShanghaiToday } from '../shared/format'
import { Panel } from '../shared/ui'

export function Overview({ feed }: { feed: WorkbenchFeed }) {
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
      </div>
    </section>
  )
}
