import { useState, type FormEvent } from 'react'
import type { CompletenessDimension, KnowledgeSummary, KnowledgeWorkFilters, KnowledgeWorksPage } from './feed'
import { readKnowledgeWorksPage } from './feed'
import { formatDateTime } from './shared/format'
import { Panel, SectionHeading } from './shared/ui'

const itemNames: Record<string, string> = {
  core_fields: '基础档案字段', episode_total: '总集数', introductions: '人物介绍', relationships: '人物关系',
  storylines: '故事线', chapters: '章节', episodes: '分集覆盖', scenes: '场景', locations: '地点',
  organizations: '组织', typed_settings: '设定类型', events: '时间线事件', chapter_links: '关联章节',
  order_checks: '顺序校验', main_visuals: '主视觉', portraits: '人物立绘', chapter_images: '章节配图',
}
const dimensionNames: Record<string, string> = {
  basic_profile: '基础档案', people: '人物知识', plot: '剧情知识', scenes: '场景知识',
  world: '世界观', timeline: '时间线', images: '图片资产',
}
const statusNames: Record<string, string> = {
  completed: '已完成', partial: '部分完成', missing: '缺失', not_applicable: '不适用',
  pending_verification: '待验证', no_data: '无数据',
}
const productionStatusNames: Record<string, string> = {
  idle: '空闲', pending: '待处理', blocked: '阻塞', waiting_human: '待人工',
}

const defaultFilters: KnowledgeWorkFilters = {
  q: '', has_knowledge_version: 'true', type: '', completeness_status: '', production_status: '', sort: 'last_updated_at', after: '', source_review: false,
}

export function KnowledgeCenter({ summary, works }: { summary: KnowledgeSummary | null; works: KnowledgeWorksPage | null }) {
  const [page, setPage] = useState(works)
  const [filters, setFilters] = useState<KnowledgeWorkFilters>(defaultFilters)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  async function applyFilters(next: KnowledgeWorkFilters, after = '') {
    setFilters(next)
    setLoading(true)
    setError('')
    try {
      const result = await readKnowledgeWorksPage({ data: { ...next, after } })
      if (!result) setError('作品列表暂不可用。')
      else setPage((current) => after ? { ...result, items: [...(current?.items ?? []), ...result.items] } : result)
    } catch {
      setError('作品列表暂不可用。')
    } finally {
      setLoading(false)
    }
  }

  function drillDown(patch: Partial<KnowledgeWorkFilters>) {
    const next: KnowledgeWorkFilters = { ...defaultFilters, ...patch, after: '' }
    void applyFilters(next)
    document.getElementById('knowledge-works-title')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  return <div className="wb-report-stack">
    <SummaryPanel summary={summary} onDrillDown={drillDown} />
    <WorkList
      page={page}
      filters={filters}
      loading={loading}
      error={error}
      types={summary?.works.by_type ?? []}
      onFilterChange={(key, value) => setFilters((current) => ({ ...current, [key]: value }))}
      onSubmit={(event) => { event.preventDefault(); void applyFilters(filters) }}
      onReset={() => void applyFilters(defaultFilters)}
      onLoadMore={(after) => void applyFilters(filters, after)}
    />
  </div>
}

function SummaryPanel({ summary, onDrillDown }: { summary: KnowledgeSummary | null; onDrillDown: (patch: Partial<KnowledgeWorkFilters>) => void }) {
  if (!summary) return <section className="wb-panel wb-inventory" role="status"><h2>知识资产摘要暂不可用</h2><p>无法读取管理接口返回的数据。</p></section>
  const updated = formatDateTime(summary.as_of)
  const gov = summary.governance
  const scaleMetrics: { title: string; value: number | string; detail: string; definition: string; action?: () => void; actionLabel?: string }[] = [
    {
      title: '活跃目录作品', value: summary.works.active_total, detail: '含仅目录与已建世界作品',
      definition: 'works.active = true', action: () => onDrillDown({ has_knowledge_version: 'all' }), actionLabel: '查看全部活跃目录',
    },
    {
      title: '已发布作品世界', value: summary.works.published_worlds, detail: '已建立公开知识版本的作品',
      definition: 'published_worlds 行数', action: () => onDrillDown({ has_knowledge_version: 'true' }), actionLabel: '筛选已建世界',
    },
    {
      title: '不可变发行版本', value: summary.works.release_versions, detail: '历史与当前公开版本累计',
      definition: 'world_releases 行数',
    },
    {
      title: '尚无知识版本', value: Math.max(0, summary.works.active_total - summary.works.published_worlds), detail: '停留在基础目录阶段的作品',
      definition: '活跃目录 - 已发布世界', action: () => onDrillDown({ has_knowledge_version: 'false' }), actionLabel: '查看仅目录作品',
    },
  ]
  const assetMetrics = [
    ['人物', summary.people.total, '当前发布版本中的人物', '公开人物库存', 'characters'],
    ['人物介绍', summary.people.introductions_complete == null ? '—' : `${summary.people.introductions_complete} / ${summary.people.total}`, '独立人物简介完整数 / 人物数', '完整度扫描结果'],
    ['故事线', summary.plot.storylines, '当前公开故事线', '公开知识库存'],
    ['章节', summary.plot.chapters, '当前公开章节', '公开知识库存'],
    ['分集覆盖', `${summary.plot.episodes_covered} / ${summary.plot.episodes_expected}`, '已覆盖集数 / 连续剧声明总集数', '仅统计 TV/ONA/OVA/SPECIAL', 'episodes_covered'],
    ['场景', summary.plot.scenes, '当前公开场景', '公开知识库存', 'scenes'],
    ['地点', summary.world.locations, '当前公开地点（组织/设定类型无数据）', '公开知识库存'],
    ['主视觉 / 立绘 / 配图', `${summary.images.main_visuals} / ${summary.images.portraits} / ${summary.images.chapter_images}`, '三类已发布图片分开计数，不相加', '公开资产引用', 'images'],
  ] as const
  return <>
    <section className="wb-panel wb-inventory" aria-label="目录与世界规模">
      <SectionHeading id="knowledge-scale-title" eyebrow="CATALOG & WORLDS" title="目录与世界规模" detail={`口径 v${summary.definition} · 更新于 ${updated} · 点击卡片可直接下钻筛选作品列表。`} />
      <div className="wb-metrics">
        {scaleMetrics.map((metric) => <Panel className="wb-metric wb-tone-cyan" key={metric.title}>
          <div className="wb-metric-top"><span className="wb-eyebrow">{metric.title}</span></div>
          <strong>{typeof metric.value === 'number' ? metric.value.toLocaleString('zh-CN') : metric.value}</strong>
          <p>{metric.detail}</p>
          <small>{metric.definition}</small>
          {metric.action && <button type="button" className="wb-drill-btn" onClick={metric.action}>{metric.actionLabel} →</button>}
        </Panel>)}
      </div>
    </section>
    {gov && <section className="wb-panel wb-inventory" aria-label="治理与线上状态">
      <SectionHeading id="knowledge-gov-title" eyebrow="GOVERNANCE & ONLINE" title="治理、验证与线上状态" detail="区分完整度待验证、目录来源冲突、目录质量问题与正式站线上核验。" />
      <div className="wb-metrics">
        <Panel className="wb-metric wb-tone-amber">
          <div className="wb-metric-top"><span className="wb-eyebrow">待验证作品</span></div>
          <strong>{gov.pending_verification_works.toLocaleString('zh-CN')}</strong>
          <p>已有来源或取证依据但范围验收尚未通过</p>
          <small>knowledge_completeness_results</small>
          <button type="button" className="wb-drill-btn" onClick={() => onDrillDown({ has_knowledge_version: 'true', completeness_status: 'pending_verification' })}>筛选待验证作品 →</button>
        </Panel>
        <Panel className="wb-metric wb-tone-amber">
          <div className="wb-metric-top"><span className="wb-eyebrow">待裁决来源冲突</span></div>
          <strong>{gov.open_match_candidates.toLocaleString('zh-CN')}</strong>
          <p>上游弱匹配或身份候选，需确认归属</p>
          <small>knowledge_match_candidates</small>
        </Panel>
        <Panel className="wb-metric wb-tone-amber">
          <div className="wb-metric-top"><span className="wb-eyebrow">来源待复核</span></div>
          <strong>{(gov.source_review_works ?? 0).toLocaleString('zh-CN')}</strong>
          <p>有上游来源变化待核对的作品</p>
          <button type="button" className="wb-drill-btn" onClick={() => onDrillDown({ has_knowledge_version: 'all', source_review: true })}>查看待复核作品 →</button>
        </Panel>
        <Panel className="wb-metric wb-tone-cyan">
          <div className="wb-metric-top"><span className="wb-eyebrow">目录质量未处理项</span></div>
          <strong>{gov.open_quality_issues.toLocaleString('zh-CN')}</strong>
          <p>仅反映目录字段规范，不代表剧情事实正确性</p>
          <small>knowledge_quality_issues (open)</small>
        </Panel>
        <Panel className="wb-metric wb-tone-cyan">
          <div className="wb-metric-top"><span className="wb-eyebrow">线上已验证范围</span></div>
          <strong>{gov.online_verified_topics.toLocaleString('zh-CN')}</strong>
          <p>已投递正式站并通过公开页与图片摘要核验</p>
          <small>reader_release_attempts</small>
        </Panel>
        <Panel className="wb-metric wb-tone-amber">
          <div className="wb-metric-top"><span className="wb-eyebrow">线上投递失败</span></div>
          <strong>{gov.release_failed_topics.toLocaleString('zh-CN')}</strong>
          <p>最近一次正式站投递或公网校验失败</p>
          <small>reader_release_attempts</small>
          <a className="wb-drill-btn" href="/workbench/exceptions">前往异常与决策 →</a>
        </Panel>
      </div>
    </section>}
    <section className="wb-panel wb-inventory" aria-label="知识资产分项">
      <SectionHeading id="knowledge-assets-title" eyebrow="KNOWLEDGE ASSETS" title="知识资产分项" detail="只统计当前公开版本资产；人物关系、组织、设定类型与时间线暂无结构，不计入分母。" />
      <div className="wb-metrics">
        {assetMetrics.map(([title, value, detail, definition, sort]) => <Panel className="wb-metric wb-tone-cyan" key={title}>
          <div className="wb-metric-top"><span className="wb-eyebrow">{title}</span></div>
          <strong>{typeof value === 'number' ? value.toLocaleString('zh-CN') : value}</strong>
          <p>{detail}</p>
          <small>{definition} · 更新于 {updated}</small>
          {sort && <button type="button" className="wb-drill-btn" onClick={() => onDrillDown({ has_knowledge_version: 'true', sort })}>按{sortLabel(sort)}查看作品 →</button>}
        </Panel>)}
      </div>
    </section>
    <section className="wb-panel wb-overview-types" aria-labelledby="knowledge-types-title">
      <SectionHeading id="knowledge-types-title" eyebrow="WORK TYPES" title="作品类型分布" detail="按 works.type 原值分组；点击任意类型可下钻筛选全部活跃目录。" />
      <ul className="wb-type-distribution">
        {summary.works.by_type.map((row) => <li key={row.type ?? 'unregistered'}>
          <button type="button" className="wb-type-btn" onClick={() => onDrillDown({ has_knowledge_version: 'all', type: row.type ?? '__null__' })}>
            <span>{row.type === null ? '未登记' : row.type}</span><strong>{row.count.toLocaleString('zh-CN')}</strong>
          </button>
        </li>)}
      </ul>
    </section>
    <SourcesPanel summary={summary} />
    <TrendPanel summary={summary} />
  </>
}

function SourcesPanel({ summary }: { summary: KnowledgeSummary }) {
  return <section className="wb-panel wb-inventory" aria-labelledby="knowledge-sources-title">
    <SectionHeading id="knowledge-sources-title" eyebrow="DATA SOURCES" title="数据来源" detail="来源记录与外部作品 ID 分别统计，不相加。" />
    <div className="wb-worlds">
      <article><h3>来源记录</h3>{summary.sources.records_by_source.length ? <ul>{summary.sources.records_by_source.map((row) =>
        <li key={row.name}>{row.name} · {row.records} 条记录 · {row.works} 部作品</li>)}</ul> : <p>暂无活跃来源记录。</p>}</article>
      <article><h3>外部作品 ID</h3>{summary.sources.external_ids_by_provider.length ? <ul>{summary.sources.external_ids_by_provider.map((row) =>
        <li key={row.provider}>{row.provider} · {row.external_ids} 个 ID · {row.works} 部作品</li>)}</ul> : <p>暂无外部作品 ID。</p>}</article>
    </div>
  </section>
}

function TrendPanel({ summary }: { summary: KnowledgeSummary }) {
  const points = summary.trend.points.slice(-10)
  return <section className="wb-panel wb-inventory" aria-labelledby="knowledge-trend-title">
    <SectionHeading id="knowledge-trend-title" eyebrow="DAILY SNAPSHOTS" title="知识资产趋势" detail="按 definition 分段；完整度从 definition 2 首次快照开始，之前日期留空。" />
    {!points.length ? <p>暂无日快照。</p> : <ul>{points.map((point) => <li key={`${point.day}-${point.definition}`}>
      {point.day} · definition {point.definition} · 作品 {formatTrend(point.active_works)} · 发布版本 {formatTrend(point.published_worlds)}
      {point.definition >= 2 && point.completeness ? ' · 已记录完整度分项' : ' · 完整度留空'}
    </li>)}</ul>}
  </section>
}

function formatTrend(value: unknown) {
  return typeof value === 'number' ? value.toLocaleString('zh-CN') : '—'
}

function WorkList({
  page, filters, loading, error, types, onFilterChange, onSubmit, onReset, onLoadMore,
}: {
  page: KnowledgeWorksPage | null
  filters: KnowledgeWorkFilters
  loading: boolean
  error: string
  types: { type: string | null; count: number }[]
  onFilterChange: <K extends keyof KnowledgeWorkFilters>(key: K, value: KnowledgeWorkFilters[K]) => void
  onSubmit: (event: FormEvent) => void
  onReset: () => void
  onLoadMore: (after: string) => void
}) {
  return <section className="wb-panel wb-inventory" aria-labelledby="knowledge-works-title">
    <SectionHeading id="knowledge-works-title" eyebrow="WORK CATALOG" title="作品列表" detail="默认显示有已发布知识版本的作品；可切换查看全部活跃目录或下钻特定类型/状态。" />
    <form className="wb-media-filters" onSubmit={onSubmit}>
      <label>搜索<input value={filters.q} onChange={(event) => onFilterChange('q', event.target.value)} placeholder="作品名或别名" /></label>
      <label>目录范围<select value={filters.has_knowledge_version} onChange={(event) => onFilterChange('has_knowledge_version', event.target.value as KnowledgeWorkFilters['has_knowledge_version'])}>
        <option value="true">有知识版本</option><option value="all">全部活跃目录</option><option value="false">尚无知识版本</option>
      </select></label>
      <label>作品类型<select value={filters.type} onChange={(event) => onFilterChange('type', event.target.value)}>
        <option value="">全部类型</option>{types.map((row) => <option key={row.type ?? '__null__'} value={row.type ?? '__null__'}>{row.type ?? '未登记'}</option>)}
      </select></label>
      <label>完整度状态<select value={filters.completeness_status} onChange={(event) => onFilterChange('completeness_status', event.target.value)}>
        <option value="">全部状态</option>{['completed', 'partial', 'missing', 'pending_verification'].map((status) => <option key={status} value={status}>{statusNames[status]}</option>)}
      </select></label>
      <label>生产状态<select value={filters.production_status} onChange={(event) => onFilterChange('production_status', event.target.value)}>
        <option value="">全部状态</option>{(['idle', 'pending', 'blocked', 'waiting_human'] as const).map((status) => <option key={status} value={status}>{productionStatusNames[status]}</option>)}
      </select></label>
      <label>来源复核<select value={filters.source_review ? 'pending' : 'all'} onChange={(event) => onFilterChange('source_review', event.target.value === 'pending')}>
        <option value="all">全部作品</option><option value="pending">来源待复核</option>
      </select></label>
      <label>排序<select value={filters.sort} onChange={(event) => onFilterChange('sort', event.target.value)}>
        {['last_updated_at', 'title', 'type', 'release_versions', 'episodes_covered', 'characters', 'scenes', 'images', 'completeness', 'acceptance_pass_rate'].map((sort) => <option key={sort} value={sort}>{sortLabel(sort)}</option>)}
      </select></label>
      <button type="submit" disabled={loading}>{loading ? '读取中…' : '筛选作品'}</button>
      <button type="button" disabled={loading} onClick={onReset}>重置</button>
    </form>
    {error && <p role="status">{error}</p>}
    {page?.items.length === 0 && <p>没有符合条件的作品。</p>}
    {!page && !error && <p>作品列表暂不可用。</p>}
    <div className="wb-worlds">
      {page?.items.map((work) => <article key={work.work_id}>
        <h3><a href={`/workbench/knowledge/${encodeURIComponent(work.world_slug ?? work.work_id)}`}>{work.title}</a></h3>
        <p>{work.type ?? '未登记'} · {work.has_knowledge_version ? `${work.release_versions} 个发行版本` : '尚无知识版本'}{(work.open_conflicts ?? 0) > 0 ? ` · ${work.open_conflicts} 条待裁决冲突` : ''}</p>
        {(work.open_source_reviews ?? 0) > 0 && <p>来源待复核 · {work.open_source_reviews} 条</p>}
        <dl>
          <div><dt>本地分集覆盖</dt><dd>{work.episodes_total == null ? '—' : `${work.episodes_covered ?? 0} / ${work.episodes_total}`}</dd></div>
          <div><dt>线上已验证</dt><dd>{work.has_knowledge_version ? `${work.online_verified_episodes ?? 0} 集` : '—'}</dd></div>
          <div><dt>人物</dt><dd>{work.characters ?? '—'}</dd></div>
          <div><dt>场景</dt><dd>{work.scenes ?? '—'}</dd></div>
          <div><dt>图片</dt><dd>{work.images ?? '—'}</dd></div>
          <div><dt>验收通过率</dt><dd>{work.acceptance_pass_rate == null ? '—' : `${Math.round(work.acceptance_pass_rate * 100)}%`}</dd></div>
          <div><dt>生产状态</dt><dd>{productionStatusNames[work.production_status] ?? work.production_status}</dd></div>
        </dl>
        {work.has_knowledge_version && <CompletenessSummary dimensions={work.completeness?.dimensions ?? []} />}
        <div className="wb-knowledge-card-foot">
          <small>{work.last_updated_at ? `最后更新于 ${formatDateTime(work.last_updated_at)}` : '尚无更新记录'}</small>
          {work.world_slug && <span className="wb-knowledge-card-links">
            <a href={`/workbench/assistant?world=${encodeURIComponent(work.world_slug)}&prompt=${encodeURIComponent(`评估《${work.title}》的知识覆盖与主要缺口，给出补全顺序、任务量和预算依据。`)}`}>AI 评估</a>
            {' · '}
            <a href={`/workbench/operations?world=${encodeURIComponent(work.world_slug)}`}>运行拓扑</a>
          </span>}
        </div>
      </article>)}
    </div>
    {page?.next_cursor && <p><button type="button" disabled={loading} onClick={() => onLoadMore(page.next_cursor!)}>加载更多</button></p>}
  </section>
}

export function CompletenessSummary({ dimensions }: { dimensions: CompletenessDimension[] }) {
  if (!dimensions.length) return <p className="wb-muted">完整度等待质量扫描。</p>
  return <div className="wb-completeness-list">{dimensions.map((dimension) => <div key={dimension.key}>
    <strong>{dimensionNames[dimension.key] ?? dimension.key} · {statusNames[dimension.status] ?? dimension.status}</strong>
    {Object.entries(dimension.items).map(([code, item]) => <span key={code}>{itemNames[code] ?? code}：{item.data_state === 'no_data' ? '无数据' : `${item.completed_required_items ?? 0} / ${item.applicable_required_items ?? 0}`} · {statusNames[item.status ?? ''] ?? item.status ?? '无数据'}</span>)}
  </div>)}</div>
}

function sortLabel(sort: string) {
  const labels: Record<string, string> = { last_updated_at: '最后更新时间', title: '作品名', type: '类型', release_versions: '发行版本数', episodes_covered: '已覆盖集数', characters: '人物数', scenes: '场景数', images: '图片数', completeness: '完整度状态', acceptance_pass_rate: '验收通过率' }
  return labels[sort] ?? sort
}
