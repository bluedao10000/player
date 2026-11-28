import type { IncomingMessage, ServerResponse } from 'node:http'
import { profile } from './model-api.ts'

// The research node has the direct OpenAI route. This endpoint only counts an
// inline vision-review input. The media worker still reserves before inference.
export async function countVisionInput(req: IncomingMessage, res: ServerResponse) {
  try {
    if (!['127.0.0.1','::1','::ffff:127.0.0.1'].includes(req.socket.remoteAddress ?? '')) {
      res.writeHead(403).end()
      return
    }
    const chunks: Buffer[] = []
    let size = 0
    for await (const part of req) {
      size += part.length
      if (size > 4_000_000) { res.writeHead(413).end(); return }
      chunks.push(part)
    }
    const data = JSON.parse(Buffer.concat(chunks).toString())
    const message = data.input?.[0]
    if (typeof data.model !== 'string' || !/^[\w.-]{1,80}$/.test(data.model) ||
        data.input?.length !== 1 || message?.role !== 'user' || !Array.isArray(message.content) ||
        message.content.length > 12) throw Error('invalid')
    const content = message.content.map((item: { type: string; text?: string; image_url?: string }) => {
      if (item.type === 'input_text' && typeof item.text === 'string') return { type: item.type, text: item.text }
      if (item.type === 'input_image' && typeof item.image_url === 'string' &&
          /^data:image\/jpeg;base64,[A-Za-z0-9+/]+=*$/.test(item.image_url)) return { type: item.type, image_url: item.image_url }
      throw Error('invalid')
    })
    const api = profile('content')
    if (api.gateway || api.base !== 'https://api.openai.com/v1') throw Error('direct OpenAI required')
    const response = await fetch(api.base + '/responses/input_tokens', { method: 'POST', redirect: 'error',
      headers: { Authorization: 'Bearer ' + api.key, 'Content-Type': 'application/json' },
      body: JSON.stringify({ model: data.model, input: [{role:'user',content}], text: {format:{type:'json_object'}} }),
      signal: AbortSignal.timeout(20000),
    })
    if (!response.ok) throw Error('count unavailable')
    const { input_tokens } = await response.json()
    if (!Number.isSafeInteger(input_tokens) || input_tokens <= 0) throw Error('invalid count')
    res.writeHead(200, {'Content-Type':'application/json'}).end(JSON.stringify({input_tokens}))
  } catch {
    res.writeHead(503).end()
  }
}
