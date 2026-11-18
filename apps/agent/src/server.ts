import { createServer } from 'node:http'
import { mkdir } from 'node:fs/promises'
import { dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { BACKGROUND_CONTEXT } from '@earendil-works/chord/context'
import { createModels } from '@earendil-works/pi-ai/models'
import { createRegistry, Harness } from '@earendil-works/pi-durable'
import { openNodeSqliteStorage } from '@earendil-works/pi-durable/storage/sqlite/node'
import { knowledgeTools } from './knowledge.ts'
import { health } from '@anime/shared'

const path = process.env.AGENT_DB_PATH ?? fileURLToPath(new URL('../../../data/agent.sqlite', import.meta.url))
await mkdir(dirname(path), { recursive: true })
const storage = await openNodeSqliteStorage(path)
const registry = createRegistry()
registry.install(knowledgeTools)
const harness = await Harness.open(storage, { models: createModels(), registry }, BACKGROUND_CONTEXT)
harness.resume()

const server = createServer((req, res) => {
  if (req.method === 'GET' && req.url === '/healthz') {
    res.writeHead(200, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify(health('agent')))
    return
  }
  res.writeHead(404).end()
})
server.listen(Number(process.env.AGENT_PORT ?? 4312), '127.0.0.1', () => {
  console.log('Agent listening', server.address())
})

function shutdown() {
  server.close(async () => {
    await harness.close(BACKGROUND_CONTEXT)
  })
}
process.once('SIGINT', shutdown)
process.once('SIGTERM', shutdown)
