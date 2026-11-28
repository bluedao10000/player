import { normalizedTitle as label } from './episode-source-rules.ts'
import { load } from 'cheerio'
import { maximumSourceLength, normalizeBody } from './episode-source-rules.ts'
import { fetchSourceText } from './source-fetch.ts'

type Episode = { number: number; sourceNumber?: number; title: string }
function sameSite(value: string, base: string) {
  if (!URL.canParse(value, base)) return
  const url = new URL(value, base)
  if (
    url.origin === new URL(base).origin &&
    url.protocol === 'https:' &&
    !url.username &&
    !url.password &&
    !url.hash &&
    !url.port
  )
    return url.href
}

// A publisher may give its recap an editorial headline. Its own catalog must
// bind the exact detail URL, canonical episode title and release number together.
export async function linkedGuideIdentity(detail: string, episode: Episode, get = fetchSourceText) {
  const $ = load(await get(detail))
  const heading = $('article h1')
  if (heading.length !== 1) return
  const candidates = new Set<string>()
  for (const a of $('article .entry-content a[href], article a[rel="index"]').toArray()) {
    const href = sameSite($(a).attr('href')!, detail)
    if (href && href !== detail && $(a).text().trim()) candidates.add(href)
    if (candidates.size >= 4) break
  }
  for (const url of candidates) {
    let raw: string
    try {
      raw = await get(url)
    } catch {
      continue
    }
    const index = load(raw)
    const rows = index('tr')
      .toArray()
      .filter((row) => {
        const cells = index(row)
          .find('td')
          .toArray()
          .map((td) => label(index(td).text()))
        return (
          cells.includes('第' + (episode.sourceNumber ?? episode.number) + '話') &&
          cells.includes(label(episode.title)) &&
          index(row)
            .find('a[href]')
            .toArray()
            .some((a) => sameSite(index(a).attr('href')!, url) === detail)
        )
      })
    if (rows.length === 1) return { guideIndex: url, guideTitle: heading.text().trim() }
  }
}

// Some catalogs encode <work id><zero-padded episode> in one query field.
// Require the independent work-id field and matching publisher metadata on both pages.
export async function siblingOfficial(
  sample: Episode & { official: string },
  target: Episode,
  get = fetchSourceText,
) {
  const url = new URL(sample.official)
  const n = sample.sourceNumber ?? sample.number,
    next = target.sourceNumber ?? target.number
  for (const [key, value] of url.searchParams) {
    for (const [other, prefix] of url.searchParams) {
      if (key === other || !/^\d+$/.test(prefix) || !value.startsWith(prefix)) continue
      const suffix = value.slice(prefix.length)
      if (!/^\d{1,4}$/.test(suffix) || Number(suffix) !== n) continue
      const candidate = new URL(url)
      candidate.searchParams.set(key, prefix + String(next).padStart(suffix.length, '0'))
      const work = async (href: string, e: Episode) => {
        const $ = load(await get(href))
        const tags = $('meta[property="og:title"]')
        if (tags.length !== 1) return
        const m = label(tags.attr('content') ?? '').match(/^(.+?)第(\d+)話(.+)$/u)
        if (m && Number(m[2]) === (e.sourceNumber ?? e.number) && m[3] === label(e.title)) return m[1]
      }
      try {
        const [a, b] = await Promise.all([work(url.href, sample), work(candidate.href, target)])
        if (a && a === b) return candidate.href
      } catch {
        continue
      }
    }
  }
}

// Keep complete paragraphs on unusually long recaps. The same selector must
// still pass identity, independent narrative and sample checks on both pages.
export function guideBodySelectors(documents: string[], selector = 'article .entry-content > p') {
  if (!/ > p$/.test(selector)) return [selector]
  let count = Infinity
  for (const raw of documents) {
    const $ = load(raw)
    if ($(selector.slice(0,-4)).length !== 1) return [selector]
    let length = 0
    const paragraphs = $(selector).toArray()
    for (const [index, paragraph] of paragraphs.entries()) {
      const text = normalizeBody($(paragraph).text())
      if (!text) continue
      length += (length ? 1 : 0) + text.length
      if (length > maximumSourceLength) {
        count = Math.min(count, index)
        break
      }
    }
  }
  return Number.isFinite(count) && count > 0 ? [selector, selector + ':nth-of-type(-n+' + count + ')'] : [selector]
}
