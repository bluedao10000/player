import assert from 'node:assert/strict'
import test from 'node:test'
import { load } from 'cheerio'
import { compileEpisode, locateEpisode } from '../src/episode-config.ts'

test('new episodes inherit verified origins, evidence order and existing portraits', async () => {
  const template = {
    official: 'https://www.cowboy-bebop.net/story/02.html',
    guide: 'https://en.wikipedia.org/wiki/List_of_Cowboy_Bebop_episodes#ep9',
    sourceRules: { entry: '.storybox', heading: 'h2', body: '.summary', guide: 'wiki-episode' },
    people: [{ slug: 'example', name: '示例角色' }],
    art: { sourceSHA: 'a'.repeat(64), referenceURL: 'https://assets.example.test/portrait.png' },
  }
  const official = load('<section class="storybox"><h2>#10 Example</h2><p class="summary">This episode has an official synopsis with enough text to validate its source fingerprint.</p></section>')
  const recap = load('<table><tr id="ep10"><td class="summary">"Example"</td></tr><tr><td class="shortSummaryText">A letter arrives. An old friend appears. The boat leaves the dock. The crew finally returns home.</td></tr></table>')
  const located = await locateEpisode(template, 10, official, recap)
  const draft = {
    slug: 'example-10', title: '示例', intro: '读信', place: '港口',
    sceneSlug: 'example-scene', sceneName: '港口来信', sceneBeat: 2, artBrief: '参考已有肖像',
    setting: ['港口', 3], people: [{ slug: 'example', beat: 2, evidence: 'An old friend' }],
    beats: ['A letter', 'An old friend', 'The boat', 'The crew'].map((needle, i) => ({
      needle, kind: ['motive','turn','cause','cause'][i], question: '核对出处', ...(i===0?{officialSentence:1}:{}),
    })),
  }
  const config = compileEpisode(template, located, draft)
  assert.equal(config.guide, template.guide.replace('#ep9', '#ep10'))
  assert.equal(config.art.sourceSHA, template.art.sourceSHA)
  assert.equal(config.publishToReader, false)
  const portrait = { sourceSHA: 'b'.repeat(64), referenceURL: 'https://assets.example.test/existing.webp' }
  const withPortrait = { ...template, portraitReferences: { example: portrait } }
  const portraitDraft = { ...draft, artCharacter: 'example', artBrief: '示例角色读信' }
  assert.deepEqual(compileEpisode(withPortrait, located, portraitDraft).art, {
    ...portrait, character: 'example', brief: '示例角色读信',
  })
  assert.throws(() => compileEpisode(withPortrait, located, draft), /肖像/)
  assert.throws(() => compileEpisode(withPortrait, located, { ...portraitDraft, sceneBeat: 3 }))

  assert.deepEqual(config.people, [{ slug: 'example', name: '示例角色', beat: 2, evidence: 'An old friend' }])
  assert.throws(() => compileEpisode(template, located, {...draft, beats: [...draft.beats].reverse()}))
  assert.throws(() => compileEpisode(template, located, {...draft, people: [{slug:'unknown',beat:2}]}))
  assert.throws(() => compileEpisode(template, located, { ...draft, people: [{ slug: 'example', beat: 2, evidence: 'The boat' }] }), /原文/)
  await assert.rejects(locateEpisode(template, 11, official, recap))
})

test('character selections resolve to one exact paragraph and source, including shared official sentences', async () => {
  const {castEvidenceMenu,bindCastEvidence,outlineFingerprint} = await import('../src/episode-configuration-task.ts')
  const source={number:3}
  const docs=[{id:'guide-3',sentences:['Anna leaves.','Ben returns.','Anna sees Ben.','The crew departs.']},{id:'official-3',sentences:['Anna opens the door.','Ben finds the key.']}]
  const draft={slug:'test',sceneSlug:'door',title:'测试',intro:'简介',place:'港口',setting:['港口',1],beats:docs[0].sentences.map((needle,i)=>({
    needle,kind:['motive','turn','cause','turn'][i],question:'谁在场？',...(i<2?{officialSentence:1}:{})
  }))}
  const located={source,docs}, menu=castEvidenceMenu(located,draft)
  const previous={people:[{slug:'anna',name:'安娜'}],portraitReferences:{anna:{}}}
  const choice={artCharacter:'anna',artBrief:'安娜开门',sceneName:'开门',people:[{slug:'anna',key:'b2/official-3#1',beat:99}]}
  const cast=bindCastEvidence(previous,menu,choice)
  assert.equal(cast.sceneBeat,2)
  assert.deepEqual(cast.people,[{slug:'anna',beat:2,evidence:'Anna opens the door.'}])
  for(const key of ['b3/official-3#1','b1/official-3#2','b1/guide-4#1'])
    assert.throws(()=>bindCastEvidence(previous,menu,{...choice,people:[{slug:'anna',key}]}),/anna.*允许引用/)
  assert.notEqual(outlineFingerprint(located),outlineFingerprint({source,docs:docs.slice(0,1)}))
})

test('restores source order without moving the setting to another event', async () => {
  const {orderOutline} = await import('../src/episode-configuration-task.ts')
  const beats = [{guideSentence:9},{guideSentence:2},{guideSentence:5}]
  assert.deepEqual(orderOutline(beats,['约定',1]), {beats:[beats[1],beats[2],beats[0]],setting:['约定',3]})
})
