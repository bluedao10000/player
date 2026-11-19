import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import {
  profileHref,
  profileLabel,
  type EditorPages,
  type ProfilePage,
} from '../features/workbench/editor-pages'
import { Character } from '../features/post/character'
import characterCss from '../features/post/character.css?url'
import { Reader } from '../features/post/editorial'
import {
  ChapterPage,
  ScenePage,
  PersonPage,
  WorkPage,
} from '../features/worlds/pages'
import worldCss from '../features/worlds/world.css?url'

export const Route = createFileRoute('/editor-preview/$id')({
  head: () => ({
    meta: [
      { title: '手机审稿预览' },
      { name: 'robots', content: 'noindex,nofollow' },
    ],
    links: [
      { rel: 'stylesheet', href: worldCss },
      { rel: 'stylesheet', href: characterCss },
    ],
  }),
  component: Preview,
})
function Preview() {
  const { id } = Route.useParams()
  const [data, setData] = useState<EditorPages | null>(null)
  const [error, setError] = useState('')
  useEffect(() => {
    const params = new URLSearchParams(location.search),
      controller = new AbortController()
    fetch(
      '/editor-api/tasks/' +
        id +
        '/preview?' +
        new URLSearchParams({
          round: params.get('round') ?? '',
          attempt: params.get('attempt') ?? '',
        }),
      { cache: 'no-store', signal: controller.signal },
    )
      .then(async (r) => {
        if (!r.ok)
          throw Error(
            r.status === 401
              ? '请先登录编辑工作台，再打开此预览。'
              : '稿件版本已变化，请从任务重新预览。',
          )
        setData(await r.json())
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(e.message)
      })
    return () => controller.abort()
  }, [id])
  useEffect(() => {
    if (data && location.hash)
      document.getElementById(location.hash.slice(1))?.scrollIntoView()
  }, [data])
  if (!data) return <p role="status">{error || '正在打开手机预览…'}</p>
  if ('profile_pages' in data)
    return <ProfilePreview pages={data.profile_pages} />
  const scene =
    data.scenes[new URLSearchParams(location.search).get('scene') ?? '']
  return (
    <div
      onClickCapture={(e) => {
        const link = (e.target as Element).closest('a')
        if (!link) return
        const dest = new URL(link.href, location.href)
        const target = Object.entries(data.scenes).find(
          ([, s]) => s.scene.href === dest.pathname,
        )
        if (target || dest.pathname === data.chapter.chapter.href) {
          e.preventDefault()
          const u = new URL(location.href)
          if (target) u.searchParams.set('scene', target[0])
          else u.searchParams.delete('scene')
          u.hash = dest.hash
          location.assign(u.href)
        }
      }}
    >
      {scene ? (
        <ScenePage {...scene} />
      ) : data.chapter.work.origin === 'original' ? (
        <Reader page={data.chapter} />
      ) : (
        <ChapterPage {...data.chapter} />
      )}
    </div>
  )
}

function ProfilePreview({ pages }: { pages: Record<string, ProfilePage> }) {
  const url = new URL(location.href)
  const current = url.searchParams.get('page') ?? 'world/setting'
  const page = pages[current] ?? pages['world/setting']
  const linkTo = (key: string, hash: string) => {
    const next = new URL(url)
    next.searchParams.set('page', key)
    next.hash = hash
    return next.href
  }
  return (
    <>
      <nav className="editor-preview-tabs" aria-label="审稿预览页面">
        {Object.entries(pages).map(([key, p]) => (
          <a
            key={key}
            aria-current={current === key ? 'page' : undefined}
            href={linkTo(
              key,
              key === 'world/setting' ? 'world-setting' : 'character-intro',
            )}
          >
            {profileLabel(p)}
          </a>
        ))}
      </nav>
      <div
        onClickCapture={(e) => {
          const anchor = (e.target as Element).closest('a')
          if (!anchor) return
          const destination = new URL(anchor.href, location.href)
          const target = Object.entries(pages).find(
            ([, p]) => profileHref(p) === destination.pathname,
          )
          if (target) {
            e.preventDefault()
            location.assign(linkTo(target[0], destination.hash))
          }
        }}
      >
        {'character' in page ? (
          page.character.href === '/character/mio' ? (
            <Character {...page} />
          ) : (
            <PersonPage {...page} />
          )
        ) : (
          <WorkPage {...page} />
        )}
      </div>
    </>
  )
}
