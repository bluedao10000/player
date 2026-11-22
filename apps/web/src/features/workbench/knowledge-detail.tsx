import type { KnowledgeCompleteness, KnowledgeWorkFeed } from './feed'
import { CompletenessSummary } from './knowledge-center'
import { MediaGallery } from './media-gallery'
import { ScopeReport } from './scope-report'

const dimensions = [
  ['overview', '总览'], ['characters', '人物'], ['storylines', '故事线'], ['episodes', '分集'], ['scenes', '场景'],
  ['world-settings', '世界设定'], ['timeline', '时间线'], ['images', '图片'], ['quality-report', '质量报告'],
] as const

export function KnowledgeWorkDetail({ feed, candidate }: { feed: KnowledgeWorkFeed; candidate?: number }) {
  const detail = feed.detail
  if (!detail) return <section className="wb-panel wb-inventory" role="status"><h2>作品详情暂不可用</h2><p>请检查管理员接口和工作台会话。</p></section>
  const overview = detail.overview
  const completeness = overview.completeness as KnowledgeCompleteness | { dimensions: KnowledgeCompleteness['dimensions'] } | null
  const completenessDimensions = completeness?.dimensions ?? []
  const missing = completenessDimensions.flatMap((dimension) => Object.entries(dimension.items).flatMap(([code, item]) =>
    item.details.filter((entry) => entry.status === 'missing' || entry.status === 'pending_verification').map((entry) => ({ dimension: dimension.key, code, ...entry }))))
  return <div className="wb-report-stack">
    <nav className="wb-knowledge-nav" aria-label="作品知识分区">{dimensions.map(([id, label]) => <a key={id} href={`#${id}`}>{label}</a>)}</nav>
    {!overview.has_knowledge_version && <p className="wb-panel wb-inventory" role="status">尚无知识版本，当前不显示完整度。</p>}
    <section className="wb-panel wb-inventory" id="overview">
      <h2>{overview.title}</h2>
      <p>{stringValue(overview.type) ?? '未登记'} · {stringValue(overview.edition) ?? '尚无发行版本'} · {overview.world_slug ?? '尚无知识版本'}</p>
      {overview.has_knowledge_version && <>
        <h3>知识完整度分项</h3>
        <CompletenessSummary dimensions={completenessDimensions} />
        <h3>缺项与待验证</h3>
        {missing.length ? <ul>{missing.map((item) => <li key={`${item.dimension}-${item.code}-${item.item_key}`}>
          {dimensionLabel(item.dimension)} · {item.item_key} · {item.reason ?? '待验证'}
        </li>)}</ul> : <p>当前完整度结果没有缺项。</p>}
        {!completenessDimensions.length && <p>完整度等待质量扫描。</p>}
      </>}
    </section>
    <section className="wb-panel wb-inventory" id="characters"><h2>人物</h2>
      {detail.characters.length ? detail.characters.map((person) => <article key={person.slug}><h3>{person.name ?? person.slug}</h3>
        <p>{person.role ?? '未登记角色'} · {person.introduction ? '已登记独立人物简介' : '缺少独立人物简介'}</p>
        {person.introduction && <p>{person.introduction}</p>}
      </article>) : <p>暂无发布人物。</p>}
    </section>
    <section className="wb-panel wb-inventory" id="storylines"><h2>故事线</h2>
      {detail.storylines.length ? detail.storylines.map((line) => <article key={line.slug}><h3>{line.title}</h3><p>{line.intro}</p>
        <ul>{line.chapters.map((chapter) => <li key={chapter.slug}>{chapter.title}</li>)}</ul></article>) : <p>暂无发布故事线。</p>}
    </section>
    <section className="wb-panel wb-inventory" id="episodes"><h2>分集</h2>
      <p>总集数：{detail.episodes.total ?? '未登记'} · 已覆盖：{detail.episodes.readable.length}</p>
      <p>已覆盖集号：{detail.episodes.readable.length ? detail.episodes.readable.join('、') : '暂无'}</p>
      {detail.episodes.scopes.map((scope) => <p key={scope.topic}>{scope.topic} · 第 {scope.first}–{scope.last} 集 · {scope.passed == null ? '尚无验收结果' : scope.passed ? '通过' : '未通过'}</p>)}
    </section>
    <section className="wb-panel wb-inventory" id="scenes"><h2>场景</h2>
      {detail.scenes.length ? detail.scenes.map((scene) => <article key={scene.slug}><h3>{scene.name}</h3>
        <p>{scene.chapter} · {scene.place}</p>{scene.text ? <p>{scene.text}</p> : <p>缺少独立场景文本。</p>}
      </article>) : <p>暂无发布场景。</p>}
    </section>
    <section className="wb-panel wb-inventory" id="world-settings"><h2>世界设定</h2>
      <h3>地点</h3><ul>{detail.world_settings.locations.map((location) => <li key={location.name}>{location.name}</li>)}</ul>
      <p>组织：无数据</p><p>设定类型：无数据</p>
    </section>
    <section className="wb-panel wb-inventory" id="timeline"><h2>时间线</h2><p>事件结构、关联章节和顺序校验暂无数据。</p></section>
    <section className="wb-panel wb-inventory" id="images"><h2>图片</h2>
      <p>主视觉：{detail.images.main_visual ? '已登记' : '缺失'}</p>
      <p>人物立绘：{detail.images.portraits.filter((person) => person.image).length} / {detail.images.portraits.length}</p>
      <p>章节配图：{detail.images.chapters.filter((chapter) => chapter.image).length} / {detail.images.chapters.length}</p>
      <MediaGallery key={`${overview.world_slug}-${candidate ?? ''}`} items={feed.media} candidate={candidate} />
    </section>
    <section className="wb-report-stack" id="quality-report"><div className="wb-panel wb-inventory"><h2>质量报告</h2>
      {detail.quality_report.score ? <p>最近质量扫描 {detail.quality_report.score.score} 分 · {detail.quality_report.score.checked_at}</p> : <p>尚无质量评分记录。</p>}
      {detail.quality_report.open_issues.length ? <ul>{detail.quality_report.open_issues.map((issue) => <li key={`${issue.rule_id}-${issue.field}`}>{issue.field} · {issue.reason}</li>)}</ul> : <p>没有未处理的质量问题。</p>}
    </div><ScopeReport scopes={feed.scopes} /></section>
  </div>
}

function stringValue(value: unknown): string | null {
  return typeof value === 'string' && value !== '' ? value : null
}

function dimensionLabel(key: string) {
  return ({ basic_profile: '基础档案', people: '人物知识', plot: '剧情知识', scenes: '场景知识', world: '世界观', timeline: '时间线', images: '图片资产' } as Record<string, string>)[key] ?? key
}
