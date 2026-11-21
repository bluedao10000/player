import assert from 'node:assert/strict'
import test from 'node:test'
import { episodeOutline } from '../src/episode-plan.ts'

test('a repeated fact in a synopsis does not move a later plot beat backwards', () => {
  const plan = {
    number: 1,
    beats: [
      { kind: 'motive', needle: 'family loss', question: 'What happened?' },
      { kind: 'turn', needle: 'found the diary', question: 'What changed?' },
      { kind: 'cause', needle: 'first exchange', question: 'What followed?' },
    ],
  }
  const documents = [
    { id: 'official-1', sentences: [] },
    {
      id: 'guide-1',
      sentences: [
        'A family loss turned her towards her first exchange.',
        'She found the diary several years before taking part.',
        'Her first exchange went badly.',
      ],
    },
  ]
  const outline = episodeOutline(plan, documents)
  assert.deepEqual(outline.map((beat) => beat.evidence[0].sentence), [1, 2, 3])
})

test('operator rules pin both edition number and heading', async () => {
  const {readConfiguredOfficial} = await import('../src/configured-sources.ts')
  const {load} = await import('cheerio')
  const html='<article><h2>#52 Spring</h2><p>Synopsis</p></article>'
  const rules={entry:'article',heading:'h2',body:'p',guide:'wiki-episode'}
  assert.equal(readConfiguredOfficial(load(html),rules,52,'Spring').text,'Synopsis')
  assert.throws(()=>readConfiguredOfficial(load(html),rules,5,'Spring'))
  assert.throws(()=>readConfiguredOfficial(load(html+html),rules,52,'Spring'))
})
