import { defineExtension, defineTool } from '@earendil-works/pi-durable'
import { obj, str } from '@earendil-works/pi-schema'
import { getWork, searchWorks } from '@anime/shared/knowledge'

const search = defineTool({
  name: 'search_anime_works',
  description:
    '在本地作品知识库按原名、译名或别名查找作品，返回 canonical UUID。',
  parameters: obj({ q: str() }),
  execute: async ({ q }, api) => {
    if (!q.trim() || q.length > 128)
      throw new Error('请输入 1 至 128 个字符的作品名')
    api.output(JSON.stringify((await searchWorks(q, '', 8)).items))
    return {}
  },
})
const detail = defineTool({
  name: 'read_anime_work',
  description:
    '按 canonical UUID 读取作品基础资料、关联来源与版本，用于区分资料与虚构剧情。',
  parameters: obj({ id: str() }),
  execute: async ({ id }, api) => {
    const work = await getWork(id)
    const {
      id: workId,
      canonical_title,
      type,
      season_year,
      episodes,
      status,
      studios,
      provenance,
    } = work
    api.output(
      JSON.stringify({
        id: workId,
        canonical_title,
        type,
        season_year,
        episodes,
        status,
        studios,
        titles: work.titles.slice(0, 20),
        tags: work.tags.slice(0, 30),
        related: work.relations.slice(0, 12),
        provenance,
      }),
    )
    return {}
  },
})
export const knowledgeTools = defineExtension({
  name: 'anime-knowledge',
  tools: [search, detail],
})
