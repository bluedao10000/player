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
  const part =
    url.pathname.match(/(^|[-/])ep(\d+)(?=[-/.]|$)/i) ??
    url.pathname.match(/(^|[-/])(\d+)(?:wa)?(?:\.html?)?\/?$/i)
  if (part && Number(part[2]) === from) {
    url.pathname = url.pathname.replace(
      part[0],
      part[0].replace(part[2], String(to).padStart(part[2].length, '0')),
    )
    return url.href
  }
  for (const [key, value] of url.searchParams) {
    if (/^\d+$/.test(value) && Number(value) === from) {
      url.searchParams.set(key, String(to).padStart(value.length, '0'))
      return url.href
    }
  }
}

// Infer an offset only when two registered episodes share the exact URL pattern
// and the differing numeric field advances by their episode-number difference.
export function learnedSibling(
  base: string,
  number: number,
  target: number,
  other: { number: number; url: string },
) {
  if (other.number === number || !URL.canParse(other.url)) return
  const a = new URL(base),
    b = new URL(other.url)
  if (a.origin !== b.origin || a.hash || b.hash) return
  const path = a.pathname + a.search,
    peer = b.pathname + b.search
  for (const match of path.matchAll(/\d+/g)) {
    const before = path.slice(0, match.index),
      after = path.slice(match.index + match[0].length)
    if (!peer.startsWith(before) || !peer.endsWith(after)) continue
    const field = peer.slice(before.length, peer.length - after.length)
    const n = Number(match[0])
    if (!/^\d+$/.test(field) || Number(field) - n !== other.number - number) continue
    const next = n + target - number
    if (!Number.isSafeInteger(next) || next < 1) continue
    return (
      a.origin + before + String(next).padStart(match[0].startsWith('0') ? match[0].length : 1, '0') + after
    )
  }
}

export async function discoverRegisteredCandidates(
  sample: Sample,
  target: Episode,
  get = fetchSourceText,
  examples: Sample[] = [],
) {
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
    async function chooseSearchResult(entries: ReturnType<typeof links>) {
      const exact = choose(entries)
      if (exact || !series || official) return exact
      // Some review headlines print the work and episode number, while the
      // canonical subtitle appears only in the opening paragraphs.
      const related = [...new Set(entries.filter(a => episodeLabel.test(a.text) && a.text.includes(norm(series))).map(a => a.url))]
      if (related.length !== 1) return
      const page = load(await read(related[0]))
      const headline = norm(page('article h1').text())
      const opening = page('article .entry-content > p').slice(0,3).toArray().map(e=>norm(page(e).text()))
      if (headline.includes(norm(series)) && episodeLabel.test(headline) && opening.some(text=>text.includes(title))) return related[0]
    }
    const direct = choose(anchors)
    if (direct) return direct
    const candidates = examples.flatMap((entry) => {
      const url = official ? entry.official : entry.guide
      return url ? [learnedSibling(base, sample.number, target.number, { url, number: entry.number })] : []
    })
    const sibling = numberedSibling(base, from, to) ?? candidates.find(Boolean)
    if (sibling) {
      const page = load(await read(sibling))
      const heading = norm(
        page('h1,title,meta[property="og:title"]')
          .map((_, e) => page(e).attr('content') ?? page(e).text())
          .get()
          .join(' '),
      )
      if (
        episodeLabel.test(heading) &&
        (heading.includes(title) || norm(page('article .entry-content, .post h2').text()).includes(title))
      )
        return sibling
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
            const match = await chooseSearchResult(candidates)
            if (match) return match
          }
        } catch {
          /* Publishers can disable the optional REST API. */
        }
      }
    }
    // WordPress sites may expose search through a form even when the REST
    // endpoint is disabled. Use only the publisher's own GET form.
    if (!official) {
      const form = $('form').filter((_, el) => $(el).find('input[name="s"]').length > 0).first()
      const action = form.attr('action') || base
      if (form.length && (form.attr('method') || 'get').toLowerCase() === 'get' && URL.canParse(action, base)) {
        const search = new URL(action, base)
        if (search.origin === new URL(base).origin && !search.username && !search.password) {
          search.searchParams.set('s', target.title)
          const match = await chooseSearchResult(links(await read(search.href), search.href))
          if (match) return match
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
