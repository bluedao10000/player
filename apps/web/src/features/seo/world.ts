import { imageUrl } from '../../lib/images'
import type { ChapterDetail, CharacterDetail, SceneDetail, WorldOverview } from '@anime/shared/knowledge'
import { siteOrigin, siteTitle } from '../../site'

const language = 'zh-CN'

export function publicUrl(href: string) {
  return siteOrigin + href
}

const ref = (href: string) => ({ '@id': publicUrl(href) })


function worldEntity(work: WorldOverview['work']) {
  return {
    '@type': work.origin === 'original' ? 'CreativeWork'
      : work.format === 'film' ? 'Movie' : 'CreativeWorkSeries',
    ...ref(work.href + '#work'),
    url: publicUrl(work.href),
    name: work.title,
    description: work.intro,
    image: imageUrl(work.hero_asset_key),
  }
}

function pageHead(
  href: string,
  title: string,
  description: string,
  crumbs: { name: string; href: string }[],
  entity: Record<string, unknown>,
  image?: string,
) {
  const url = publicUrl(href)
  const breadcrumb = {
    '@type': 'BreadcrumbList',
    '@id': url + '#breadcrumb',
    itemListElement: [{ name: '首页', href: '/' }, ...crumbs].map(({ name, href }, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      name,
      item: publicUrl(href),
    })),
  }
  return {
    links: [{ rel: 'canonical', href: url }],
    meta: [
      { title },
      { name: 'description', content: description },
      { property: 'og:url', content: url },
      { property: 'og:title', content: title },
      { property: 'og:description', content: description },
      ...(image ? [{ property: 'og:image', content: image }] : []),
      { 'script:ld+json': {
        '@context': 'https://schema.org',
        '@graph': [
          {
            '@type': 'WebPage',
            '@id': url,
            url,
            name: title,
            description,
            inLanguage: language,
            breadcrumb: { '@id': breadcrumb['@id'] },
            mainEntity: { '@id': entity['@id'] },
          },
          entity,
          breadcrumb,
        ],
      } },
    ],
  }
}

export function workHead({ work, reader_intro }: WorldOverview) {
  const description = reader_intro?.premise || work.intro
  return pageHead(work.href, siteTitle(work.title + ' · 作品世界'), description,
    [{ name: work.title, href: work.href }], worldEntity(work), imageUrl(work.hero_asset_key))
}

export function characterHead({ work, character, intro }: CharacterDetail) {
  const description = intro || character.role
  const image = imageUrl(character.portrait)
  return pageHead(character.href, siteTitle(character.name + ' · ' + work.title), description,
    [{ name: work.title, href: work.href }, { name: character.name, href: character.href }], {
      '@type': 'Person',
      ...ref(character.href + '#character'),
      name: character.name,
      description,
      image,
      subjectOf: ref(work.href + '#work'),
    }, image)
}

export function chapterHead({ work, chapter, accepted_art }: ChapterDetail) {
  const image = imageUrl(accepted_art?.[0]?.image || chapter.image)
  return pageHead(chapter.href, siteTitle(chapter.title + ' · ' + work.title), chapter.intro,
    [{ name: work.title, href: work.href }, { name: chapter.title, href: chapter.href }], {
      '@type': 'Article',
      ...ref(chapter.href + '#article'),
      headline: chapter.title,
      description: chapter.intro,
      inLanguage: language,
      image,
      about: chapter.episode ? {
        '@type': 'Episode',
        name: chapter.title,
        episodeNumber: chapter.episode,
        partOfSeries: ref(work.href + '#work'),
      } : ref(work.href + '#work'),
    }, image)
}

export function sceneHead({ work, chapter, scene, return_to }: SceneDetail) {
  const image = imageUrl(scene.image)
  return pageHead(scene.href, siteTitle(scene.title + ' · ' + work.title), scene.text,
    [
      { name: work.title, href: work.href },
      { name: chapter.title, href: chapter.href },
      { name: scene.title, href: scene.href },
    ], {
      '@type': 'CreativeWork',
      ...ref(scene.href + '#scene'),
      name: scene.title,
      description: scene.text,
      inLanguage: language,
      image,
      isPartOf: ref(chapter.href + '#article'),
      about: ref(work.href + '#work'),
      url: publicUrl(scene.href),
      citation: publicUrl(return_to.href),
    }, image)
}
