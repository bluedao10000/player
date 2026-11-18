import { createServerFn } from '@tanstack/react-start'
import { notFound } from '@tanstack/react-router'
import {
  homeWorlds,
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
const child = (data: Child) => data
export const discovery = createServerFn({ method: 'GET' }).handler(() =>
  homeWorlds(),
)
export const overview = createServerFn({ method: 'GET' })
  .validator((slug: string) => slug)
  .handler(({ data }) => read(() => getWorld(data)))
export const character = createServerFn({ method: 'GET' })
  .validator(child)
  .handler(({ data }) => read(() => worldCharacter(data.work, data.slug)))
export const chapter = createServerFn({ method: 'GET' })
  .validator(child)
  .handler(({ data }) => read(() => worldChapter(data.work, data.slug)))
export const scene = createServerFn({ method: 'GET' })
  .validator(child)
  .handler(({ data }) => read(() => worldScene(data.work, data.slug)))
