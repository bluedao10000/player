import { defineExtension, defineTool } from '@earendil-works/pi-durable'
import { unsafe } from '@earendil-works/pi-schema'
import { getWork, searchWorks } from '@anime/shared/knowledge'
import { knowledgeTools as manifest } from '@anime/shared/knowledge-tools'

type CatalogTool = Extract<typeof manifest[number], { scope: 'public_catalog' }>

export function catalogTools() {
  return manifest
    .filter((tool): tool is CatalogTool => tool.scope === 'public_catalog')
    .map(tool => defineTool({
      name: tool.name,
      description: tool.description,
      parameters: unsafe<Record<string, string>>(tool.schema),
      execute: async (args, api) => {
        const data = await readCatalog(tool, args)
        if (Buffer.byteLength(data) > tool.limits.bytes) throw Error('目录资料过长。')
        api.output(data)
        return {}
      },
    }))
}

async function readCatalog(tool: CatalogTool, args: Record<string, string>) {
  switch (tool.name) {
    case 'search_anime_works': {
      const { q } = args
      if (!q?.trim() || q.length > tool.schema.properties.q.maxLength)
        throw Error('请输入 1 至 128 个字符的作品名')
      return JSON.stringify((await searchWorks(q, '', tool.limits.rows)).items)
    }
    case 'read_anime_work': {
      const work = await getWork(args.id ?? '')
      const { id, canonical_title, type, season_year, episodes, status, studios, provenance } = work
      return JSON.stringify({
        id, canonical_title, type, season_year, episodes, status, studios,
        titles: work.titles.slice(0, tool.limits.titles),
        tags: work.tags.slice(0, tool.limits.tags),
        related: work.relations.slice(0, tool.limits.related),
        provenance,
      })
    }
  }
}

export const knowledgeTools = defineExtension({
  name: 'anime-knowledge',
  tools: catalogTools(),
})

export function conversationReadInstructions(input: string) {
  const { capabilities, allow_reads, max_reads } = JSON.parse(input) as {
    capabilities?: unknown[]
    allow_reads?: boolean
    max_reads?: number
  }
  const allowed = new Set(Array.isArray(capabilities) ? capabilities : [])
  const entries = manifest.filter(tool => allowed.has(tool.name))
  const facts = entries.filter(tool => tool.mode === 'snapshot')
    .map(tool => `${tool.name}：${tool.description}`).join('\n')
  const callable = entries.filter(tool => tool.mode === 'callable')
    .map(tool => `${tool.name}：${tool.description} 参数 ${JSON.stringify(tool.schema)}`).join('\n')
  return [
    facts && '当前作品已注入的工作台数据\n' + facts,
    allow_reads && callable
      ? `确需补充事实时可请求一批只读工具，最多 ${max_reads} 次。只返回 JSON 对象 {"read_calls":[{"name":"工具名","arguments":{"参数名":"值"}}]}。同一次响应不得同时请求读取和提出变更。工作台读取只限 selected_work 标注的当轮选定作品。\n${callable}`
      : '读取环节已结束。以已提供的事实回答。若 read_results 中有错误，说明相应资料未能读取。',
  ].filter(Boolean).join('\n')
}
