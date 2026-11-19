import { useState } from 'react'
import { Icon } from './icons'
import type { Bookmark } from '../reader/bookmarks'
import { useShelf } from '../reader/account'
export function Save(item: Bookmark) {
  const { items, save, reader } = useShelf()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(false)
  const saved = items?.some((row) => row.href === item.href) ?? false
  return (
    <>
      <button
        className="save"
        disabled={!items || busy}
        aria-pressed={saved}
        onClick={async () => {
          setBusy(true)
          setError(!(await save(item, !saved)))
          setBusy(false)
        }}
      >
        <Icon name={saved ? 'check' : 'bookmark'} />
        {error ? '保存失败，重试' : saved ? '已加入书架' : '加入书架'}
      </button>
      {reader === null && saved && (
        <a
          className="return"
          href={
            '/login?returnTo=' +
            encodeURIComponent(
              item.href +
                (typeof window !== 'undefined' ? window.location.hash : ''),
            )
          }
        >
          登录以同步
        </a>
      )}
    </>
  )
}
