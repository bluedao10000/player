import { createServer } from 'node:http'
import { mkdir } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { homedir } from 'node:os'
import { BACKGROUND_CONTEXT } from '@earendil-works/chord/context'
import { createModels } from '@earendil-works/pi-ai/models'
import { createRegistry, Harness } from '@earendil-works/pi-durable'
import { openNodeSqliteStorage } from '@earendil-works/pi-durable/storage/sqlite/node'
import { mediaExtension, mediaStatus } from './media.ts'
import { openUsage, pendingUsage, acknowledgeUsage } from './model-api.ts'
import { prepareEpisodeConfiguration } from './episode-configuration-task.ts'
import { runProductionModel } from './production-model.ts'
import { research } from './research.ts'
import { ExecutionCounters, readAgentStatus } from './executions.ts'
import { health } from '@anime/shared'

const path = process.env.AGENT_DB_PATH ?? join(homedir(), '.local/share/folio/agent.sqlite')
await mkdir(dirname(path), { recursive: true })
openUsage(dirname(path))
const storage = await openNodeSqliteStorage(path)
const registry = createRegistry()
registry.install(mediaExtension)
const harness = await Harness.open(storage, { models: createModels(), registry }, BACKGROUND_CONTEXT)
harness.resume()
const appVersion = process.env.APP_VERSION ?? 'dev'
const startedAt = new Date().toISOString()
const executionCounters = new ExecutionCounters()

const server = createServer(async (req, res) => {
  if (req.method === 'GET' && req.url === '/healthz') {
    res.writeHead(200, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify(health('agent')))
    return
  }
  if (req.method === 'GET' && req.url === '/internal/status') {
    try {
      const status = await readAgentStatus(
        harness,
        BACKGROUND_CONTEXT,
        appVersion,
        startedAt,
        executionCounters,
      )
      res
        .writeHead(200, { 'Content-Type': 'application/json' })
        .end(JSON.stringify(status))
    } catch {
      res.writeHead(503).end('status unavailable')
    }
    return
  }
  if (req.url === '/internal/model-usage' && req.method === 'GET') {
    res
      .writeHead(200, { 'Content-Type': 'application/json' })
      .end(JSON.stringify(pendingUsage()))
    return
  }
  if (req.url === '/internal/model-usage/ack' && req.method === 'POST') {
    try {
      let body = ''
      for await (const chunk of req) {
        body += chunk
        if (body.length > 4096) {
          res.writeHead(413).end()
          return
        }
      }
      const ids = JSON.parse(body)
      if (
        !Array.isArray(ids) ||
        ids.length > 50 ||
        !ids.every((id) => typeof id === 'string' && /^[a-f0-9-]{36}$/.test(id))
      )
        throw Error()
      acknowledgeUsage(ids)
      res.writeHead(204).end()
    } catch {
      res.writeHead(400).end()
    }
    return
  }
  if (req.method === 'POST' && req.url === '/internal/media') {
    try {
      const chunks: Buffer[] = []
      let size = 0
      for await (const chunk of req) {
        size += chunk.length
        if (size > 4096) {
          res.writeHead(413).end()
          return
        }
        chunks.push(chunk)
      }
      const result = await mediaStatus(
        harness,
        JSON.parse(Buffer.concat(chunks).toString()),
      )
      res
        .writeHead(200, { 'Content-Type': 'application/json' })
        .end(JSON.stringify(result))
    } catch {
      res.writeHead(503).end('media task unavailable')
    }
    return
  }
  const configuration = /^\/internal\/episode-configuration\/([0-9a-f-]{36})$/.exec(req.url ?? '')
  if (req.method === 'POST' && configuration) {
    const stopExecution = executionCounters.start('configuration')
    try {
      let body = ''
      for await (const part of req) {
        body += part
        if (Buffer.byteLength(body) > 16000) { res.writeHead(413).end(); return }
      }
      const result = await prepareEpisodeConfiguration(dirname(path), configuration[1], JSON.parse(body))
      res.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(result))
    } catch {
      res.writeHead(502).end()
    } finally {
      stopExecution()
    }
    return
  }
  const planning = /^\/internal\/editor\/([0-9a-f-]{36})\/(\d+)\/(\d+)\/planning$/.exec(req.url ?? '')
  if (req.method === 'POST' && planning) {
    const stopExecution = executionCounters.start('planning')
    try {
      const parts: Buffer[] = []
      let bytes = 0
      for await (const part of req) {
        bytes += part.length
        if (bytes > 100000) {
          res.writeHead(413).end()
          return
        }
        parts.push(part)
      }
      const result = await runProductionModel(
        dirname(path),
        planning[1],
        Number(planning[2]),
        Number(planning[3]),
        'planning',
        Buffer.concat(parts).toString(),
      )
      res
        .writeHead(200, { 'Content-Type': 'application/json' })
        .end(JSON.stringify(result))
    } catch (e) {
      res
        .writeHead(502)
        .end(
          JSON.stringify({
            error:
              e instanceof Error ? e.message.slice(0, 240) : '规划模型调用失败',
          }),
        )
    } finally {
      stopExecution()
    }
    return
  }
  const route = /^\/internal\/research\/([a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12})\/(collect|write)$/.exec(req.url ?? '')
  if (req.method === 'POST' && route) {
    const stopExecution = executionCounters.start('research')
    try {
      const chunks: Buffer[] = []
      let size = 0
      for await (const chunk of req) {
        size += chunk.length
        if (size > 160 * 1024) { res.writeHead(413).end(); return }
        chunks.push(chunk)
      }
      const result = await research(dirname(path), route[1], route[2], Buffer.concat(chunks).toString('utf8'))
      res.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(result))
    } catch (err) {
      const message = err instanceof Error ? err.message : '取证失败'
      res.writeHead(502, { 'Content-Type': 'application/json' }).end(JSON.stringify({ error: message.slice(0, 200) }))
    } finally {
      stopExecution()
    }
    return
  }
  res.writeHead(404).end()
})
server.listen(Number(process.env.AGENT_PORT ?? 4312), process.env.AGENT_HOST ?? '127.0.0.1', () => {
  console.log('Agent listening', server.address())
})

function shutdown() {
  server.close(async () => {
    await harness.close(BACKGROUND_CONTEXT)
  })
}
process.once('SIGINT', shutdown)
process.once('SIGTERM', shutdown)
