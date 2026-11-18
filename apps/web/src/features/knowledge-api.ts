import { createServerFn } from '@tanstack/react-start'
import { notFound } from '@tanstack/react-router'
import { getWork, KnowledgeError, searchWorks } from '@anime/shared/knowledge'

export const findWorks = createServerFn({ method: 'GET' })
  .validator((data: { q: string; after: string }) => data)
  .handler(({ data }) => searchWorks(data.q, data.after))
export const readWork = createServerFn({ method: 'GET' })
  .validator((id: string) => id)
  .handler(async ({ data }) => {
    try {
      const {
        id,
        canonical_title,
        world_href,
        cover_image,
        type,
        season_year,
        status,
        episodes,
        titles,
        studios,
        provenance,
      } = await getWork(data)
      return {
        id,
        canonical_title,
        world_href,
        cover_image,
        type,
        season_year,
        status,
        episodes,
        titles,
        studios,
        provenance,
      }
    } catch (error) {
      if (error instanceof KnowledgeError && [400, 404].includes(error.status))
        throw notFound()
      throw error
    }
  })
