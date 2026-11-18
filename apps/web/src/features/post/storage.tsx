import { useEffect, useState } from 'react'
import { Icon } from './icons'
import type { Story } from './catalog'
export const shelfKey = 'tidepost.shelf.v1'
export function readLocal(key: string): string[] {
  try {
    const v: unknown = JSON.parse(localStorage.getItem(key) ?? '[]')
    return Array.isArray(v)
      ? v.filter((x): x is string => typeof x === 'string')
      : []
  } catch {
    return []
  }
}
export function writeLocal(key: string, items: string[]) {
  try {
    localStorage.setItem(key, JSON.stringify(items))
    return true
  } catch {
    return false
  }
}
export function Save({ story }: { story: Story }) {
  const [saved, setSaved] = useState(false),
    [error, setError] = useState(false)
  useEffect(
    () => setSaved(readLocal(shelfKey).includes(story.slug)),
    [story.slug],
  )
  function toggle() {
    const current = readLocal(shelfKey)
    const next = current.includes(story.slug)
      ? current.filter((x) => x !== story.slug)
      : [...current, story.slug]
    const ok = writeLocal(shelfKey, next)
    setError(!ok)
    if (ok) setSaved(next.includes(story.slug))
  }
  return (
    <button className="save" aria-pressed={saved} onClick={toggle}>
      <Icon name={saved ? 'check' : 'bookmark'} />
      {error ? '保存失败，重试' : saved ? '已加入书架' : '加入书架'}
    </button>
  )
}
