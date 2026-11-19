import { useEffect, useSyncExternalStore } from 'react'
import {
  readBookmarks,
  setBookmark,
  useBrowserRecord,
  type Bookmark,
} from './bookmarks'

export type Position = Bookmark & {
  anchor: string
  percent: number
  excerpt: string
  recordedAt: number
}
type Reader = {
  id: string
  email: string
  pending: boolean
  bookmarks: Bookmark[]
  positions: Position[]
}
type Account = Reader | null | undefined | 'error'
let current: Account
let revision = 0
const listeners = new Set<() => void>()
const channel = 'folio.identity-change'
function notify(value: Account) {
  current = value
  listeners.forEach((l) => l())
}
const subscribe = (fn: () => void) => {
  listeners.add(fn)
  return () => {
    listeners.delete(fn)
  }
}
export async function refresh() {
  const version = ++revision
  try {
    const value = (await request('state')) as Reader | null
    if (version === revision) notify(value)
  } catch {
    if (version === revision) notify('error')
  }
}
export function identityChanged() {
  notify(undefined)
  try {
    localStorage.setItem(channel, crypto.randomUUID())
  } catch {}
  return refresh()
}
let mounts = 0
export function useAccount() {
  const state = useSyncExternalStore(
    subscribe,
    () => current,
    () => undefined,
  )
  useEffect(() => {
    if (mounts++ === 0) {
      void refresh()
      window.addEventListener('pageshow', restore)
      window.addEventListener('storage', changed)
    }
    return () => {
      if (--mounts === 0) {
        window.removeEventListener('pageshow', restore)
        window.removeEventListener('storage', changed)
      }
    }
  }, [])
  return state
}
function changed(e: StorageEvent) {
  if (e.key === channel) {
    notify(undefined)
    void refresh()
  }
}
function restore(e: PageTransitionEvent) {
  if (e.persisted) {
    notify(undefined)
    void refresh()
  }
}
export async function request(
  path: string,
  data?: unknown,
  reader?: string,
  keepalive = false,
) {
  const response = await fetch('/reader-auth/' + path, {
    method: data === undefined ? 'GET' : 'POST',
    credentials: 'same-origin',
    cache: 'no-store',
    headers: {
      'Content-Type': 'application/json',
      'X-Reader-ID': reader ?? '',
    },
    body: data === undefined ? undefined : JSON.stringify(data),
    keepalive,
  })
  if (!response.ok) {
    if (reader && (response.status === 401 || response.status === 409))
      void refresh()
    const result = await response.json().catch(() => ({}))
    throw new Error(result?.error || '暂时无法连接读者服务，请重试。')
  }
  return response.status === 204 ? null : response.json()
}
export function useShelf() {
  const reader = useAccount()
  const local = useBrowserRecord(readBookmarks)
  const items =
    reader === null
      ? local
      : reader && reader !== 'error'
        ? reader.bookmarks
        : undefined
  async function save(item: Bookmark, saved: boolean) {
    if (reader === null) return setBookmark(item, saved)
    if (!reader || reader === 'error') return false
    try {
      await request('bookmark', { ...item, saved }, reader.id)
      await refresh()
      return true
    } catch {
      return false
    }
  }
  return { items, save, reader }
}
export function accountSnapshot() {
  return current
}
export function lastPosition(reader: Reader) {
  const last = reader.positions[0]
  return last
    ? { ...last, resume: last.href + '#' + (last.anchor || 'resume') }
    : null
}
