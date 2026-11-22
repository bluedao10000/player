import { siteTitle } from '../site'
import { createFileRoute } from '@tanstack/react-router'
import { findWorks } from '../features/knowledge-api'
import { Layout } from '../features/post/design'

export const Route = createFileRoute('/knowledge/')({
  head: () => ({
    meta: [
      { title: siteTitle('作品知识库') },
      { name: 'robots', content: 'noindex' },
    ],
  }),
  validateSearch: (s: Record<string, unknown>) => ({
    q:
      typeof s.q === 'string'
        ? s.q.trim().slice(0, 128) || undefined
        : undefined,
    after:
      typeof s.after === 'string' && /^[a-f\d-]{36}$/i.test(s.after)
        ? s.after
        : undefined,
  }),
  loaderDeps: ({ search }) => search,
  loader: ({ deps }) =>
    findWorks({ data: { q: deps.q ?? '', after: deps.after ?? '' } }),
  component: Knowledge,
})
function Knowledge() {
  const page = Route.useLoaderData(),
    { q } = Route.useSearch()
  return (
    <Layout>
      <div className="page-intro">
        <h1>作品知识库</h1>
        <form method="get" action="/knowledge">
          <label>
            作品名称 <input name="q" defaultValue={q} maxLength={128} />
          </label>{' '}
          <button>搜索</button>
        </form>
      </div>
      {page.items.length ? (
        <ul>
          {page.items.map((w) => (
            <li key={w.id}>
              <a href={w.world_href ?? '/knowledge/' + w.id}>
                {w.cover_image && (
                  <img
                    className="catalog-cover"
                    src={w.cover_image}
                    alt=""
                    width="92"
                    height="60"
                    loading="lazy"
                  />
                )}
                {w.canonical_title}
              </a>{' '}
              · {w.type} {w.season_year}
            </li>
          ))}
        </ul>
      ) : (
        <p>没有找到相关作品。</p>
      )}
      {page.next_cursor && (
        <a
          className="button"
          href={
            '/knowledge?' +
            new URLSearchParams({ q: q ?? '', after: page.next_cursor })
          }
        >
          下一页
        </a>
      )}
      <p>
        资料来自{' '}
        <a href="https://github.com/cedya77/anime-offline-database">
          anime-offline-database（ODbL / DbCL）
        </a>{' '}
        与{' '}
        <a href="https://github.com/bangumi-data/bangumi-data">
          bangumi-data（CC BY 4.0）
        </a>
        ，本库整合来源映射与译名。
      </p>
    </Layout>
  )
}
