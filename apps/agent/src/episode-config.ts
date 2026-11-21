import type { CheerioAPI } from 'cheerio'
import {
  readEpisodeDocuments,
  episodeOutline,
  type EpisodePlan,
} from './episode-plan.ts'

type Template = EpisodePlan & {
  people: { slug: string; name: string }[]
  art: { sourceSHA: string; referenceURL: string }
}
export async function locateEpisode(
  previous: Template,
  number: number,
  official: CheerioAPI,
  guide: CheerioAPI,
) {
  const rules = previous.sourceRules
  if (rules?.guide !== 'wiki-episode') throw Error('缺少可复用的分集来源规则')
  const labels = official(rules.entry)
    .toArray()
    .map((el) => {
      const node = official(el).find(rules.heading)
      return rules.attribute ? node.attr(rules.attribute) : node.text()
    })
    .map((s) => s?.normalize('NFKC').match(/^#\s*(\d+)\s+(.+)$/))
    .filter((m) => m && Number(m[1]) === number)
  const row = guide('#ep' + number).closest('tr')
  const guideTitle = row
    .find('.summary')
    .first()
    .text()
    .match(/"([^"]+)"/)?.[1]
  if (labels.length !== 1 || !guideTitle)
    throw Error('分集编号或标题无法唯一定位')
  const source = {
    number,
    official: previous.official,
    guide: previous.guide.split('#')[0] + '#ep' + number,
    sourceRules: rules,
    officialTitle: labels[0]![2].trim(),
    guideTitle,
  }
  const docs = await readEpisodeDocuments(source, async (url) =>
    url === source.official ? official : guide,
  )
  return { source, docs }
}
export function compileEpisode(
  previous: Template,
  located: Awaited<ReturnType<typeof locateEpisode>>,
  draft: {
    slug: string
    title: string
    intro: string
    place: string
    sceneSlug: string
    sceneName: string
    sceneBeat: number
    artBrief: string
    setting: [string, number]
    people: { slug: string; beat: number }[]
    beats: EpisodePlan['beats']
  },
) {
  const {
    beats,
    slug,
    title,
    intro,
    place,
    sceneSlug,
    sceneName,
    sceneBeat,
    setting,
    artBrief,
  } = draft
  if (
    ![slug, sceneSlug].every(
      (s) => typeof s === 'string' && /^[a-z0-9]+(-[a-z0-9]+)*$/.test(s),
    ) ||
    ![title, intro, place, sceneName, artBrief].every(
      (s) => typeof s === 'string' && s.trim(),
    ) ||
    !Array.isArray(beats) ||
    beats.length < 4 ||
    beats.length > 6
  )
    throw Error('分集配置字段不完整')
  const validBeat = (n: number) =>
    Number.isInteger(n) && n > 0 && n <= beats.length
  if (
    !validBeat(sceneBeat) ||
    !Array.isArray(setting) ||
    !setting[0] ||
    !validBeat(setting[1]) ||
    !Array.isArray(draft.people) ||
    !draft.people.length ||
    !beats.some((b) => b.officialSentence) ||
    !['cause', 'turn', 'motive'].every((kind) =>
      beats.some((b) => b.kind === kind),
    )
  )
    throw Error('缺少五维验收所需定位')
  if (
    typeof setting[0] !== 'string' ||
    ['世界观', '设定', '人物', '剧情', '时间线', '图片'].includes(
      setting[0].trim(),
    )
  )
    throw Error('世界设定必须是本集具体概念')
  const people = draft.people.map(({ slug, beat }) => {
    const existing = previous.people.find((p) => p.slug === slug)
    if (!existing || !validBeat(beat)) throw Error('角色或段落未登记')
    return { slug, name: existing.name, beat }
  })
  const spec = {
    ...located.source,
    beats,
    slug,
    title,
    intro,
    place,
    sceneSlug,
    sceneName,
    sceneBeat,
    people,
    setting,
    art: {
      brief: artBrief,
      referenceURL: previous.art.referenceURL,
      sourceSHA: previous.art.sourceSHA,
    },
    publishToReader: true,
  }
  episodeOutline(spec, located.docs)
  return spec
}
