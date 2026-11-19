import { siteName } from '../../site'
import type { SceneCard } from '@anime/shared/knowledge'
import type { ReactNode } from 'react'
import { artUrl, storyUrl } from './catalog'
import type { Story } from './catalog'
import { Icon } from './icons'
import type { IconName } from './icons'

const nav = [
  ['发现', '/', 'compass'],
  ['作品', '/works', 'map'],
  ['角色', '/characters', 'people'],
  ['故事', '/stories', 'book'],
  ['我的', '/me', 'user'],
] as const
function NavLink({
  item,
  active,
}: {
  item: (typeof nav)[number]
  active: string
}) {
  const [label, url, icon] = item
  return (
    <a href={url} aria-current={active === label ? 'page' : undefined}>
      <Icon name={icon} size={19} />
      {label}
    </a>
  )
}
export function BottomNavigation({ active }: { active: string }) {
  return (
    <nav className="bottom-nav glass" aria-label="移动端主导航">
      {nav.map((item) => (
        <NavLink key={item[0]} item={item} active={active} />
      ))}
    </nav>
  )
}
export function Layout({
  active,
  children,
}: {
  active: string
  children: ReactNode
}) {
  return (
    <>
      <a className="skip" href="#main">
        跳到正文
      </a>
      <header className="mast">
        <div className="wrap mast-inner">
          <a className="brand" href="/" aria-label={siteName + '首页'}>
            <span className="brand-mark">
              <Icon name="plane" size={30} />
            </span>
            <b>{siteName}</b>
          </a>
          <nav className="top-nav" aria-label="主导航">
            {nav.slice(0, -1).map((item) => (
              <NavLink key={item[0]} item={item} active={active} />
            ))}
          </nav>
          <a
            className="shelf-link"
            aria-label="我的"
            href="/me"
            aria-current={active === '我的' ? 'page' : undefined}
          >
            <Icon name="user" size={17} />
            <span>我的</span>
          </a>
        </div>
      </header>
      <main id="main" className="wrap">
        {children}
      </main>
      <footer className="wrap foot">
        <Icon name="plane" />
        {siteName}
      </footer>
      <BottomNavigation active={active} />
    </>
  )
}
export function Button({
  href,
  children,
}: {
  href: string
  children: ReactNode
}) {
  return (
    <a className="button" href={href}>
      {children}
      <Icon name="arrow" size={17} />
    </a>
  )
}
export function IconButton({
  label,
  icon,
  onClick,
}: {
  label: string
  icon: IconName
  onClick: () => void
}) {
  return (
    <button className="icon-btn" aria-label={label} onClick={onClick}>
      <Icon name={icon} size={16} />
    </button>
  )
}
export function Tag({ children }: { children: ReactNode }) {
  return <span className="tag">{children}</span>
}
export function SectionHeader({
  children,
  href,
  link,
}: {
  children: ReactNode
  href?: string
  link?: string
}) {
  return (
    <header className="section-head">
      <h2>{children}</h2>
      {href && (
        <a href={href}>
          {link}
          <Icon name="forward" size={14} />
        </a>
      )}
    </header>
  )
}
export const people = [
  {
    id: 'mio',
    name: '阿澪',
    role: '夜班邮差',
    image: 'mio',
    href: '/character/mio',
    story: 'night-courier',
  },
  {
    id: 'captain',
    name: '渡船船长',
    role: '旧渡口的引路人',
    image: 'captain',
    href: '/work/tidepost/character/captain',
    story: 'last-ferry',
  },
  {
    id: 'keeper',
    name: '守塔人',
    role: '北岬的守望者',
    image: 'keeper',
    href: '/character/mio#keeper',
    story: 'lighthouse',
  },
] as const
export function Avatar({ image }: { image: string }) {
  return (
    <img
      className="avatar"
      src={'/art/' + image + '.webp'}
      alt=""
      width="40"
      height="40"
    />
  )
}
export function CharacterChip() {
  return (
    <a className="chip" href="/character/mio">
      <Avatar image="mio" />
      阿澪
      <Icon name="forward" size={13} />
    </a>
  )
}
export function ContentMeta({ story }: { story: Story }) {
  return (
    <span className="meta">
      <Icon name="pin" size={14} />
      {story.location}
      <i />约 1 分钟
    </span>
  )
}
export function StoryPreview({ story }: { story: Story }) {
  return (
    <article className="story-preview">
      <a href={storyUrl(story)}>
        <img
          src={artUrl(story)}
          alt={story.alt}
          width="766"
          height="510"
          loading="lazy"
        />
        <div>
          <small>{story.category}</small>
          <h3>{story.title}</h3>
          <p>{story.intro}</p>
          <ContentMeta story={story} />
        </div>
      </a>
    </article>
  )
}
export function ScenePreview({
  story,
  scene,
}: {
  story: Story
  scene?: SceneCard
}) {
  return (
    <a
      className="scene-preview"
      href={scene?.href ?? storyUrl(story) + '#scene'}
    >
      <img src={artUrl(story)} alt="" loading="lazy" />
      <div>
        <small>{story.location}</small>
        <h3>{scene?.title ?? story.title}</h3>
      </div>
      <Icon name="forward" />
    </a>
  )
}
export function WorldPreview({ story }: { story: Story }) {
  return (
    <a className="world-preview" href={storyUrl(story)}>
      <Icon name="map" />
      <span>{story.location}</span>
      <Icon name="right" size={14} />
    </a>
  )
}
export function GlassSurface({ children }: { children: ReactNode }) {
  return <div className="glass scene-callout">{children}</div>
}
