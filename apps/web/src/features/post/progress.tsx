import { useEffect, useState } from 'react'
import {
  chapterPath,
  readLocal,
  useBrowserRecord,
  writeLocal,
} from '../reader/bookmarks'
import { catalog, storyUrl } from './catalog'
import { Icon } from './icons'

const lastKey = 'tidepost.last-read.v1'
function position(slug: string) {
  const [n, id, excerpt] = readLocal('tidepost.progress.' + slug)
  return {
    percent: Math.max(0, Math.min(100, Number(n) || 0)),
    anchor: id && /^[\w-]+$/.test(id) ? id : '',
    excerpt,
  }
}
export function readLastChapter() {
  const [slug, path, name, work] = readLocal(lastKey)
  const story = catalog.find((s) => s.slug === slug)
  const href = path && chapterPath.test(path) ? path : story && storyUrl(story)
  const title = name || story?.title
  if (!href || !title) return null
  const { percent, anchor, excerpt } = position(slug)
  return {
    href,
    title,
    excerpt,
    work: work || '潮汐邮局',
    percent,
    resume: href + '#' + (anchor || 'resume'),
  }
}
export function ReadingProgress({
  slug,
  href,
  title,
  work,
  containerId = 'prose',
}: {
  slug: string
  href: string
  title: string
  work: string
  containerId?: string
}) {
  const [value, setValue] = useState(0)
  useEffect(() => {
    const prose = document.getElementById(containerId)
    if (!prose) return
    const key = 'tidepost.progress.' + slug
    const mark = () => writeLocal(lastKey, [slug, href, title, work])
    mark()
    const start = () =>
      Math.max(
        0,
        prose.getBoundingClientRect().top +
          window.scrollY -
          window.innerHeight * 0.4,
      )
    const span = () => Math.max(1, prose.offsetHeight)
    const previous = position(slug)
    const requested = location.hash.slice(1)
    if (
      requested === 'resume' ||
      (requested &&
        requested === previous.anchor &&
        !document.getElementById(requested))
    ) {
      const target = previous.anchor && document.getElementById(previous.anchor)
      if (target) target.scrollIntoView()
      else window.scrollTo(0, start() + (span() * previous.percent) / 100)
    }
    const paragraphs = Array.from(
      prose.querySelectorAll<HTMLElement>('p[data-block][id]'),
    )
    let moved = false
    const percentage = () =>
      Math.round(
        Math.max(0, Math.min(1, (window.scrollY - start()) / span())) * 100,
      )
    function update() {
      moved = true
      setValue(percentage())
    }
    function save() {
      if (!moved && !paragraphs.some((p) => p.id === requested)) return
      const line = window.innerHeight * 0.4
      const linked = paragraphs.find((p) => p.id === requested)
      const atLinked =
        linked && Math.abs(linked.getBoundingClientRect().top) < 48
      const block =
        (!moved || atLinked) && linked
          ? linked
          : paragraphs
              .filter((p) => p.getBoundingClientRect().top <= line)
              .at(-1)
      const text = block?.textContent?.trim() ?? ''
      const excerpt = text.length > 40 ? text.slice(0, 40) + '…' : text
      writeLocal(key, [String(percentage()), block?.id || '', excerpt])
    }
    function visibility() {
      if (document.visibilityState === 'hidden') save()
      else mark()
    }
    setValue(percentage())
    window.addEventListener('scroll', update, { passive: true })
    window.addEventListener('pagehide', save)
    window.addEventListener('pageshow', mark)
    document.addEventListener('visibilitychange', visibility)
    return () => {
      save()
      window.removeEventListener('scroll', update)
      window.removeEventListener('pagehide', save)
      window.removeEventListener('pageshow', mark)
      document.removeEventListener('visibilitychange', visibility)
    }
  }, [slug, href, title, work, containerId])
  return (
    <div
      className="progress"
      role="progressbar"
      aria-label="阅读进度"
      aria-valuenow={value}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <span style={{ width: value + '%' }} />
    </div>
  )
}
export function ContinueReading() {
  const last = useBrowserRecord(readLastChapter)
  if (!last) return null
  return (
    <a className="continue" href={last.resume}>
      <Icon name="bookmark" />
      <div>
        <small>继续阅读 · {last.work}</small>
        <strong>{last.title}</strong>
      </div>
      <span>{last.percent}%</span>
      <Icon name="arrow" />
    </a>
  )
}
