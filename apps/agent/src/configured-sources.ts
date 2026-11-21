import type { CheerioAPI } from 'cheerio'

export type SourceRules = {
  entry: string
  heading: string
  attribute?: string
  body: string
  guide: 'wiki-episode' | 'paragraphs'
}

// Select one episode by its printed number AND its registered title. CSS comes
// from the operator's locked spec and only queries an already downloaded DOM.
export function readConfiguredOfficial(site: CheerioAPI, rules: SourceRules, number: number, title: string) {
  const entries = site(rules.entry).toArray().filter(el => {
    const node = site(el).find(rules.heading)
    if (node.length !== 1) return false
    const label = (rules.attribute ? node.attr(rules.attribute) : node.text())?.normalize('NFKC') ?? ''
    const ep = label.match(/(?:#\s*|第\s*)(\d+)(?:\s*[話回]|\b)/u)
    return Number(ep?.[1]) === number && label.includes(title.normalize('NFKC'))
  })
  if (entries.length !== 1) throw Error('官方分集编号或标题不唯一')
  const text = site(entries[0]).find(rules.body).toArray().map(el => site(el).text().trim()).join(' ')
  return {number, title, text}
}
