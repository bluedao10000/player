import { useEffect, useState } from 'react'
import {
  chapterPath,
  readLocal,
  useBrowserRecord,
  writeLocal,
} from '../reader/bookmarks'
import {
  accountSnapshot,
  lastPosition,
  request,
  useAccount,
  type Position,
} from '../reader/account'
import { catalog, storyUrl } from './catalog'
import { Icon } from './icons'

const lastKey = 'tidepost.last-read.v1'
function position(slug: string) {
  const [n, id, excerpt] = readLocal('tidepost.progress.' + slug)
  return {
    percent: Math.max(0, Math.min(100, Number(n) || 0)),
    anchor: id && /^[\w-]+$/.test(id) ? id : '',
    excerpt: excerpt || '',
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
    work: work || (story ? '潮汐邮局' : ''),
    percent,
    resume: href + '#' + (anchor || 'resume'),
  }
}
export function guestPositions(
  chapters: { href: string; title: string; work: string }[],
): Position[] {
  const rows: Position[] = []
  const directory = new Map(chapters.map((c) => [c.href, c]))
  const recent = readLastChapter()
  const prefix = 'tidepost.progress.'
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (!key?.startsWith(prefix)) continue
      const slug = key.slice(prefix.length)
      const story = catalog.find((s) => s.slug === slug)
      const href = story ? storyUrl(story) : slug
      if (!chapterPath.test(href)) continue
      const [, , , stamp, oldTitle, oldWork] = readLocal(key)
      const meta = directory.get(href)
      rows.push({
        href,
        title:
          meta?.title || oldTitle || story?.title || href.split('/').at(-1)!,
        work:
          meta?.work || oldWork || (story ? '潮汐邮局' : href.split('/')[2]),
        ...position(slug),
        recordedAt: Number(stamp) || 0,
      })
    }
  } catch {
    /* Keep readable records if storage is restricted. */
  }
  // The legacy latest pointer also covers chapters opened without scrolling.
  if (recent) {
    const existing = rows.find((p) => p.href === recent.href)
    if (!existing)
      rows.push({
        ...recent,
        anchor: recent.resume.split('#')[1].replace(/^resume$/, ''),
        excerpt: recent.excerpt || '',
        recordedAt: 1,
      })
    else if (!existing.recordedAt) existing.recordedAt = 1
  }
  return rows
}
export function useLastChapter() {
  const reader = useAccount()
  const local = useBrowserRecord(readLastChapter)
  return reader === null
    ? local
    : reader && reader !== 'error'
      ? lastPosition(reader)
      : undefined
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
  const reader = useAccount()
  const actor =
    reader === null
      ? null
      : reader && reader !== 'error' && !reader.pending
        ? reader.id
        : undefined
  const [syncError, setSyncError] = useState(false)
  useEffect(() => {
    if (actor === undefined) return
    let timer: ReturnType<typeof setTimeout> | undefined
    let active = true
    let sequence = 0
    const prose = document.getElementById(containerId)
    if (!prose) return
    const key = 'tidepost.progress.' + slug
    const cloud = accountSnapshot()
    const previous =
      cloud && cloud !== 'error'
        ? (cloud.positions.find((p) => p.href === href) ?? {
            percent: 0,
            anchor: '',
            excerpt: '',
          })
        : position(slug)
    let lastSaved = ''
    const persist = (percent: number, anchor: string, excerpt: string) => {
      const now = accountSnapshot()
      if (
        actor === null
          ? now !== null
          : !now || now === 'error' || now.id !== actor
      )
        return
      const signature = JSON.stringify([percent, anchor, excerpt])
      if (signature === lastSaved) return
      lastSaved = signature
      const recordedAt = Date.now()
      if (!actor) {
        writeLocal(key, [
          String(percent),
          anchor,
          excerpt,
          String(recordedAt),
          title,
          work,
        ])
        writeLocal(lastKey, [slug, href, title, work])
      } else {
        const serial = ++sequence
        void request(
          'progress',
          { href, title, work, percent, anchor, excerpt, recordedAt },
          actor,
          true,
        )
          .then(() => {
            if (active && serial === sequence) setSyncError(false)
          })
          .catch(() => {
            if (active && serial === sequence) {
              lastSaved = ''
              setSyncError(true)
            }
          })
      }
    }
    const start = () =>
      Math.max(
        0,
        prose.getBoundingClientRect().top +
          window.scrollY -
          window.innerHeight * 0.4,
      )
    const span = () => Math.max(1, prose.offsetHeight)
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
      clearTimeout(timer)
      timer = setTimeout(save, 350)
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
      persist(percentage(), block?.id || '', excerpt)
    }
    function visibility() {
      if (document.visibilityState === 'hidden') save()
    }
    setValue(percentage())
    window.addEventListener('scroll', update, { passive: true })
    window.addEventListener('pagehide', save)
    if (paragraphs.some((p) => p.id === requested)) save()
    else persist(previous.percent, previous.anchor, previous.excerpt || '')
    document.addEventListener('visibilitychange', visibility)
    return () => {
      clearTimeout(timer)
      save()
      active = false
      window.removeEventListener('scroll', update)
      window.removeEventListener('pagehide', save)
      document.removeEventListener('visibilitychange', visibility)
    }
  }, [slug, href, title, work, containerId, actor])
  return (
    <>
      {syncError && (
        <button
          className="reader-retry"
          onClick={() => window.dispatchEvent(new Event('scroll'))}
        >
          进度同步失败 · 重试
        </button>
      )}
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
    </>
  )
}
export function ContinueReading() {
  const last = useLastChapter()
  if (!last) return null
  return (
    <a className="continue" href={last.resume}>
      <Icon name="bookmark" />
      <div>
        <small>{['继续阅读', last.work].filter(Boolean).join(' · ')}</small>
        <strong>{last.title}</strong>
      </div>
      <span>{last.percent}%</span>
      <Icon name="arrow" />
    </a>
  )
}
