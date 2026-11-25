import legacyGuideParagraphs from './legacy-guide-paragraphs.json' with {type:'json'}
import {readConfiguredOfficial, type SourceRules} from './configured-sources.ts'
import { createHash } from 'node:crypto'
import { defineTool } from '@earendil-works/pi-durable'
import { obj } from '@earendil-works/pi-schema'
import type { CheerioAPI } from 'cheerio'
import { checkIndependentSources, extractEpisodeSource } from './episode-source-rules.ts'
import { fetchSourceText } from './source-fetch.ts'

export interface EpisodePlan {
  sourceRules?: SourceRules
  number: number
  sourceNumber?: number
  official: string
  guide: string
  officialTitle: string
  guideTitle: string
  guideParagraphs?: string[]
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
  capture?: Awaited<ReturnType<typeof readEpisodeDocuments>>,
) {
  return defineTool({
    name: 'read_episode_sources',
    description:
      '读取选题指定的官方分集与独立分集资料，校验标题和集号并保存原文句子。',
    parameters: obj({}),
    async execute(_, api) {
      const docs = capture ?? await readEpisodeDocuments(p, page)
      api.output(JSON.stringify(docs))
      return {}
    },
  })
}
export async function readEpisodeDocuments(
  p: Pick<
    EpisodePlan,
    'official' | 'guide' | 'number' | 'sourceNumber' | 'officialTitle' | 'guideTitle' | 'guideParagraphs' | 'sourceRules'
  >,
  page: (url: string) => Promise<CheerioAPI>,
  rawPage = fetchSourceText,
) {
  if (p.sourceRules?.guide === 'paired') {
    const rules = p.sourceRules
    const number = p.sourceNumber ?? p.number
    const [official, guide] = await Promise.all([
      rawPage(p.official).then((raw) => extractEpisodeSource(raw, rules.officialRule, { number, title: p.officialTitle })),
      rawPage(p.guide).then((raw) => extractEpisodeSource(raw, rules.guideRule, { number, title: p.guideTitle })),
    ])
    checkIndependentSources(official, guide)
    return [sourceDocument(p.official, official, 'official', p.officialTitle, p.number, number, 'ja'),
      sourceDocument(p.guide, guide, 'guide', p.guideTitle, p.number, number, 'ja')]
  }
  const [site, wiki] = await Promise.all([
    page(p.official),
    page(p.guide.split('#')[0]),
  ])
  const bebop = !p.sourceRules && p.official === "https://www.cowboy-bebop.net/story/"
  const fromTbs = new URL(p.official).hostname === 'www.tbs.co.jp'
  const film = !p.sourceRules && p.official === 'https://lookback-anime.com/'
  const apothecary = !p.sourceRules && new URL(p.guide).hostname === 'en.wikipedia.org' && /^https:\/\/www\.animatetimes\.com\/news\/details\.php\?id=\d{10}$/.test(p.official)
  const council = !p.sourceRules && p.official === 'https://nama-anaru.com/episode/?id=' + p.number
  const sourceNumber = p.sourceNumber ?? p.number
  const press = !p.sourceRules && !council && Boolean(p.guideParagraphs?.length)
  const japanese = !!p.sourceRules || bebop || fromTbs || film || apothecary || council || press
  const paragraphText = ($: CheerioAPI, selector: string) =>
    $(selector).toArray().map((x) => $(x).text().trim()).join(' ')
  const apTitle = site('.news-content h2').filter((_, el) => site(el).text() === p.officialTitle)
  const box = site('.storypage-wrap')
  const item = p.sourceRules ? readConfiguredOfficial(site,p.sourceRules,sourceNumber,p.officialTitle) : bebop ? (() => {
    const entry = site('.storybox').filter((_, el) => site(el).find('img').attr('alt')?.split(/\s+/)[0] === '#' + p.number)
    if (entry.length !== 1) throw Error('星际牛仔官方分集编号不唯一')
    const [label, ...title] = entry.find('img').attr('alt')!.split(/\s+/)
    return {number: Number(label.slice(1)), title: title.join(' '), text: entry.find('.storytxt').text()}
  })() : press ? readPressSynopsis(site, p) : apothecary ? {
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
  const guideTitle = p.sourceRules ? (p.sourceRules.guide === 'wiki-episode' ? row.find('.summary').text() : wiki('h1').first().text()) : film ? plot.text().trim() : (council || press) ? wiki('h1').text() : row.find('.summary').text()
  const councilParagraphs = wiki('.entry-content p.wp-block-paragraph').toArray().map((x) => wiki(x).text())
  const councilSections = legacyGuideParagraphs[String(p.number) as keyof typeof legacyGuideParagraphs] ?? []
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
  const guideBlocks = p.guideParagraphs?.length ? readGuideParagraphs(wiki, p.guideParagraphs!) : undefined
  return [
    [p.official, item.text, 'official', item.title],
    [
      p.guide,
      film ? filmText.toArray().map((x) => wiki(x).text()).join(' ') : guideBlocks ? guideBlocks.join(' ') : council ? schoolSynopsis : row.next('tr').find('.shortSummaryText').text(),
      'guide',
      p.guideTitle,
    ],
  ].map(([url, raw, key, title]) => {
    const original = String(raw || '')
    const text = (apothecary || council || press ? original.normalize('NFKC') : original)
      .replace(/\s+/g, ' ')
      .trim()
    if (Buffer.byteLength(text) < 60 || text.length > 6000) throw Error('分集原文不完整')
    const segmenter = new Intl.Segmenter((key === 'official' && japanese) || council || press ? 'ja' : 'en', { granularity: 'sentence' })
    // This recap omits sentence punctuation between its short paragraph notes.
    const blocks = key === 'guide' && guideBlocks && new URL(p.guide).hostname === 'sayo.blog'
      ? guideBlocks.map((block) => block.normalize('NFKC').replace(/\s+/g, ' ').trim())
      : [text]
    const sentences = blocks.flatMap((block) =>
      Array.from(segmenter.segment(block), (x) => x.segment.trim()),
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

function sourceDocument(url: string, text: string, key: string, title: string, episode: number, sourceNumber: number, language: string) {
  const segmenter = new Intl.Segmenter(language, { granularity: 'sentence' })
  return {
    id: `${key}-${episode}`, episode, url, locator: `${title} · Episode ${sourceNumber}`, text,
    sentences: Array.from(segmenter.segment(text), ({ segment }) => segment.trim()).filter(Boolean),
    fetched_at: new Date().toISOString(), sha256: createHash('sha256').update(text).digest('hex'),
  }
}
function readPressSynopsis(site: CheerioAPI, plan: Pick<EpisodePlan, 'official' | 'officialTitle' | 'number'>) {
  const url = new URL(plan.official)
  if (url.hostname === 'www.aniplex.co.jp') {
    const number = (text: string) => Number(text.trim().match(/^(?:第)?(\d+)(?:話)?$/)?.[1])
    const episodes = site('[class="p-work:story-episode__container"]')
      .filter((_, el) => number(site(el).find('[class="p-work:story-episode__no"]').text()) === plan.number)
    return {
      number: episodes.length === 1 ? plan.number : NaN,
      title: episodes.find('[class="p-work:story-outline__title"]').text().trim(),
      text: episodes.find('[class="p-work:story-outline__introduction"]').text().trim(),
    }
  }
  if (plan.official === 'https://anime-yukionna.af-original.com/story/') {
    const first = site('#story'+plan.number)
    const number = Number(first.find('.story-title .title-num').text().trim().match(/^#(\d+)$/)?.[1])
    first.find('.story-title .title-num').remove()
    return { number, title: first.find('.story-title').text().trim(), text: first.find('.story-txt').text().trim() }
  }
  if (plan.official === 'https://darksummoner.deregula.com/story/') {
    const first = site('#js_story .story').filter((_, node) => site(node).find('.story_num span').text().trim() === String(plan.number))
    const main = first.find('.story_foot').first()
    return {
      number: first.length === 1 ? Number(first.find('.story_num span').text()) : NaN,
      title: main.find('.story_foot_title').text().trim(),
      text: main.find('.story_foot_txt').text().trim(),
    }
  }
  const onAirTitle = new Map([
    ['29245002', 'ダークサモナーとデキている'],
    ['29247002', '人付き合いが苦手な未亡人の雪女さんと呪いの指輪'],
  ]).get(url.searchParams.get('partId') ?? '')
  if (url.hostname === 'animestore.docomo.ne.jp' && onAirTitle) {
    const heading = site('meta[property="og:title"]').attr('content') ?? ''
    const prefix = onAirTitle + '【オンエア版】 '
    const match = heading.startsWith(prefix) ? heading.slice(prefix.length).match(/^第(\d+)話 (.+)$/) : null
    return {
      number: Number(match?.[1]),
      title: match?.[2] ?? '',
      text: site('meta[property="og:description"]').attr('content') ?? '',
    }
  }
  if (plan.official === 'https://www.b-ch.com/titles/9932/002') {
    return {
      number: Number(site('#bch-story-title').text().trim().match(/^第(\d+)話/)?.[1]),
      title: site('.bch-p-heading-mov__summary').text().trim(),
      text: site('.bch-p-heading-mov__detail-txt').text().trim(),
    }
  }
  if (plan.official === 'https://fxkurumi-info.com/story/episodes.json') {
    const records: { epID?: string; title?: string; text?: string }[] = JSON.parse(site.root().text())
    const second = records.find((entry) => Number(entry.epID) === plan.number)
    if (typeof second?.title !== 'string' || typeof second.text !== 'string')
      throw Error('官方分集数据缺少本集')
    return { number: Number(second.epID), title: second.title, text: second.text }
  }
  if (plan.official === 'https://romelia-senki.com/news/episode_data') {
    const records: { acf?: { season_id: string; index: string; title: string; story: string } }[] =
      JSON.parse(site.root().text())
    const index = String(plan.number).padStart(2, '0')
    const episode = records.find((record) => record.acf?.season_id === 'season1' && record.acf.index === index)?.acf
    if (typeof episode?.title !== 'string' || typeof episode.story !== 'string')
      throw Error('官方剧集数据缺少第一季第 ' + plan.number + ' 集')
    return { number: Number(episode.index), title: episode.title, text: episode.story }
  }
  const item = site('#cms_block .line_00 .epHead b, #cms_block .line_01 .storyHead__ep')
  if (item.length === 1) return {
    number: Number(item.text().normalize('NFKC')),
    title: site('#cms_block .line_00 .articleHead, #cms_block .line_01 .storyHead__title').text().trim(),
    text: site('#cms_block .line_02 .read').text().trim(),
  }
  const heading = site('.news-content h2, .news-content h3')
    .filter((_, node) => site(node).text().trim() === plan.officialTitle)
    .first()
  const title = heading.text().trim()
  const label = title.match(/第([0-9０-９一二三]+)[話回]/)?.[1]
    ?? site('.news-content').prevAll().find('h1').first().text().match(/第([0-9０-９一二三]+)[話回]/)?.[1]
    ?? site('h1').first().text().match(/第([0-9０-９一二三]+)[話回]/)?.[1]
  const japaneseNumbers: Record<string, number> = { 一: 1, 二: 2, 三: 3 }
  const number = label ? japaneseNumbers[label] ?? Number(label.normalize('NFKC')) : NaN
  const synopsis = heading.next('h3').text().trim() === 'あらすじ' ? heading.next('h3') : heading
  const text = synopsis.nextUntil('h2,h3','p').toArray().map((x) => site(x).text().trim()).join(' ')
  return { number, title, text }
}

function readGuideParagraphs(page: CheerioAPI, starts: string[]) {
  const paragraphs = page('.entry-content p, article p, main p').toArray()
    .map((x) => page(x).text().trim().replace(/\s+/g, ' '))
  const matches = starts.map((start) => paragraphs.find((text) => text.startsWith(start)))
  if (matches.some((text) => !text)) throw Error('分集补充资料缺少指定原文段落')
  return matches as string[]
}

export function episodeOutline(p: EpisodePlan & { people?: { beat: number; name?: string; evidence?: string }[]; setting?: [string, number] }, docs: Doc[]) {
  const guide = docs.find((d) => d.id === 'guide-' + p.number)!
  const official = docs.find((d) => d.id === 'official-' + p.number)!
  const starts: number[] = []
  for (const beat of p.beats) {
    const previous = starts.at(-1) ?? -1
    const found = guide.sentences.findIndex((sentence, i) => i > previous && sentence.includes(beat.needle))
    if (found < 0) throw Error('剧情出处需更新：' + beat.needle)
    starts.push(found)
  }
  return p.beats.map((b, i) => {
    const start = starts[i],
      end = starts[i + 1] ?? guide.sentences.length
    if (start < 0 || end <= start) throw Error('剧情出处需更新：' + b.needle)
    const source = guide.sentences
      .slice(start, end)
      .map((text, k) => ({ id: guide.id, sentence: start + k + 1, text }))
    const maxGuide = b.officialSentence ? 7 : 8
    const required = new Set([0])
    const people = (p.people ?? []).filter((person) => person.beat === i + 1)
    for (const person of people) {
      if (!person.evidence) continue
      const quote = person.evidence
      const position = source.findIndex(({ text }) => text.includes(quote))
      if (position >= 0) required.add(position)
    }
    if (required.size > maxGuide) throw Error('该段角色证据超出引用上限')
    for (let k = 0; k < source.length && required.size < maxGuide; k++) required.add(k)
    const evidence = source.filter((_, k) => required.has(k))
    if (b.officialSentence) {
      const text = official.sentences[b.officialSentence - 1]
      if (!text) throw Error('缺少官方定位')
      evidence.push({ id: official.id, sentence: b.officialSentence, text })
    }
    const names = people.flatMap(({ name }) => name ? [name] : [])
    const setting = p.setting?.[1] === i + 1 ? p.setting[0] : ''
    const question = b.question + (names.length ? ` 使用登记姓名「${names.join('、')}」。` : '') +
      (setting ? ` 根据本段证据解释「${setting}」，正文保留这个术语。` : '')
    return { episode: p.number, kind: b.kind, question, evidence }
  })
}
