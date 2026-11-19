import { catalog, feature, storyUrl } from './catalog'
import {
  Button,
  CharacterChip,
  ContentMeta,
  GlassSurface,
  Layout,
} from './design'
import { Icon } from './icons'
import { Save } from './storage'
import { ReadingProgress } from './progress'

import type { ChapterDetail } from '@anime/shared/knowledge'
import {
  ChapterPage,
  ReadingBlocks,
  SceneLink,
  WorldTrail,
} from '../worlds/pages'

const nightImage = '/art/night-post.webp'
const nightAlt = '夜班邮差阿澪提着灯，回望月光下的白帆港'
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
      <ReadingProgress slug={story.slug} href={chapter.href} title={chapter.title} work={work.title} />
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
