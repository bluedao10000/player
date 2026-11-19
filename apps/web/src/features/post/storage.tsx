import { useState } from 'react'
import { Icon } from './icons'
import {
  readBookmarks,
  setBookmark,
  useBrowserRecord,
  type Bookmark,
} from '../reader/bookmarks'

export function Save(item: Bookmark) {
  const items = useBrowserRecord(readBookmarks)
  const [error, setError] = useState(false)
  const saved = items?.some((row) => row.href === item.href) ?? false
  return (
    <button
      className="save"
      disabled={!items}
      aria-pressed={saved}
      onClick={() => setError(!setBookmark(item, !saved))}
    >
      <Icon name={saved ? 'check' : 'bookmark'} />
      {error ? '保存失败，重试' : saved ? '已加入书架' : '加入书架'}
    </button>
  )
}
