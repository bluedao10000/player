import { imageUrl } from '../../lib/images'
import { artUrl, feature, stories, storyUrl } from './catalog'
import {
  Avatar,
  Button,
  Layout,
  people,
  ScenePreview,
  SectionHeader,
} from './design'
import { Icon } from './icons'

import type { CharacterDetail } from '@anime/shared/knowledge'
import { WorldTrail } from '../worlds/pages'

const [ferry, letter, lighthouse] = stories
const opening = '/play/' + feature.slug
const connections = {
  captain:
    '船长掌着旧渡口的末班船。只要看见阿澪还在栈桥上跑，他总愿意再等一会儿。',
  keeper:
    '守塔人教过阿澪用铜哨回应灯语。即使隔着一片黑漆漆的海，两个人也能认出对方的回音。',
}
const stops = [
  {
    story: letter,
    detail: '夜班从分拣信件开始。有些寄不出去的信，要等窗台上的花先开。',
  },
  {
    story: ferry,
    detail: '到了渡口，她会数一数船上的人，看看还有谁没赶上回家的船。',
  },
  {
    story: lighthouse,
    detail: '九点的灯光扫过邮局二楼，就是北岬向阿澪报平安的时候。',
  },
]

function CharacterHero({ name, role }: { name: string; role: string }) {
  return (
    <header className="mio-arrival" aria-labelledby="mio-name">
      <img
        className="mio-key-art"
        src={imageUrl("/art/mio-invitation.webp")}
        width="1672"
        height="941"
        alt="白帆港的夜色里，阿澪站在亮着灯的邮局门前，向你递出一封信。"
        fetchPriority="high"
      />
      <div className="mio-voice">
        <h1 id="mio-name">{name}</h1>
        <p className="mio-byline">
          <Icon name="moon" size={15} />
          {role}
        </p>
        <blockquote>
          「替每一封迷路的信，
          <br />
          找到靠岸的灯。」
        </blockquote>
        <Button href={opening}>与阿澪进入剧情</Button>
      </div>
    </header>
  )
}

export function Character({
  work,
  character,
  appearances,
  intro,
  scene_previews,
}: CharacterDetail) {
  return (
    <Layout active="cast">
      <article className="mio-world">
        <WorldTrail work={work} />
        <CharacterHero name={character.name} role={character.role} />
        <div className="mio-trail">
          <section aria-label="出场章节">
            <h2>出场章节</h2>
            {intro && <p id="character-intro" className="mio-intro">{intro}</p>}
            <div className="mio-stories">
              {appearances.map((story) => (
                <a key={story.slug} href={story.href}>
                  <img
                    src={imageUrl(story.image)}
                    alt=""
                    width="766"
                    height="510"
                    loading="lazy"
                  />
                  <div>
                    <small>{story.label}</small>
                    <h3>{story.title}</h3>
                  </div>
                  <Icon name="forward" size={15} />
                </a>
              ))}
            </div>
          </section>
          <section aria-label="相关场景">
            <SectionHeader>夜班途中的两幕</SectionHeader>
            <div className="mio-scenes">
              <ScenePreview
                story={ferry}
                scene={scene_previews.find(
                  (s) => s.chapter_slug === ferry.slug,
                )}
              />
              <ScenePreview story={lighthouse} />
            </div>
          </section>
          <section id="relationships" aria-label="角色关系">
            <SectionHeader>邮路上的熟人</SectionHeader>
            <div className="mio-people">
              {people.map((p) =>
                p.id === 'mio' ? null : (
                  <article id={p.id} key={p.id}>
                    <Avatar image={p.image} />
                    <div>
                      <h3>{p.name}</h3>
                      <p>{connections[p.id]}</p>
                      <a href={'/story/' + p.story}>
                        读她与{p.name}的故事 <Icon name="arrow" size={15} />
                      </a>
                    </div>
                  </article>
                ),
              )}
            </div>
          </section>
          <section aria-labelledby="mio-port">
            <h2 id="mio-port">白帆港的夜间邮路</h2>
            <ol className="mio-stops">
              {stops.map(({ story, detail }) => (
                <li key={story.slug}>
                  <a href={storyUrl(story)}>
                    <h3>{story.location}</h3>
                    <p>{detail}</p>
                  </a>
                </li>
              ))}
            </ol>
          </section>
          <section className="mio-departure" aria-label="与阿澪开始剧情">
            <img src={artUrl(letter)} alt="" loading="lazy" />
            <div>
              <h2>一封没有邮票的信</h2>
              <p>{feature.scene}</p>
              <Button href={opening}>陪阿澪去送信</Button>
            </div>
          </section>
        </div>
      </article>
    </Layout>
  )
}
