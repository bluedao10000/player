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
export function ReadingProgress({ slug }: { slug: string }) {
  const [value, setValue] = useState(0)
  useEffect(() => {
    const prose = document.getElementById('prose')
    if (!prose) return
    const key = 'tidepost.progress.' + slug
    writeLocal(lastKey, [slug])
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
  }, [slug])
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
  const [slug, setSlug] = useState('')
  useEffect(() => {
    const sync = () => setSlug(readLocal(lastKey)[0] ?? '')
    sync()
    window.addEventListener('pageshow', sync)
    return () => window.removeEventListener('pageshow', sync)
  }, [])
  const story = catalog.find((s) => s.slug === slug)
  if (!story) return null
  return (
    <a className="continue" href={storyUrl(story) + '#resume'}>
      <Icon name="bookmark" />
      <div>
        <small>继续阅读</small>
        <strong>{story.title}</strong>
      </div>
      <span>{percent(story.slug)}%</span>
      <Icon name="arrow" />
    </a>
  )
}
