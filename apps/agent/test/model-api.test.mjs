import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { once } from 'node:events'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createModels } from '@earendil-works/pi-ai/models'
import { contentProvider } from '../src/content-provider.ts'
import {
  profile,
  modelJSON,
  streamFetch,
  openUsage,
  pendingUsage,
  acknowledgeUsage,
} from '../src/model-api.ts'

test('Pi transports and image/vision calls retain independent, replayable provider receipts', async (t) => {
  const dir = await mkdtemp(join(tmpdir(), 'model-receipts-'))
  t.after(() => rm(dir, { recursive: true, force: true }))
  openUsage(dir)
  const requests = []
  const usage = {
    input_tokens: 11,
    output_tokens: 5,
    total_tokens: 16,
    input_tokens_details: { cached_tokens: 3 },
  }
  const server = createServer(async (req, res) => {
    let raw = ''
    for await (const chunk of req) raw += chunk
    requests.push({
      path: req.url,
      key: req.headers.authorization,
      body: raw ? JSON.parse(raw) : {},
    })
    res.setHeader('x-request-id', 'req-test')
    if (req.url === '/error') {
      res.writeHead(429).end('{}')
      return
    }
    if (req.url === '/images/generations') {
      res.end(JSON.stringify({ data: [{ b64_json: 'private-pixels' }] }))
      return
    }
    if (req.url === '/vision') {
      res.end(JSON.stringify({ id: 'v1', model: 'actual-vision', usage }))
      return
    }
    res.setHeader('content-type', 'text/event-stream')
    const emit = (o) => res.write('data: ' + JSON.stringify(o) + '\n\n')
    if (req.url === '/chat/completions') {
      emit({
        id: 'chat1',
        model: 'content-fixture',
        choices: [
          {
            index: 0,
            delta: { role: 'assistant', content: 'ok' },
            finish_reason: null,
          },
        ],
      })
      emit({
        id: 'chat1',
        choices: [{ index: 0, delta: {}, finish_reason: 'stop' }],
      })
      emit({
        id: 'chat1',
        choices: [],
        usage: { prompt_tokens: 7, completion_tokens: 1, total_tokens: 8 },
      })
      res.end('data: [DONE]\n\n')
      return
    }
    if (req.url === '/truncated') {
      emit({
        type: 'response.created',
        response: { id: 'unfinished', usage: { total_tokens: 0 } },
      })
      res.end()
      return
    }
    const response = {
      id: 'resp1',
      model: 'resolved-content',
      status: 'completed',
      usage,
      output: [],
    }
    emit({
      type: 'response.created',
      response: { ...response, status: 'in_progress', usage: undefined },
    })
    emit({ type: 'response.completed', response })
    res.end()
  })
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  t.after(() => server.close())
  const base = 'http://127.0.0.1:' + server.address().port
  for (const [prefix, model] of [
    ['CONTENT_AI', 'content-fixture'],
    ['VISION_AI', 'vision-fixture'],
    ['MEDIA_AI', 'image-fixture'],
  ]) {
    process.env[prefix + '_BASE_URL'] = base
    process.env[prefix + '_API_KEY'] = prefix
    process.env[prefix + '_MODEL'] = model
  }
  for (const mode of ['responses', 'chat-completions']) {
    process.env.CONTENT_AI_PROTOCOL = mode
    const models = createModels(),
      provider = contentProvider('test', mode)
    models.setProvider(provider)
    const result = await models
      .streamSimple(provider.getModels()[0], {
        messages: [{ role: 'user', timestamp: 1, content: 'private-prompt' }],
      })
      .result()
    assert.notEqual(result.stopReason, 'error', result.errorMessage)
  }
  for (const [purpose, path] of [
    ['image', '/images/generations'],
    ['vision', '/vision'],
  ]) {
    const p = profile(purpose)
    await modelJSON(
      { ...p, subject: 'media:1', operation: 'fixture' },
      base + path,
      { method: 'POST', headers: { Authorization: 'Bearer ' + p.key } },
    )
  }
  const p = profile('vision'),
    meta = { ...p, subject: 'fixture', operation: 'error' }
  await assert.rejects(
    modelJSON(meta, base + '/error', { method: 'POST' }),
    /429/,
  )
  await (await streamFetch(meta)(base + '/truncated')).text()
  const records = pendingUsage()
  assert.equal(records.length, 12)
  const finals = records.map((r) => r.event).filter((e) => e.finished)
  assert.deepEqual(
    finals.map((e) => e.outcome),
    ['ok', 'ok', 'ok', 'ok', 'http_error', 'unknown'],
  )
  assert.equal(finals[0].usage.input_tokens_details.cached_tokens, 3)
  assert.equal(finals[0].resolved, 'resolved-content')
  assert.equal(finals[1].usage.prompt_tokens, 7)
  assert.equal(requests[1].body.stream_options.include_usage, true)
  assert.equal(finals[2].images, 1)
  assert.equal(finals[2].usage, undefined)
  assert.equal(finals[4].status, 429)
  assert.equal(finals[5].usage, undefined)
  assert.deepEqual(
    requests.slice(0, 4).map((r) => [r.path, r.key]),
    [
      ['/responses', 'Bearer CONTENT_AI'],
      ['/chat/completions', 'Bearer CONTENT_AI'],
      ['/images/generations', 'Bearer MEDIA_AI'],
      ['/vision', 'Bearer VISION_AI'],
    ],
  )
  assert(!JSON.stringify(records).match(/private-prompt|private-pixels|Bearer/))
  openUsage(dir)
  assert.deepEqual(pendingUsage(), records)
  acknowledgeUsage(records.map((r) => r.event_id))
  assert.equal(pendingUsage().length, 0)
})
