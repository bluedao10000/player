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

export function KnowledgeCenter({ summary, works }: { summary: KnowledgeSummary | null; works: KnowledgeWorksPage | null }) {
  return <div className="wb-report-stack">
    <SummaryPanel summary={summary} />
    <WorkList initial={works} types={summary?.works.by_type ?? []} />
  </div>
}

function SummaryPanel({ summary }: { summary: KnowledgeSummary | null }) {
  if (!summary) return <section className="wb-panel wb-inventory" role="status"><h2>知识资产摘要暂不可用</h2><p>无法读取管理接口返回的数据。</p></section>
  const updated = formatDateTime(summary.as_of)
  const metrics = [
    ['作品总数', summary.works.active_total, '活跃目录作品', 'works.active = true'],
    ['已发布作品世界数', summary.works.published_worlds, '当前公开知识版本', 'published_worlds 行数'],
    ['发行版本数', summary.works.release_versions, '不可变发布版本', 'world_releases 行数'],
    ['人物', summary.people.total, '当前发布版本中的人物', '公开人物库存'],
    ['人物介绍', summary.people.introductions_complete == null ? '—' : `${summary.people.introductions_complete} / ${summary.people.total}`, '独立人物简介完整数 / 人物数', '完整度扫描结果'],
    ['故事线', summary.plot.storylines, '当前公开故事线', '公开知识库存'],
    ['章节', summary.plot.chapters, '当前公开章节', '公开知识库存'],
    ['分集覆盖', `${summary.plot.episodes_covered} / ${summary.plot.episodes_expected}`, '已覆盖集数 / 声明总集数', '连续剧分集库存'],
    ['场景', summary.plot.scenes, '当前公开场景', '公开知识库存'],
    ['地点', summary.world.locations, '当前公开地点', '公开知识库存'],
    ['主视觉', summary.images.main_visuals, '已发布主视觉引用', '公开资产引用'],
    ['人物立绘', summary.images.portraits, '已发布人物立绘引用', '公开资产引用'],
    ['章节配图', summary.images.chapter_images, '已发布章节配图引用', '公开资产引用'],
  ] as const
  return <>
    <section className="wb-metrics" aria-label="知识资产统计">
      {metrics.map(([title, value, detail, definition]) => <Panel className="wb-metric wb-tone-cyan" key={title}>
        <div className="wb-metric-top"><span className="wb-eyebrow">{title}</span></div>
        <strong>{typeof value === 'number' ? value.toLocaleString('zh-CN') : value}</strong>
        <p>{detail}</p>
        <small>口径 v{summary.definition} · {definition} · 更新于 {updated}</small>
      </Panel>)}
    </section>
    <section className="wb-panel wb-overview-types" aria-labelledby="knowledge-types-title">
      <SectionHeading id="knowledge-types-title" eyebrow="WORK TYPES" title="作品类型分布" detail="按 works.type 原值分组；NULL 单列为未登记，UNKNOWN 保留原值。" />
      <ul className="wb-type-distribution">
        {summary.works.by_type.map((row) => <li key={row.type ?? 'unregistered'}>
          <span>{row.type === null ? '未登记' : row.type}</span><strong>{row.count.toLocaleString('zh-CN')}</strong>
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

function WorkList({ initial, types }: { initial: KnowledgeWorksPage | null; types: { type: string | null; count: number }[] }) {
  const [page, setPage] = useState(initial)
  const [filters, setFilters] = useState<KnowledgeWorkFilters>({
    q: '', has_knowledge_version: 'true', type: '', completeness_status: '', production_status: '', sort: 'last_updated_at', after: '',
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  async function load(event?: FormEvent, after = '') {
    event?.preventDefault()
    setLoading(true); setError('')
    try {
      const result = await readKnowledgeWorksPage({ data: { ...filters, after } })
      if (!result) setError('作品列表暂不可用。')
      else setPage((current) => after ? { ...result, items: [...(current?.items ?? []), ...result.items] } : result)
    } catch { setError('作品列表暂不可用。') }
    finally { setLoading(false) }
  }
  function update<K extends keyof KnowledgeWorkFilters>(key: K, value: KnowledgeWorkFilters[K]) {
    setFilters((current) => ({ ...current, [key]: value }))
  }
  return <section className="wb-panel wb-inventory" aria-labelledby="knowledge-works-title">
    <SectionHeading id="knowledge-works-title" eyebrow="WORK CATALOG" title="作品列表" detail="默认显示有已发布知识版本的作品；可切换查看全部活跃目录。" />
    <form className="wb-media-filters" onSubmit={(event) => void load(event)}>
      <label>搜索<input value={filters.q} onChange={(event) => update('q', event.target.value)} placeholder="作品名或别名" /></label>
      <label>目录范围<select value={filters.has_knowledge_version} onChange={(event) => update('has_knowledge_version', event.target.value as KnowledgeWorkFilters['has_knowledge_version'])}>
        <option value="true">有知识版本</option><option value="all">全部活跃目录</option><option value="false">尚无知识版本</option>
      </select></label>
      <label>作品类型<select value={filters.type} onChange={(event) => update('type', event.target.value)}>
        <option value="">全部类型</option>{types.map((row) => <option key={row.type ?? '__null__'} value={row.type ?? '__null__'}>{row.type ?? '未登记'}</option>)}
      </select></label>
      <label>完整度状态<select value={filters.completeness_status} onChange={(event) => update('completeness_status', event.target.value)}>
        <option value="">全部状态</option>{['completed','partial','missing','pending_verification'].map((status) => <option key={status} value={status}>{statusNames[status]}</option>)}
      </select></label>
      <label>生产状态<select value={filters.production_status} onChange={(event) => update('production_status', event.target.value)}>
        <option value="">全部状态</option>{['idle','pending','blocked','waiting_human'].map((status) => <option key={status}>{status}</option>)}
      </select></label>
      <label>排序<select value={filters.sort} onChange={(event) => update('sort', event.target.value)}>
        {['last_updated_at','title','type','release_versions','episodes_covered','characters','scenes','images','completeness','acceptance_pass_rate'].map((sort) => <option key={sort} value={sort}>{sortLabel(sort)}</option>)}
      </select></label>
      <button type="submit" disabled={loading}>{loading ? '读取中…' : '筛选作品'}</button>
    </form>
    {error && <p role="status">{error}</p>}
    {page?.items.length === 0 && <p>没有符合条件的作品。</p>}
    {!page && !error && <p>作品列表暂不可用。</p>}
    <div className="wb-worlds">
      {page?.items.map((work) => <article key={work.work_id}>
        <h3><a href={`/workbench/knowledge/${encodeURIComponent(work.world_slug ?? work.work_id)}`}>{work.title}</a></h3>
        <p>{work.type ?? '未登记'} · {work.has_knowledge_version ? `${work.release_versions} 个发行版本` : '尚无知识版本'}</p>
        <dl>
          <div><dt>分集</dt><dd>{work.episodes_total == null ? '—' : `${work.episodes_covered ?? 0} / ${work.episodes_total}`}</dd></div>
          <div><dt>人物</dt><dd>{work.characters ?? '—'}</dd></div>
          <div><dt>场景</dt><dd>{work.scenes ?? '—'}</dd></div>
          <div><dt>图片</dt><dd>{work.images ?? '—'}</dd></div>
          <div><dt>验收通过率</dt><dd>{work.acceptance_pass_rate == null ? '—' : `${Math.round(work.acceptance_pass_rate * 100)}%`}</dd></div>
          <div><dt>生产状态</dt><dd>{statusNames[work.production_status] ?? work.production_status}</dd></div>
        </dl>
        {work.has_knowledge_version && <CompletenessSummary dimensions={work.completeness?.dimensions ?? []} />}
        <small>{work.last_updated_at ? `最后更新于 ${formatDateTime(work.last_updated_at)}` : '尚无更新记录'}</small>
      </article>)}
    </div>
    {page?.next_cursor && <p><button type="button" disabled={loading} onClick={() => void load(undefined, page.next_cursor!)}>加载更多</button></p>}
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
