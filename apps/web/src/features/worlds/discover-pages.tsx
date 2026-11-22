import { Layout, SectionHeader } from '../post/design'
import { Icon } from '../post/icons'
import { ContinueReading } from '../post/progress'
import type {
  DiscoverCard,
  Directory,
  Portrait,
  StoryEntry,
  discoverDirectory,
  discoverHome,
  discoverySearch,
} from './discover'
import './discover.css'

function WorkTile({ work }: { work: DiscoverCard }) {
  return (
    <a className="folio-tile" href={work.href}>
      <div className="folio-cover">
        {work.image && <img src={work.image} loading="lazy" alt="" />}
      </div>
      <small>{work.edition}</small>
      <strong>{work.title}</strong>
      <p>{work.chapters} 篇可读</p>
    </a>
  )
}
function PortraitTile({ person }: { person: Portrait }) {
  return (
    <a className="folio-person" href={person.href}>
      <div className="folio-portrait">
        {person.image ? (
          <img src={person.image} alt="" loading="lazy" />
        ) : (
          <Icon name="people" size={36} />
        )}
      </div>
      <small>{person.work}</small>
      <strong>{person.title}</strong>
      <p>{person.subtitle}</p>
    </a>
  )
}
function ChapterRow({ story }: { story: StoryEntry }) {
  return (
    <a className="folio-chapter" href={story.href}>
      <div className="folio-thumb">
        {story.image && <img src={story.image} alt="" loading="lazy" />}
      </div>
      <div>
        <small>
          {story.work} · {story.label}
        </small>
        <strong>{story.title}</strong>
        <p>{story.subtitle}</p>
      </div>
    </a>
  )
}
export function DiscoverHome({
  page,
}: {
  page: Awaited<ReturnType<typeof discoverHome>>
}) {
  const [lead, ...features] = page.featured
  return (
    <Layout active="discover">
      {lead ? (
        <section className="folio-feature" aria-label="精选作品">
          <article className="folio-lead">
            {lead.image && (
              <img
                className="folio-art"
                src={lead.image}
                alt=""
                fetchPriority="high"
              />
            )}
            <div className="folio-copy">
              <small>
                <Icon name="spark" size={15} />
                本期精选 <i /> {lead.edition}
              </small>
              <h1>
                <a href={lead.href}>{lead.title}</a>
              </h1>
              <p>{lead.intro}</p>
              <a className="button" href={lead.href}>
                进入作品世界 <Icon name="arrow" size={16} />
              </a>
            </div>
          </article>
          {features.length > 0 && (
            <div className="folio-companions">
              {features.map((w) => (
                <a href={w.href} key={w.href}>
                  {w.image && (
                    <img className="folio-art" src={w.image} alt="" />
                  )}
                  <div>
                    <small>{w.edition}</small>
                    <strong>{w.title}</strong>
                  </div>
                  <Icon name="forward" size={20} />
                </a>
              ))}
            </div>
          )}
        </section>
      ) : (
        <h1>发现作品</h1>
      )}
      <ContinueReading />
      <section className="folio-section" id="world">
        <SectionHeader href="/works" link={'全部 ' + page.total + ' 部作品'}>
          探索作品
        </SectionHeader>
        {page.covers.length > 0 ? (
          <div className="folio-strip">
            {page.covers.map((w) => (
              <WorkTile work={w} key={w.href} />
            ))}
          </div>
        ) : (
          <a href="/knowledge">浏览作品知识库</a>
        )}
      </section>
      {page.portraits.length > 0 && (
        <section className="folio-section">
          <SectionHeader href="/characters" link="全部角色">
            遇见角色
          </SectionHeader>
          <div className="folio-strip">
            {page.portraits.map((p) => (
              <PortraitTile person={p} key={p.href} />
            ))}
          </div>
        </section>
      )}
      {page.stories.length > 0 && (
        <section className="folio-section">
          <SectionHeader href="/stories" link="全部章节">
            章节选读
          </SectionHeader>
          <div className="folio-chapters">
            {page.stories.map((c) => (
              <ChapterRow story={c} key={c.href} />
            ))}
          </div>
        </section>
      )}
      {page.scenes.length > 0 && (
        <section className="folio-section">
          <SectionHeader>剧情片段</SectionHeader>
          <div className="folio-scenes">
            {page.scenes.map((scene) => (
              <a href={scene.href} key={scene.href}>
                {scene.image && <img src={scene.image} alt="" loading="lazy" />}
                <div>
                  <small>
                    {scene.work} · {scene.place}
                  </small>
                  <strong>{scene.title}</strong>
                </div>
              </a>
            ))}
          </div>
        </section>
      )}
      <a className="folio-knowledge" href="/knowledge">
        <Icon name="map" />
        作品知识库
        <Icon name="arrow" size={16} />
      </a>
    </Layout>
  )
}
const titles = {
  works: '作品目录',
  characters: '角色图鉴',
  stories: '故事书库',
}
export function DirectoryPage({
  kind,
  page,
  search,
}: {
  kind: Directory
  page: Awaited<ReturnType<typeof discoverDirectory>>
  search: ReturnType<typeof discoverySearch>
}) {
  const { groups, options } = page
  const isCast = kind === 'characters',
    isWorks = kind === 'works'
  const count = groups.reduce(
    (n, g) =>
      n + (isWorks ? 1 : isCast ? g.portraits.length : g.stories.length),
    0,
  )
  const portraits = groups
    .flatMap((g) => g.portraits.map((person, index) => ({ person, index })))
    .sort((a, b) => a.index - b.index)
  const label = isWorks
    ? '搜索作品名或版本'
    : isCast
      ? '搜索角色名或作品名'
      : '搜索章节名或作品名'
  return (
    <Layout active={isWorks ? 'works' : isCast ? 'cast' : 'stories'}>
      <header className="folio-index-head">
        <h1>{titles[kind]}</h1>
        <form action={'/' + kind} method="get" key={JSON.stringify(search)}>
          {!isWorks && (
            <select
              aria-label="按作品筛选"
              name="work"
              defaultValue={search.work ?? ''}
              onChange={(e) => e.currentTarget.form?.requestSubmit()}
            >
              <option value="">全部作品</option>
              {options.map((w) => (
                <option key={w.slug} value={w.slug}>
                  {w.title} · {w.edition}
                </option>
              ))}
            </select>
          )}
          <div className="folio-search">
            <input
              type="search"
              name="q"
              defaultValue={search.q}
              maxLength={160}
              aria-label={label}
              placeholder={label}
            />
            <button type="submit">搜索</button>
          </div>
        </form>
      </header>
      <div className="folio-results">
        <span>
          {count} {isWorks ? '部作品' : isCast ? '位角色' : '篇可读章节'}
        </span>
        {(search.q || search.work) && <a href={'/' + kind}>清除筛选</a>}
        {isWorks && <a href="/knowledge">作品知识库 ↗</a>}
      </div>
      {count === 0 ? (
        <p className="folio-empty">
          没有找到匹配的{isWorks ? '作品' : isCast ? '角色' : '章节'}。
        </p>
      ) : isWorks ? (
        <div className="folio-grid">
          {groups.map((g) => (
            <WorkTile key={g.work.slug} work={g.work} />
          ))}
        </div>
      ) : isCast ? (
        <div className="folio-grid folio-people">
          {portraits.map(({ person }) => (
            <PortraitTile key={person.href} person={person} />
          ))}
        </div>
      ) : (
        groups.map((g) => (
          <section className="folio-story-group" key={g.work.slug}>
            <header>
              <small>{g.work.edition}</small>
              <h2>
                <a href={g.work.href}>
                  {g.work.title} <Icon name="forward" size={16} />
                </a>
              </h2>
            </header>
            <div className="folio-chapters">
              {g.stories.map((c) => (
                <ChapterRow key={c.href} story={c} />
              ))}
            </div>
          </section>
        ))
      )}
    </Layout>
  )
}
