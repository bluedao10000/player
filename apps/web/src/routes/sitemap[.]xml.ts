import { createFileRoute } from '@tanstack/react-router'
import { getWorld, homeWorlds } from '@anime/shared/knowledge'
import { siteOrigin } from '../site'

const publicWorkPath = /^\/work\/[a-z0-9-]+(?:\/(?:chapter|character|scene)\/[a-z0-9-]+)?$/

async function sitemap() {
  try {
    const { items } = await homeWorlds()
    const worlds = await Promise.all(items.map((work) => getWorld(work.slug)))
    const paths = new Set(worlds.flatMap(({ work, characters, scenes, storylines }) => [
      work.href,
      ...characters.map(({ href }) => href),
      ...scenes.map(({ href }) => href),
      ...storylines.flatMap(({ chapters }) => chapters.map(({ href }) => href)),
    ]).filter((path) => publicWorkPath.test(path)))
    const entries = ['/', '/works', '/characters', '/stories', ...Array.from(paths).sort()]
      .map((path) => '  <url><loc>' + siteOrigin + path + '</loc></url>')
      .join('\n')
    return new Response(`<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${entries}\n</urlset>\n`, {
      headers: { 'Content-Type': 'application/xml; charset=utf-8', 'Cache-Control': 'public, max-age=300' },
    })
  } catch (error) {
    console.error('Sitemap could not read published works', error)
    return new Response('Sitemap temporarily unavailable\n', {
      status: 503, headers: { 'Cache-Control': 'no-store', 'Retry-After': '60' },
    })
  }
}

export const Route = createFileRoute('/sitemap.xml')({
  server: { handlers: { GET: sitemap } },
})
