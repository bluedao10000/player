import assert from 'node:assert/strict'
import test from 'node:test'
import { checkIndependentSources, extractEpisodeSource } from '../src/episode-source-rules.ts'
import { validateEpisodeSourceProposal } from '../src/episode-source-task.ts'
import { episodeOutline, readEpisodeDocuments } from '../src/episode-plan.ts'

const official = 'https://fxkurumi-info.com/story/episodes.json'
const cases = [3, 4].map((number) => ({ number, title: `分集${number}`, official, guide: `https://skypenguin.net/episode-${number}/` }))
const officialBody = (number) => `公式第${number}話。${'公式あらすじの文章です。'.repeat(6)}`
const guideBody = (number) => `視聴者による第${number}話の感想。${'放送後のストーリーを独自に考察します。'.repeat(5)}`
const records = cases.map(({ number, title }) => ({ id: `0${number}`, title, text: officialBody(number) }))
const proposal = {
  officialRule: { kind: 'json_episode_list', episode: 'id', title: 'title', body: 'text' },
  guideRule: { kind: 'html_episode_article', heading: 'h1', body: '.entry p' },
}

test('validates one proposal against an unseen episode and captures both sources', async () => {
  const requests = []
  const download = async (url) => {
    requests.push(url)
    if (url === official) return JSON.stringify(records)
    const sample = cases.find(({ guide }) => guide === url)
    return `<article><h1>第${sample.number}話「${sample.title}」</h1><div class="entry"><p>${guideBody(sample.number)}</p></div></article>`
  }
  let proposals = 0
  const { sourceRules, proofs } = await validateEpisodeSourceProposal(cases, async (sample) => {
    assert.equal(sample.target.number, 3)
    proposals++
    return { value: proposal }
  }, download)
  assert.equal(proposals, 1)
  assert.equal(requests.filter((url) => url === official).length, 1)
  assert.deepEqual(proofs.official.map(({ episode }) => episode), [3, 4])
  const { number, title, guide } = cases[1]
  const docs = await readEpisodeDocuments({ number, sourceRules, official, guide, officialTitle: title, guideTitle: title },
    async () => assert.fail('paired rules use raw responses'), download)
  assert.deepEqual(docs.map(({ id, text }) => [id, text]), [['official-4', officialBody(4)], ['guide-4', guideBody(4)]])
})

test('reads the printed title from its own element when the heading only carries the number', () => {
  const body = '本文'.repeat(40)
  const html = `<article><h1>第3話 ネタバレ感想</h1><p class="ep-title">分集3</p><div class="entry"><p>${body}</p></div></article>`
  const rule = { kind: 'html_episode_article', heading: 'h1', titleSelector: 'p.ep-title', body: '.entry p' }
  assert.equal(extractEpisodeSource(html, rule, { number: 3, title: '分集3' }), body)
  assert.throws(() => extractEpisodeSource(html, rule, { number: 3, title: '分集4' }), /标题/)
  assert.throws(() => extractEpisodeSource(html, { kind: 'html_episode_article', heading: 'h1', body: '.entry p' },
    { number: 3, title: '分集3' }), /标题/)
})

test('rejects wrong titles, duplicate episodes, and copies of the official text', () => {
  assert.throws(() => extractEpisodeSource(JSON.stringify(records), proposal.officialRule, { number: 3, title: '分集4' }), /标题/)
  assert.throws(() => extractEpisodeSource(JSON.stringify([...records, records[0]]), proposal.officialRule, cases[0]), /重复/)
  assert.throws(() => checkIndependentSources(officialBody(3), `転載: ${officialBody(3)}`), /同一通稿/)
})

test('a single official source attaches the episode and writes the outline', async () => {
  const solo = [3, 4].map((number) => ({ number, title: `分集${number}`, official }))
  const requests = []
  const download = async (url) => {
    requests.push(url)
    if (url === official) return JSON.stringify(records)
    throw Error('unexpected source ' + url)
  }
  const { sourceRules, proofs } = await validateEpisodeSourceProposal(solo, async (sample) => {
    assert.equal(sample.guide, undefined)
    return { value: { officialRule: proposal.officialRule } }
  }, download)
  assert.deepEqual(sourceRules, { version: 1, guide: 'official_only', officialRule: proposal.officialRule })
  assert.equal(proofs.guide, undefined)
  assert.ok(requests.every((url) => url === official))

  const docs = await readEpisodeDocuments({ number: 4, sourceRules, official, officialTitle: '分集4', guideTitle: '分集4' },
    async () => assert.fail('official_only uses raw responses'), download)
  assert.deepEqual(docs.map(({ id }) => id), ['official-4'])

  const sentences = ['第一句', '第二句']
  const [outline] = episodeOutline({ number: 4,
    beats: [{ kind: 'cause', needle: '第一句', question: '考察', officialSentence: 2 }],
    people: [{ beat: 1, name: '斯派克', evidence: '第二句' }],
  }, [{ id: 'official-4', sentences }])
  assert.deepEqual(outline.evidence.map(({ id, sentence }) => [id, sentence]), [['official-4', 1], ['official-4', 2]])
  assert.match(outline.question, /斯派克/)

  await assert.rejects(validateEpisodeSourceProposal([solo[0], cases[1]], async () => ({ value: proposal }), download),
    /同一对独立站点/)
})
test('long recaps retain the beat anchor and character proof within the acceptance limit', () => {
  const guide = Array.from({ length: 12 }, (_, i) => `第 ${i + 1} 句`)
  const [outline] = episodeOutline({ number: 3,
    beats: [{ kind: 'cause', needle: guide[0], question: '考察', officialSentence: 1 }],
    people: [{ beat: 1, evidence: guide.at(-1) }],
  }, [{ id: 'guide-3', sentences: guide }, { id: 'official-3', sentences: ['官方资料'] }])
  assert.equal(outline.evidence.length, 8)
  assert.equal(outline.evidence[0].text, guide[0])
  assert.equal(outline.evidence[6].text, guide.at(-1))
  assert.equal(outline.evidence[7].id, 'official-3')
})

test('matches each episode in a shared page using its printed identity and scoped body', async () => {
  const { checkEpisodeSourceRule } = await import('../src/episode-source-rules.ts')
  const raw = '<main><section><h4>第2話</h4><h3>港口</h3><p>' + '潮水把来船推向东边的旧码头。'.repeat(5) +
    '</p></section><section><h4>第3話</h4><h3>来信</h3><p>' + '早晨的信使带回了家人的消息。'.repeat(5) + '</p></section></main>'
  const scope = 'section:has(h4:contains("第{episode}話"))'
  const rule = { kind: 'html_episode_article', heading: scope + ' h4', titleSelector: scope + ' h3', body: scope + ' p' }
  const samples = [{ raw, target: { number: 2, title: '港口' } }, { raw, target: { number: 3, title: '来信' } }]
  assert.equal(checkEpisodeSourceRule(rule, samples).length, 2)
  assert.throws(() => checkEpisodeSourceRule(rule, [samples[0], { raw, target: { number: 3, title: '港口' } }]))
})

test('binds a guide headline to the catalog title through the exact same-site episode link', async () => {
  const hub = 'https://guide.example.test/series'
  const cases = [2,3].map(number => ({ number, title: '本集' + number, official: official, guide: 'https://guide.example.test/review/' + number,
    guideIndex: hub, guideTitle: '第' + number + '話 回顧と感想' }))
  const rows = cases.map(c => '<tr><td>第' + c.number + '話</td><td>' + c.title + '</td><td><a href="' + c.guide + '">感想</a></td></tr>')
  const data = new Map([[hub, '<table>' + rows.join('') + '</table>'],[official, JSON.stringify(cases.map(c =>
    ({no: c.number, name: c.title, text: ('官方给出本场演出的时间和地点' + c.number).repeat(5)})))]])
  for (const c of cases) data.set(c.guide, '<article><h1>' + c.guideTitle + '</h1><p>' + ('来信说明了父亲出海后发生的变化' + c.number).repeat(6) + '</p></article>')
  const propose = async () => ({value: {officialRule: {kind:'json_episode_list',episode:'no',title:'name',body:'text'},
    guideRule: {kind:'html_episode_article',heading:'article h1',body:'article p'}}})
  const result = await validateEpisodeSourceProposal(cases, propose, async url => data.get(url))
  assert.equal(result.proofs.guideIndex[0].episode, 2)
  data.set(hub, '<table>' + rows[0].replace('/review/2','/review/3') + rows[1] + '</table>')
  await assert.rejects(validateEpisodeSourceProposal(cases, propose, async url => data.get(url)), /未唯一指向/)
})
