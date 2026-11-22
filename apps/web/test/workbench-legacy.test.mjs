import test from 'node:test'
import assert from 'node:assert/strict'
import { legacyWorkbenchHref, resolveLegacyWorkbench } from '../src/features/workbench/legacy-links.ts'

const cases = [
  ['data defaults to overview', { view: 'data' }, '/workbench'],
  ['data section stays on overview', { view: 'data', section: 'data' }, '/workbench'],
  ['knowledge section', { view: 'data', section: 'knowledge' }, '/workbench/knowledge'],
  ['knowledge section opens requested work', { view: 'data', section: 'knowledge', world: 'tidepost' }, '/workbench/knowledge/tidepost?world=tidepost'],
  ['media section and candidate', { view: 'data', section: 'media', candidate: '42' }, '/workbench/knowledge?candidate=42'],
  ['media section opens work and preserves candidate', { view: 'data', section: 'media', world: 'cowboy-bebop-tv', candidate: '42' }, '/workbench/knowledge/cowboy-bebop-tv?world=cowboy-bebop-tv&candidate=42'],
  ['history section', { view: 'data', section: 'history' }, '/workbench/operations'],
  ['chat keeps world, plan, and task', { view: 'chat', world: 'tidepost', plan: 'plan-1', task: 'task-1' }, '/workbench/assistant?world=tidepost&plan=plan-1&task=task-1'],
  ['editor review keeps the task', { view: 'editor', world: 'tidepost', task: 'task-1' }, '/workbench/exceptions?world=tidepost&task=task-1'],
  ['account', { view: 'me' }, '/workbench/me'],
  ['section-only overview', { section: 'data' }, '/workbench'],
  ['section-only knowledge deep link', { section: 'knowledge' }, '/workbench/knowledge'],
  ['section-only knowledge opens work', { section: 'knowledge', world: 'tidepost' }, '/workbench/knowledge/tidepost?world=tidepost'],
  ['section-only media deep link', { section: 'media', candidate: 24 }, '/workbench/knowledge?candidate=24'],
  ['section-only media opens work and preserves candidate', { section: 'media', world: 'tidepost', candidate: 24 }, '/workbench/knowledge/tidepost?world=tidepost&candidate=24'],
  ['section-only history deep link', { section: 'history', run_key: 'quality:1', source_table: 'knowledge_quality_runs' }, '/workbench/operations?run_key=quality%3A1&source_table=knowledge_quality_runs'],
]

for (const [name, search, expected] of cases) {
  test(name, () => {
    const target = resolveLegacyWorkbench(search)
    assert.ok(target)
    assert.equal(legacyWorkbenchHref(target), expected)
  })
}

test('canonical workbench URL does not redirect', () => {
  assert.equal(resolveLegacyWorkbench({ world: 'tidepost' }), null)
})

test('detail metadata is carried to new routes', () => {
  const target = resolveLegacyWorkbench({
    view: 'data', section: 'history', world: 'tidepost', plan: 'plan-1', task: 'task-1', candidate: '42',
    item_key: 'item-1', run_key: 'quality:run-1', source_table: 'knowledge_quality_runs', source_id: 'run-1',
    episode: '7', worker: 'gz/sync', loop: 'quality',
  })
  assert.ok(target)
  assert.equal(legacyWorkbenchHref(target), '/workbench/operations?world=tidepost&plan=plan-1&task=task-1&candidate=42&item_key=item-1&run_key=quality%3Arun-1&source_table=knowledge_quality_runs&source_id=run-1&episode=7&worker=gz%2Fsync&loop=quality')
})
