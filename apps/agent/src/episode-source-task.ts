import { createHash } from 'node:crypto'
import { load } from 'cheerio'
import { checkEpisodeSourceRule, checkIndependentSources, extractEpisodeSource, parseEpisodeSourceRule } from './episode-source-rules.ts'
import { fetchSourceText } from './source-fetch.ts'

type Case = { number: number; title: string; official: string; guide: string }
type RuleProposal = { officialRule?: unknown; guideRule?: unknown }

function sourceURL(value: string) {
  const url = new URL(value)
  if (url.protocol !== 'https:' || url.username || url.password || url.port || url.hash)
    throw Error('规则样本需要无凭据的 HTTPS 来源地址')
  return url
}

function describe(raw: string) {
  if (raw.trimStart().startsWith('[')) {
    const rows: unknown[] = JSON.parse(raw)
    const fields = Object.entries((rows[0] ?? {}) as Record<string, unknown>).slice(0, 24)
    return { format: 'json', fields: fields.map(([key, value]) => [key, typeof value,
      typeof value === 'string' ? value.slice(0, 70) : typeof value === 'number' ? value : null]) }
  }
  const $ = load(raw)
  const root = $('article').first().length ? $('article').first() : $('body')
  root.find('script,style,nav,footer,iframe,svg').remove()
  const blocks = root.find('h1,h2,h3,h4,p').toArray().filter((el) => $(el).text().trim())
  return { format: 'html', blocks: blocks.slice(0, 22).map((el) => ({
    parent: { tag: el.parent?.type === 'tag' ? el.parent.name : undefined, class: $(el).parent().attr('class') },
    tag: el.tagName, class: $(el).attr('class'), id: $(el).attr('id'), text: $(el).text().trim().slice(0, 75),
  })) }
}

export async function validateEpisodeSourceProposal(
  cases: Case[],
  propose: (sample: unknown) => Promise<{ value: RuleProposal; tokens?: number }>,
  download = fetchSourceText,
) {
  if (!Array.isArray(cases) || cases.length < 2 || cases.length > 5 || cases.some(({ number, title }) =>
    !Number.isSafeInteger(number) || number < 1 || typeof title !== 'string' || !title.trim())) {
    throw Error('需提供 2–5 条已与目录核对的分集样本')
  }
  if (new Set(cases.map(({ number }) => number)).size !== cases.length) throw Error('规则样本集号重复')
  const hosts = [new URL(cases[0].official).host, new URL(cases[0].guide).host]
  if (hosts[0] === hosts[1] || cases.some((entry) =>
    sourceURL(entry.official).host !== hosts[0] || sourceURL(entry.guide).host !== hosts[1])) throw Error('样本需要同一对独立站点')
  const pages = new Map<string, Promise<string>>()
  const get = (url: string) => {
    const page = pages.get(url) ?? download(url)
    pages.set(url, page)
    return page
  }
  const samples = await Promise.all(cases.map(async (target) => ({
    target, official: await get(target.official), guide: await get(target.guide),
  })))
  const first = samples[0]
  const { value, tokens } = await propose({ target: { number: first.target.number, title: first.target.title },
    official: describe(first.official), guide: describe(first.guide) })
  const officialRule = parseEpisodeSourceRule(value?.officialRule)
  const guideRule = parseEpisodeSourceRule(value?.guideRule)
  const sourceRules = { version: 1 as const, guide: 'paired' as const, officialRule, guideRule }
  const proofs = {
    official: checkEpisodeSourceRule(officialRule, samples.map(({ official, target }) => ({ raw: official, target }))),
    guide: checkEpisodeSourceRule(guideRule, samples.map(({ guide, target }) => ({ raw: guide, target }))),
  }
  for (const sample of samples) checkIndependentSources(
    extractEpisodeSource(sample.official, officialRule, sample.target), extractEpisodeSource(sample.guide, guideRule, sample.target),
  )
  return { sourceRules, ruleSha256: createHash('sha256').update(JSON.stringify(sourceRules)).digest('hex'),
    sources: cases, proofs, tokens }
}
