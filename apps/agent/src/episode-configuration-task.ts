import { compileEpisode, locateEpisode, type Template } from './episode-config.ts'
import { runProductionModel } from './production-model.ts'
import { page } from './research.ts'

export async function prepareEpisodeConfiguration(dir: string, call: string, input: {
  previous: Template
  number: number
  brief: string
}) {
  const { previous, number, brief } = input
  let located: Awaited<ReturnType<typeof locateEpisode>>
  try {
    const official = new URL(previous.official)
    const guide = new URL(previous.guide)
    if (official.protocol !== 'https:' || official.username || official.password ||
        guide.origin !== 'https://en.wikipedia.org' || !guide.pathname.startsWith('/wiki/') ||
        previous.sourceRules?.guide !== 'wiki-episode' || !Number.isSafeInteger(number) || number < 1) {
      throw Error('尚无可用于自动定位的官方页与分集指南')
    }
    const [site, recap] = await Promise.all([page(official.href), page(guide.origin + guide.pathname)])
    located = await locateEpisode(previous, number, site, recap)
  } catch (err) {
    return { tokens: 0, error: err instanceof Error ? err.message : '分集来源定位失败' }
  }
  const output = await runProductionModel(dir, call, 1, 1, 'configuration', JSON.stringify({
    brief,
    title: located.source.guideTitle,
    sources: located.docs.map(d => d.sentences),
    people: previous.people,
    portraitSlugs: Object.keys(previous.portraitReferences ?? {}),
    reference: previous.art.referenceURL,
  }))
  try {
    return { tokens: output.tokens, spec: compileEpisode(previous, located, output.value), documents: located.docs }
  } catch (err) {
    return { tokens: output.tokens, error: err instanceof Error ? err.message : '分集配置未通过校验' }
  }
}
