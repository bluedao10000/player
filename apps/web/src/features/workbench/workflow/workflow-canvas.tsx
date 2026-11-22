import type { MediaCandidate, MediaStage } from '@anime/shared/knowledge'
import {
  ArrowDown,
  Bot,
  CalendarClock,
  Crop,
  Database,
  FileSearch,
  GitBranch,
  Image,
  ListChecks,
  RefreshCcw,
  ScanSearch,
  Shapes,
  TimerReset,
  type LucideIcon,
} from 'lucide-react'
import { useState } from 'react'
import type { WorkbenchFeed, WorkbenchRun } from '../feed'
import { formatClock, formatDateTime, runStatusLabels } from '../shared/format'
import { EmptyState, Panel, SectionHeading, StatusBadge } from '../shared/ui'

const mediaStageLabels: Record<MediaStage, string> = {
  reference: '参考图准备',
  queued: '等待生成',
  generate: '原图生成',
  review: '自动验图',
  recrop: '重新裁切',
  recheck: '裁切复验',
  passed: '验图通过',
  held: '等待返工',
  failed: '任务失败',
}

type Flow = 'sync' | 'media'
type NodeStatus = WorkbenchRun['status']
type NodeDefinition = {
  id: string
  kicker: string
  title: string
  subtitle: string
  description: string
  tone: string
  icon: LucideIcon
  task?: string
}

const syncNodes: NodeDefinition[] = [
  { id: 'scheduler', kicker: '01 · SCHEDULER', title: '数据服务调度', subtitle: '定时触发与到期任务', description: '数据服务按调度策略启动来源同步和全库质量扫描；此处状态来自最近运行回执。', tone: 'orange', icon: CalendarClock },
  { id: 'due', kicker: '02 · DUE CHECK', title: '到期检查', subtitle: '来源版本与质检周期', description: '检查各来源同步间隔与质检到期时间；版本未变化时保留跳过原因。', tone: 'cyan', icon: TimerReset },
  { id: 'aod', kicker: '03A · SOURCE', title: 'AOD 数据同步', subtitle: 'Anime Offline Database', description: '流式读取作品条目，验证数据结构并在事务内归并作品知识。', tone: 'blue', icon: GitBranch, task: 'anime-offline-database' },
  { id: 'bangumi', kicker: '03B · SOURCE', title: 'Bangumi 数据同步', subtitle: '作品、人物与播出映射', description: '读取 Bangumi 数据包及播出映射，保留来源版本与同步审计。', tone: 'violet', icon: GitBranch, task: 'bangumi-data' },
  { id: 'quality', kicker: '03C · QUALITY', title: '质量扫描', subtitle: '规则检查与质量评分', description: '执行知识质量规则，保存逐规则命中数、作品数和平均质量分。', tone: 'cyan', icon: ScanSearch, task: '质量扫描' },
  { id: 'postgres', kicker: '04 · STORAGE', title: 'PostgreSQL 持久化', subtitle: '事务写入与运行审计', description: '同步结果在事务中提交；失败时保留上一次可用版本与本次错误回执。', tone: 'orange', icon: Database },
]

const mediaNodes: NodeDefinition[] = [
  { id: 'planning', kicker: '01 · PLANNING', title: '场景选题', subtitle: '作品、分集与场景需求', description: '按作品、分集和场景登记候选；此处只统计数据库中实际存在的候选记录。', tone: 'orange', icon: Shapes },
  { id: 'agent', kicker: '02 · PI DURABLE', title: '任务执行', subtitle: '持久任务、重试与恢复', description: 'Pi Durable 执行候选任务并持久化每个步骤，重复轮询复用同一次执行。', tone: 'cyan', icon: Bot },
  { id: 'reference', kicker: '03A · REFERENCE', title: '参考图生成', subtitle: '参考凭证与原图指纹', description: '基于角色参考生成原图，保存参考图、模型和内容指纹。', tone: 'blue', icon: Image },
  { id: 'crop', kicker: '03B · MOBILE', title: '手机裁切', subtitle: '手机构图与验图样张', description: '生成手机比例裁切，保留原图与验图样张的对应关系。', tone: 'violet', icon: Crop },
  { id: 'review', kicker: '03C · VISION', title: '自动验图', subtitle: '五项视觉质量检查', description: '检查人物一致、场景动作、五官手部、画面干净和手机构图。', tone: 'cyan', icon: ListChecks },
  { id: 'rework', kicker: '04 · REWORK', title: '返工与复验', subtitle: '重裁、重绘与版本保留', description: '裁切问题进入重裁，原图问题进入重绘；保留每轮结果和未通过原因。', tone: 'orange', icon: RefreshCcw },
]

function latestRun(runs: WorkbenchRun[]): NodeStatus {
  return runs[0]?.status ?? 'pending'
}

function mediaStatus(id: string, items: MediaCandidate[]): NodeStatus {
  if (!items.length) return 'pending'
  if (id === 'planning') return 'success'
  if (id === 'agent') {
    if (items.some((item) => !['passed', 'held', 'failed'].includes(item.stage))) return 'running'
    if (items.some((item) => item.stage === 'failed')) return 'failed'
    return 'success'
  }
  if (id === 'reference') {
    const steps = items.flatMap((item) => item.steps.filter((step) => step.step === 'reference'))
    if (steps.some((step) => step.finished_at === null && !step.error)) return 'running'
    if (steps.some((step) => step.error)) return 'failed'
    return steps.some((step) => step.finished_at) || items.some((item) => item.reference_sha) ? 'success' : 'pending'
  }
  if (id === 'crop') {
    if (items.some((item) => item.stage === 'recrop' || item.stage === 'recheck')) return 'running'
    if (items.some((item) => item.steps.some((step) => step.result?.phone))) return 'success'
    return items.some((item) => item.stage === 'failed') ? 'failed' : 'pending'
  }
  if (id === 'review') {
    if (items.some((item) => item.stage === 'review' || item.stage === 'recheck')) return 'running'
    if (items.some((item) => item.steps.some((step) => step.result?.decision === 'pass'))) return 'success'
    return items.some((item) => item.stage === 'held' || item.stage === 'failed') ? 'failed' : 'pending'
  }
  const rework = items.filter((item) => item.round > 1 || item.stage === 'held' || item.steps.some((step) => step.step === 'recrop' || step.step === 'recheck'))
  if (!rework.length) return 'pending'
  if (rework.some((item) => item.stage === 'recrop' || item.stage === 'recheck')) return 'running'
  if (rework.some((item) => item.stage === 'held' || item.stage === 'failed')) return 'failed'
  return 'success'
}

function nodeState(definition: NodeDefinition, flow: Flow, feed: WorkbenchFeed) {
  if (flow === 'media') {
    const items = feed.media ?? []
    return {
      status: feed.media === null ? 'failed' : mediaStatus(definition.id, items),
      meta: '媒体记录不可用',
      counts: feed.media === null ? null : { candidates: items.length, passed: items.filter((item) => item.stage === 'passed').length },
      runs: [] as WorkbenchRun[],
      media: items.slice(-5).reverse(),
      latest: items.at(-1)?.steps.at(-1)?.started_at ?? null,
    }
  }
  let runs = feed.runs
  if (definition.task) runs = runs.filter((run) => run.task === definition.task)
  if (definition.id === 'postgres') runs = runs.filter((run) => run.task !== '质量扫描')
  const unavailable = definition.task === '质量扫描' ? !feed.qualityOnline : !feed.syncOnline && definition.id !== 'scheduler'
  return {
    status: unavailable ? 'failed' : definition.id === 'scheduler' && !feed.syncOnline && !feed.qualityOnline ? 'failed' : latestRun(runs),
    meta: runs[0] ? `${runStatusLabels[runs[0].status]} · ${formatClock(runs[0].start)}` : '当前窗口暂无记录',
    counts: null,
    runs: runs.slice(0, 5),
    media: [] as MediaCandidate[],
    latest: runs[0]?.start ?? null,
  }
}

export function WorkflowCanvas({ feed, initialFlow = 'sync' }: { feed: WorkbenchFeed; initialFlow?: Flow }) {
  const [flow, setFlow] = useState<Flow>(initialFlow)
  const [selected, setSelected] = useState(syncNodes[0].id)
  const definitions = flow === 'sync' ? syncNodes : mediaNodes
  const activeDefinition = definitions.find((item) => item.id === selected) ?? definitions[0]
  const activeState = nodeState(activeDefinition, flow, feed)

  function changeFlow(next: Flow) {
    setFlow(next)
    setSelected(next === 'sync' ? syncNodes[0].id : mediaNodes[0].id)
  }

  function renderNode(definition: NodeDefinition) {
    const state = nodeState(definition, flow, feed)
    const Icon = definition.icon
    return (
      <button
        className={`wb-flow-node wb-tone-${definition.tone}`}
        data-status={state.status}
        aria-pressed={activeDefinition.id === definition.id}
        onClick={() => setSelected(definition.id)}
        key={definition.id}
      >
        <span className="wb-node-icon"><Icon size={18} /></span>
        <span className="wb-node-copy">
          <span className="wb-eyebrow">{definition.kicker}</span>
          <strong>{definition.title}</strong>
          <small>{definition.subtitle}</small>
        </span>
        <span className="wb-node-state">
          <i className="wb-status-dot" data-status={state.status} />
          {state.status === 'running' ? '执行中' : runStatusLabels[state.status]}
          {state.counts ? (
            <small className="wb-node-counts">
              <b>{state.counts.candidates.toLocaleString('zh-CN')}</b> 个候选 · <b>{state.counts.passed.toLocaleString('zh-CN')}</b> 个通过
            </small>
          ) : <small>{state.meta}</small>}
        </span>
      </button>
    )
  }

  return (
    <section className="wb-workflow" aria-labelledby="workflow-title">
      <Panel className="wb-workflow-canvas">
        <SectionHeading
          id="workflow-title"
          eyebrow="WORKFLOW CANVAS"
          title="工作流执行拓扑"
          detail="选择节点查看职责、真实状态与最近执行记录"
          actions={
            <div className="wb-segmented" aria-label="工作流类型">
              <button aria-pressed={flow === 'sync'} onClick={() => changeFlow('sync')}>知识同步</button>
              <button aria-pressed={flow === 'media'} onClick={() => changeFlow('media')}>图片处理</button>
            </div>
          }
        />
        <div className="wb-flow-chart">
          <div className="wb-flow-main">{renderNode(definitions[0])}</div>
          <Connector />
          <div className="wb-flow-main">{renderNode(definitions[1])}</div>
          <div className="wb-branch-connector"><GitBranch size={17} /></div>
          <div className="wb-flow-branches">{definitions.slice(2, 5).map(renderNode)}</div>
          <div className="wb-branch-connector wb-branch-join"><ArrowDown size={17} /></div>
          <div className="wb-flow-main">{renderNode(definitions[5])}</div>
        </div>
        <div className="wb-flow-legend">
          {(['success', 'running', 'pending', 'failed'] as const).map((status) => (
            <span key={status}><i className="wb-status-dot" data-status={status} />{runStatusLabels[status]}</span>
          ))}
          <span className="wb-flow-source">数据源 · PostgreSQL 运行回执</span>
        </div>
      </Panel>
      <Panel className="wb-inspector">
        <span className="wb-eyebrow">NODE INSPECTOR</span>
        <div className="wb-inspector-title">
          <activeDefinition.icon size={20} />
          <h3>{activeDefinition.title}</h3>
        </div>
        <StatusBadge status={activeState.status} />
        <p className="wb-inspector-description">{activeDefinition.description}</p>
        <dl className="wb-inspector-facts">
          <div><dt>流程</dt><dd>{flow === 'sync' ? '知识同步' : '图片处理'}</dd></div>
          <div><dt>最近记录</dt><dd>{formatDateTime(activeState.latest)}</dd></div>
          <div><dt>记录范围</dt><dd>{flow === 'sync' ? `${activeState.runs.length} 条执行回执` : `${activeState.media.length} 个最近候选`}</dd></div>
        </dl>
        <div className="wb-inspector-history">
          <span className="wb-eyebrow">RECENT EXECUTIONS</span>
          {activeState.runs.map((run) => (
            <article key={run.id}>
              <div><strong>{run.task}</strong><StatusBadge status={run.status} /></div>
              <time>{formatDateTime(run.start)}</time>
              <code>{run.id}</code>
              {run.error && <p>{run.error}</p>}
            </article>
          ))}
          {activeState.media.map((item) => (
            <article key={item.slot}>
              <div><a className="wb-candidate-link" href={`/workbench?view=data&section=media&candidate=${item.slot}#candidate-${item.slot}`}>
                <strong>{item.scene} · 候选 {item.variant}</strong>
                <span>查看详情 →</span>
              </a><StatusBadge status={item.stage} label={mediaStageLabels[item.stage]} /></div>
              <time>{formatDateTime(item.steps.at(-1)?.started_at)}</time>
              <code>候选槽位 #{item.slot} · v{item.round}</code>
              {item.reason && <p className={item.stage === 'passed' ? 'wb-candidate-reason' : undefined}>{item.reason}</p>}
            </article>
          ))}
          {!activeState.runs.length && !activeState.media.length && (
            <EmptyState icon={FileSearch} title="暂无执行记录" detail="当前数据窗口内没有可展示的节点回执。" />
          )}
        </div>
      </Panel>
    </section>
  )
}

function Connector() {
  return <div className="wb-connector" aria-hidden="true"><span /><ArrowDown size={17} /><span /></div>
}

export function MediaStageBadge({ stage }: { stage: MediaStage }) {
  return <StatusBadge status={stage} label={mediaStageLabels[stage]} />
}

export const mediaStageNames = mediaStageLabels
