import { createHash } from 'node:crypto'
import { load } from 'cheerio'

export type EpisodeIdentity = { number: number; title: string }

export type EpisodeSourceRule =
  | { kind: 'json_episode_list'; episode: string; title: string; body: string }
  | { kind: 'html_episode_article'; heading: string; titleSelector?: string; body: string }

export const normalizedTitle = (text: string) => text.normalize('NFKC').replace(/\.{3,}/g, '…').replace(/!+/g, '!').replace(/\s+/g, '').replace(/」「/g, '/').toLowerCase()
export const maximumSourceLength = 6000
export const normalizeBody = (text: string) => text.normalize('NFKC').replace(/\s+/g, ' ').trim()

export function parseEpisodeSourceRule(value: unknown): EpisodeSourceRule {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw Error('采集规则必须是对象')
  const rule = value as Record<string, unknown>
  const fields = rule.kind === 'json_episode_list' ? ['episode', 'title', 'body']
    : rule.kind === 'html_episode_article' ? ['heading', 'body'] : []
  // 标题不在标题元素里时，模型可以另给一个标题选择器。
  const optional = rule.kind === 'html_episode_article' && rule.titleSelector !== undefined ? ['titleSelector'] : []
  if (!fields.length || Object.keys(rule).length !== fields.length + optional.length + 1 ||
    fields.some((field) => typeof rule[field] !== 'string' || !rule[field].trim() || rule[field].length > 256 ||
      (rule.kind === 'json_episode_list' && !/^[\w-]+$/.test(rule[field] as string))) ||
    optional.some((field) => typeof rule[field] !== 'string' || !(rule[field] as string).trim() || (rule[field] as string).length > 256))
    throw Error('采集规则字段无效')
  const parsed = value as EpisodeSourceRule
  return parsed.kind === 'json_episode_list'
    ? { kind: parsed.kind, episode: parsed.episode, title: parsed.title, body: parsed.body }
    : { kind: parsed.kind, heading: parsed.heading, body: parsed.body,
        ...(parsed.titleSelector ? { titleSelector: parsed.titleSelector } : {}) }
}

function episodeNumber(label: string) {
  const number = label.normalize('NFKC').match(/^(\d+)$/)?.[1]
    ?? label.normalize('NFKC').match(/(?:第\s*(\d+)\s*[話回集]|#\s*(\d+)\b|(\d+)\s*[話回集])/u)?.slice(1).find(Boolean)
  return number ? Number(number) : NaN
}

export function extractEpisodeSource(raw: string, input: unknown, target: EpisodeIdentity) {
  const rule = parseEpisodeSourceRule(input)
  const title = normalizedTitle(target.title)
  if (!title || !Number.isSafeInteger(target.number) || target.number < 1) throw Error('分集校验目标无效')
  let extracted: string
  if (rule.kind === 'json_episode_list') {
    const rows: unknown = JSON.parse(raw)
    if (!Array.isArray(rows)) throw Error('来源数据已偏离分集数组')
    const matches = rows.filter((row) => row && Object.hasOwn(row, rule.episode) &&
      episodeNumber(String(row[rule.episode])) === target.number)
    if (matches.length !== 1) throw Error('来源集号缺失或重复')
    const row = matches[0]
    if (!Object.hasOwn(row, rule.title) || typeof row[rule.title] !== 'string' || normalizedTitle(row[rule.title]) !== title)
      throw Error('来源分集标题与目录不一致')
    if (!Object.hasOwn(row, rule.body) || typeof row[rule.body] !== 'string') throw Error('来源正文已偏离采集规则')
    extracted = load(row[rule.body].replace(/<br\s*\/?>/gi, ' ')).root().text()
  } else {
    const $ = load(raw)
    // Only the validated integer can be substituted in collection-page selectors.
    const select = (selector: string) => $(selector.replaceAll('{episode}', String(target.number)))
    const headings = select(rule.heading)
    const textOf = (nodes: ReturnType<typeof select>) => nodes.toArray().map(node =>
      'tagName' in node && node.tagName === 'meta' && $(node).closest('head').length ? $(node).attr('content') ?? '' : $(node).text()).join(' ')
    const heading = textOf(headings)
    if (headings.length !== 1 || episodeNumber(heading) !== target.number)
      throw Error('来源标题未唯一命中目标分集')
    const printed = rule.titleSelector ? textOf(select(rule.titleSelector)) : heading
    if (!normalizedTitle(printed).includes(title))
      throw Error('来源标题未唯一命中目标分集')
    const article = headings.closest('article, main, body, head').get(0)
    const blocks = select(rule.body).toArray()
    const outside = blocks.filter(node => $(node).closest('article, main, body, head').get(0) !== article)
    if (outside.length) {
      // A single episode can contain editorial scene articles inside its chapters.
      // Permit explicitly scoped paragraphs, never sidebars or other episodes.
      const nestedScenes = article && $(article).is('main') && $('main').length === 1 && $('h1').length === 1 &&
        headings.is('main > header > h1') && /^main > section[.#][\w-]+ > article[.#][\w-]+ > p[.#][\w-]+$/.test(rule.body)
      if (!nestedScenes || outside.some(node => {
        if ($(node).closest('main').get(0) !== article) return true
        const headings = $(node).closest('section').find('h1,h2,h3,h4,h5,h6')
        return headings.toArray().some(h => {
          const n = episodeNumber($(h).text())
          return Number.isFinite(n) && n !== target.number
        })
      })) throw Error('正文选择器超出分集容器')
    }
    extracted = blocks.map((node) => textOf($(node))).join(' ')
  }
  const text = normalizeBody(extracted)
  if (Buffer.byteLength(text) < 60 || text.length > maximumSourceLength) throw Error('分集原文不完整或超过长度限制')
  return text
}

export function checkEpisodeSourceRule(rule: EpisodeSourceRule, samples: { raw: string; target: EpisodeIdentity }[]) {
  const numbers = new Set(samples.map(({ target }) => target.number))
  if (numbers.size < 2) throw Error('采集规则至少需要两个不同集号的样本')
  const proofs = samples.map(({ raw, target }) => {
    const text = extractEpisodeSource(raw, rule, target)
    for (const wrong of [{ ...target, number: 0x7fff_ffff }, { ...target, title: `__folio_wrong_title_${target.title}` }]) {
      let matched = false
      try { extractEpisodeSource(raw, rule, wrong); matched = true } catch { /* expected mismatch */ }
      if (matched) throw Error('采集规则匹配了错误的分集')
    }
    return { episode: target.number, pageSha256: createHash('sha256').update(raw).digest('hex'), textSha256: createHash('sha256').update(text).digest('hex') }
  })
  if (new Set(proofs.map(({ textSha256 }) => textSha256)).size !== samples.length) throw Error('不同集提取到相同正文')
  return proofs
}

export function checkIndependentSources(official: string, guide: string) {
  const compact = (text: string) => normalizedTitle(text).replace(/[\p{P}\p{S}]/gu, '')
  const o = Array.from(compact(official))
  const g = Array.from(compact(guide))
  const grams = (chars: string[]) => new Set(chars.slice(0, -5).map((_, i) => chars.slice(i, i + 6).join('')))
  const a = grams(o), b = grams(g)
  const shared = [...a].filter((part) => b.has(part)).length
  if (shared / Math.max(1, Math.min(a.size, b.size)) >= 0.8) throw Error('两份来源正文疑似转载同一通稿')
}
