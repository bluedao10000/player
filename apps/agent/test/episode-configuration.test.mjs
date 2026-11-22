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
