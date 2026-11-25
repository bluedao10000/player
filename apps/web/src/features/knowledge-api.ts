import { notFound } from '@tanstack/react-router'
import { getWork, KnowledgeError, searchWorks } from '@anime/shared/knowledge'

export const findWorks = ({ data }: { data: { q: string; after: string } }) => searchWorks(data.q, data.after)
export const readWork = async ({ data }: { data: string }) => {
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
}
