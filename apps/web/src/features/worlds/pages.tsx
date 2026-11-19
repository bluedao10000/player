import { Icon } from '../post/icons'
import { Button, Layout, SectionHeader } from '../post/design'
import type {
  EpisodeIllustration,
  ChapterCard,
  ChapterDetail,
  CharacterCard,
  CharacterDetail,
  ReadingBlock,
  SceneCard,
  SceneDetail,
  WorldCard,
  WorldOverview,
} from '@anime/shared/knowledge'
import { ReadingProgress } from '../post/progress'
import { Fragment } from 'react'

export function WorldTrail({ work }: { work: WorldCard }) {
  return (
    <nav className="world-trail" aria-label="作品导航">
      <a href="/">发现</a>
      <Icon name="right" size={12} />
      <a href={work.href}>{work.title}</a>
    </nav>
  )
}

function PeopleList({ people }: { people: CharacterCard[] }) {
  return (
    <div className="world-cast">
      {people.map((person) => (
        <a key={person.slug} href={person.href}>
          {person.portrait && (
            <img src={person.portrait} width="52" height="52" alt="" />
          )}
          <span>
            <strong>{person.name}</strong>
            <small>{person.role}</small>
          </span>
          <Icon name="forward" size={16} />
        </a>
      ))}
    </div>
  )
}

function ChapterList({ chapters }: { chapters: ChapterCard[] }) {
  return (
    <div className="world-chapters">
      {chapters.map((chapter) => (
        <a key={chapter.slug} href={chapter.href}>
          {chapter.image && (
            <img
              className="chapter-thumb"
              src={chapter.image}
              alt=""
              loading="lazy"
            />
          )}
          <div>
            <small>{chapter.label}</small>
            <h3>{chapter.title}</h3>
            <p>{chapter.intro}</p>
          </div>
          <Icon name="arrow" size={18} />
        </a>
      ))}
    </div>
  )
}

export function SceneLink({ scene }: { scene: SceneCard }) {
  return (
    <a className="world-scene-link" href={scene.href}>
      {scene.image ? (
        <img className="scene-thumb" src={scene.image} alt="" loading="lazy" />
      ) : (
        <Icon name="compass" />
      )}
      <span>
        <small>阅读这一幕 · {scene.place}</small>
        <strong>{scene.title}</strong>
      </span>
      <Icon name="arrow" />
    </a>
  )
}

export function Hero({
  work,
  title,
  label,
  summary,
  image,
  focus,
}: {
  work: WorldCard
  title: string
  label: string
  summary?: string
  image?: string
  focus?: number
}) {
  return (
    <header
      className={'world-cover ' + (image ? 'world-photo' : 'world-record')}
    >
      {image && (
        <img
          src={image}
          alt=""
          fetchPriority="high"
          style={
            focus == null ? undefined : { objectPosition: `50% ${focus}%` }
          }
        />
      )}
      <div className="world-cover-text">
        <small>{label}</small>
        <h1>{title}</h1>
        {summary && <p>{summary}</p>}
      </div>
      {!image && work.slug === 'cowboy-bebop-tv' && (
        <span className="record-wordmark" aria-hidden="true">
          {work.title === title ? 'COWBOY BEBOP' : 'BEBOP'}
        </span>
      )}
    </header>
  )
}

function IllustratedOpening({
  image,
  title,
  label,
  summary,
  caption,
  credit,
}: {
  image: string
  title: string
  label: string
  summary?: string
  caption?: string
  credit?: string
}) {
  return (
    <header className="illustrated-opening">
      <figure>
        <img
          src={image}
          alt={caption ?? ''}
          fetchPriority="high"
          width="1440"
          height="960"
        />
        {(credit || caption) && (
          <figcaption>
            {[credit, caption].filter(Boolean).join(' · ')}
          </figcaption>
        )}
      </figure>
      <div>
        <small className="world-kicker">{label}</small>
        <h1>{title}</h1>
        {summary && <p>{summary}</p>}
      </div>
    </header>
  )
}

const isFeatureFilm = (work: WorldCard) => work.edition_label?.endsWith(' · 全片')

export function WorkPage({
  work,
  characters,
  storylines,
  reader_intro,
  world_setting,
  completed_scope,
  scenes,
}: WorldOverview) {
  const film = isFeatureFilm(work)
  return (
    <Layout active="作品">
      <WorldTrail work={work} />
      <Hero
        work={work}
        title={work.title}
        label={work.edition_label}
        summary={reader_intro ? undefined : work.intro}
        image={work.hero_asset_key ?? undefined}
      />
      <div className="world-body">
        {completed_scope && (
          <section className="world-section">
            <SectionHeader>
              {film ? '全片 · 已完成' : `第 ${completed_scope.first}–${completed_scope.last} 集 · 已完成`}
            </SectionHeader>
            <div className="episode-illustrations">
              {completed_scope.picks.map((pic) => (
                <a
                  key={pic.anchor}
                  href={
                    work.href +
                    '/chapter/' +
                    pic.chapter +
                    '#illustration-' +
                    pic.anchor
                  }
                >
                  <img
                    src={pic.image}
                    alt=""
                    width="1536"
                    height="1024"
                    loading="lazy"
                  />
                  <strong>{pic.name}</strong>
                </a>
              ))}
            </div>
          </section>
        )}
        {reader_intro && (
          <p className="world-premise">{reader_intro.premise}</p>
        )}
        <section className="world-section">
          <SectionHeader>{work.cast_title}</SectionHeader>
          <PeopleList people={characters} />
        </section>
        {(reader_intro?.setting || world_setting) && (
          <section className="world-section" id="world-setting">
            <SectionHeader>世界观</SectionHeader>
            <p>{reader_intro?.setting || world_setting}</p>
          </section>
        )}
        {storylines.map((line) => (
          <section key={line.slug} className="world-section">
            <small className="world-kicker">
              {film ? '全片导读' : reader_intro ? '分集故事' : '故事线'}
            </small>
            <SectionHeader>{line.title}</SectionHeader>
            <p>{line.intro}</p>
            <ChapterList chapters={line.chapters} />
          </section>
        ))}
        {scenes.length > 0 && (
          <section className="world-section">
            <SectionHeader>故事中的场景</SectionHeader>
            {scenes.map((s) => (
              <SceneLink key={s.slug} scene={s} />
            ))}
          </section>
        )}
      </div>
    </Layout>
  )
}
export function PersonPage({
  work,
  character,
  appearances,
  intro,
}: CharacterDetail) {
  return (
    <Layout active="角色">
      <WorldTrail work={work} />
      <header className="person-opening">
        {character.portrait && (
          <img
            src={character.portrait}
            alt={character.name}
            fetchPriority="high"
          />
        )}
        <div>
          <small className="world-kicker">{work.title} · 人物</small>
          <h1>{character.name}</h1>
          <p className="person-role">{character.role}</p>
          {intro && <p id="character-intro">{intro}</p>}
        </div>
      </header>
      <div className="world-body">
        <section className="world-section" aria-label="出场章节">
          <SectionHeader>出场章节</SectionHeader>
          <ChapterList chapters={appearances} />
        </section>
      </div>
    </Layout>
  )
}

export function ReadingBlocks({
  blocks,
  illustrations = [],
}: {
  blocks: ReadingBlock[]
  illustrations?: EpisodeIllustration[]
}) {
  return (
    <div id="prose" className="prose">
      {blocks.map((block) => (
        <Fragment key={block.key}>
          {block.anchors.slice(1).map((id) => (
            <span key={id} id={id} aria-hidden="true" />
          ))}
          <p id={block.anchors[0]} data-block={block.key}>
            {block.text}
          </p>
          {illustrations
            .filter((pic) => pic.block === block.key)
            .map((pic) => (
              <figure
                className="episode-plate"
                id={'illustration-' + pic.anchor}
                key={pic.anchor}
              >
                <img
                  src={pic.image}
                  alt={pic.name}
                  width="1536"
                  height="1024"
                  loading="lazy"
                />
                <figcaption>{pic.name} · AI 场景插画</figcaption>
              </figure>
            ))}
        </Fragment>
      ))}
    </div>
  )
}

function Sources({ items }: { items: ChapterDetail['attribution'] }) {
  const links = [
    ...new Set(
      items.map((p) => p.uri).filter((uri) => uri.startsWith('https://')),
    ),
  ]
  return (
    <>
      {links.map((url) => (
        <a key={url} className="world-source" href={url}>
          资料来源 · {new URL(url).hostname} ↗
        </a>
      ))}
    </>
  )
}

export function ChapterPage({
  work,
  chapter,
  blocks,
  research,
  reader_passages,
  accepted_art,
  next_chapter,
  scenes,
  attribution,
}: ChapterDetail) {
  return (
    <Layout active="故事">
      <WorldTrail work={work} />
      <ReadingProgress slug={chapter.href} href={chapter.href} title={chapter.title} work={work.title} containerId="chapter-body" />
      {accepted_art?.length ? (
        <header className="column episode-heading">
          <small className="world-kicker">
            {work.title} · {isFeatureFilm(work) ? '全片已完成' : `${chapter.label} · 本集已完成`}
          </small>
          <h1>{chapter.title}</h1>
          <p>{chapter.intro}</p>
        </header>
      ) : chapter.image && work.origin === 'external' ? (
        <IllustratedOpening
          image={chapter.image}
          caption={chapter.image_caption}
          credit={chapter.image_credit}
          title={chapter.title}
          label={chapter.label}
          summary={chapter.intro}
        />
      ) : (
        <Hero
          work={work}
          title={chapter.title}
          label={chapter.label}
          summary={chapter.intro}
          image={chapter.image}
        />
      )}
      <article className="column world-reading" id="chapter-body">
        {reader_passages && (
          <ReadingBlocks
            blocks={reader_passages}
            illustrations={accepted_art ?? []}
          />
        )}
        {!reader_passages && research && (
          <section className="episode-guide" aria-label="本集剧情">
            <div className="prose">
              {research.paragraphs.map((p, i) => (
                <p key={i}>{p.text}</p>
              ))}
            </div>
          </section>
        )}
        {!reader_passages &&
          (research ? (
            blocks
              .filter((b) => b.anchors.length > 0)
              .map((b) => (
                <aside className="scene-passage" key={b.key}>
                  <small>
                    {
                      scenes.find((s) =>
                        b.anchors.includes('passage-' + s.slug),
                      )?.title
                    }
                  </small>
                  <ReadingBlocks blocks={[b]} />
                </aside>
              ))
          ) : (
            <ReadingBlocks blocks={blocks} />
          ))}
        {scenes.map((s) => (
          <SceneLink key={s.slug} scene={s} />
        ))}
        {next_chapter && research && (
          <Button href={next_chapter.href}>
            下一集 · {next_chapter.title}
          </Button>
        )}
        {!research && <Sources items={attribution} />}
      </article>
    </Layout>
  )
}
export function ScenePage({
  work,
  chapter,
  scene,
  scene_cast,
  return_to,
  attribution,
}: SceneDetail) {
  return (
    <Layout active="故事">
      <WorldTrail work={work} />
      {scene.image ? (
        <IllustratedOpening
          image={scene.image}
          title={scene.title}
          label={'场景 · ' + scene.place}
          caption={scene.image_caption}
          credit={scene.image_credit}
        />
      ) : (
        <Hero
          work={work}
          title={scene.title}
          label={'场景 · ' + scene.place}
          image={chapter.image}
        />
      )}
      <article className="column world-reading">
        <small className="world-kicker">
          {chapter.label} · {chapter.title}
        </small>
        <div className="prose">
          <p>{scene.text}</p>
        </div>
        <Button href={return_to.href}>回到故事 · {chapter.title}</Button>
        <section className="world-section">
          <SectionHeader>本幕人物</SectionHeader>
          <PeopleList people={scene_cast} />
        </section>
        <Sources items={attribution} />
      </article>
    </Layout>
  )
}
