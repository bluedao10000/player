import { useEffect, useState } from 'react'
import { readLocal, writeLocal } from './storage'
import { catalog, storyUrl } from './catalog'
import { Icon } from './icons'

const lastKey = 'tidepost.last-read.v1'
function percent(slug: string) {
  return Math.max(
    0,
    Math.min(100, Number(readLocal('tidepost.progress.' + slug)[0]) || 0),
  )
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
    writeLocal(lastKey, [slug, href, title, work])
    const start = () =>
      Math.max(
        0,
        prose.getBoundingClientRect().top +
          window.scrollY -
          window.innerHeight * 0.4,
      )
    const span = () => Math.max(1, prose.offsetHeight)
    let current = percent(slug)
    let moved = false
    if (location.hash === '#resume')
      window.scrollTo(0, start() + (span() * current) / 100)
    function update() {
      moved = true
      current = Math.round(
        Math.max(0, Math.min(1, (window.scrollY - start()) / span())) * 100,
      )
      setValue(current)
    }
    function save() {
      if (moved) writeLocal(key, [String(current)])
    }
    setValue(
      Math.round(
        Math.max(0, Math.min(1, (window.scrollY - start()) / span())) * 100,
      ),
    )
    window.addEventListener('scroll', update, { passive: true })
    window.addEventListener('pagehide', save)
    return () => {
      save()
      window.removeEventListener('scroll', update)
      window.removeEventListener('pagehide', save)
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
  const [record, setRecord] = useState<string[]>([])
  useEffect(() => {
    const sync = () => setRecord(readLocal(lastKey))
    sync()
    window.addEventListener('pageshow', sync)
    return () => window.removeEventListener('pageshow', sync)
  }, [])
  const [slug, path, name, work] = record
  const story = catalog.find((s) => s.slug === slug)
  const href =
    path &&
    /^\/(?:story\/[a-z0-9-]+|work\/[a-z0-9-]+\/chapter\/[a-z0-9-]+)$/.test(path)
      ? path
      : story && storyUrl(story)
  const title = name || story?.title
  if (!title || !href) return null
  return (
    <a className="continue" href={href + '#resume'}>
      <Icon name="bookmark" />
      <div>
        <small>继续阅读 · {work || '潮汐邮局'}</small>
        <strong>{title}</strong>
      </div>
      <span>{percent(slug)}%</span>
      <Icon name="arrow" />
    </a>
  )
}
