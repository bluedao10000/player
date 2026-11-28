import type { ServerResponse } from 'node:http'

// Send 413 before leaving the iterator: exiting a Node request iterator can close the stream.
export async function readRequestText(req: AsyncIterable<Buffer>, res: ServerResponse, maxBytes: number) {
  const parts: Buffer[] = []
  let size = 0
  for await (const part of req) {
    size += part.length
    if (size > maxBytes) {
      res.writeHead(413).end()
      return null
    }
    parts.push(part)
  }
  return Buffer.concat(parts, size).toString('utf8')
}
