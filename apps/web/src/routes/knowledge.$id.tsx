import { createFileRoute } from '@tanstack/react-router'
import { readWork } from '../features/knowledge-api'
import { Layout } from '../features/post/design'

export const Route = createFileRoute('/knowledge/$id')({
  loader: ({ params }) => readWork({ data: params.id }),
  head: ({ loaderData }) => ({
    meta: [
      { title: (loaderData?.canonical_title ?? '作品') + ' · 作品知识库' },
      { name: 'robots', content: 'noindex' },
    ],
  }),
  component: Work,
})
function Work() {
  const w = Route.useLoaderData()
  return (
    <Layout active="">
      <div className="page-intro">
        <a href="/knowledge">返回作品知识库</a>
        <h1>{w.canonical_title}</h1>
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
              <a href={p.repository}>{p.source}</a> ·{' '}
              {typeof p.license === 'string' ? p.license : p.license.name}
              <br />
              <small style={{ overflowWrap: 'anywhere' }}>{p.version}</small>
            </li>
          ))}
        </ul>
      </section>
    </Layout>
  )
}
