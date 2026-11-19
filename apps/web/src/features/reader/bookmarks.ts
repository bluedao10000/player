import { useEffect, useState } from 'react'
import { catalog, storyUrl } from '../post/catalog'

export const legacyShelf = 'tidepost.shelf.v1'
const prefix = 'folio.bookmark.v1:'
const changed = 'folio-bookmarks'
export const chapterPath =
  /^\/(?:story\/[a-z0-9-]+|work\/[a-z0-9-]+\/chapter\/[a-z0-9-]+)$/

export function readLocal(key: string): string[] {
  try {
    const data: unknown = JSON.parse(localStorage.getItem(key) ?? '[]')
    return Array.isArray(data)
      ? data.filter((v): v is string => typeof v === 'string')
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

// Each chapter has a separate key so two tabs can save different works independently.
// A removal overrides the original slug without rewriting the old shelf.
export type Bookmark = { href: string; title: string; work: string }
export function readBookmarks() {
  const items = new Map<string, Bookmark & { order: number }>()
  readLocal(legacyShelf).forEach((slug, order) => {
    const story = catalog.find((s) => s.slug === slug)
    if (story) {
      const href = storyUrl(story)
      items.set(href, { href, title: story.title, work: '潮汐邮局', order })
    }
  })
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i)
      if (!key?.startsWith(prefix)) continue
      const href = key.slice(prefix.length)
      if (!chapterPath.test(href)) continue
      const [stamp, title, work] = readLocal(key)
      if (stamp === 'removed') items.delete(href)
      else if (title && work && Number.isFinite(Number(stamp))) {
        items.set(href, { href, title, work, order: Number(stamp) })
      }
    }
  } catch {
    /* Some visitors disable browser storage. */
  }
  return [...items.values()].sort((a, b) => b.order - a.order)
}

export function setBookmark(item: Bookmark, saved: boolean) {
  const key = prefix + item.href
  const isLegacy =
    item.href.startsWith('/story/') &&
    readLocal(legacyShelf).includes(item.href.slice(7))
  try {
    if (saved)
      localStorage.setItem(
        key,
        JSON.stringify([String(Date.now()), item.title, item.work]),
      )
    else if (isLegacy) localStorage.setItem(key, JSON.stringify(['removed']))
    else localStorage.removeItem(key)
    window.dispatchEvent(new Event(changed))
    return true
  } catch {
    return false
  }
}

// Read private state only after hydration. Recheck when a cached tab is restored.
export function useBrowserRecord<T>(read: () => T) {
  const [value, setValue] = useState<T | undefined>(undefined)
  useEffect(() => {
    const sync = () => setValue(read())
    sync()
    window.addEventListener('storage', sync)
    window.addEventListener(changed, sync)
    window.addEventListener('pageshow', sync)
    return () => {
      window.removeEventListener('storage', sync)
      window.removeEventListener(changed, sync)
      window.removeEventListener('pageshow', sync)
    }
  }, [read])
  return value
}
