import { createFileRoute } from '@tanstack/react-router'
import { Character } from '../features/post/character'
import characterCss from '../features/post/character.css?url'

export const Route = createFileRoute('/character/mio')({
  head: () => ({
    links: [{ rel: 'stylesheet', href: characterCss }],
    meta: [
      { title: '阿澪 · 潮汐邮局' },
      {
        name: 'description',
        content:
          '认识白帆港的夜班邮差阿澪，阅读她出场的故事，和她一起进入剧情。',
      },
    ],
  }),
  component: Character,
})
