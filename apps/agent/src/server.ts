import { createServer } from 'node:http'
import { mkdir } from 'node:fs/promises'
import { dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { BACKGROUND_CONTEXT } from '@earendil-works/chord/context'
import { createModels } from '@earendil-works/pi-ai/models'
import { createRegistry, Harness } from '@earendil-works/pi-durable'
import { openNodeSqliteStorage } from '@earendil-works/pi-durable/storage/sqlite/node'
import { mediaExtension, mediaStatus } from './media.ts'
import { openUsage, pendingUsage, acknowledgeUsage } from './model-api.ts'
import { research } from './research.ts'
import { knowledgeTools } from './knowledge.ts'
import { health } from '@anime/shared'

const path = process.env.AGENT_DB_PATH ?? fileURLToPath(new URL('../../../data/agent.sqlite', import.meta.url))
await mkdir(dirname(path), { recursive: true })
openUsage(dirname(path))
const storage = await openNodeSqliteStorage(path)
const registry = createRegistry()
registry.install(knowledgeTools)
registry.install(mediaExtension)
const harness = await Harness.open(storage, { models: createModels(), registry }, BACKGROUND_CONTEXT)
harness.resume()

const server = createServer(async (req, res) => {
  if (req.method === 'GET' && req.url === '/healthz') {
    res.writeHead(200, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify(health('agent')))
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
  const route = /^\/internal\/research\/([a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12})\/(collect|write)$/.exec(req.url ?? '')
  if (req.method === 'POST' && route) {
    try {
      let body = ''
      for await (const chunk of req) {
        body += chunk.toString()
        if (body.length > 16000) { res.writeHead(413).end(); return }
      }
      const result = await research(dirname(path), route[1], route[2], body)
      res.writeHead(200, { 'Content-Type': 'application/json' }).end(JSON.stringify(result))
    } catch (err) {
      const message = err instanceof Error ? err.message : '取证失败'
      res.writeHead(502, { 'Content-Type': 'application/json' }).end(JSON.stringify({ error: message.slice(0, 200) }))
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
