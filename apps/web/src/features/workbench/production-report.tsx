import type { ProductionUsage } from '@anime/shared/knowledge'

const labels: Record<string, string> = {
  'content:collect': '取证',
  'content:write': '正文写作',
  'content:review': '正文复核',
  'image:edit': '原图生成',
  'image:reference': '参考图生成',
  'vision:review': '验图',
  'vision:second-review': '二次验图',
}
const number = new Intl.NumberFormat('zh-CN')
const duration = (start: string | null, end: string | null) =>
  start && end ? number.format(Math.max(0, Math.round((Date.parse(end) - Date.parse(start)) / 1000))) + ' 秒' : '—'

export function ProductionReport({ works }: { works: ProductionUsage[] | null }) {
  return (
    <section className="wb-inventory" id="production-usage">
      <h2>作品制作 · 用量与耗时</h2>
      <p>按回执去重，包含失败和返工。模型耗时按请求累计，端到端耗时包含调度和验收等待。</p>
      {works === null && <p role="status">用量记录暂不可用。</p>}
      {works?.length === 0 && <p>等待选题登记。</p>}
      {works?.map((work) => (
        <article key={work.world}>
          <h3><a href={'/work/' + work.world}>{work.title} · {work.edition}</a></h3>
          <p>
            {work.published_at ? '已发布' : work.started_at ? '制作中' : '待调度'}
            {' · 调度到发布 '}{duration(work.started_at, work.published_at)}
            {' · 末次验图到发布 '}{duration(work.last_media_at, work.published_at)}
            {' · 取证返工 '}{work.failed_research} 次
          </p>
          {work.episodes?.map((episode) => (
            <details key={episode.number} open={episode.number === work.episodes?.at(-1)?.number}>
              <summary>
                第 {episode.number} 集
                {episode.source_number ? ' · 全剧第 ' + episode.source_number + ' 话' : ''}
                {' · '}{episode.published_at ? '已发布' : episode.started_at ? '制作中' : '待调度'}
                {' · 调度到发布 '}{duration(episode.started_at, episode.published_at)}
                {episode.failed_research ? ' · 取证返工 ' + episode.failed_research + ' 次' : ''}
              </summary>
              <Operations operations={work.operations.filter((step) => step.number === episode.number)} />
            </details>
          ))}
          {work.episodes ? (
            <details>
              <summary>作品共享肖像</summary>
              <Operations operations={work.operations.filter((step) => step.number == null)} />
            </details>
          ) : (
            <Operations operations={work.operations} />
          )}
        </article>
      ))}
    </section>
  )
}

function Operations({ operations }: { operations: ProductionUsage['operations'] }) {
  if (!operations.length) return <p>暂无模型回执。</p>
  return (
    <ul>
      {operations.map((step) => (
        <li key={step.purpose + ':' + step.operation + ':' + step.model}>
          <strong>{labels[step.purpose + ':' + step.operation] ?? step.operation}</strong>
          {' · '}{step.model}{' · '}{step.calls} 次
          {' · '}{step.tokens === null ? '用量未报告' : number.format(step.tokens) + ' token'}
          {' · '}{step.seconds === null ? '耗时未报告' : number.format(step.seconds) + ' 秒'}
          {step.images ? ' · ' + step.images + ' 张' : ''}
          {step.errors ? ' · ' + step.errors + ' 次失败' : ''}
          {step.unmetered ? ' · ' + step.unmetered + ' 条未计量' : ''}
        </li>
      ))}
    </ul>
  )
}
