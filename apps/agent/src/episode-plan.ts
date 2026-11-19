import { createHash } from 'node:crypto'
import { defineTool } from '@earendil-works/pi-durable'
import { obj } from '@earendil-works/pi-schema'
import type { CheerioAPI } from 'cheerio'

export interface EpisodePlan {
  number: number
  official: string
  guide: string
  officialTitle: string
  guideTitle: string
  beats: {
    kind: string
    needle: string
    question: string
    officialSentence?: number
  }[]
}
type Doc = { id: string; sentences: string[] }
export function episodeSources(
  p: EpisodePlan,
  page: (url: string) => Promise<CheerioAPI>,
) {
  return defineTool({
    name: 'read_episode_sources',
    description:
      '读取选题指定的官方分集与独立分集资料，校验标题和集号并保存原文句子。',
    parameters: obj({}),
    async execute(_, api) {
      const docs = await readEpisodeDocuments(p, page)
      api.output(JSON.stringify(docs))
      return {}
    },
  })
}
export async function readEpisodeDocuments(
  p: Pick<
    EpisodePlan,
    'official' | 'guide' | 'number' | 'officialTitle' | 'guideTitle'
  >,
  page: (url: string) => Promise<CheerioAPI>,
) {
  const [site, wiki] = await Promise.all([
    page(p.official),
    page(p.guide.split('#')[0]),
  ])
  const item = JSON.parse(site('#__NEXT_DATA__').text()).props.pageProps
    .animeData
  const row = wiki('#ep' + p.number).closest('tr')
  if (
    Number(item.anime.story_no) !== p.number ||
    item.title !== p.officialTitle ||
    !row.find('.summary').text().includes(p.guideTitle)
  )
    throw Error('分集编号或标题与配置不一致')
  return [
    [p.official, item.anime.text, 'official', item.title],
    [
      p.guide,
      row.next('tr').find('.shortSummaryText').text(),
      'guide',
      p.guideTitle,
    ],
  ].map(([url, raw, key, title]) => {
    const text = String(raw || '')
      .replace(/\s+/g, ' ')
      .trim()
    if (text.length < 60 || text.length > 6000) throw Error('分集原文不完整')
    const sentences = Array.from(
      new Intl.Segmenter('en', { granularity: 'sentence' }).segment(text),
      (x) => x.segment.trim(),
    ).filter(Boolean)
    return {
      id: key + '-' + p.number,
      episode: p.number,
      url,
      locator: title + ' · Episode ' + p.number,
      text,
      sentences,
      fetched_at: new Date().toISOString(),
      sha256: createHash('sha256').update(text).digest('hex'),
    }
  })
}
export function episodeOutline(p: EpisodePlan, docs: Doc[]) {
  const guide = docs.find((d) => d.id === 'guide-' + p.number)!
  const official = docs.find((d) => d.id === 'official-' + p.number)!
  const starts = p.beats.map((b) =>
    guide.sentences.findIndex((s) => s.includes(b.needle)),
  )
  return p.beats.map((b, i) => {
    const start = starts[i],
      end = starts[i + 1] ?? guide.sentences.length
    if (start < 0 || end <= start) throw Error('剧情出处需更新：' + b.needle)
    const evidence = guide.sentences
      .slice(start, end)
      .map((text, k) => ({ id: guide.id, sentence: start + k + 1, text }))
    if (b.officialSentence) {
      const text = official.sentences[b.officialSentence - 1]
      if (!text) throw Error('缺少官方定位')
      evidence.push({ id: official.id, sentence: b.officialSentence, text })
    }
    return { episode: p.number, kind: b.kind, question: b.question, evidence }
  })
}
