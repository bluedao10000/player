import { compileEpisode, locateEpisode, type Template } from './episode-config.ts'
import { runProductionModel } from './production-model.ts'
import { page } from './research.ts'
import { createHash } from 'node:crypto'
import { episodeOutline, readEpisodeDocuments } from './episode-plan.ts'

export function configurationSentence(doc: { id: string, sentences: { number: number, text: string }[] } | undefined, pointer: unknown) {
  if (!doc) return
  let number = pointer
  if (typeof pointer === 'string' && pointer.startsWith(doc.id + '#')) {
    const suffix = pointer.slice(doc.id.length + 1)
    if (!/^[1-9]\d*$/.test(suffix)) return
    number = Number(suffix)
  } else if (pointer && typeof pointer === 'object' && 'doc' in pointer && 'number' in pointer && pointer.doc === doc.id) {
    number = pointer.number
  }
  return doc.sentences.find(sentence => sentence.number === number)
}

type Located = Parameters<typeof compileEpisode>[1]
type Draft = Parameters<typeof compileEpisode>[2]
type Outline = Pick<Draft, 'slug' | 'title' | 'intro' | 'place' | 'sceneSlug' | 'setting' | 'beats'>

export function outlineFingerprint(located: Located) {
  const snapshot = located.docs.map(({id, sentences}) => [id, sentences])
  return createHash('sha256').update(JSON.stringify([located.source, snapshot])).digest('hex')
}

export function castEvidenceMenu(located: Located, draft: Outline) {
  if (![draft.slug, draft.sceneSlug].every(slug => typeof slug === 'string' && /^[a-z0-9]+(-[a-z0-9]+)*$/.test(slug)) ||
      ![draft.title, draft.intro, draft.place].every(text => typeof text === 'string' && text.trim()) ||
      !Array.isArray(draft.beats) || draft.beats.length < 4 || draft.beats.length > 6 ||
      !['cause','turn','motive'].every(kind => draft.beats.some(b => b.kind === kind)) ||
      new Set(draft.beats.map(b => b.needle)).size !== draft.beats.length ||
      !draft.beats.every(b => ['cause','turn','motive'].includes(b.kind) && typeof b.question === 'string' && b.question.trim() && typeof b.needle === 'string' && b.needle.trim()) ||
      !draft.beats.some(b => b.officialSentence) || !Array.isArray(draft.setting) ||
      typeof draft.setting[0] !== 'string' || !draft.setting[0].trim() || Array.from(draft.setting[0]).length > 24 ||
      /[、,，;；]/.test(draft.setting[0]) || ['世界观','设定','人物','剧情','时间线','图片'].includes(draft.setting[0].trim()) ||
      !Number.isInteger(draft.setting[1]) || draft.setting[1] < 1 || draft.setting[1] > draft.beats.length)
    throw Error('分集提纲字段或段落定位无效')
  return episodeOutline({...located.source, beats:draft.beats}, located.docs).map((segment, i) => ({
    beat:i+1, question:draft.beats[i].question,
    choices:segment.evidence.filter(item => item.text.trim().length >= 3).map(item => ({
      key:`b${i+1}/${item.id}#${item.sentence}`, text:Array.from(item.text).slice(0,120).join('')
    })),
  }))
}

export function bindCastEvidence(previous: Template, menu: ReturnType<typeof castEvidenceMenu>,
  selection: {people:{slug:string; key:string}[]; artCharacter:string; artBrief:string; sceneName:string}) {
  if (!Array.isArray(selection.people) || !selection.people.length) throw Error('人物引用缺少角色和证据编号')
  const options = new Map(menu.flatMap(section => section.choices.map(choice => [choice.key, {beat:section.beat, evidence:choice.text}] as const)))
  const seen = new Set<string>()
  const people = selection.people.map(({slug,key}) => {
    if (seen.has(slug) || !previous.people.some(p => p.slug === slug)) throw Error('人物未登记或重复：'+slug)
    seen.add(slug)
    const match = options.get(key)
    if (!match) throw Error('人物 '+slug+' 所选引用 '+String(key).slice(0,100)+' 不在本集清单。允许引用：'+[...options.keys()].join('、'))
    return {slug,...match}
  })
  const primary = people.find(p => p.slug === selection.artCharacter)
  if (!primary) throw Error('场景人物缺少已选原句：'+selection.artCharacter)
  const name = previous.people.find(person => person.slug === primary.slug)!.name
  if (typeof selection.artBrief !== 'string' || !selection.artBrief.trim()) throw Error('分集缺少场景画面说明')
  const artBrief = selection.artBrief.includes(name) ? selection.artBrief : name + '。' + selection.artBrief
  return {people, sceneBeat:primary.beat, sceneName:selection.sceneName, artBrief,
    ...(Object.keys(previous.portraitReferences ?? {}).length && {artCharacter:primary.slug})}
}

// A model can return the right source pointers in narrative order instead of
// page order. Keep each question and the setting attached to its source beat.
export function orderOutline<T extends {guideSentence?:unknown}>(beats:T[], setting:[string,number]) {
  if (!Array.isArray(beats) || !Array.isArray(setting) || !beats[setting[1]-1] ||
      !beats.every(beat => Number.isSafeInteger(beat.guideSentence))) return {beats, setting}
  const ordered = [...beats].sort((a,b) => Number(a.guideSentence)-Number(b.guideSentence))
  return {beats:ordered, setting:[setting[0], ordered.indexOf(beats[setting[1]-1])+1] as [string,number]}
}

export async function prepareEpisodeConfiguration(dir: string, call: string, input: {
  previous: Template & {configurationOutline?: {fingerprint:string; draft:Outline; refreshed?:boolean}}
  number: number
  brief: string
}) {
  const { previous, number, brief } = input
  let located: Parameters<typeof compileEpisode>[1]
  try {
    if (previous.sourceBundle) {
      const source = previous.sourceBundle
      const guide = source.sourceRules?.guide
      if (source.number !== number || (guide !== 'paired' && guide !== 'official_only')) throw Error('分集与已核对的来源不一致')
      try {
        located = { source, docs: await readEpisodeDocuments(source, page) }
      } catch (error) {
        if (!(error instanceof Error) || error.message !== '来源标题未唯一命中目标分集') throw error
        await new Promise(resolve => setTimeout(resolve, 1000))
        located = { source, docs: await readEpisodeDocuments(source, page) }
      }
    } else {
      const official = new URL(previous.official)
      const guide = new URL(previous.guide)
      if (official.protocol !== 'https:' || official.username || official.password ||
          guide.origin !== 'https://en.wikipedia.org' || !guide.pathname.startsWith('/wiki/') ||
          previous.sourceRules?.guide !== 'wiki-episode' || !Number.isSafeInteger(number) || number < 1) {
        throw Error('尚无可用于自动定位的官方页与分集指南')
      }
      const [site, recap] = await Promise.all([page(official.href), page(guide.origin + guide.pathname)])
      located = await locateEpisode(previous, number, site, recap)
    }
  } catch (err) {
    return { tokens: 0, error: err instanceof Error ? err.message : '分集来源定位失败' }
  }
  const narrative = located.docs.find(doc => doc.id.startsWith('guide-')) ?? located.docs[0]
  if (narrative.sentences.length < 4) {
    return { tokens: 0, error: '指南正文不足以定位四个独立剧情节点，需要完整分集文章' }
  }
  // Keep original sentence numbers even when a long recap requires sampling.
  const sources = located.docs.map(doc => {
    const size = Math.min(doc.sentences.length, 32)
    const sentences = Array.from({ length: size }, (_, i) => {
      const index = Math.round(i * (doc.sentences.length - 1) / Math.max(size - 1, 1))
      return { number: index + 1, text: Array.from(doc.sentences[index]).slice(0, 100).join('') }
    })
    return { id: doc.id, sentences }
  })
  const fingerprint = outlineFingerprint(located)
  const saved = previous.configurationOutline
  if (saved?.fingerprint === fingerprint) {
    let menu: ReturnType<typeof castEvidenceMenu>
    try { menu = castEvidenceMenu(located, saved.draft) }
    catch (err) { return {tokens:0,error:err instanceof Error ? err.message : '已存提纲无效'} }
    const output = await runProductionModel(dir, call, 1, 1, 'configuration', JSON.stringify({
      phase:'cast', title:saved.draft.title, place:saved.draft.place, sceneSlug:saved.draft.sceneSlug, menu, people:previous.people.map(({slug,name})=>({slug,name})),
      portraitSlugs:Object.keys(previous.portraitReferences ?? {}), brief,
    }))
    try {
      const cast = bindCastEvidence(previous, menu, output.value)
      return {tokens:output.tokens, spec:compileEpisode(previous, located, {...saved.draft,...cast}), documents:located.docs}
    } catch (error) {
      return {tokens:output.tokens,error:error instanceof Error ? error.message : '人物引用未通过校验'}
    }
  }
  if (saved?.refreshed) return {tokens:0,error:'来源快照连续变化，需重新核对分集来源'}
  const output = await runProductionModel(dir, call, 1, 1, 'configuration', JSON.stringify({
    phase:'outline', brief, title:located.source.guideTitle, sources,
  }))
  try {
    const guide = sources.find(doc => doc.id.startsWith('guide-')) ?? sources[0]
    const official = sources.find(doc => doc.id.startsWith('official-'))
    for (const beat of output.value.beats ?? []) {
      if (beat.officialSentence != null) {
        const sentence = configurationSentence(official, beat.officialSentence)
        if (!sentence) throw Error('官方定位不在本集提供的来源句中')
        beat.officialSentence = sentence.number
      }
      if (beat.guideSentence === undefined) continue
      const sentence = configurationSentence(guide, beat.guideSentence)
      if (!sentence) throw Error('剧情定位不在本集提供的来源句中')
      beat.guideSentence = sentence.number
      beat.needle = sentence.text
    }
    const {slug,title,intro,place,sceneSlug,setting,beats} = output.value
    const draft: Outline = {slug,title,intro,place,sceneSlug,...orderOutline<Outline['beats'][number] & {guideSentence?:number}>(beats,setting)}
    castEvidenceMenu(located, draft)
    return {tokens:output.tokens, outline:{fingerprint,draft,refreshed:Boolean(saved)}}
  } catch (err) {
    return { tokens: output.tokens, error: err instanceof Error ? err.message : '分集配置未通过校验' }
  }
}
