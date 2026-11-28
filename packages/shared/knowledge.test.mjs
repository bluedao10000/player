import assert from 'node:assert/strict'
import { test } from 'node:test'
import { searchWorks } from './knowledge.ts'

test('knowledge reads use the configured server origin, but always stay same-origin in the browser', async (t) => {
  const old = process.env.KNOWLEDGE_API_URL
  t.after(() => {
    if (old === undefined) delete process.env.KNOWLEDGE_API_URL
    else process.env.KNOWLEDGE_API_URL = old
    delete globalThis.window
  })
  const fetch = t.mock.method(globalThis, 'fetch', () => Response.json({ items: [], next_cursor: null }))
  delete process.env.KNOWLEDGE_API_URL
  await searchWorks('test')
  process.env.KNOWLEDGE_API_URL = 'http://fixture.invalid:9123'
  await searchWorks('test')
  globalThis.window = { location: { origin: 'https://reader.invalid' } }
  await searchWorks('test')
  assert.deepEqual(fetch.mock.calls.map(({ arguments: [url] }) => url.origin), [
    'http://127.0.0.1:4321', 'http://fixture.invalid:9123', 'https://reader.invalid',
  ])
})
