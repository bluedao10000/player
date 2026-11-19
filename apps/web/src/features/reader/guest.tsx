import { useState } from 'react'
import { Button, Layout, SectionHeader } from '../post/design'
import { Icon } from '../post/icons'
import { readLastChapter } from '../post/progress'
import { readBookmarks, setBookmark, useBrowserRecord } from './bookmarks'
import type { libraryCatalog } from './library'
import './guest.css'

type Catalog = Awaited<ReturnType<typeof libraryCatalog>>
const workKey = (href: string) =>
  href.startsWith('/story/') ? 'tidepost' : href.split('/')[2]

export function GuestLibrary({
  catalog,
  full = false,
}: {
  catalog: Catalog
  full?: boolean
}) {
  const saved = useBrowserRecord(readBookmarks)
  const last = useBrowserRecord(readLastChapter)
  const [filter, setFilter] = useState('')
  const [error, setError] = useState('')
  const published = new Map(catalog?.map((c) => [c.href, c]))
  const recent = last && published.get(last.href)
  const canResume = last && (recent || !catalog)
  const works = new Map(
    saved?.map((b) => {
      const chapter = published.get(b.href)
      return [
        workKey(b.href),
        chapter ? chapter.work + ' · ' + chapter.edition : b.work,
      ]
    }),
  )
  const selected = works.has(filter) ? filter : ''
  const shown = full
    ? saved?.filter((b) => !selected || workKey(b.href) === selected)
    : saved?.slice(0, 4)
  const backdrop = catalog?.find((c) => c.href.startsWith('/work/') && c.image)
  const heroImage = recent?.image || (!last && backdrop?.image)
  return (
    <Layout active="我的">
      <div className="guest-library">
        <header className="guest-heading">
          {full && (
            <a className="return" href="/me">
              <Icon name="back" size={16} />
              我的
            </a>
          )}
          <h1>{full ? '我的书架' : '我的'}</h1>
          <p>{full ? '按最近收藏排列' : '访客 · 阅读记录保存在此浏览器'}</p>
        </header>
        {catalog === null && (
          <p className="guest-notice" role="status">
            暂时无法核对公开目录。你仍可以打开本机保存的章节。
          </p>
        )}
        {!full && last !== undefined && (
          <section className="guest-feature">
            {heroImage && <img src={heroImage} alt="" />}
            <div>
              {last ? (
                <>
                  <small>继续阅读 · {recent?.work ?? last.work}</small>
                  <h2>{recent?.title ?? last.title}</h2>
                  <p>
                    {recent && !last.title.includes(recent.label) && (
                      <>{recent.label} · </>
                    )}
                    已读 {last.percent}%
                  </p>
                  {last.excerpt && <blockquote>{last.excerpt}</blockquote>}
                  {canResume ? (
                    <Button href={last.resume}>接着读</Button>
                  ) : (
                    <p>此章暂未公开</p>
                  )}
                </>
              ) : (
                <>
                  <small>继续阅读</small>
                  <h2>从一部作品开始</h2>
                  <p>读过的章节会在这里留下进度。</p>
                  <Button href="/works">发现作品</Button>
                </>
              )}
            </div>
          </section>
        )}
        {!full && (
          <SectionHeader href="/shelf" link="查看全部">
            我的书架
          </SectionHeader>
        )}
        {full && works.size > 0 && (
          <label className="guest-filter">
            按作品筛选
            <select
              value={selected}
              onChange={(e) => setFilter(e.target.value)}
            >
              <option value="">全部作品</option>
              {Array.from(works, ([key, title]) => (
                <option key={key} value={key}>
                  {title}
                </option>
              ))}
            </select>
          </label>
        )}
        {error && <p role="alert">{error}</p>}
        {shown === undefined ? (
          <p role="status">正在打开本机书架…</p>
        ) : shown.length === 0 ? (
          <div className="guest-empty">
            <Icon name="bookmark" size={25} />
            <div>
              <strong>书架还空着</strong>
              <p>在章节里点「加入书架」，就能在这里重读。</p>
              <a className="return" href="/stories">
                去找故事 <Icon name="arrow" size={16} />
              </a>
            </div>
          </div>
        ) : (
          <ul className="guest-books">
            {shown.map((b) => {
              const chapter = published.get(b.href)
              const accessible = !!chapter || !catalog
              const title = chapter?.title ?? b.title
              const content = (
                <>
                  {chapter?.image && (
                    <img src={chapter.image} alt="" loading="lazy" />
                  )}
                  <span>
                    <small>
                      {chapter?.work ?? b.work}
                      {chapter && ' · ' + chapter.label}
                    </small>
                    <strong>{title}</strong>
                    {!accessible && <small>暂未公开</small>}
                  </span>
                </>
              )
              return (
                <li key={b.href}>
                  {accessible ? (
                    <a href={b.href}>{content}</a>
                  ) : (
                    <div>{content}</div>
                  )}
                  {full && (
                    <button
                      type="button"
                      className="guest-remove"
                      aria-label={'移除《' + title + '》'}
                      onClick={() =>
                        setError(
                          setBookmark(b, false) ? '' : '移除失败，请重试。',
                        )
                      }
                    >
                      移除
                    </button>
                  )}
                </li>
              )
            })}
          </ul>
        )}
        {full && (
          <a className="return guest-commission-link" href="/commissions">
            我的约稿 <Icon name="arrow" size={16} />
          </a>
        )}
        {!full && (
          <section className="guest-orders">
            <SectionHeader>我的创作</SectionHeader>
            <a href="/commissions">
              <span>
                我的约稿<small>查看订单与余额</small>
              </span>
              <Icon name="arrow" size={18} />
            </a>
          </section>
        )}
      </div>
    </Layout>
  )
}
