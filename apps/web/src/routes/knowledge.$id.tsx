import { imageUrl } from '../lib/images'
import { siteTitle } from '../site'
import { createFileRoute } from '@tanstack/react-router'
import { readWork } from '../features/knowledge-api'
import { Layout } from '../features/post/design'

export const Route = createFileRoute('/knowledge/$id')({
  loader: ({ params }) => readWork({ data: params.id }),
  head: ({ loaderData }) => ({
    meta: [
      { title: siteTitle((loaderData?.canonical_title ?? '作品') + ' · 作品知识库') },
      { name: 'robots', content: 'noindex' },
    ],
  }),
  component: Work,
})
function Work() {
  const w = Route.useLoaderData()
  return (
    <Layout>
      <div className="page-intro">
        <a href="/knowledge">返回作品知识库</a>
        {w.world_href && w.cover_image && (
          <a
            href={w.world_href}
            aria-label={'进入作品世界 · ' + w.canonical_title}
          >
            <img
              className="catalog-world-cover"
              src={imageUrl(w.cover_image)}
              alt=""
              fetchPriority="high"
            />
          </a>
        )}
        <h1>{w.canonical_title}</h1>
        {w.world_href && (
          <a className="button" href={w.world_href}>
            进入作品世界
          </a>
        )}
        <p>
          {[
            w.type,
            w.season_year,
            w.status,
            w.episodes != null && w.episodes > 0 ? w.episodes + ' 集' : null,
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
      </div>
      <section className="section">
        <h2>名称与别名</h2>
        <ul>
          {w.titles.map((t) => (
            <li key={t.lang + '|' + t.title}>
              {t.title} · {t.lang}
            </li>
          ))}
        </ul>
      </section>
      {!!w.studios.length && (
        <section className="section">
          <h2>制作公司</h2>
          <p>{w.studios.join('、')}</p>
        </section>
      )}
      <section className="section">
        <h2>资料来源</h2>
        <ul>
          {w.provenance.map((p, i) => (
            <li key={i}>
              {p.repository.startsWith('https://') ? (
                <a href={p.repository}>{p.source}</a>
              ) : (
                '潮汐邮局原创清单'
              )}{' '}
              ·{' '}
              {typeof p.license === 'string'
                ? p.license
                : (p.license?.name ?? '来源记录')}
              <br />
              <small style={{ overflowWrap: 'anywhere' }}>{p.version}</small>
            </li>
          ))}
        </ul>
      </section>
    </Layout>
  )
}
