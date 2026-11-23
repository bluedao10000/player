import type { MediaStage } from '@anime/shared/knowledge'
import {
  Activity, Archive, BookOpenCheck, Boxes, BrainCircuit, CheckCircle2, Clock3, FileImage,
  FileSearch, GitBranch, Images, ListChecks, Network, PenLine, ScanSearch, Settings2,
  ShieldCheck, Sparkles, Timer, type LucideIcon,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import {
  readOpsTopologyDetail,
  type OpsTopology, type OpsTopologyAgents, type OpsTopologyDetail,
  type OpsTopologyStage, type OpsTopologyWindow,
} from '../feed'
import { useNavigate } from '@tanstack/react-router'
import type { WorkKnowledgeStatus } from '@anime/shared/knowledge'
import { formatDateTime } from '../shared/format'
import { EmptyState, Panel, SectionHeading, StatusBadge } from '../shared/ui'

const stageIcons: Record<string, LucideIcon> = {
  source_sync: Network,
  normalize: Boxes,
  quality_scan: ScanSearch,
  inventory: BookOpenCheck,
  plan: ListChecks,
  configuration: Settings2,
  admission: CheckCircle2,
  capture: FileSearch,
  research: BrainCircuit,
  editorial: PenLine,
  media_reference: FileImage,
  media_generate: Sparkles,
  media_review: ShieldCheck,
  media_rework: Images,
  acceptance: CheckCircle2,
  release: Archive,
  backup: Archive,
}

const responsibilities: Record<string, string> = {
  source_sync: '按来源读取作品知识，并记录同步回执。',
  normalize: '规范并归并来源数据；此阶段没有独立执行记录。',
  quality_scan: '运行知识质量规则并保存扫描结果。',
  inventory: '盘点可制作分集与当前覆盖情况；此阶段没有独立执行记录。',
  plan: '生成或装载分集制作计划。',
  configuration: '为作品和分集生成制作配置。',
  admission: '根据补集策略检查预算并登记准入。',
  capture: '采集补集原文并保留每次尝试。',
  research: '为作品和分集整理取证资料。',
  editorial: '执行编辑任务并保存版本与审稿状态。',
  media_reference: '准备图片生成所需的参考凭证。',
  media_generate: '生成图片候选原图。',
  media_review: '自动检查图片质量并记录业务结果。',
  media_rework: '处理重绘、重裁和裁切复验。',
  acceptance: '按验收范围检查知识版本。',
  release: '把通过保护检查的内容发布到阅读器。',
  backup: '执行制作库备份和校验。',
}

const statusLabels: Record<string, string> = {
  running: '运行中', pending: '等待', blocked: '阻塞', success: '成功', failed: '失败',
  idle: '暂无执行',
}
const windowLabels: Record<OpsTopologyWindow, string> = { '24h': '24 小时', '7d': '7 天', '30d': '30 天' }

export function WorkflowCanvas({
  initialData,
  worlds,
  initialWorld = '',
  initialWindow = '24h',
  initialStage = '',
}: {
  initialData: OpsTopology | null
  worlds: WorkKnowledgeStatus[]
  initialWorld?: string
  initialWindow?: OpsTopologyWindow
  initialStage?: string
}) {
  const navigate = useNavigate({ from: '/workbench/operations' })
  const [topology, setTopology] = useState(initialData)
  const [world, setWorld] = useState(initialWorld)
  const [window, setWindow] = useState<OpsTopologyWindow>(initialWindow)
  const [selectedStage, setSelectedStage] = useState(initialStage)
  const [detail, setDetail] = useState<OpsTopologyDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState(false)
  const [paging, setPaging] = useState(false)

  useEffect(() => setTopology(initialData), [initialData])
  useEffect(() => setWorld(initialWorld), [initialWorld])
  useEffect(() => setWindow(initialWindow), [initialWindow])

  useEffect(() => {
    let cancelled = false
    if (!selectedStage) {
      setDetail(null)
      setDetailError(false)
      return
    }
    setDetailLoading(true)
    setDetailError(false)
    setDetail(null)
    void readOpsTopologyDetail({ data: { stage: selectedStage, window, world, limit: 20 } })
      .then((result) => {
        if (!cancelled) {
          setDetail(result)
          setDetailError(result === null)
        }
      })
      .catch(() => { if (!cancelled) setDetailError(true) })
      .finally(() => { if (!cancelled) setDetailLoading(false) })
    return () => { cancelled = true }
  }, [selectedStage, window, world])

  const stages = [...(topology?.groups.flatMap((group) => group.stages) ?? []), ...(topology?.infrastructure.stages ?? [])]
  const activeStage = stages.find((stage) => stage.id === selectedStage) ?? null
  const downstream = activeStage ? stages.filter((stage) => stage.upstream.includes(activeStage.id)).map((stage) => stage.title) : []

  async function changeFilter(nextWindow: OpsTopologyWindow, nextWorld: string) {
    setWindow(nextWindow)
    setWorld(nextWorld)
    setDetail(null)
    await navigate({ search: (previous) => ({ ...previous, window: nextWindow, world: nextWorld || undefined }) })
  }

  async function loadMore() {
    if (!detail || paging) return
    setPaging(true)
    try {
      const next = await readOpsTopologyDetail({ data: {
        stage: selectedStage, window, world, limit: detail.pagination.limit,
        ...(detail.pagination.queue_after ? { queue_after: detail.pagination.queue_after } : {}),
        ...(detail.pagination.runs_after ? { runs_after: detail.pagination.runs_after } : {}),
      } })
      if (next) setDetail({
        ...detail,
        current_queue: [...detail.current_queue, ...next.current_queue],
        recent_runs: [...detail.recent_runs, ...next.recent_runs],
        pagination: next.pagination,
      })
    } finally { setPaging(false) }
  }

  return (
    <section className="wb-topology" aria-labelledby="workflow-title">
      <Panel className="wb-workflow-canvas">
        <SectionHeading
          id="workflow-title"
          eyebrow="OPERATIONS TOPOLOGY"
          title="全局运营拓扑"
          detail="工作流定义、实际任务和 Agent 执行实例分别展示；窗口按北京时间统计。"
          actions={<div className="wb-topology-filters">
            <label>统计窗口<select value={window} onChange={(event) => void changeFilter(event.target.value as OpsTopologyWindow, world)}>
              {Object.entries(windowLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select></label>
            <label>作品<select value={world} onChange={(event) => void changeFilter(window, event.target.value)}>
              <option value="">全部作品</option>
              {worlds.filter((item) => item.slug).map((item) => <option key={item.slug!} value={item.slug!}>{item.title} · {item.slug}</option>)}
            </select></label>
          </div>}
        />
        {topology ? <>
          <div className="wb-topology-meta">
            <span>窗口：{formatDateTime(topology.window_start)} 至 {formatDateTime(topology.window_end)}（左闭右开）</span>
            <span>更新于 {formatDateTime(topology.as_of)}</span>
          </div>
          {topology.selected_world && <WorldProgress world={topology.selected_world} />}
          <DependencyEdges stages={stages} onSelect={setSelectedStage} />
          <div className="wb-topology-groups">
            {topology.groups.map((group) => <details className="wb-topology-group" key={group.id} open>
              <summary><span>{group.eyebrow}</span><h3>{group.label}</h3></summary>
              <div className="wb-topology-stages">
                {group.stages.map((stage) => <StageCard key={stage.id} stage={stage} selected={selectedStage === stage.id}
                  faded={Boolean(world) && stage.has_records === false} onSelect={() => setSelectedStage(stage.id)} />)}
              </div>
            </details>)}
          </div>
          {topology.infrastructure.stages.length > 0 && <section className="wb-topology-infrastructure" aria-label="基础设施">
            <header><span>{topology.infrastructure.eyebrow}</span><h3>{topology.infrastructure.label}</h3><small>拓扑之外的全库任务</small></header>
            <div className="wb-topology-stages">{topology.infrastructure.stages.map((stage) => <StageCard key={stage.id} stage={stage}
              selected={selectedStage === stage.id} faded={false} onSelect={() => setSelectedStage(stage.id)} />)}</div>
          </section>}
          <div className="wb-flow-legend">
            <span><i className="wb-status-dot" data-status="running" />运行中</span>
            <span><i className="wb-status-dot" data-status="pending" />等待</span>
            <span><i className="wb-status-dot" data-status="blocked" />阻塞位置</span>
            <span className="wb-flow-source">节点定义来自 ops_stages · 任务来自 Phase 0 视图</span>
          </div>
        </> : <EmptyState icon={Network} title="拓扑暂不可用" detail="请检查管理员接口后刷新页面。" />}
      </Panel>
      <StageInspector stage={activeStage} detail={detail} loading={detailLoading} failed={detailError}
        downstream={downstream} onLoadMore={() => void loadMore()} paging={paging} />
    </section>
  )
}

function DependencyEdges({ stages, onSelect }: { stages: OpsTopologyStage[]; onSelect: (stage: string) => void }) {
  const titles = new Map(stages.map((stage) => [stage.id, stage.title]))
  const edges = stages.flatMap((stage) => stage.upstream.map((source) => ({ source, target: stage.id })))
  return <details className="wb-topology-edges">
    <summary>阶段依赖 · {edges.length} 条连接</summary>
    <div>{edges.map(({ source, target }) => <span key={`${source}:${target}`}>
      <button type="button" onClick={() => onSelect(source)}>{titles.get(source) ?? source}</button>
      <i aria-hidden="true">→</i>
      <button type="button" onClick={() => onSelect(target)}>{titles.get(target) ?? target}</button>
    </span>)}</div>
  </details>
}

function StageCard({ stage, selected, faded, onSelect }: { stage: OpsTopologyStage; selected: boolean; faded: boolean; onSelect: () => void }) {
  const Icon = stageIcons[stage.id] ?? Activity
  const current = stage.current
  const stats = stage.window_stats
  const status = !stage.tracked ? 'pending' : current?.blocked || current?.waiting_human ? 'blocked'
    : current?.running ? 'running' : stats?.failed ? 'failed' : current?.pending ? 'pending' : 'success'
  return <button type="button" className="wb-topology-node" data-status={status} data-selected={selected} data-faded={faded}
    onClick={onSelect} aria-pressed={selected}>
    <span className="wb-topology-node-head"><span className="wb-node-icon"><Icon size={16} /></span>
      <span><small>{stage.grp.toUpperCase()}</small><strong>{stage.title}</strong></span>
      {stage.bottleneck && <span className="wb-topology-blocked">阻塞</span>}
    </span>
    {stage.tracked ? <>
      <span className="wb-topology-current">
        <b>当前 {current?.total ?? 0}</b><span>运行 {current?.running ?? 0}</span><span>等待 {current?.pending ?? 0}</span>
        <span>阻塞 {current?.blocked ?? 0}</span><span>待人工 {current?.waiting_human ?? 0}</span>
      </span>
      <span className="wb-topology-window">
        <span>成功 {stats?.success ?? 0} · 失败 {stats?.failed ?? 0}</span>
        <span>成功率 {percent(stats?.success_rate)} · {stats?.throughput_per_hour.toFixed(2) ?? '0.00'} 次 / 小时{stats?.sample_quality === 'low' ? ' · 样本较少' : ''}</span>
        <span>中位数 {seconds(stats?.duration.median_seconds)} · 平均 {seconds(stats?.duration.average_seconds)} · P90 {seconds(stats?.duration.p90_seconds)}</span>
        <span>最近执行 {formatDateTime(stats?.recent_execution_at)}</span>
      </span>
      <span className="wb-topology-agents">{stage.agents?.mapped
        ? <>在线工作进程 {stage.agents.online_workers ?? 0} · 运行中执行实例 {stage.agents.running_executions ?? (stage.agents.execution_coverage === 'legacy_aggregate' ? '未细分' : '—')}</>
        : '无对应工作进程'}
      </span>
    </> : <span className="wb-topology-untracked">无独立执行记录</span>}
    {worldLabel(stage, Boolean(faded)) && <span className="wb-topology-scope">{worldLabel(stage, Boolean(faded))}</span>}
  </button>
}

function worldLabel(stage: OpsTopologyStage, faded: boolean) {
  if (stage.global_scope) return '全库任务'
  return faded ? '该作品暂无记录' : ''
}

function StageInspector({ stage, detail, loading, failed, downstream, onLoadMore, paging }: {
  stage: OpsTopologyStage | null; detail: OpsTopologyDetail | null; loading: boolean; failed: boolean
  downstream: string[]; onLoadMore: () => void; paging: boolean
}) {
  if (!stage) return <Panel className="wb-inspector"><EmptyState icon={GitBranch} title="选择一个阶段" detail="查看职责、工作进程、队列和执行记录。" /></Panel>
  const Icon = stageIcons[stage.id] ?? Activity
  const hasMore = Boolean(detail?.pagination.queue_after || detail?.pagination.runs_after)
  return <Panel className="wb-inspector wb-topology-inspector">
    <span className="wb-eyebrow">STAGE DETAIL</span>
    <div className="wb-inspector-title"><Icon size={20} /><h3>{stage.title}</h3></div>
    <p className="wb-inspector-description">{responsibilities[stage.id] ?? stage.description}</p>
    <dl className="wb-inspector-facts">
      <div><dt>上游阶段</dt><dd>{stage.upstream.length ? stage.upstream.join('、') : '无'}</dd></div>
      <div><dt>下游阶段</dt><dd>{downstream.length ? downstream.join('、') : '无'}</dd></div>
      <div><dt>最近执行</dt><dd>{formatDateTime(stage.window_stats?.recent_execution_at)}</dd></div>
      {stage.window_stats && <div><dt>成功率</dt><dd>{percent(stage.window_stats.success_rate)}</dd></div>}
      {(stage.agents?.legacy_unclassified ?? 0) > 0 && <div><dt>旧版未细分执行</dt><dd>{stage.agents?.legacy_unclassified ?? 0}</dd></div>}
    </dl>
    {loading && <p role="status">正在读取阶段详情…</p>}
    {failed && <p role="alert">阶段详情暂不可用。</p>}
    {detail && <>
      <section className="wb-topology-detail-section"><h4>关联工作进程</h4>
        {!detail.workers.length && <p>无对应工作进程</p>}
        {detail.workers.map((worker) => <article key={`${worker.role}/${worker.loop}/${worker.worker_id ?? 'none'}`}>
          <strong>{worker.worker_id ?? `${worker.role} / ${worker.loop}`}</strong><StatusBadge status={worker.state ?? 'offline'} label={workerState(worker.state)} />
          <small>{worker.node ?? '节点未知'} · {worker.version ?? '版本未知'} · 最近心跳 {formatDateTime(worker.last_seen)}</small>
          {worker.last_error && <p>{worker.last_error}</p>}
        </article>)}
      </section>
      <section className="wb-topology-detail-section"><h4>当前队列 <small>{detail.current_queue.length}</small></h4>
        {!detail.current_queue.length && <p>当前没有待处理项。</p>}
        {detail.current_queue.map((item) => <article key={item.item_key}>
          <strong>{item.subject || item.item_key}</strong><StatusBadge status={item.state} label={item.state} />
          <small>{item.world_slug ?? '全库任务'}{item.episode ? ` · 第 ${item.episode} 集` : ''} · {formatDateTime(item.since)}</small>
          <p>{item.reason}</p>
          <a href={detailHref(item.link)}>查看队列项 →</a>
        </article>)}
      </section>
      <section className="wb-topology-detail-section"><h4>失败原因</h4>
        {!detail.failure_reasons.length && <p>所选窗口内没有失败记录。</p>}
        {detail.failure_reasons.map((failure) => <article key={failure.reason}><strong>{failure.count} 次</strong><p>{failure.reason}</p></article>)}
      </section>
      <section className="wb-topology-detail-section"><h4>业务结果分布</h4>
        {Object.entries(stage.window_stats?.outcome_distribution ?? {}).length === 0 && <p>所选窗口内没有业务结果。</p>}
        {Object.entries(stage.window_stats?.outcome_distribution ?? {}).map(([outcome, count]) => <article key={outcome}><strong>{outcome}</strong><small>{count} 次</small></article>)}
      </section>
      <section className="wb-topology-detail-section"><h4>最近执行记录</h4>
        {!detail.recent_runs.length && <p>当前窗口内没有执行记录。</p>}
        {detail.recent_runs.map((run) => <article key={run.run_key}>
          <div><strong>{run.world_slug ?? '全库任务'}{run.episode_first ? ` · 第 ${run.episode_first}${run.episode_last && run.episode_last !== run.episode_first ? `–${run.episode_last}` : ''} 集` : ''}</strong>
            <StatusBadge status={run.status} label={run.status} /></div>
          <small>{formatDateTime(run.started_at)}{run.finished_at ? ` · 完成于 ${formatDateTime(run.finished_at)}` : ''} · {run.outcome ?? '无业务结果'}</small>
          {run.duration_seconds !== null && <small>执行耗时 {seconds(run.duration_seconds)}</small>}
          <code>{run.source_table} · {run.source_id}</code>
          {run.error && <p>{run.error}</p>}
          <a href={detailHref(run.link)}>查看执行记录 →</a>
        </article>)}
      </section>
      {hasMore && <button type="button" className="wb-button wb-topology-more" disabled={paging} onClick={onLoadMore}>{paging ? '正在加载…' : '加载更多'}</button>}
    </>}
  </Panel>
}

function WorldProgress({ world }: { world: NonNullable<OpsTopology['selected_world']> }) {
  const states = [
    ['未登记', world.progress.unregistered], ['已登记', world.progress.registered], ['已规划', world.progress.planned],
    ['生产中', world.progress.in_production], ['已验收', world.progress.accepted], ['已发布', world.progress.published],
  ] as const
  return <section className="wb-topology-progress" aria-label="作品生产进度">
    <div><strong>{world.title}</strong><span>{world.has_records ? '已存在阶段记录' : '尚无阶段记录'}</span>
      <b>{world.progress.scope_total} 集制作范围</b></div>
    <div className="wb-topology-progress-states">{states.map(([label, count]) => <span key={label}><small>{label}</small><strong>{count}</strong></span>)}</div>
  </section>
}

function percent(value: number | null | undefined) {
  return value == null ? '—' : `${(value * 100).toFixed(1)}%`
}

function seconds(value: number | null | undefined) {
  if (value == null) return '—'
  if (value < 60) return `${value.toFixed(1)} 秒`
  return `${(value / 60).toFixed(1)} 分`
}

function workerState(value: string | null) {
  if (value === 'online') return '在线'
  if (value === 'online_error') return '在线但有错误'
  if (value === 'offline') return '离线'
  return '未知'
}

function detailHref(link: { page: string; params: Record<string, string | number | null> }) {
  const params = new URLSearchParams()
  Object.entries(link.params).forEach(([key, value]) => { if (value !== null) params.set(key, String(value)) })
  return `/workbench/${link.page}${params.size ? `?${params.toString()}` : ''}`
}

export function MediaStageBadge({ stage }: { stage: MediaStage }) {
  return <StatusBadge status={stage} label={mediaStageNames[stage]} />
}

export const mediaStageNames: Record<MediaStage, string> = {
  reference: '参考图准备', queued: '等待生成', generate: '原图生成', review: '自动验图',
  recrop: '重新裁切', recheck: '裁切复验', passed: '验图通过', held: '等待返工', failed: '任务失败',
}
