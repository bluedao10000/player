import assert from 'node:assert/strict'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { createServer } from 'vite'

// Find the paragraph by its visible fixture text, then check its own attributes.
function paragraph(html, text, id, block) {
  const match = [...html.matchAll(/<p\b([^>]*)>([^<]*)<\/p>/g)].find(([, , body]) => body === text)
  assert.ok(match, `missing paragraph: ${text}`)
  assert.ok(match[1].includes(`id="${id}"`) && match[1].includes(`data-block="${block}"`), match[0])
  return match.index
}

test('chapter selection preserves the H5 markup and native blocks contracts', async (t) => {
  const vite = await createServer({ root: fileURLToPath(new URL('..', import.meta.url)), configFile: false,
    server: { middlewareMode: true, hmr: false, watch: null }, optimizeDeps: { noDiscovery: true } })
  t.after(() => vite.close())
  const { Reader } = await vite.ssrLoadModule('/src/features/post/editorial.tsx')
  const { iosReaderResponse } = await vite.ssrLoadModule('/src/features/reader/ios-data.server.ts')
  const raw = { key: 'raw', text: 'RAW TEXT', anchors: [] }
  const scene = { key: 'scene', text: 'SCENE TEXT', anchors: ['passage-pier', 'old-pier'] }
  const frozen = { key: 'final', text: 'FINAL TEXT', anchors: ['published-anchor'] }
  const research = { paragraphs: [{ text: 'FIRST GUIDE' }, { text: 'SECOND GUIDE' }] }
  const image = 'https://img.codeiy.com/i/' + 'a'.repeat(64)
  const art = { block: 'final', anchor: 'plate', name: '插画', image }
  const fetch = t.mock.method(globalThis, 'fetch')

  async function read(overrides, slug = 'fixture') {
    const page = {
      work: { slug: 'tidepost', title: '作品', href: '/work/tidepost' },
      chapter: { slug, title: '章节', href: '/story/' + slug },
      blocks: [raw, scene], scenes: [{ slug: 'pier', title: '码头' }], attribution: [], ...overrides,
    }
    fetch.mock.mockImplementation(() => Response.json(page))
    const html = renderToStaticMarkup(createElement(Reader, { page }))
    const response = await iosReaderResponse('v1/works/tidepost/chapters/' + slug)
    assert.equal(response.status, 200)
    return { html, native: await response.json() }
  }

  const original = await read({ reader_passages: [frozen], research }, 'night-courier')
  assert.match(original.html, /RAW TEXT/)
  assert.doesNotMatch(original.html, /FINAL TEXT|FIRST GUIDE/)
  assert.deepEqual(original.native.blocks, [frozen])
  assert.ok(original.native.play)

  // Historical responses omit both selection fields. Native returns art even
  // for raw / research chapters; H5 only attaches plates to published passages.
  const sceneArt = { ...art, block: scene.key }
  const base = await read({ accepted_art: [sceneArt] })
  paragraph(base.html, raw.text, 'reading-raw', raw.key)
  assert.match(base.html, /id="old-pier"/)
  paragraph(base.html, scene.text, scene.anchors[0], scene.key)
  assert.doesNotMatch(base.html, /illustration-plate/)
  assert.deepEqual(base.native.blocks, [raw, scene])
  assert.deepEqual(base.native.illustrations, [{ block: scene.key, image, name: art.name }])

  const guide = await read({ reader_passages: null, research, accepted_art: [sceneArt] })
  const guideSection = guide.html.match(/<section class="episode-guide"[^>]*>(.*?)<\/section>/s)?.[1] ?? ''
  assert.ok(paragraph(guideSection, 'FIRST GUIDE', 'reading-research-1', 1) < paragraph(guideSection, 'SECOND GUIDE', 'reading-research-2', 2))
  const aside = guide.html.match(/<aside class="scene-passage">(.*?)<\/aside>/s)?.[1] ?? ''
  assert.match(aside, /<small>码头<\/small>/)
  assert.match(aside, /id="old-pier"/)
  paragraph(aside, scene.text, scene.anchors[0], scene.key)
  assert.doesNotMatch(guide.html, /RAW TEXT|illustration-plate/)
  assert.deepEqual(guide.native.illustrations, base.native.illustrations)
  assert.deepEqual(guide.native.blocks, [
    { key: 'research-1', text: 'FIRST GUIDE', anchors: [] },
    { key: 'research-2', text: 'SECOND GUIDE', anchors: [] }, scene,
  ])

  const empty = await read({ reader_passages: [], research })
  assert.doesNotMatch(empty.html, /RAW TEXT|SCENE TEXT|FIRST GUIDE|episode-guide/)
  assert.deepEqual(empty.native.blocks, [])

  const published = await read({ reader_passages: [frozen], research, accepted_art: [art] })
  paragraph(published.html, frozen.text, frozen.anchors[0], frozen.key)
  assert.doesNotMatch(published.html, /RAW TEXT|SCENE TEXT|FIRST GUIDE/)
  assert.deepEqual(published.native.blocks, [frozen])
  const plate = published.html.match(/<figure[^>]*id="illustration-plate"[^>]*>(.*?)<\/figure>/s)?.[1] ?? ''
  assert.ok(plate.includes(`src="${image}"`))
  assert.deepEqual(published.native.illustrations, [{ block: frozen.key, image, name: art.name }])
})
