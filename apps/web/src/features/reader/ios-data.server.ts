import { imageUrl } from '../../lib/images'
import {catalog,artUrl} from '../post/catalog'
import {
  discoveryCatalog,
  getWorld,
  worldCharacter,
  worldChapter,
  worldScene,
  KnowledgeError,
  type WorldCard,
  type ChapterCard,
  type SceneCard,
  type CharacterCard,
} from '@anime/shared/knowledge'

const work = ({
  slug,
  title,
  edition_label,
  intro,
  hero_asset_key,
  href,
}: WorldCard) => ({
  slug,
  title,
  edition: edition_label,
  intro,
  image: imageUrl(hero_asset_key),
  href,
})
const person = ({ slug, name, role, portrait, href }: CharacterCard) => ({
  slug,
  title: name,
  subtitle: role,
  image: imageUrl(portrait),
  href,
})
const chapter = ({ slug, title, label, intro, image, href }: ChapterCard) => ({
  slug,
  title,
  subtitle: label,
  intro,
  image: imageUrl(image),
  href,
})
const scene = ({ slug, title, place, image, href }: SceneCard) => ({
  slug,
  title,
  subtitle: place,
  image: imageUrl(image),
  href,
})

async function read(path: string) {
  if (path === 'v1/discovery') {
    const { items } = await discoveryCatalog()
    return {
      items: items.map((w) => ({
        ...work(w),
        characters: w.characters.map(person),
        chapters: w.chapters.map(chapter),
      })),
    }
  }
  const match =
    /^v1\/works\/([a-z0-9-]+)(?:\/(characters|chapters|scenes)\/([a-z0-9-]+))?$/.exec(
      path,
    )
  if (!match) throw new KnowledgeError(404)
  const [, world, kind, slug] = match
  if (!kind) {
    const w = await getWorld(world)
    return {
      work: work(w.work),
      premise: w.reader_intro?.premise ?? w.work.intro,
      setting: w.world_setting || w.reader_intro?.setting,
      stories: w.storylines.map((s) => ({
        title: s.title,
        chapters: s.chapters.map(chapter),
      })),
      characters: w.characters.map(person),
      scenes: w.scenes.map(scene),
    }
  }
  if (kind === 'characters') {
    const p = await worldCharacter(world, slug)
    return {
      person: person(p.character),
      intro: p.intro,
      chapters: p.appearances.map(chapter),
      scenes: p.scene_previews.map(scene),
    }
  }
  if (kind === 'scenes') {
    const s = await worldScene(world, slug)
    return {
      scene: scene(s.scene),
      text: s.scene.text,
      chapter: chapter(s.chapter),
      anchor: s.return_to.href.split('#')[1],
      cast: s.scene_cast.map(person),
    }
  }
  const c = await worldChapter(world, slug)
  const blocks =
    c.reader_passages ??
    (c.research
      ? [
          ...c.research.paragraphs.map((p, i) => ({
            key: 'research-' + (i + 1),
            text: p.text,
            anchors: [],
          })),
          ...c.blocks.filter((b) => b.anchors.length),
        ]
      : c.blocks)
  const branch = world === 'tidepost' ? catalog.find(story => story.slug === slug) : undefined
  return {
    play: branch && {scene:branch.scene, options:branch.options, replies:branch.replies, image:artUrl(branch)},
    worldTitle: c.work.title,
    chapter: chapter(c.chapter),
    blocks: blocks.map(({ key, text, anchors }) => ({ key, text, anchors })),
    illustrations: (c.accepted_art ?? []).map(({ block, image, name }) => ({
      block,
      image: imageUrl(image),
      name,
    })),
    scenes: c.scenes.map(scene),
    next: c.next_chapter && chapter(c.next_chapter),
  }
}

export async function iosReaderResponse(path: string) {
  try {
    return Response.json(await read(path), {
      headers: {
        'Cache-Control': 'public, max-age=60',
        'X-Robots-Tag': 'noindex',
      },
    })
  } catch (e) {
    return new Response(null, {
      status: e instanceof KnowledgeError && e.status === 404 ? 404 : 503,
    })
  }
}
