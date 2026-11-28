// Fail the build when reader-facing copy drifts from docs/界面文案规范.md.
// Usage: node docs/copy-check.mjs. A line ending with the comment `copy-check: allow` is skipped.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')

const reader = [
  'apps/web/src/features/post',
  'apps/web/src/features/reader',
  'apps/web/src/features/worlds',
  'apps/web/src/features/seo',
  'apps/web/src/routes',
  'apps/web/src/site.ts',
  'apps/ios/Folio',
]

// [pattern, what to write instead]
const readerTerms = [
  [/收藏/, '书架（加入书架 / 移出书架 / 我的书架）'],
  [/阅读位置/, '阅读进度'],
  [/剧情试玩|私人剧情|预设分支|角色剧情|角色互动/, '剧情互动'],
  [/最近读过/, '继续阅读'],
  [/回到原文|回到故事|返回阅读/, '回到章节'],
  [/下一章/, '下一集（分集导读）或下一篇'],
  [/登场角色|在场人物|人物关系|本幕人物|· 人物/, '角色、本幕角色、角色关系'],
  [/电子邮箱/, '邮箱'],
  [/六位/, '6 位'],
  [/已完成/, '已收录第 N–M 集、本集导读'],
  [/待配置|尚未生效|尚未开放/, '隐藏尚未就绪的入口，不向读者说明开发状态'],
  [/已购买/, '已付款'],
  [/读者服务|我的读者账户/, '读者账户'],
  [/您/, '你'],
]
const logicTerms = [
  [/edition_label\??\.(endsWith|startsWith|includes)\(/, '作品卡片的 format 字段'],
  [/active=["'{][^"'}]*[\u4e00-\u9fff]/, 'Layout 的 NavKey（discover、works、cast、stories、me）'],
]

function* files(path) {
  const full = join(root, path)
  if (statSync(full, { throwIfNoEntry: false })?.isFile()) {
    yield full
    return
  }
  for (const entry of readdirSync(full, { withFileTypes: true, recursive: true })) {
    if (entry.isFile() && /\.(ts|tsx|swift)$/.test(entry.name) && !entry.name.endsWith('.gen.ts'))
      yield join(entry.parentPath, entry.name)
  }
}

const problems = []
function scan(paths, rules) {
  for (const path of paths) {
    for (const file of files(path)) {
      readFileSync(file, 'utf8').split('\n').forEach((line, index) => {
        const text = line.trim()
        if (/^(\/\/|\/\*|\*)/.test(text) || text.includes('copy-check: allow')) return
        for (const [pattern, instead] of rules) {
          const hit = text.match(pattern)
          if (hit) problems.push(`${relative(root, file)}:${index + 1}: 「${hit[0]}」→ ${instead}`)
        }
      })
    }
  }
}

scan(reader, readerTerms)
scan(['apps/web/src'], logicTerms)

if (problems.length) {
  console.error(problems.join('\n'))
  console.error(`\n${problems.length} 处文案与 docs/界面文案规范.md 不一致。`)
  process.exit(1)
}
console.log('OK: 界面文案检查通过')
