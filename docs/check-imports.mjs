// Static runtime imports only: lazy imports and erased type contracts do not
// execute a module during initialization. Reuse the compiler's workspace resolution.
import { readFileSync } from 'node:fs'
import { relative, resolve, sep } from 'node:path'
import ts from 'typescript'

const pathOf = file => relative(process.cwd(), file).split(sep).join('/')
const owner = file => pathOf(file).split('/').slice(0, 2).join('/')
const roots = ['apps/web/src', 'apps/agent/src', 'packages/shared']
const graph = new Map(roots.flatMap(root =>
  ts.sys.readDirectory(resolve(root), ['.ts', '.tsx'], ['**/node_modules/**'])
    .filter(file => !file.endsWith('.d.ts')).map(file => [file, new Set()]),
))
const options = { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ESNext,
  jsx: ts.JsxEmit.Preserve, verbatimModuleSyntax: true, moduleResolution: ts.ModuleResolutionKind.Bundler }
for (const [file, deps] of graph) {
  const js = ts.transpileModule(readFileSync(file, 'utf8'), { fileName: file, compilerOptions: options }).outputText
  const emitted = ts.createSourceFile(file + '.jsx', js, ts.ScriptTarget.ESNext, false, ts.ScriptKind.JSX)
  for (const stmt of emitted.statements) {
    if (!(ts.isImportDeclaration(stmt) || ts.isExportDeclaration(stmt)) || !stmt.moduleSpecifier) continue
    const target = ts.resolveModuleName(stmt.moduleSpecifier.text, file, options, ts.sys).resolvedModule?.resolvedFileName
    if (target && owner(target).startsWith('apps/') && owner(target) !== owner(file)) {
      throw new Error('Cross-application import: ' + pathOf(file) + ' -> ' + pathOf(target))
    }
    if (graph.has(target)) deps.add(target)
  }
}

const active = new Set(), done = new Set()
function visit(file) {
  if (done.has(file)) return
  if (active.has(file)) {
    const path = [...active, file]
    throw new Error('Static import cycle: ' + path.slice(path.indexOf(file)).map(pathOf).join(' -> '))
  }
  active.add(file)
  for (const dep of graph.get(file)) {
    if (process.argv.includes('--list')) console.log(pathOf(file) + ' -> ' + pathOf(dep))
    visit(dep)
  }
  active.delete(file)
  done.add(file)
}
for (const file of graph.keys()) visit(file)
console.log(`OK: ${graph.size} TypeScript modules, ${[...graph.values()].reduce((n, deps) => n + deps.size, 0)} static runtime imports`)
