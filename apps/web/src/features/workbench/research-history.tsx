import type { ResearchRun } from '@anime/shared/knowledge'
import { formatClock, formatDate } from './shared/format'

const stage = {
  collect: '联网取证中',
  write: '生成导读中',
  ready: '导读可预览',
  failed: '本次取证失败',
}

export function ResearchHistory({ runs }: { runs: ResearchRun[] | null }) {
  return (
    <section
      className="wb-inventory wb-research"
      aria-labelledby="research-title"
    >
      <h2 id="research-title">分集取证过程</h2>
      {runs === null && <p role="status">取证记录暂不可用。</p>}
      {runs?.length === 0 && <p>尚无选题运行记录。</p>}
      {runs?.map((r, index) => (
        <details key={r.id} open={index === 0}>
          <summary>
            {r.edition_title ?? r.world_slug} · 第 {r.first_episode}–
            {r.last_episode} 集 ·{' '}
            {r.current_policy
              ? stage[r.stage]
              : '历史记录 · ' +
                (r.stage === 'ready' ? '旧版导读' : stage[r.stage])}
          </summary>
          <p>
            知识版本 {r.version_id} · 取证规则 v{r.policy} · 第 {r.attempt}{' '}
            次执行 · {formatDate(r.started_at)}{r.model && ' · ' + r.model}
          </p>
          <ol>
            {r.events?.map((e, i) => (
              <li key={i}>
                <time>{utc(e.at)}</time> {e.message}
              </li>
            ))}
          </ol>
          {!!r.documents?.length && (
            <details>
              <summary>查看来源快照索引</summary>
              <ul>
                {r.documents.map((d) => (
                  <li key={d.id}>
                    <a href={d.url}>{d.locator} ↗</a> · {formatClock(d.fetched_at)}{' '}
                    <code title={d.sha256}>{d.sha256.slice(0, 12)}</code>
                  </li>
                ))}
              </ul>
            </details>
          )}
          {r.stage === 'ready' && r.current_policy && (
            <a href={'/work/' + r.world_slug}>打开作品世界，阅读分集导读 →</a>
          )}
        </details>
      ))}
    </section>
  )
}
