import { useState } from 'react'
import { artUrl, catalog, feature, stories, storyUrl } from './catalog'
import type { Story } from './catalog'
import {
  Avatar,
  Button,
  CharacterChip,
  ContentMeta,
  GlassSurface,
  IconButton,
  Layout,
  people,
  ScenePreview,
  SectionHeader,
  StoryPreview,
  Tag,
  WorldPreview,
} from './design'
import { Icon } from './icons'
import { Save } from './storage'
import { ContinueReading, ReadingProgress } from './progress'

const nightImage = '/art/night-post.webp'
const nightAlt = '夜班邮差阿澪提着灯，回望月光下的白帆港'
const editions = [feature, stories[0], stories[2]]
function StoryHero() {
  const [index, setIndex] = useState(0)
  const story = editions[index]
  function shift(delta: number) {
    setIndex((i) => (i + delta + editions.length) % editions.length)
  }
  return (
    <section className={'headline edition-' + index} aria-label="精选故事">
      <img
        className="headline-art"
        src={index === 0 ? nightImage : artUrl(story)}
        alt={index === 0 ? nightAlt : story.alt}
        fetchPriority="high"
      />
      <div className="headline-copy" aria-live="polite">
        <div className="eyebrow">
          <Icon name="spark" size={15} />
          本期精选
          <span className="divider" />
          {story.category}
        </div>
        <h1>
          {index === 0 ? (
            <>
              <span>海风里的</span>
              <em>夜班邮差</em>
            </>
          ) : (
            story.title
          )}
        </h1>
        <p>{story.intro}</p>
        <Button href={storyUrl(story)}>阅读故事</Button>
      </div>
      <div className="edition-controls">
        <span className="counter">
          {String(index + 1).padStart(2, '0')} <b>/ 03</b>
        </span>
        <IconButton label="上一篇精选" icon="left" onClick={() => shift(-1)} />
        <IconButton label="下一篇精选" icon="right" onClick={() => shift(1)} />
      </div>
    </section>
  )
}
export function Home() {
  return (
    <Layout active="发现">
      <StoryHero />
      <section className="section">
        <SectionHeader href="/character/mio#relationships" link="人物关系">
          港口人物
        </SectionHeader>
        <div className="cast-rail">
          {people.map((person) => (
            <a
              className={'cast ' + person.id}
              href={person.href}
              key={person.id}
            >
              <img
                src={'/art/' + person.image + '.webp'}
                alt={person.name}
                width="640"
                height="380"
              />
              <div>
                <small>{person.role}</small>
                <h3>{person.name}</h3>
              </div>
              <span className="cast-arrow">
                <Icon name="forward" size={16} />
              </span>
            </a>
          ))}
        </div>
      </section>
      <ContinueReading />
      <section className="section">
        <SectionHeader href="/stories" link="全部故事">
          故事来信
        </SectionHeader>
        <div className="reading-shelf">
          {stories.slice(0, 2).map((s) => (
            <StoryPreview key={s.slug} story={s} />
          ))}
        </div>
      </section>
      <section className="section">
        <SectionHeader>剧情片段</SectionHeader>
        <div className="scenes">
          {[stories[2], stories[0]].map((s) => (
            <ScenePreview key={s.slug} story={s} />
          ))}
        </div>
      </section>
      <section className="section" id="world">
        <SectionHeader>探索白帆港</SectionHeader>
        <p className="world-intro">顺着邮路，读一读这些地方发生的故事。</p>
        <div className="places">
          {stories.map((s) => (
            <WorldPreview key={s.slug} story={s} />
          ))}
        </div>
      </section>
    </Layout>
  )
}
function CharacterHero() {
  return (
    <section className="character-hero">
      <img src={nightImage} alt={nightAlt} fetchPriority="high" />
      <div className="character-copy">
        <div className="eyebrow">
          <Icon name="people" size={15} />
          角色档案
        </div>
        <h1>阿澪</h1>
        <p>替每一封迷路的信，找到靠岸的灯。</p>
        <div className="tags">
          <Tag>白帆港</Tag>
          <Tag>夜班邮差</Tag>
        </div>
        <Button href="/play/night-courier">进入角色剧情</Button>
      </div>
    </section>
  )
}
export function Character() {
  return (
    <Layout active="角色">
      <CharacterHero />
      <section className="bio section">
        <SectionHeader>关于阿澪</SectionHeader>
        <p>
          阿澪熟悉港口的每一盏航灯。她随身带着潮汐表和一枚铜哨，相信那些没有地址的信，也有一个等着收信的人。
        </p>
      </section>
      <section className="section">
        <SectionHeader>她出场的故事</SectionHeader>
        <div className="reading-shelf">
          {catalog
            .filter((s) => s.category !== '剧情片段')
            .map((s) => (
              <StoryPreview key={s.slug} story={s} />
            ))}
        </div>
      </section>
      <section className="section">
        <SectionHeader>相关场景</SectionHeader>
        <div className="scenes">
          <ScenePreview story={stories[2]} />
          <ScenePreview story={stories[0]} />
        </div>
      </section>
      <section className="section" id="relationships">
        <SectionHeader>人物关系</SectionHeader>
        <div className="relations">
          {people.slice(1).map((p, i) => (
            <article id={p.id} key={p.id}>
              <Avatar image={p.image} />
              <div>
                <h3>{p.name}</h3>
                <p>
                  {i === 0
                    ? '她掌着末班船，也替阿澪留意迟到的乘客。'
                    : '他教给阿澪的灯语，让海岸两端的人在夜里相认。'}
                </p>
                <a href={'/story/' + p.story}>
                  读出场故事 <Icon name="arrow" size={14} />
                </a>
              </div>
            </article>
          ))}
        </div>
      </section>
    </Layout>
  )
}
export function Reader({ story }: { story: Story }) {
  const next =
    catalog[
      (catalog.findIndex((s) => s.slug === story.slug) + 1) % catalog.length
    ]
  const isFeature = story.slug === feature.slug
  return (
    <Layout active="故事">
      <ReadingProgress slug={story.slug} />
      <article className="reading">
        <header className="reading-cover">
          <img
            src={isFeature ? nightImage : artUrl(story)}
            alt={isFeature ? nightAlt : story.alt}
            fetchPriority="high"
          />
          <div>
            <div className="eyebrow">
              <Icon name="book" size={15} />
              {story.category}
            </div>
            <h1>{story.title}</h1>
            <p>{story.intro}</p>
          </div>
        </header>
        <div className="column">
          <div className="byline">
            <CharacterChip />
            <ContentMeta story={story} />
            <Save story={story} />
          </div>
          <div id="prose" className="prose">
            {story.paragraphs.map((p) => (
              <p key={p}>{p}</p>
            ))}
          </div>
          <section id="scene">
            <GlassSurface>
              <div className="eyebrow">
                <Icon name="compass" size={16} />
                {story.location}
              </div>
              <h2>从这一幕开始</h2>
              <p>{story.scene}</p>
              <Button href={'/play/' + story.slug}>进入剧情</Button>
            </GlassSurface>
          </section>
          <a className="next-read" href={storyUrl(next)}>
            <div>
              <small>接着读</small>
              <h3>{next.title}</h3>
            </div>
            <Icon name="arrow" />
          </a>
        </div>
      </article>
    </Layout>
  )
}
