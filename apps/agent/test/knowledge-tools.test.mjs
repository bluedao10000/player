import assert from 'node:assert/strict'
import test from 'node:test'
import { knowledgeTools } from '@anime/shared/knowledge-tools'
import { catalogTools, conversationReadInstructions } from '../src/knowledge.ts'

test('Pi public directory tools use the generated names, schemas and limits', async (t) => {
  const id = '10000000-0000-4000-8000-100000000000'
  const provenance = [{ source: 'catalog', version: 'v1' }]
  const items = [{ id, canonical_title: '测试目录' }]
  t.mock.method(globalThis, 'fetch', async (url) => {
    if (url.pathname.endsWith('/works')) {
      assert.equal(url.searchParams.get('q'), '测试目录')
      assert.equal(url.searchParams.get('limit'), '8')
      return Response.json({ items, next_cursor: null })
    }
    assert.equal(url.pathname, '/api/v1/knowledge/works/' + id)
    return Response.json({
      id, canonical_title: '测试目录', type: 'TV', season_year: null,
      episodes: 26, status: null, studios: [], provenance,
      titles: Array.from({ length: 25 }, (_, title) => ({ lang: 'en', title })),
      tags: Array.from({ length: 40 }, (_, tag) => String(tag)),
      relations: Array.from({ length: 15 }, (_, related_work_id) => ({ related_work_id })),
      sources: [{ provider: 'example', external_id: 'catalog-1', url: 'https://example.test/' }],
    })
  })
  const tools = catalogTools()
  const publicEntries = knowledgeTools.filter(entry => entry.scope === 'public_catalog')
  assert.deepEqual(tools.map(tool => tool.name), publicEntries.map(entry => entry.name))
  for (const [i, tool] of tools.entries()) {
    assert.deepEqual(tool.parameters, publicEntries[i].schema)
    assert.equal(tool.description, publicEntries[i].description)
  }
  const output = []
  const api = { output: data => output.push(JSON.parse(data)) }
  await tools[0].execute({ q: '测试目录' }, api)
  await tools[1].execute({ id }, api)
  assert.deepEqual(output[0], items)
  assert.deepEqual(Object.keys(output[1]).sort(), [
    'canonical_title', 'episodes', 'id', 'provenance', 'related', 'season_year',
    'status', 'studios', 'tags', 'titles', 'type',
  ])
  assert.equal(output[1].titles.length, 20)
  assert.equal(output[1].tags.length, 30)
  assert.equal(output[1].related.length, 12)
  assert.deepEqual(output[1].provenance, provenance)
  await assert.rejects(tools[0].execute({ q: ' ' }, api))
})

test('conversation instructions render only allowed generated entries for this phase', () => {
  const capabilities = ['get_execution_history', 'search_anime_works', 'unregistered']
  const first = conversationReadInstructions(JSON.stringify({ capabilities, allow_reads: true, max_reads: 3 }))
  assert.match(first, /get_execution_history.*取证运行历史.*12/)
  assert.match(first, /search_anime_works/)
  assert.doesNotMatch(first, /unregistered|read_selected_work|read_anime_work/)
  const second = conversationReadInstructions(JSON.stringify({ capabilities, allow_reads: false }))
  assert.match(second, /get_execution_history/)
  assert.doesNotMatch(second, /read_calls|search_anime_works/)
})
