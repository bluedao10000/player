import { createHash } from 'node:crypto'
import { defineTool } from '@earendil-works/pi-durable'
import { obj } from '@earendil-works/pi-schema'
import type { CheerioAPI } from 'cheerio'

export interface EpisodePlan {
  number: number
  sourceNumber?: number
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
    'official' | 'guide' | 'number' | 'sourceNumber' | 'officialTitle' | 'guideTitle'
  >,
  page: (url: string) => Promise<CheerioAPI>,
) {
  const [site, wiki] = await Promise.all([
    page(p.official),
    page(p.guide.split('#')[0]),
  ])
  const fromTbs = new URL(p.official).hostname === 'www.tbs.co.jp'
  const film = p.official === 'https://lookback-anime.com/'
  const apothecary = /^https:\/\/www\.animatetimes\.com\/news\/details\.php\?id=(1790842934|1791440404)$/.test(p.official)
  const council = p.number <= 2 && p.official === 'https://nama-anaru.com/episode/?id=' + p.number
  const sourceNumber = p.sourceNumber ?? p.number
  const japanese = fromTbs || film || apothecary || council
  const paragraphText = ($: CheerioAPI, selector: string) =>
    $(selector).toArray().map((x) => $(x).text().trim()).join(' ')
  const apTitle = site('.news-content h2').filter((_, el) => site(el).text() === p.officialTitle)
  const box = site('.storypage-wrap')
  const item = apothecary ? {
    number: Number(apTitle.text().match(/第(\d+)話/)?.[1]),
    title: apTitle.text(),
    text: apTitle.next('h3').nextUntil('h2,h3','p').toArray().map((x) => site(x).text()).join(' '),
  } : council ? {
    number: Number(site('.p-episode__head-number').text().match(/第(\d+)話/)?.[1]),
    title: site('.p-episode__head-ttl').text().replace(/\s+/g, ' ').trim(),
    text: paragraphText(site, '.p-episode__story-inner'),
  } : film ? {
    number: 1,
    title: site('title').text().trim(),
    text: site('.p-story .p-intro__content-detail p').toArray().map((x) => site(x).text()).join(' '),
  } : fromTbs ? {
    number: Number(box.find('.story-number span').text()),
    title: box.find('.story-subtitle').text().trim(),
    text: box.find('.story-text').text(),
  } : (() => {
    const entry = JSON.parse(site('#__NEXT_DATA__').text()).props.pageProps.animeData
    return {number: Number(entry.anime.story_no), title: entry.title, text: entry.anime.text}
  })()
  const row = wiki('#ep' + sourceNumber).closest('tr')
  const plot = wiki('#Plot')
  const guideTitle = film ? plot.text().trim() : council ? wiki('h1').text() : row.find('.summary').text()
  const councilParagraphs = wiki('.entry-content p.wp-block-paragraph').toArray().map((x) => wiki(x).text())
  const councilEpisodeOne = ['主人公の水之江梅は', '1本目では、梅が', '仕事の説明でも', '2本目は、朝のあいさつ運動', 'そして全校集会での生徒会の発表', 'ところが発表の最後に映像のトラブルが起き']
  const councilEpisodeTwo = ['1本目「メシアたん」は', '本題はここからでした', '2本目は、夜の校舎の見回り', '青く沈んだ夜の校舎', '3本目で、ついにこまろが登場', 'そして会長は、先生が梅を']
  const councilSections = p.number === 2 ? councilEpisodeTwo : councilEpisodeOne
  const schoolSynopsis = councilParagraphs.filter((text) => councilSections.some((start) => text.startsWith(start))).join(' ')
  const paragraphs = plot.closest('section').children('p')
  const filmText = paragraphs.length ? paragraphs : plot.parent().nextUntil('h2,.mw-heading','p')
  filmText.find('sup').remove()
  if (
    item.number !== sourceNumber ||
    item.title !== p.officialTitle ||
    !guideTitle.includes(p.guideTitle)
  )
    throw Error('分集编号或标题与配置不一致')
  return [
    [p.official, item.text, 'official', item.title],
    [
      p.guide,
      film ? filmText.toArray().map((x) => wiki(x).text()).join(' ') : council ? schoolSynopsis : row.next('tr').find('.shortSummaryText').text(),
      'guide',
      p.guideTitle,
    ],
  ].map(([url, raw, key, title]) => {
    const original = String(raw || '')
    const text = (apothecary || council ? original.normalize('NFKC') : original)
      .replace(/\s+/g, ' ')
      .trim()
    if (Buffer.byteLength(text) < 60 || text.length > 6000) throw Error('分集原文不完整')
    const sentences = Array.from(
      new Intl.Segmenter((key === 'official' && japanese) || council ? 'ja' : 'en', { granularity: 'sentence' }).segment(text),
      (x) => x.segment.trim(),
    ).filter(Boolean)
    return {
      id: key + '-' + p.number,
      episode: p.number,
      url,
      locator: title + (film ? ' · 2024 电影版' : ' · Episode ' + sourceNumber),
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
