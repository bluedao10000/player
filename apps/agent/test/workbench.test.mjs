import test from 'node:test'
import assert from 'node:assert/strict'
import { once } from 'node:events'
import { mkdtemp, rm } from 'node:fs/promises'
import { createServer } from 'node:http'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { openUsage, pendingUsage } from '../src/model-api.ts'
import { streamWorkbenchRun } from '../src/workbench.ts'

async function listen(t, handler) {
  const server = createServer(handler)
  server.listen(0, '127.0.0.1')
  await once(server, 'listening')
  t.after(() => {
    server.closeAllConnections()
    server.close()
  })
  return 'http://127.0.0.1:' + server.address().port
}

test('workbench counts request bytes, accepting the limit and rejecting one extra byte', async (t) => {
  const url = await listen(t, (req, res) => streamWorkbenchRun(req, res, 'unused'))
  const text = JSON.stringify({ note: '中' }) // No intent: validation stops before a model call.
  const body = text + ' '.repeat(256 * 1024 - Buffer.byteLength(text))
  assert.equal((await fetch(url, { method: 'POST', body })).status, 400)
  assert.equal((await fetch(url, { method: 'POST', body: body + ' ' })).status, 413)
})

test('workbench streams model text and plan arguments and cancels a disconnected inference', async (t) => {
  const dir = await mkdtemp(join(tmpdir(), 'folio-workbench-'))
  t.after(() => rm(dir, { recursive: true, force: true }))
  openUsage(dir)
  const run = '12345678-1234-1234-1234-123456789012'
  const plan = {
    work_id: 'selected-work',
    title: '知识补全',
    objective: '核对已有角色出处',
    tasks: [{ title: '核对引用', priority: 'high' }],
  }

  let received
  let providerClosed
  const stopped = new Promise((resolve) => { providerClosed = resolve })
  const provider = await listen(t, async (req, res) => {
    const chunks = []
    for await (const chunk of req) chunks.push(chunk)
    received = JSON.parse(Buffer.concat(chunks).toString())
    const emit = (choices, usage) => {
      res.write('data: ' + JSON.stringify({
        id: 'req-workbench', model: 'content-test', choices, usage,
      }) + '\n\n')
    }
    res.writeHead(200, { 'Content-Type': 'text/event-stream' })
    emit([{ index: 0, delta: { role: 'assistant', content: '已核对。' }, finish_reason: null }])
    if (received.messages.at(-1).content.includes('取消测试')) {
      res.once('close', providerClosed)
      return
    }
    emit([{ index: 0, delta: { tool_calls: [{
      index: 0, id: 'call-plan', type: 'function',
      function: { name: 'propose_knowledge_plan', arguments: JSON.stringify(plan) },
    }] }, finish_reason: null }])
    emit([{ index: 0, delta: {}, finish_reason: 'tool_calls' }])
    emit([], { prompt_tokens: 21, completion_tokens: 8, total_tokens: 29 })
    res.end('data: [DONE]\n\n')
  })

  for (const [name, value] of Object.entries({
    CONTENT_AI_BASE_URL: provider,
    CONTENT_AI_API_KEY: 'test-token',
    CONTENT_AI_MODEL: 'content-test',
    CONTENT_AI_MODE: 'direct',
    CONTENT_AI_PROTOCOL: 'chat-completions',
  })) {
    const prior = process.env[name]
    t.after(() => prior === undefined ? delete process.env[name] : process.env[name] = prior)
    process.env[name] = value
  }
  const agent = await listen(t, (req, res) => streamWorkbenchRun(req, res, run))
  const send = (intent, signal) => fetch(agent, {
    method: 'POST', signal, headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ intent, plan_work_id: plan.work_id, works: [{ work: { work_id: plan.work_id }, gaps: [] }] }),
  })
  const response = await send('制定计划')
  assert.equal(response.status, 200)
  const events = (await response.text()).trim().split('\n').map((line) => JSON.parse(line))
  const { receipt, ...done } = events[2]
  assert.deepEqual([events[0], events[1], done], [
    { type: 'token', t: '已核对。' },
    { type: 'plan', plan },
    { type: 'done', input_tokens: 21, output_tokens: 8 },
  ])
  assert.equal(receipt.subject, 'workbench:' + run)
  assert.equal(receipt.usage.total_tokens, 29)
  assert.equal(receipt.outcome, 'ok')
  assert.equal(received.tools[0].function.name, 'propose_knowledge_plan')
  assert.equal(pendingUsage().at(-1).event.subject, 'workbench:' + run)

  const abort = new AbortController()
  const pending = await send('取消测试', abort.signal)
  const reader = pending.body.getReader()
  const chunk = await reader.read()
  assert.match(Buffer.from(chunk.value).toString(), /已核对/)
  abort.abort()
  let timeout
  try {
    await Promise.race([stopped, new Promise((_, reject) => {
      timeout = setTimeout(() => reject(Error('provider did not stop')), 4000)
    })])
  } finally {
    clearTimeout(timeout)
  }
})
