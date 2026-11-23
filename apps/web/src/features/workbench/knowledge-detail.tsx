import { useState } from 'react'
import { useRouter } from '@tanstack/react-router'
import type { JsonValue, KnowledgeCompleteness, KnowledgeEpisodeMatrixRow, KnowledgeWorkFeed, SourceReviewMark } from './feed'
import { api } from './editor-api'
import { CompletenessSummary } from './knowledge-center'
import { MediaGallery } from './media-gallery'
import { ScopeReport } from './scope-report'
import { formatDateTime } from './shared/format'

const dimensions = [
  ['overview', '总览与依据'], ['episodes', '分集矩阵'], ['characters', '人物'], ['storylines', '故事线'], ['scenes', '场景'],
  ['world-settings', '世界设定'], ['timeline', '时间线'], ['images', '图片'], ['quality-report', '质量与验收'],
] as const

const progressLabels: Record<KnowledgeEpisodeMatrixRow['state'], string> = {
  unregistered: '未登记', registered: '已登记', planned: '已规划',
  in_production: '生产中', accepted: '已验收', published: '已发布',
}

export function KnowledgeWorkDetail({ feed, candidate }: { feed: KnowledgeWorkFeed; candidate?: number }) {
  const [matrixFilter, setMatrixFilter] = useState<'all' | 'gaps' | 'issues'>('all')
  const detail = feed.detail
  if (!detail) return <section className="wb-panel wb-inventory" role="status"><h2>作品详情暂不可用</h2><p>请检查管理员接口和工作台会话。</p></section>
  const overview = detail.overview
  const completeness = overview.completeness as KnowledgeCompleteness | { dimensions: KnowledgeCompleteness['dimensions'] } | null
  const completenessDimensions = completeness?.dimensions ?? []
  const missing = completenessDimensions.flatMap((dimension) => Object.entries(dimension.items).flatMap(([code, item]) =>
    item.details.filter((entry) => entry.status === 'missing' || entry.status === 'pending_verification').map((entry) => ({ dimension: dimension.key, code, ...entry }))))
  const matrix = detail.episodes.matrix ?? []
  const filteredMatrix = matrix.filter((row) => {
    if (matrixFilter === 'gaps') return !row.local_published || !row.online_verified || row.acceptance_gaps.length > 0
    if (matrixFilter === 'issues') return Boolean(row.research_error || row.release_error || row.acceptance_passed === false || row.media.failed > 0 || row.media.held > 0)
    return true
  })
  const wvp = overview.working_vs_public
  const progress = detail.episodes.progress
  const evidence = overview.world_evidence ?? []
  const conflicts = overview.conflicts ?? []
  const sourceReviews = overview.source_reviews ?? []
  const mediaList = feed.media ?? []
  const mediaPool = {
    active: mediaList.filter((m) => ['reference', 'queued', 'generate', 'review', 'recrop', 'recheck'].includes(m.stage)).length,
    passed: mediaList.filter((m) => m.stage === 'passed').length,
    held: mediaList.filter((m) => m.stage === 'held').length,
    failed: mediaList.filter((m) => m.stage === 'failed').length,
  }
  const slug = overview.world_slug
  const evaluateHref = (prompt: string, episode?: number) => {
    if (!slug) return ''
    const params = new URLSearchParams({ world: slug, prompt })
    if (episode) params.set('episode', String(episode))
    return '/workbench/assistant?' + params.toString()
  }

  return <div className="wb-report-stack">
    <nav className="wb-knowledge-nav" aria-label="作品知识分区">{dimensions.map(([id, label]) => <a key={id} href={`#${id}`}>{label}</a>)}</nav>
    {!overview.has_knowledge_version && <p className="wb-panel wb-inventory" role="status">尚无知识版本，当前不显示完整度与发布资产。</p>}

    <section className="wb-panel wb-inventory" id="overview">
      <div className="wb-knowledge-header">
        <div>
          <h2>{overview.title}</h2>
          <p>
            {stringValue(overview.type) ?? '未登记类型'} · {stringValue(overview.edition) ?? '尚无发行版本'} ·{' '}
            {overview.origin === 'original' ? '原创策划' : overview.origin === 'external' ? '外部来源整理' : '目录档案'} ·{' '}
            <code>{overview.work_id}</code>
            {slug && <> · <code>{slug}</code></>}
          </p>
        </div>
        {slug && <div className="wb-knowledge-actions">
          <a className="wb-button" href={evaluateHref(`评估《${overview.title}》前 10 话的知识覆盖，解释主要缺口，给出补全顺序、任务量和预算依据。`)}>用 AI 评估并规划补全</a>
          <a className="wb-link-pill" href={`/workbench/operations?world=${encodeURIComponent(slug)}`}>查看运行拓扑 →</a>
          <a className="wb-link-pill" href="/workbench/projects">查看制作计划 →</a>
          <a className="wb-link-pill" href={`/work/${encodeURIComponent(slug)}`} target="_blank" rel="noreferrer">公开作品页 ↗</a>
        </div>}
      </div>

      {wvp && overview.has_knowledge_version && <div className="wb-wvp-strip" aria-label="工作态与公开版差异">
        <span><small>当前公开集数</small><strong>{wvp.published_episodes}</strong></span>
        <span><small>线上已验证集数</small><strong>{wvp.online_verified_episodes}</strong></span>
        <span><small>本地已验收未公开</small><strong>{wvp.local_accepted_unpublished}</strong></span>
        <span><small>线上投递失败</small><strong>{wvp.release_failed_episodes}</strong></span>
        <span><small>活跃制作计划</small><strong>{wvp.active_plans}</strong></span>
        <span><small>在途编辑任务</small><strong>{wvp.open_editor_tasks}</strong></span>
      </div>}

      {overview.has_knowledge_version && <>
        <h3>知识完整度分项</h3>
        <CompletenessSummary dimensions={completenessDimensions} />
        <h3>缺项与待验证</h3>
        {missing.length ? <ul>{missing.map((item) => <li key={`${item.dimension}-${item.code}-${item.item_key}`}>
          <strong>{dimensionLabel(item.dimension)}</strong> · {item.item_key} · {item.reason ?? '待验证'}
          {item.evidence_ref && <> · <code className="wb-knowledge-evidence-ref">{item.evidence_ref}</code></>}
        </li>)}</ul> : <p>当前完整度结果没有缺项。</p>}
        {!completenessDimensions.length && <p>完整度等待质量扫描。</p>}
      </>}

      {sourceReviews.length > 0 && <section className="wb-evidence-box wb-source-reviews" id="source-review" aria-labelledby="source-review-title">
        <h3 id="source-review-title">来源待复核 · {sourceReviews.length} 条</h3>
        {sourceReviews.map((mark) => <SourceReviewItem key={mark.id} mark={mark} />)}
      </section>}

      <details className="wb-evidence-box" open={conflicts.length > 0}>
        <summary>身份来源、证据与待裁决冲突（外部 ID {overview.sources.external_ids.length} · 来源记录 {overview.sources.records.length} · 世界依据 {evidence.length} · 冲突 {conflicts.length}）</summary>
        <div className="wb-worlds">
          <article>
            <h4>外部标识与活跃来源记录</h4>
            {overview.sources.external_ids.length ? <ul>{overview.sources.external_ids.map((ext) => <li key={`${ext.provider}-${ext.external_id}`}>
              <strong>{ext.provider}</strong> · {ext.url ? <a href={ext.url} target="_blank" rel="noreferrer">{ext.external_id} ↗</a> : ext.external_id}
            </li>)}</ul> : <p>无外部 provider ID。</p>}
            {overview.sources.records.length ? <ul>{overview.sources.records.map((rec) => <li key={`${rec.source}-${rec.record_key}`}>
              {rec.source}（{rec.type}）· <code>{rec.record_key}</code> · v{rec.version_id}
            </li>)}</ul> : <p>暂无活跃来源记录。</p>}
          </article>
          <article>
            <h4>作品世界依据（world_evidence）</h4>
            {evidence.length ? <ul>{evidence.map((ev) => <li key={ev.key}>
              <strong>{ev.key}</strong> · {ev.locator} · {ev.uri.startsWith('http') ? <a href={ev.uri} target="_blank" rel="noreferrer">{ev.uri} ↗</a> : <code>{ev.uri}</code>}
            </li>)}</ul> : <p>当前版本暂无 world_evidence 记录。</p>}
          </article>
          <article>
            <h4>待裁决来源冲突</h4>
            {conflicts.length ? <ul>{conflicts.map((c) => <li key={`${c.source_record_id}-${c.candidate_work_id}-${c.reason}`}>
              <strong>{c.source}</strong> · <code>{c.record_key}</code> 疑似匹配 {c.candidate_title}（{c.reason} · {formatDateTime(c.first_seen_at)}）
            </li>)}</ul> : <p>当前无待裁决的身份冲突候选。</p>}
          </article>
        </div>
      </details>
    </section>

    <section className="wb-panel wb-inventory" id="episodes">
      <div className="wb-knowledge-header">
        <div>
          <h2>分集范围覆盖矩阵</h2>
          <p>
            {detail.episodes.applicable === false
              ? '当前作品为非分集类型（电影或短篇集），分集覆盖维度不适用。'
              : `声明总集数：${detail.episodes.total ?? '未登记'} · 本地可读：${detail.episodes.readable.length} 集 · 线上已验证：${progress?.online_verified ?? 0} 集`}
          </p>
        </div>
        {detail.episodes.applicable !== false && matrix.length > 0 && <div className="wb-knowledge-actions">
          <button type="button" className={matrixFilter === 'all' ? 'wb-pill-active' : 'wb-link-pill'} onClick={() => setMatrixFilter('all')}>全部（{matrix.length}）</button>
          <button type="button" className={matrixFilter === 'gaps' ? 'wb-pill-active' : 'wb-link-pill'} onClick={() => setMatrixFilter('gaps')}>仅未完成/未验证</button>
          <button type="button" className={matrixFilter === 'issues' ? 'wb-pill-active' : 'wb-link-pill'} onClick={() => setMatrixFilter('issues')}>仅失败/缺项</button>
        </div>}
      </div>

      {progress && progress.scope_total > 0 && <div className="wb-wvp-strip" aria-label="策略范围六态进度">
        <span><small>策略范围（去重）</small><strong>{progress.scope_total}</strong></span>
        <span><small>未登记</small><strong>{progress.unregistered}</strong></span>
        <span><small>已登记</small><strong>{progress.registered}</strong></span>
        <span><small>已规划</small><strong>{progress.planned}</strong></span>
        <span><small>生产中</small><strong>{progress.in_production}</strong></span>
        <span><small>已验收</small><strong>{progress.accepted}</strong></span>
        <span><small>已发布</small><strong>{progress.published}</strong></span>
      </div>}

      {detail.episodes.applicable !== false && filteredMatrix.length > 0 ? <div className="wb-matrix-wrap">
        <table className="wb-matrix-table">
          <thead>
            <tr>
              <th>集号</th>
              <th>策略范围</th>
              <th>六态进度</th>
              <th>取证/生产</th>
              <th>五维验收</th>
              <th>图片候选</th>
              <th>发布与线上核验</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {filteredMatrix.map((row) => <tr key={row.number}>
              <td><strong>第 {row.number} 集</strong>{row.title ? <small>{row.title}</small> : null}{row.source_review_pending && <small><a href="#source-review">来源待复核</a></small>}</td>
              <td>{row.in_policy ? '策略内' : '策略外'}</td>
              <td>{progressLabels[row.state]}</td>
              <td>{row.research_stage ? `${row.research_stage}${row.research_error ? ` · ${row.research_error}` : ''}` : '—'}</td>
              <td>
                {row.acceptance_passed === true ? '通过' : row.acceptance_passed === false ? '未通过' : '未检查'}
                {row.acceptance_gaps.length > 0 && <small className="wb-gap-list">{row.acceptance_gaps.join('；')}</small>}
              </td>
              <td>
                {row.media.total === 0 ? '—' : `共 ${row.media.total}（过 ${row.media.passed}${row.media.active ? ` / 跑 ${row.media.active}` : ''}${row.media.held ? ` / 返工 ${row.media.held}` : ''}${row.media.failed ? ` / 败 ${row.media.failed}` : ''}）`}
              </td>
              <td>
                {row.online_verified ? <>线上已验证{row.release_receipt ? ` #${row.release_receipt}` : ''}{row.release_href && <> · <a href={row.release_href} target="_blank" rel="noreferrer">查看 ↗</a></>}</>
                  : row.release_error ? <>投递失败：{row.release_error}</>
                  : row.local_published ? '仅本地已发布（待投递正式站）'
                  : '未发布'}
              </td>
              <td>
                {slug && <a href={evaluateHref(`评估《${overview.title}》第 ${row.number} 集的知识覆盖、缺口依据及补全方案。`, row.number)}>AI 评估</a>}
              </td>
            </tr>)}
          </tbody>
        </table>
      </div> : detail.episodes.applicable !== false ? <p>暂无可展示的分集矩阵记录。</p> : null}
    </section>

    <section className="wb-panel wb-inventory" id="characters"><h2>人物</h2>
      {detail.characters.length ? detail.characters.map((person) => <article key={person.slug}><h3>{person.name ?? person.slug}</h3>
        <p>{person.role ?? '未登记角色'} · {person.introduction ? '已登记独立人物简介' : '缺少独立人物简介'}{person.evidence ? ` · 依据 ${person.evidence}` : ''}</p>
        {person.introduction && <p>{person.introduction}</p>}
      </article>) : <p>暂无发布人物。</p>}
    </section>

    <section className="wb-panel wb-inventory" id="storylines"><h2>故事线</h2>
      {detail.storylines.length ? detail.storylines.map((line) => <article key={line.slug}><h3>{line.title}</h3><p>{line.intro}</p>
        <ul>{line.chapters.map((chapter) => <li key={chapter.slug}>{chapter.title}</li>)}</ul></article>) : <p>暂无发布故事线。</p>}
    </section>

    <section className="wb-panel wb-inventory" id="scenes"><h2>场景</h2>
      {detail.scenes.length ? detail.scenes.map((scene) => <article key={scene.slug}><h3>{scene.name}</h3>
        <p>{scene.chapter} · {scene.place}</p>{scene.text ? <p>{scene.text}</p> : <p>缺少独立场景文本。</p>}
      </article>) : <p>暂无发布场景。</p>}
    </section>

    <section className="wb-panel wb-inventory" id="world-settings"><h2>世界设定</h2>
      <h3>地点</h3><ul>{detail.world_settings.locations.map((location) => <li key={location.name}>{location.name}</li>)}</ul>
      <p>组织：无数据（尚未建表，不计入分母）</p><p>设定类型：无数据（尚未建表，不计入分母）</p>
    </section>

    <section className="wb-panel wb-inventory" id="timeline"><h2>时间线</h2><p>事件结构、关联章节和顺序校验暂无数据（不以集号伪造剧情时间线）。</p></section>

    <section className="wb-panel wb-inventory" id="images">
      <h2>图片资产</h2>
      <p className="wb-muted">严格区分「当前公开生效引用」与「生产候选池（审查通过 ≠ 已入选/已发布）」。</p>
      <div className="wb-wvp-strip" aria-label="图片四态分布">
        <span><small>公开主视觉</small><strong>{detail.images.main_visual ? '1 / 1' : '0 / 1'}</strong></span>
        <span><small>公开人物立绘</small><strong>{detail.images.portraits.filter((person) => person.image).length} / {detail.images.portraits.length}</strong></span>
        <span><small>公开章节配图</small><strong>{detail.images.chapters.filter((chapter) => chapter.image).length} / {detail.images.chapters.length}</strong></span>
        <span><small>候选审查通过</small><strong>{mediaPool.passed}</strong></span>
        <span><small>候选生产中</small><strong>{mediaPool.active}</strong></span>
        <span><small>候选待返工/失败</small><strong>{mediaPool.held + mediaPool.failed}</strong></span>
      </div>
      <MediaGallery key={`${overview.world_slug}-${candidate ?? ''}`} items={feed.media} candidate={candidate} />
    </section>

    <section className="wb-report-stack" id="quality-report">
      <div className="wb-panel wb-inventory">
        <h2>目录字段质量（基础档案扫描）</h2>
        <p className="wb-muted">仅检查目录基础字段完整性与规范性，不代表作品世界剧情或设定的事实正确性。</p>
        {detail.quality_report.score ? <p>最近目录质量扫描 {detail.quality_report.score.score} 分 · {formatDateTime(detail.quality_report.score.checked_at)}</p> : <p>尚无目录质量评分记录。</p>}
        {detail.quality_report.open_issues.length ? <ul>{detail.quality_report.open_issues.map((issue) => <li key={`${issue.rule_id}-${issue.field}`}>{issue.field} · {issue.reason}</li>)}</ul> : <p>没有未处理的目录质量问题。</p>}
      </div>
      <div className="wb-panel wb-inventory">
        <p className="wb-muted">以下为基于来源快照与验图回执的分集五维验收，与上方目录字段质量相互独立。</p>
        <p>{detail.acceptance_pass_rate == null ? '暂无五维验收通过率。' : `五维验收通过率 ${Math.round(detail.acceptance_pass_rate * 100)}%`}</p>
        <ScopeReport scopes={feed.scopes} />
      </div>
    </section>
  </div>
}

const sourceChangeLabels = { added: '新增来源', restored: '来源已恢复', updated: '来源有更新', withdrawn: '来源已撤回' }
const sourceFieldLabels: Record<string, string> = {
  active: '条目状态', type: '作品类型', title: '标题', original: '原名', episodes: '总集数',
  status: '播出状态', year: '年份', synopsis: '简介', refs: '来源标识', titles: '别名',
}

function SourceReviewItem({ mark }: { mark: SourceReviewMark }) {
  const router = useRouter()
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const proposed = Object.entries(mark.changes.catalog)
  const fields = Object.entries(mark.changes.fields)

  async function resolve() {
    setSaving(true)
    setError('')
    try {
      await api(`source-reviews/${mark.id}/resolve`, { last_run_id: mark.last_run_id })
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '暂时无法保存核对结果。请重试。')
      setSaving(false)
      return
    }
    try {
      await router.invalidate()
    } finally {
      setSaving(false)
    }
  }

  return <article className="wb-source-review">
    <div className="wb-knowledge-actions">
      <strong>{sourceChangeLabels[mark.changes.kind]} · {mark.number == null ? '整部作品' : `第 ${mark.number} 集`}</strong>
      <button type="button" className="wb-link-pill" disabled={saving} onClick={() => void resolve()}>{saving ? '正在保存…' : '确认已核对'}</button>
    </div>
    <p>{mark.source} · {mark.record_key} · {formatDateTime(mark.updated_at)}</p>
    {proposed.length > 0 && <p>待确认的目录变更 · {proposed.map(([field, change]) =>
      `${sourceFieldLabels[field] ?? field} ${sourceValue(change.before)} → ${sourceValue(change.after)}`).join('，')}</p>}
    <details>
      <summary>查看来源差异</summary>
      <p>来源版本 {mark.previous_version_id ?? '—'} → {mark.source_version_id}</p>
      <ul>
        {fields.map(([field, change]) => <li key={field}>{sourceFieldLabels[field] ?? field} · {sourceValue(change.before)} → {sourceValue(change.after)}</li>)}
        <li>原始记录摘要 · {mark.changes.raw.before ?? '—'} → {mark.changes.raw.after ?? '—'}</li>
        <li>规范记录摘要 · {mark.changes.norm.before ?? '—'} → {mark.changes.norm.after ?? '—'}</li>
      </ul>
    </details>
    {error && <p role="alert">{error}</p>}
  </article>
}

function sourceValue(value: JsonValue) {
  if (value == null) return '—'
  if (typeof value === 'boolean') return value ? '活跃' : '停用'
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return text.length > 180 ? text.slice(0, 180) + '…' : text
}

function stringValue(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null
}

function dimensionLabel(key: string) {
  return ({ basic_profile: '基础档案', people: '人物知识', plot: '剧情知识', scenes: '场景知识', world: '世界观', timeline: '时间线', images: '图片资产' } as Record<string, string>)[key] ?? key
}
