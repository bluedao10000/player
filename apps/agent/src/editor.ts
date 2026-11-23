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
  const budgeted = stage === 'planning' || stage === 'configuration'
  const name = `editor-${id}-${round}-${attempt}-${stage}`
  if (active.has(name)) throw Error('任务正在执行')
  active.add(name)
  try {
    const models = createModels()
    models.setProvider(contentProvider(id, stage, budgeted ? 'plan' : stage === 'conversation' ? 'workbench-chat' : 'editor', budgeted || stage === 'conversation'))
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
              (stage === 'conversation'
                ? '你是 Folio 工作台助手。以 tools 的服务端数据为准，历史只用于理解指代。工具包含 get_workspace_context、get_knowledge_completeness（七维完整度与缺项）、get_production_plan、get_episode_status、get_media_candidates、get_execution_history。被问到评估作品某集数范围的知识覆盖与缺口时，先总结已有覆盖、逐项列出主要缺口与依据，再给出推荐补全顺序、任务量和预算依据（默认仅走本地验收，不自动触发公网发布）。只返回一个 JSON 对象。查询示例：{"text":"状态说明","patch":null}。只能为当前对话的作品新建制作计划，且作品必须已登记总集数。新建时省略 world 即使用当前作品。新建制作计划示例（作品尚无合适计划时使用）：{"text":"请确认新建本地验收计划","patch":{"tool":"create_production_plan","first_episode":1,"last_episode":10,"token_limit":600000,"character_consistency":"standard","preserve_published":true,"instruction":"补齐第1-10集缺口（默认本地验收）"}}。组合修改示例：{"text":"请确认差异","patch":{"tool":"update_production_plan","first_episode":1,"last_episode":20,"token_limit":1000000,"preserve_published":true}}。所有变更字段必须直接放到 patch 内，和 tool 同级，禁止使用 changes、fields、arguments、scope 或其他嵌套对象。只写用户明确要求的字段。工具可选 create_production_plan、update_production_plan、update_episode_scope、update_token_budget、update_production_policy、pause_production、resume_production、retry_episode、rebuild_episode。字段可选 world（作品 slug）、first_episode、last_episode、token_limit（整数）、character_consistency（standard 或 strict）、preserve_published（布尔值）、episode（整数）、instruction（新建计划说明）。始终保留已发布内容。修改与新建计划只生成建议，确认必须由用户点击，不可声称执行。'
                : stage === 'configuration'
                ? '依据输入的本集官方和指南句子生成制作配置。只输出 JSON 对象，字段 slug,title,intro,place,sceneSlug,sceneName,sceneBeat,setting:[设定词,段号],artBrief,people:[{slug,beat,evidence}],beats:[{kind,needle,question,officialSentence?}]。标题中文，slug 英文短横线。4至6个节点按时间排列，kind覆盖motive,turn,cause。needle必须从各不相同且依次递增的英文指南句子摘录连续原文。至少一项用officialSentence对应日语官方句子编号。question中文明确要写什么。所有编号从1开始。people 只选来源原文明确出现且有已登记肖像的角色。evidence 从该段对应的英语指南或官方句子中摘录角色原名或行动原文，未写到的人物直接省略。场景图与artBrief只选有据场景。若 portraitSlugs 非空，必须返回 artCharacter，其值为该列表中的角色 slug，该角色必须出现在 people 中，beat 等于 sceneBeat，artBrief 用选中角色的姓名说明画面动作。选中角色必须是原文记载的动作执行者。只介绍作品情节。'
                : stage === 'planning'
                ? '你负责分集制作规划。依据给定的作品、已核实缺口和历次修订意见，只返回 JSON 对象 {"episodes":[集号],"brief":"下一阶段执行的取证与制作要求"}。集号必须等于 gaps，按升序排列。不要补写未经取证的剧情，不得变更预算、授权或发布门槛。五维固定为人物、剧情、世界观、时间线和图片。'
                : rules + '\n' + (stage === 'drafting' ? writer : critic)) +
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
      const value = JSON.parse(text.replace(/^`{3}(?:json)?\s*|\s*`{3}$/g, ''))
      return budgeted ? { value, tokens: msg.usage?.totalTokens } : value
    } finally {
      await harness.close(ctx)
    }
  } finally {
    active.delete(name)
  }
}
