import { randomUUID } from 'node:crypto'
import { mkdir, readFile } from 'node:fs/promises'
import { openUsage, profile } from './model-api.ts'
import { runProductionModel } from './production-model.ts'
import { validateEpisodeSourceProposal } from './episode-source-task.ts'

const [casesFile, stateDir, verifiedRuleFile] = process.argv.slice(2)
if (!casesFile || !stateDir) throw Error('用法: node src/episode-source-cli.ts <episodes.json> <私有状态目录> [已验证规则.json]')
const cases = JSON.parse(await readFile(casesFile, 'utf8'))
const rule = verifiedRuleFile ? JSON.parse(await readFile(verifiedRuleFile, 'utf8')).sourceRules : undefined
if (!rule) {
  const content = profile('content')
  if (content.gateway || new URL(content.base).origin !== 'https://api.openai.com') throw Error('采集规则生成需使用 OpenAI API')
  await mkdir(stateDir, { recursive: true, mode: 0o700 })
  openUsage(stateDir)
}
// 校验失败时把原因交回模型改一版；规则仍要过同一套确定性检查。
let feedback = ''
for (let attempt = 1; ; attempt++) {
  try {
    const result = await validateEpisodeSourceProposal(cases, async (sample) => rule
      ? { value: rule }
      : runProductionModel(stateDir, randomUUID(), attempt, 1, 'source_rule',
          JSON.stringify(sample) + (feedback ? '\n上一次规则未通过校验：' + feedback : '')))
    console.log(JSON.stringify(result, null, 2))
    break
  } catch (error) {
    if (rule || attempt >= 2 || !(error instanceof Error)) throw error
    feedback = error.message
  }
}
