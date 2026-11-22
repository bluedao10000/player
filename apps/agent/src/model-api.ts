import { existsSync } from 'node:fs'
import { randomUUID } from 'node:crypto'
import { DatabaseSync } from 'node:sqlite'
import { join } from 'node:path'

type Purpose = 'content' | 'image' | 'vision'
export function profile(purpose: Purpose, originalImage = false) {
  const prefix = {
    content: 'CONTENT_AI',
    image: 'MEDIA_AI',
    vision: 'VISION_AI',
  }[purpose]
  const originalMode = purpose === 'image' && originalImage ? process.env.ORIGINAL_MEDIA_AI_MODE : undefined
  const mode = purpose === 'vision' ? 'direct' : originalMode || process.env[prefix + '_MODE'] || 'direct'
  if (mode !== 'direct' && mode !== 'magpie') throw Error(prefix + '_MODE 无效')
  const gateway = mode === 'magpie'
  const base = new URL(gateway
    ? process.env.MAGPIE_BASE_URL || 'http://127.0.0.1:3425/v1'
    : process.env[prefix + '_BASE_URL'] || 'https://api.openai.com/v1')
  const local = ['localhost', '127.0.0.1', '[::1]'].includes(base.hostname)
    || (gateway && base.hostname === 'host.docker.internal')
  if (
    base.username ||
    base.password ||
    base.search ||
    base.hash ||
    (base.protocol !== 'https:' && !(base.protocol === 'http:' && local))
  )
    throw Error(prefix + '_BASE_URL 无效')
  if (gateway) {
    const route = originalMode ? 'MAGPIE_ORIGINAL_IMAGE_MODEL' : 'MAGPIE_' + purpose.toUpperCase() + '_MODEL'
    const model = process.env[route]?.trim()
    if (!model || !/^[^/\s]+\/\S+$/.test(model)) throw Error('需要配置 ' + route + ' 完整路由')
    const key = process.env.MAGPIE_API_KEY || (local ? 'magpie' : '')
    if (!key) throw Error('远程网关需要 MAGPIE_API_KEY')
    return { purpose, gateway, account: process.env.MAGPIE_ACCOUNT || 'magpie', base: base.href.replace(/\/$/, ''), key, model }
  }
  const key =
    process.env[prefix + '_API_KEY'] ||
    (purpose !== 'image' && !process.env[prefix + '_BASE_URL']
      ? process.env.RESEARCH_OPENAI_API_KEY || process.env.OPENAI_API_KEY
      : '')
  if (!key) throw Error('需要配置 ' + prefix + '_API_KEY')
  const model =
    process.env[prefix + '_MODEL'] ||
    (purpose === 'content'
      ? process.env.RESEARCH_MODEL
      : purpose === 'vision'
        ? process.env.MEDIA_REVIEW_MODEL
        : '') ||
    (purpose === 'image' ? 'gpt-image-2' : 'gpt-4.1-mini')
  return {
    purpose,
    gateway,
    account: process.env[prefix + '_ACCOUNT'] || 'default',
    base: base.href.replace(/\/$/, ''),
    key,
    model,
  }
}
type Meta = {
  gateway?: boolean
  account: string
  size?: string
  quality?: string
  purpose: Purpose
  base: string
  model: string
  subject: string
  operation: string
}
type Counts = Record<string, unknown>
type Receipt = {
  usage?: Counts
  model?: string
  id?: string
  status?: string
  error?: unknown
  data?: { b64_json?: string }[]
  output?: { content?: { text?: string }[] }[]
}
let db: DatabaseSync
export function openUsage(dir: string) {
  db?.close()
  db = new DatabaseSync(join(dir, 'model-usage.sqlite'))
  db.exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;
    CREATE TABLE IF NOT EXISTS outbox (event_id TEXT PRIMARY KEY, payload TEXT NOT NULL)`)
}
function append(payload: object) {
  db.prepare('INSERT INTO outbox VALUES(?,?)').run(
    randomUUID(),
    JSON.stringify(payload),
  )
}
export function pendingUsage() {
  return db
    .prepare('SELECT event_id,payload FROM outbox ORDER BY rowid LIMIT 50')
    .all()
    .map((r) => ({
      event_id: r.event_id,
      event: JSON.parse(String(r.payload)),
    }))
}
export function acknowledgeUsage(ids: string[]) {
  const del = db.prepare('DELETE FROM outbox WHERE event_id=?')
  db.exec('BEGIN')
  try {
    for (const id of ids) del.run(id)
    db.exec('COMMIT')
  } catch (e) {
    db.exec('ROLLBACK')
    throw e
  }
}
function counters(value: unknown, depth = 0): Counts | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return
  const kept: Counts = {}
  for (const [k, v] of Object.entries(value)) {
    if (k.length > 64) continue
    if (typeof v === 'number' && Number.isSafeInteger(v) && v >= 0) kept[k] = v
    else if (depth < 2 && v && typeof v === 'object' && !Array.isArray(v))
      kept[k] = counters(v, depth + 1)
  }
  return kept
}
function start(m: Meta, idempotencyKey?: string | null) {
  const event = {
    id: randomUUID(),
    purpose: m.purpose,
    account: m.account,
    size: m.size,
    quality: m.quality,
    idempotency_key: idempotencyKey || undefined,
    endpoint: m.base,
    model: m.model,
    subject: m.subject,
    operation: m.operation,
    started: new Date().toISOString(),
  }
  append({ ...event, outcome: 'started' })
  return (
    res?: Response,
    body?: Receipt,
    complete = true,
    providerFailed = false,
  ) => {
    append({
      ...event,
      finished: new Date().toISOString(),
      outcome:
        !res || !complete
          ? 'unknown'
          : res.ok
            ? providerFailed
              ? 'provider_error'
              : 'ok'
            : 'http_error',
      status: res?.status,
      request: res?.headers.get('x-request-id') || undefined,
      response: typeof body?.id === 'string' ? body.id : undefined,
      resolved: typeof body?.model === 'string' ? body.model : undefined,
      usage: counters(body?.usage),
      images:
        m.purpose === 'image' && Array.isArray(body?.data)
          ? body.data.length
          : undefined,
    })
  }
}

async function budgetDispatch(meta:Meta, init:RequestInit) {
  const noop = async (_body?:Receipt, _complete?:boolean) => {}
  if((process.env.EPISODE_CALL_BUDGET !== 'required' && !existsSync('/state/episode-budget.required')) || !/^(research|media):/.test(meta.subject))return noop
  const call=randomUUID()
  const endpoint=process.env.EPISODE_BUDGET_URL || 'http://127.0.0.1:14328'
  const post=(path:string,body:unknown)=>fetch(endpoint+path,{method:'POST',body:JSON.stringify(body),signal:AbortSignal.timeout(10000)})
  const text=typeof init.body==='string'?init.body:''
  const allowance=meta.purpose==='content'?Buffer.byteLength(text)+5120:16000
  const result=await post('/reserve',{subject:meta.subject,call,step:meta.operation,allowance})
  if(!result.ok)throw Error('分集预算未获预留 HTTP '+result.status)
  const owner=await result.json()
  if(owner==='unmanaged')return noop
  if(meta.purpose!=='image'){
    const payload=JSON.parse(text)
    const limit=meta.purpose==='vision'?1024:4096
    if('max_completion_tokens' in payload)payload.max_completion_tokens=limit
    else if('input' in payload)payload.max_output_tokens=limit
    else payload.max_tokens=limit
    init.body=JSON.stringify(payload)
  }
  return async (body?:Receipt,complete=true)=>{
    const u=body?.usage
    const tokens=u?.total_tokens??(typeof u?.input_tokens==='number'&&typeof u.output_tokens==='number'?u.input_tokens+u.output_tokens:null)
    const reply=await post('/settle',{owner,call,tokens:complete&&typeof tokens==='number'&&tokens>0?tokens:null})
    if(!reply.ok)throw Error('分集用量结算失败，预留已保留')
  }
}

// Consume the provider response once. Record it before image validation or review parsing.
export async function modelJSON(
  meta: Meta,
  url: string,
  init: RequestInit,
): Promise<Receipt> {
  const settle=await budgetDispatch(meta,init)
  const finish = start(meta, new Headers(init.headers).get('idempotency-key'))
  let res: Response | undefined,
    body: Receipt | undefined,
    complete = false
  try {
    res = await fetch(url, { ...init, redirect: 'error' })
    if (!res.ok) {
      body = await res.json().catch(() => undefined)
      complete = true
      throw Error('模型服务 HTTP ' + res.status)
    }
    const result: Receipt = await res.json()
    // Magpie Images synthesizes three zero counters when its upstream omitted usage.
    // Keep the image count, without treating that unknown cost as measured zero.
    const u = result.usage
    const unknownImageUsage = meta.purpose === 'image' && meta.gateway
      && result.data?.length && u?.input_tokens === 0 && u?.output_tokens === 0 && u?.total_tokens === 0
    body = unknownImageUsage ? { ...result, usage: undefined } : result
    complete = true
    return result
  } finally {
    await settle(body,complete)
    finish(
      res,
      body,
      complete,
      body?.status === 'failed' || Boolean(body?.error),
    )
  }
}

// Pi can perform multiple turns and transport retries for one durable step.
// Observe each actual HTTP dispatch, including a final streaming usage receipt.
export function streamFetch(meta: Meta): typeof fetch {
  let sent = false
  return async (input, init) => {
    if (meta.subject.startsWith('plan:')) {
      if (sent) throw Error('再次规划调用需要独立预算预留')
      if (typeof init?.body !== 'string' || Buffer.byteLength(init.body) > 12000)
        throw Error('规划输入超过预留范围')
      const payload = JSON.parse(init.body)
      if ('max_completion_tokens' in payload) payload.max_completion_tokens = 2048
      else if ('max_output_tokens' in payload || String(input).endsWith('/responses')) payload.max_output_tokens = 2048
      else payload.max_tokens = 2048
      init = {...init, body:JSON.stringify(payload)}
      sent = true
    }
    const url = new URL(input instanceof Request ? input.url : String(input))
    if (url.origin !== new URL(meta.base).origin)
      throw Error('模型请求地址与配置不一致')
    init={...init}
    const settle=await budgetDispatch(meta,init)
    const finish = start(meta)
    let res: Response
    try {
      res = await fetch(input, { ...init, redirect: 'error' })
    } catch (e) {
      await settle(undefined,false)
      finish(undefined, undefined, false)
      throw e
    }
    if (!res.ok) {
      const body = await res
        .clone()
        .json()
        .catch(() => undefined)
      await settle(body)
      finish(res, body)
      return res
    }
    if (!res.body) {
      await settle(undefined,false)
      finish(res, undefined, false)
      return res
    }
    const reader = res.body.getReader(),
      decoder = new TextDecoder()
    let line = '',
      receipt: Receipt | undefined,
      terminal = false,
      closed = false,
      providerFailed = false
    function observe(data: string) {
      if (!data.startsWith('data:')) return
      const s = data.slice(5).trim()
      if (s === '[DONE]') {
        terminal = true
        return
      }
      try {
        const obj = JSON.parse(s)
        const b = obj.response ?? obj
        // Responses may expose provisional counters before its terminal frame.
        // Chat Completions supplies final usage in a separate chunk.
        if (!receipt || (!obj.type && b.usage))
          receipt = {
            id: b.id ?? receipt?.id,
            model: b.model ?? receipt?.model,
            usage: obj.type ? undefined : b.usage,
          }
        if (obj.type === 'response.failed' || obj.error) providerFailed = true
        if (
          [
            'response.completed',
            'response.incomplete',
            'response.failed',
          ].includes(obj.type)
        ) {
          receipt = b
          terminal = true
        }
      } catch {
        /* Truncated SSE frames have no authoritative usage. */
      }
    }
    async function end() {
      if (!closed) {
        closed = true
        await settle(receipt,terminal)
        finish(res, receipt, terminal, providerFailed)
      }
    }
    const body = new ReadableStream<Uint8Array>({
      async pull(c) {
        try {
          const { done, value } = await reader.read()
          if (done) {
            observe(line + decoder.decode())
            await end()
            c.close()
            return
          }
          const lines = (line + decoder.decode(value, { stream: true })).split(
            '\n',
          )
          line = lines.pop()!
          for (const l of lines) observe(l)
          if (terminal) await end()
          c.enqueue(value)
        } catch (e) {
          await end()
          c.error(e)
        }
      },
      async cancel(reason) {
        try {
          await reader.cancel(reason)
        } finally {
          await end()
        }
      },
    })
    return new Response(body, {
      status: res.status,
      statusText: res.statusText,
      headers: res.headers,
    })
  }
}
