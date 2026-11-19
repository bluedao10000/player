import type {
  ChapterDetail,
  CharacterDetail,
  SceneDetail,
  WorldOverview,
} from '@anime/shared/knowledge'

export type ProfilePage = CharacterDetail | WorldOverview
export type EditorPages =
  | { chapter: ChapterDetail; scenes: Record<string, SceneDetail> }
  | { profile_pages: Record<string, ProfilePage> }

export function profileLabel(page: ProfilePage) {
  return 'character' in page ? page.character.name + ' · 人物介绍' : '世界观'
}
export function profileHref(page: ProfilePage) {
  return 'character' in page ? page.character.href : page.work.href
}
export function editedField(base: EditorPages, key: string) {
  if ('profile_pages' in base) {
    const page = base.profile_pages[key]
    const before =
      'character' in page
        ? page.intro
        : (page.reader_intro?.setting ?? page.world_setting)
    return {
      label: profileLabel(page),
      before: before || '此处尚无正文',
      href: profileHref(page),
    }
  }
  const id = key.split('/')[1]
  if (key.startsWith('scene/')) {
    const { scene } = base.scenes[id]
    return {
      label: '场景 · ' + scene.title,
      before: scene.text,
      href: scene.href,
    }
  }
  const blocks = base.chapter.reader_passages ?? base.chapter.blocks
  const i = blocks.findIndex((b) => b.key === id)
  return {
    label: '章节 · 第 ' + (i + 1) + ' 段',
    before: blocks[i]?.text,
    href: base.chapter.chapter.href,
  }
}
