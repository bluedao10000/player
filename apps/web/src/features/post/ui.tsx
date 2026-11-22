import { Dialogue, Narration, RoleplayComposer } from './roleplay-controls'
import { useEffect, useState } from 'react'
import { Icon } from './icons'
import { artUrl, storyUrl } from './catalog'
import type { Story } from './catalog'
import { Layout } from './design'
import { readLocal, writeLocal } from '../reader/bookmarks'
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
    <Layout>
      <div className="crumb">
        <a href={storyUrl(story)}>{story.title}</a> / 剧情互动
      </div>
      <section className="play reader">
        <img className="reader-art" src={artUrl(story)} alt={story.alt} />
        <div className="reader-body">
          <div className="kicker">
            <Icon name="compass" />
            剧情互动
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
            回到章节
          </a>
        </div>
      </section>
    </Layout>
  )
}
