import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import type { ChapterDetail, SceneDetail } from '@anime/shared/knowledge'
import { Reader } from '../features/post/editorial'
import { ChapterPage, ScenePage } from '../features/worlds/pages'
import worldCss from '../features/worlds/world.css?url'

export const Route = createFileRoute('/editor-preview/$id')({
  head: () => ({
    meta: [
      { title: '手机审稿预览' },
      { name: 'robots', content: 'noindex,nofollow' },
    ],
    links: [{ rel: 'stylesheet', href: worldCss }],
  }),
  component: Preview,
})
function Preview() {
  const { id } = Route.useParams()
  const [data, setData] = useState<{
    chapter: ChapterDetail
    scenes: Record<string, SceneDetail>
  } | null>(null)
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
