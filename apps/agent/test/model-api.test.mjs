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

test('Magpie content and image use their gateway routes independently of direct vision', async (t) => {
  const values = {
    CONTENT_AI_MODE: 'magpie',
    CONTENT_AI_API_KEY: 'direct-only',
    CONTENT_AI_MODEL: 'direct-model',
    MEDIA_AI_MODE: 'magpie',
    MAGPIE_IMAGE_MODEL: 'google/gemini-3.1-flash-image',
    MEDIA_AI_API_KEY: 'direct-image',
    VISION_AI_API_KEY: 'direct-vision',
    ORIGINAL_MEDIA_AI_MODE: 'magpie',
    MAGPIE_ORIGINAL_IMAGE_MODEL: 'volcengine/doubao-seedream-5-0-pro',
    MAGPIE_CONTENT_MODEL: 'vendor/suite/model',
    MAGPIE_API_KEY: 'gateway-only',
    MAGPIE_ACCOUNT: 'gateway-account',
    MAGPIE_BASE_URL: 'https://gateway.example/v1',
  }
  for (const [name, value] of Object.entries(values)) {
    const previous = process.env[name]
    t.after(() => previous === undefined ? delete process.env[name] : process.env[name] = previous)
    process.env[name] = value
  }
  const p = profile('content')
  assert.equal(p.model, values.MAGPIE_CONTENT_MODEL)
  assert.equal(p.key, 'gateway-only')
  assert.equal(p.account, 'gateway-account')
  assert.equal(profile('image').model, values.MAGPIE_IMAGE_MODEL)
  assert.equal(profile('image').key, 'gateway-only')
  assert.equal(profile('vision').key, 'direct-vision')
  process.env.MEDIA_AI_MODE = 'direct'
  assert.equal(profile('image').key, 'direct-image')
  assert.equal(profile('image', true).model, values.MAGPIE_ORIGINAL_IMAGE_MODEL)
  assert.equal(profile('image', true).key, 'gateway-only')
  assert.equal(contentProvider('test', 'research').getModels()[0].api, 'openai-completions')
  delete process.env.MAGPIE_API_KEY
  assert.throws(() => profile('content'), /MAGPIE_API_KEY/)
  process.env.MAGPIE_BASE_URL = 'http://host.docker.internal:3425/v1'
  assert.equal(profile('content').key, 'magpie')
  process.env.MAGPIE_BASE_URL = 'http://example.com/v1'
  assert.throws(() => profile('content'), /BASE_URL/)
  process.env.CONTENT_AI_MODE = 'direct'
  assert.equal(profile('content').key, 'direct-only')
  assert.equal(profile('content').model, 'direct-model')
})

test('Magpie image receipts retain the image count when the upstream omits token usage', async (t) => {
  const dir = await mkdtemp(join(tmpdir(), 'gateway-image-receipts-'))
  t.after(() => rm(dir, { recursive: true, force: true }))
  openUsage(dir)
  const server = createServer((req, res) => {
    const input = req.url === '/known' ? 8 : 0
    res.end(JSON.stringify({model: 'google/gemini-3.1-flash-image', data: [{b64_json: 'pixels'}],
      usage: {input_tokens: input, output_tokens: input, total_tokens: input * 2}}))
  })
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  t.after(() => server.close())
  const previous = process.env.MEDIA_AI_MODE
  t.after(() => previous === undefined ? delete process.env.MEDIA_AI_MODE : process.env.MEDIA_AI_MODE = previous)
  process.env.MEDIA_AI_MODE = 'magpie'
  const base = 'http://127.0.0.1:' + server.address().port
  const meta = {purpose: 'image', gateway: true, model: 'google/gemini-3.1-flash-image', account: 'magpie', base, subject: 'media:fixture', operation: 'edit'}
  for (const path of ['/unknown', '/known']) await modelJSON(meta, base + path, {method: 'POST'})
  const [unknown, known] = pendingUsage().map(r => r.event).filter(e => e.finished)
  assert.equal(unknown.images, 1)
  assert.equal(unknown.usage, undefined)
  assert.equal(known.usage.total_tokens, 16)
  assert.equal(known.resolved, meta.model)
})

test('plan inputs keep paired Japanese sources inside the reserved range', async (t) => {
  const dir = await mkdtemp(join(tmpdir(), 'model-plan-'))
  t.after(() => rm(dir, { recursive: true, force: true }))
  openUsage(dir)
  const server = createServer(async (req, res) => {
    for await (const _ of req) void _
    res.setHeader('content-type', 'text/event-stream')
    res.write('data: ' + JSON.stringify({ id: 'p1', choices: [{ index: 0, delta: { content: 'ok' } }] }) + '\n\n')
    res.write('data: ' + JSON.stringify({ id: 'p1', choices: [], usage: { prompt_tokens: 9, completion_tokens: 1, total_tokens: 10 } }) + '\n\n')
    res.end('data: [DONE]\n\n')
  })
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  t.after(() => server.close())
  const base = 'http://127.0.0.1:' + server.address().port
  const saved = Object.fromEntries(['BASE_URL', 'API_KEY', 'MODEL'].map((k) => [k, process.env['CONTENT_AI_' + k]]))
  t.after(() => {
    for (const [k, v] of Object.entries(saved)) {
      if (v === undefined) delete process.env['CONTENT_AI_' + k]
      else process.env['CONTENT_AI_' + k] = v
    }
  })
  process.env.CONTENT_AI_BASE_URL = base
  process.env.CONTENT_AI_API_KEY = 'plan-fixture'
  process.env.CONTENT_AI_MODEL = 'content-fixture'
  const meta = { ...profile('content'), subject: 'plan:11111111-1111-1111-1111-111111111111', operation: 'configuration' }
  const body = (chars) => JSON.stringify({ model: 'content-fixture', messages: [{ role: 'user', content: 'あ'.repeat(chars) }] })
  const paired = body(6400)
  assert.ok(Buffer.byteLength(paired) > 19000)
  const res = await streamFetch(meta)(base + '/chat/completions', { method: 'POST', headers: { Authorization: 'Bearer plan-fixture' }, body: paired })
  assert.match(await res.text(), /DONE/)
  await assert.rejects(
    streamFetch(meta)(base + '/chat/completions', { method: 'POST', body: body(14000) }),
    /预留范围/,
  )
})

test('media HTTP errors retain actionable diagnostics without echoing private input', async () => {
  const { mediaProviderFailure } = await import('../src/media-provider-error.ts')
  const failure = (status, error) => mediaProviderFailure(new Response(null, {
    status, headers: { 'x-request-id': 'req_test' },
  }), error, 'https://api.openai.com/v1/images/edits', 'gpt-image-2')
  const blocked = failure(400, {code:'moderation_blocked', message:'private prompt', moderation_details:{moderation_stage:'input',categories:['violence']}})
  assert.equal(blocked.action, 'moderation')
  assert.equal(blocked.moderation_stage, 'input')
  assert.equal(blocked.request_id, 'req_test')
  assert.deepEqual(blocked.categories, ['violence'])
  assert.ok(!JSON.stringify(blocked).includes('private prompt'))
  const invalid = failure(400,{type:'invalid_request_error',param:'image[0]',message:'Unsupported image format: private-filename'})
  assert.equal(invalid.action,'correct_input')
  assert.equal(invalid.message,'Unsupported image format')
  assert.equal(invalid.param,'image[0]')
  assert.equal(failure(400,{}).action,'inspect')
  assert.equal(failure(429,{code:'insufficient_quota'}).action,'inspect')
  assert.equal(failure(429,{}).action,'retry')
  assert.equal(failure(503,{}).action,'retry')
})
