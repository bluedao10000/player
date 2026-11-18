import { useState } from 'react'
import { artUrl, catalog, feature, stories, storyUrl } from './catalog'
import {
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
  WorldPreview,
} from './design'
import { Icon } from './icons'
import { Save } from './storage'
import { ContinueReading, ReadingProgress } from './progress'

import type { ChapterDetail, HomeWorld } from '@anime/shared/knowledge'
import {
  ChapterPage,
  ReadingBlocks,
  SceneLink,
  WorldTrail,
} from '../worlds/pages'

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
          <a href="/work/tidepost">
            潮汐邮局 <span aria-hidden="true">↗</span>
          </a>
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
export function Home({ works }: { works: HomeWorld[] }) {
  return (
    <Layout active="发现">
      <StoryHero />
      <nav className="work-discovery" aria-label="探索作品世界">
        <h2>作品世界</h2>
        <div className="work-covers">
          {works.map((work) => (
            <a key={work.slug} href={work.href}>
              {work.hero_asset_key && (
                <img src={work.hero_asset_key} alt="" loading="lazy" />
              )}
              <span>
                <small>{work.edition_label}</small>
                <strong>{work.title}</strong>
              </span>
              <Icon name="forward" size={16} />
            </a>
          ))}
        </div>
      </nav>
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
            <ScenePreview
              key={s.slug}
              story={s}
              scene={works
                .find((w) => w.slug === 'tidepost')
                ?.scene_previews.find((scene) => scene.chapter_slug === s.slug)}
            />
          ))}
        </div>
      </section>
      <section className="section" id="world">
        <SectionHeader href="/knowledge" link="作品知识库">
          探索白帆港
        </SectionHeader>
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
export function Reader({ page }: { page: ChapterDetail }) {
  const { chapter, work, blocks, scenes } = page
  const story = catalog.find((s) => s.slug === chapter.slug)
  if (!story) return <ChapterPage {...page} />
  const next =
    catalog[
      (catalog.findIndex((s) => s.slug === story.slug) + 1) % catalog.length
    ]
  const isFeature = story.slug === feature.slug
  return (
    <Layout active="故事">
      <ReadingProgress slug={story.slug} />
      <WorldTrail work={work} />
      <article className="reading">
        <header className="reading-cover">
          <img
            src={isFeature ? nightImage : chapter.image}
            alt={isFeature ? nightAlt : story.alt}
            fetchPriority="high"
          />
          <div>
            <div className="eyebrow">
              <Icon name="book" size={15} />
              {chapter.label}
            </div>
            <h1>{chapter.title}</h1>
            <p>{chapter.intro}</p>
          </div>
        </header>
        <div className="column">
          <div className="byline">
            <CharacterChip />
            <ContentMeta story={story} />
            <Save story={story} />
          </div>
          <ReadingBlocks blocks={blocks} />
          {scenes.map((scene) => (
            <SceneLink key={scene.slug} scene={scene} />
          ))}
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
