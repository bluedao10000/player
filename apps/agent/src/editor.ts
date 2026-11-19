import { join } from 'node:path'
import { BACKGROUND_CONTEXT as ctx } from '@earendil-works/chord/context'
import { createModels } from '@earendil-works/pi-ai/models'
import {
  Harness,
  AssistantEntry,
  createRegistry,
} from '@earendil-works/pi-durable'
import { openNodeSqliteStorage } from '@earendil-works/pi-durable/storage/sqlite/node'
import { contentProvider } from './content-provider.ts'
import { profile } from './model-api.ts'

const active = new Set<string>()
const rules =
  '你是作品知识库的中文编辑。给定材料中 instruction 和 feedback 是编辑意图，sources 是事实依据。仅在指定字段范围内修订；不得编造原作事实、动机或台词。原创保留人称、留白与文风。读者正文不放来源、编辑说明或小标题。'
const writer =
  '依据意见和自检问题做最小改动。只返回 JSON 数组，每项 {"key":"被改字段原键","text":"修订后完整正文","source":"材料 id","quote":"该材料 text 中连续、逐字一致的原文摘录"}。至少修改一个字段，每个字段只出现一次。若需修两处，在同一批次给出。引用英文材料时保留英文原文。缺乏依据时不要补造。'
const critic =
  '独立审稿，逐项检查 draft 中每条 text（将要发布的正文），previous_published_text 是修改前的旧文。只评价新正文是否落实编辑意见，对照来源和引文核验事实。检查事实、前后因果、文风、任务是否完成和证据能否支撑新增事实。不要因没有修改无关段落而退回。只返回 JSON 对象 {"pass":true或false,"note":"明确中文理由或可执行的返工要求"}。'

export async function editText(
  dir: string,
  id: string,
  round: number,
  attempt: number,
  stage: string,
  input: string,
) {
  const name = `editor-${id}-${round}-${attempt}-${stage}`
  if (active.has(name)) throw Error('任务正在执行')
  active.add(name)
  try {
    const models = createModels()
    models.setProvider(contentProvider(id, stage, 'editor'))
    const storage = await openNodeSqliteStorage(join(dir, name + '.sqlite'))
    const harness = await Harness.open(
      storage,
      {
        models,
        registry: createRegistry(),
        settings: { retry: { maxRetries: 1 } },
      },
      ctx,
    )
    try {
      const root = await harness.root(ctx, {
        agent: {
          model: { provider: 'openai', modelId: profile('content').model },
          thinkingLevel: 'off',
          tools: [],
          maxTurns: 1,
        },
      })
      harness.resume()
      const result = await (
        await root.submit(
          {
            type: 'input',
            requestId: name,
            content:
              rules +
              '\n' +
              (stage === 'drafting' ? writer : critic) +
              '\n' +
              input,
          },
          ctx,
        )
      ).wait(ctx)
      if (result.type !== 'input' || result.status !== 'done')
        throw Error('内容模型未完成编辑步骤')
      const entry = await root.commit(
        (tx) => tx.entry(AssistantEntry, result.answer),
        ctx,
      )
      const msg = entry?.model?.[0]
      if (msg?.role !== 'assistant') throw Error('模型未返回正文')
      if (msg.stopReason === 'error')
        throw Error(
          '内容模型调用失败：' +
            (msg.errorMessage || '请检查内容模型配置').slice(0, 160),
        )
      const text = msg.content
        .flatMap((p) => (p.type === 'text' ? [p.text] : []))
        .join('')
        .trim()
      if (!text || msg.stopReason === 'length')
        throw Error('内容模型未返回完整修订，终止原因：' + msg.stopReason)
      return JSON.parse(text.replace(/^`{3}(?:json)?\s*|\s*`{3}$/g, ''))
    } finally {
      await harness.close(ctx)
    }
  } finally {
    active.delete(name)
  }
}
