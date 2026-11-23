import type { ScopeAcceptance } from '@anime/shared/knowledge'

export function ScopeReport({ scopes }: { scopes: ScopeAcceptance[] | null }) {
  return (
    <section className="wb-inventory" id="scope-acceptance">
      <h2>范围验收</h2>
      {scopes === null && <p role="status">验收记录暂不可用。</p>}
      {scopes?.length === 0 && <p>等待数据服务检查已选范围。</p>}
      {scopes?.map((run) => {
        const gaps = run.checks.flatMap((c) =>
          c.items
            .filter((i) => i.gap)
            .map((i) => c.name + ' · ' + i.label + '：' + i.gap),
        )
        return (
          <article key={run.id}>
            <h3>
              <a href={'/work/' + run.world}>
                {run.edition_title ?? run.world} · 第{' '}
                {run.first === run.last
                  ? run.first
                  : run.first + '–' + run.last}{' '}
                集
              </a>
            </h3>
            <p>
              {run.published
                ? '已完成并发布'
                : run.passed
                  ? '本集已通过，等待前置集数发布'
                  : '验收未通过'}{' '}
              · 知识版本 {run.version} · 规则 v{run.rule} · 记录 #{run.id}
            </p>
            <h4>缺项</h4>
            {run.checks.length === 0 ? (
              <p>暂无验收明细。</p>
            ) : gaps.length ? (
              <ul>
                {gaps.map((g) => (
                  <li key={g}>{g}</li>
                ))}
              </ul>
            ) : (
              <p>本范围无必需项缺失。</p>
            )}
            <div className="scope-dimensions">
              {run.checks.map((c) => (
                <details key={c.name}>
                  <summary>
                    {c.name} · {c.items.filter((i) => !i.gap).length}/
                    {c.items.length}
                  </summary>
                  <ul>
                    {c.items.map((i) => (
                      <li key={i.label}>
                        <strong>{i.label}</strong>　{i.gap ?? i.evidence}
                        {i.source && (
                          <>
                            　<a href={i.source}>依据</a>
                          </>
                        )}
                      </li>
                    ))}
                  </ul>
                </details>
              ))}
            </div>
            {run.previous_gaps && (
              <details>
                <summary>
                  上一次未通过的验收 · 记录 #{run.previous_gaps.id}
                </summary>
                <ul>
                  {run.previous_gaps.items.map((g) => (
                    <li key={g}>{g}</li>
                  ))}
                </ul>
              </details>
            )}
            <h4>{run.published ? '已发布场景图' : '入选候选图'}</h4>
            <p>
              先筛除未通过或原图、裁切凭证不匹配的候选。优先原图一次通过、无需重裁、初检无异议，同级按候选编号。
            </p>
            <ul>
              {run.picks.map((p) => (
                <li key={p.anchor}>
                  {run.published ? (
                    <a
                      href={
                        '/work/' +
                        run.world +
                        '/chapter/' +
                        p.chapter +
                        '#illustration-' +
                        p.anchor
                      }
                    >
                      {p.name}
                    </a>
                  ) : (
                    p.name
                  )}{' '}
                  · 候选 {p.variant} · 第 {p.round} 轮
                </li>
              ))}
            </ul>
          </article>
        )
      })}
    </section>
  )
}
