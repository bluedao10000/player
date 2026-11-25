import { notFound } from '@tanstack/react-router'
import {
  getWorld,
  worldCharacter,
  worldChapter,
  worldScene,
  KnowledgeError,
} from '@anime/shared/knowledge'

async function read<T>(fn: () => Promise<T>) {
  try {
    return await fn()
  } catch (error) {
    if (error instanceof KnowledgeError && error.status === 404)
      throw notFound()
    throw error
  }
}
type Child = { work: string; slug: string }
export const overview = ({ data }: { data: string }) => read(() => getWorld(data))
export const character = ({ data }: { data: Child }) => read(() => worldCharacter(data.work, data.slug))
export const chapter = ({ data }: { data: Child }) => read(() => worldChapter(data.work, data.slug))
export const scene = ({ data }: { data: Child }) => read(() => worldScene(data.work, data.slug))
