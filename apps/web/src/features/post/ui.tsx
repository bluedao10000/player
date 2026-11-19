import { Dialogue, Narration, RoleplayComposer } from './roleplay-controls'
import { useEffect, useState } from 'react'
import { Icon } from './icons'
import { artUrl, catalog, storyUrl } from './catalog'
import type { Story } from './catalog'
import { Layout, StoryPreview } from './design'
import { readLocal, writeLocal, shelfKey } from './storage'
export function Play({ story }: { story: Story }) {
  const key = 'tidepost.turn.v1.' + story.slug
  const [choice, setChoice] = useState<number | null>(null),
    [ready, setReady] = useState(false)
  useEffect(() => {
    const [n] = readLocal(key)
    setChoice(n === '0' || n === '1' ? Number(n) : null)
    setReady(true)
  }, [key])
  function choose(n: number | null) {
    writeLocal(key, n === null ? [] : [String(n)])
    setChoice(n)
  }
  return (
    <Layout active="">
      <div className="crumb">
        <a href={storyUrl(story)}>{story.title}</a> / 私人剧情
      </div>
      <section className="play reader">
        <img className="reader-art" src={artUrl(story)} alt={story.alt} />
        <div className="reader-body">
          <div className="kicker">
            <Icon name="compass" />
            剧情试玩 · 预设分支
          </div>
          <h1>{story.location}的来信</h1>
          <Dialogue text={story.scene} />
          {choice === null ? (
            <RoleplayComposer
              options={story.options}
              ready={ready}
              onChoose={choose}
            />
          ) : (
            <div aria-live="polite">
              <div className="chosen">你的行动 · {story.options[choice]}</div>
              <Narration>{story.replies[choice]}</Narration>
              <div className="ending">
                <span>本幕完</span>
                <button className="save" onClick={() => choose(null)}>
                  尝试另一种选择
                </button>
              </div>
            </div>
          )}
          <a className="return" href={storyUrl(story)}>
            <Icon name="back" />
            回到故事
          </a>
        </div>
      </section>
    </Layout>
  )
}
export function Shelf() {
  const [items, setItems] = useState<string[] | null>(null)
  useEffect(() => setItems(readLocal(shelfKey)), [])
  const saved = catalog.filter((s) => items?.includes(s.slug))
  return (
    <Layout active="我的">
      <div className="page-intro">
        <div className="kicker">
          <Icon name="bookmark" />
          私人收藏
        </div>
        <h1>我的书架</h1>
        <p>你在这个浏览器里收藏的故事。</p>
        <a className="return" href="/commissions">我的约稿 <Icon name="arrow" size={17} /></a>
      </div>
      {items === null ? (
        <p>正在打开书架…</p>
      ) : saved.length ? (
        <div className="grid">
          {saved.map((story) => (
            <StoryPreview key={story.slug} story={story} />
          ))}
        </div>
      ) : (
        <div className="empty">
          <Icon name="book" size={36} />
          <h2>书架还空着</h2>
          <p>在故事阅读页点“加入书架”，就能在这里重读。</p>
          <a className="button" href="/stories">
            去读故事 <Icon name="arrow" />
          </a>
        </div>
      )}
    </Layout>
  )
}
