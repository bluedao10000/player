import { normalizedTitle as norm } from './episode-source-rules.ts'
import { load } from 'cheerio'
import { fetchSourceText } from './source-fetch.ts'

type Episode = { number: number; sourceNumber?: number; title: string }
type Sample = Episode & { official: string; guide?: string }
function links(raw: string, base: string) {
  const $ = load(raw)
  return $('a[href]')
    .toArray()
    .flatMap((el) => {
      const href = $(el).attr('href')!
      if (!URL.canParse(href, base)) return []
      const url = new URL(href, base)
      if (url.origin !== new URL(base).origin || url.hash || url.username || url.password) return []
      return [{ url: url.href, text: norm($(el).text()) }]
    })
}
function numberedSibling(base: string, from: number, to: number) {
  const url = new URL(base)
  const part = url.pathname.match(/(^|[-/])ep(\d+)(?=[-/.]|$)/i) ?? url.pathname.match(/(^|[-/])(\d+)(?:wa)?(?:\.html?)?\/?$/i)
  if (part && Number(part[2]) === from) {
    url.pathname =
      url.pathname.replace(part[0],part[0].replace(part[2], String(to).padStart(part[2].length, '0')))
    return url.href
  }
  for (const [key, value] of url.searchParams) {
    if (/^\d+$/.test(value) && Number(value) === from) {
      url.searchParams.set(key, String(to).padStart(value.length, '0'))
      return url.href
    }
  }
}

export async function discoverRegisteredCandidates(sample: Sample, target: Episode, get = fetchSourceText) {
  const read = async (url: string) => {
    try {
      return await get(url)
    } catch {
      return ''
    }
  }
  const from = sample.sourceNumber ?? sample.number,
    to = target.sourceNumber ?? target.number
  const title = norm(target.title)
  const episodeLabel = new RegExp('(?:第|(?<![0-9]))0*' + to + '[話回集]')
  async function detail(base: string, official: boolean) {
    const raw = await read(base)
    const $ = load(raw)
    const printed = $('h1').first().text()
    const series = printed.match(/『([^』]+)』/)?.[1]
    const anchors = links(raw, base)
    function choose(entries: ReturnType<typeof links>) {
      const matches = entries.filter((a) => episodeLabel.test(a.text) && a.text.includes(title))
      const urls = new Set(
        matches
          .filter(
            (a) => !official || !matches.some((x) => /あらすじ/.test(x.text)) || /あらすじ/.test(a.text),
          )
          .map((a) => a.url),
      )
      if (urls.size === 1) return [...urls][0]
    }
    const direct = choose(anchors)
    if (direct) return direct
    const sibling = numberedSibling(base, from, to)
    if (sibling) {
      const page = load(await read(sibling))
      const heading = norm(
        page('h1,title,meta[property="og:title"]')
          .map((_, e) => page(e).attr('content') ?? page(e).text())
          .get()
          .join(' '),
      )
      if (episodeLabel.test(heading) && (heading.includes(title) || norm(page('article .entry-content').text()).includes(title))) return sibling
    }
    // A work tag or a series table is a candidate directory. Only use same-site
    // links and run the normal two-episode proof before binding any address.
    // WordPress exposes its own search API in the page head. Retain the
    // registered origin and accept only an exact episode and title candidate.
    const api = $('link[rel="https://api.w.org/"]').attr('href')
    if (!official && api && URL.canParse(api, base)) {
      const search = new URL('wp/v2/search', new URL(api, base))
      if (search.origin === new URL(base).origin) {
        search.searchParams.set('search', target.title)
        search.searchParams.set('per_page', '15')
        const data = await read(search.href)
        try {
          const entries: unknown = JSON.parse(data)
          if (Array.isArray(entries)) {
            const candidates = entries
              .filter(
                (e) =>
                  typeof e?.url === 'string' &&
                  typeof e?.title === 'string' &&
                  URL.canParse(e.url) &&
                  new URL(e.url).origin === search.origin,
              )
              .map((e) => ({ url: e.url as string, text: norm(load(e.title).text()) }))
            const match = choose(candidates)
            if (match) return match
          }
        } catch {
          /* Publishers can disable the optional REST API. */
        }
      }
    }
    const directories = [
      ...new Set(
        anchors
          .filter(
            (a) => /matome\/?$/.test(a.url) || (series && a.text === norm(series) && /\/tag\//.test(a.url)),
          )
          .map((a) => a.url),
      ),
    ].slice(0, 3)
    for (const directory of directories) {
      const data = await read(directory),
        idx = load(data)
      const row = idx('tr')
        .toArray()
        .filter((r) =>
          idx(r)
            .find('td')
            .toArray()
            .some((td) => norm(idx(td).text()) === '第' + to + '話'),
        )
      if (row.length === 1) {
        const choices = links(idx.html(row[0]), directory)
        if (choices.length === 1) return choices[0].url
      }
      const here = links(data, directory)
      const first = choose(here)
      if (first) return first
      const origin = new URL(directory)
      const pages = [
        ...new Set(
          here
            .filter((a) => {
              const next = new URL(a.url)
              return (
                /^\d+$/.test(a.text) &&
                next.pathname === origin.pathname &&
                [...origin.searchParams].every(([key, v]) => next.searchParams.get(key) === v)
              )
            })
            .map((a) => a.url),
        ),
      ].slice(0, 6)
      for (const url of pages) {
        const match = choose(links(await read(url), url))
        if (match) return match
      }
    }
  }
  const [official, guide] = await Promise.all([
    detail(sample.official, true),
    sample.guide ? detail(sample.guide, false) : undefined,
  ])
  return { official, guide }
}
