import { normalizedTitle as label } from './episode-source-rules.ts'
import { createHash } from 'node:crypto'
import { load } from 'cheerio'
import { checkEpisodeSourceRule, checkIndependentSources, extractEpisodeSource, parseEpisodeSourceRule } from './episode-source-rules.ts'
import { fetchSourceText } from './source-fetch.ts'

type Case = { number: number; sourceNumber?: number; title: string; official: string; guide?: string; guideIndex?: string; guideTitle?: string }
type RuleProposal = { officialRule?: unknown; guideRule?: unknown }

function sourceURL(value: string) {
  const url = new URL(value)
  if (url.protocol !== 'https:' || url.username || url.password || url.port || url.hash)
    throw Error('规则样本需要无凭据的 HTTPS 来源地址')
  return url
}

function describe(raw: string, title: string) {
  if (raw.trimStart().startsWith('[')) {
    const rows: unknown[] = JSON.parse(raw)
    const fields = Object.entries((rows[0] ?? {}) as Record<string, unknown>).slice(0, 24)
    return { format: 'json', fields: fields.map(([key, value]) => [key, typeof value,
      typeof value === 'string' ? value.slice(0, 70) : typeof value === 'number' ? value : null]) }
  }
  const $ = load(raw)
  const root = $('article').first().length ? $('article').first() : $('body')
  root.find('script,style,nav,footer,iframe,svg').remove()
  const match = root.find('h1,h2,h3,h4').toArray().find(el => $(el).text().trim() === title)
  const section = match && $(match).closest('section')
  const scope = section?.length ? section : root
  const blocks = scope.find('h1,h2,h3,h4,p,b,strong').toArray().filter((el) => $(el).text().trim())
  blocks.sort((a,b) => Number($(b).text().includes(title)) - Number($(a).text().includes(title)))
  const metadata = $('head meta[property="og:title"],head meta[property="og:description"]').slice(0, 4)
    .toArray().map(el => ({ property: $(el).attr('property'), content: $(el).attr('content')?.slice(0, 180) }))
  return { format: 'html', metadata, blocks: blocks.slice(0, 22).map((el) => ({
    ancestors: $(el).parents().slice(0, 5).toArray().map(parent => [parent.name, $(parent).attr('class')]),
    parent: { tag: el.parent?.type === 'tag' ? el.parent.name : undefined, class: $(el).parent().attr('class') },
    tag: el.tagName, class: $(el).attr('class'), id: $(el).attr('id'), text: $(el).text().trim().slice(0, 75),
  })) }
}

export async function validateEpisodeSourceProposal(
  cases: Case[],
  propose: (sample: unknown) => Promise<{ value: RuleProposal; tokens?: number }>,
  download = fetchSourceText,
  minimumNarrativeSentences = 0,
) {
  if (!Array.isArray(cases) || cases.length < 2 || cases.length > 5 || cases.some(({ number, sourceNumber, title }) =>
    !Number.isSafeInteger(number) || number < 1 || typeof title !== 'string' || !title.trim() ||
    (sourceNumber !== undefined && (!Number.isSafeInteger(sourceNumber) || sourceNumber < 1)))) {
    throw Error('需提供 2–5 条已与目录核对的分集样本')
  }
  if (new Set(cases.map(({ number }) => number)).size !== cases.length) throw Error('规则样本集号重复')
  const officialHost = new URL(cases[0].official).host
  const guideHost = cases[0].guide === undefined ? '' : new URL(cases[0].guide).host
  if (officialHost === guideHost || cases.some((entry) =>
    (entry.guide === undefined) !== (guideHost === '') ||
    sourceURL(entry.official).host !== officialHost ||
    (entry.guide !== undefined && sourceURL(entry.guide).host !== guideHost))) throw Error('样本需要同一对独立站点')
  const pages = new Map<string, Promise<string>>()
  const get = (url: string) => {
    const page = pages.get(url) ?? download(url)
    pages.set(url, page)
    return page
  }
  // 续作的来源集号可能与本站集号不同（季偏移），按来源集号提取与留证。
  // 只有官方来源时，独立分集来源整组省略。
  const samples = await Promise.all(cases.map(async entry => {
    const number = entry.sourceNumber ?? entry.number
    const target = { number, title: entry.title }
    let indexProof
    if (entry.guideIndex || entry.guideTitle) {
      if (!entry.guide || !entry.guideIndex || !entry.guideTitle ||
          sourceURL(entry.guideIndex).origin !== sourceURL(entry.guide).origin) throw Error('缺少同站目录与文章标题核对')
      const index = await get(entry.guideIndex)
      const $ = load(index)
      const matches = $('tr').toArray().filter(row => {
        const cells = $(row).find('td').toArray().map(td => label($(td).text()))
        return cells.includes('第' + number + '話') && cells.includes(label(entry.title)) &&
          $(row).find('a[href]').toArray().some(a => new URL($(a).attr('href')!, entry.guideIndex).href === entry.guide)
      })
      if (matches.length !== 1) throw Error('同站分集目录未唯一指向指南文章')
      indexProof = { episode: number, pageSha256: createHash('sha256').update(index).digest('hex') }
    }
    return { target, guideTarget: {number, title: entry.guideTitle ?? entry.title},
      official: await get(entry.official), guide: entry.guide === undefined ? undefined : await get(entry.guide), indexProof }
  }))
  const first = samples[0]
  const { value, tokens } = await propose({ target: { number: first.target.number, title: first.target.title },
    official: describe(first.official, first.target.title),
    ...(first.guide === undefined ? {} : { guide: describe(first.guide, first.guideTarget.title), guideTarget: first.guideTarget }) })
  const officialRule = parseEpisodeSourceRule(value?.officialRule)
  const officialProofs = checkEpisodeSourceRule(officialRule,
    samples.map(({ official, target }) => ({ raw: official, target })))
  if (first.guide === undefined) {
    const sourceRules = { version: 1 as const, guide: 'official_only' as const, officialRule }
    return { sourceRules, ruleSha256: createHash('sha256').update(JSON.stringify(sourceRules)).digest('hex'),
      sources: cases, proofs: { official: officialProofs }, tokens }
  }
  const guideRule = parseEpisodeSourceRule(value?.guideRule)
  const sourceRules = { version: 1 as const, guide: 'paired' as const, officialRule, guideRule }
  const guideProofs = checkEpisodeSourceRule(guideRule,
    samples.map(({ guide, guideTarget }) => ({ raw: guide!, target: guideTarget })))
  for (const sample of samples) {
    const narrative = extractEpisodeSource(sample.guide!, guideRule, sample.guideTarget)
    checkIndependentSources(extractEpisodeSource(sample.official, officialRule, sample.target), narrative)
    if (minimumNarrativeSentences && Array.from(new Intl.Segmenter('ja', {granularity: 'sentence'}).segment(narrative)).length < minimumNarrativeSentences)
      throw Error('指南正文不足以定位四个独立剧情节点，需要完整分集文章')
  }
  return { sourceRules, ruleSha256: createHash('sha256').update(JSON.stringify(sourceRules)).digest('hex'),
    sources: cases, proofs: { official: officialProofs, guide: guideProofs, ...(samples.some(s => s.indexProof) ? { guideIndex: samples.map(s => s.indexProof) } : {}) }, tokens }
}
