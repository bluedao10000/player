import test from 'node:test'
import assert from 'node:assert/strict'
import { ExecutionCounters, MEDIA_TASK_KIND, readAgentStatus } from '../src/executions.ts'

test('Agent reports media steps and keeps legacy aggregate keys compatible', async () => {
  const counters = new ExecutionCounters()
  const stopPlanning = counters.start('planning')
  const stopConfiguration = counters.start('configuration')
  const harness = {
    inspect: async () => ({
      tasks: [
        { record: { kind: MEDIA_TASK_KIND, state: { status: 'running' }, input: { step: 'reference' } } },
        { record: { kind: MEDIA_TASK_KIND, state: { status: 'running' }, input: { step: 'generate' } } },
        { record: { kind: MEDIA_TASK_KIND, state: { status: 'running' }, input: { step: 'review' } } },
        { record: { kind: MEDIA_TASK_KIND, state: { status: 'running' }, input: { step: 'recrop' } } },
        { record: { kind: MEDIA_TASK_KIND, state: { status: 'running' }, input: { step: 'recheck' } } },
        { record: { kind: MEDIA_TASK_KIND, state: { status: 'terminal' }, input: { step: 'review' } } },
        { record: { kind: MEDIA_TASK_KIND, state: { status: 'running' }, input: { step: 'unknown' } } },
        { record: { kind: 'other', state: { status: 'running' }, input: { step: 'reference' } } },
      ],
    }),
  }
  const status = await readAgentStatus(harness, {}, 'v-test', '2026-01-01T00:00:00Z', counters)
  assert.deepEqual(status.executions, {
    media: 6,
    research: 0,
    editorial: 0,
    configuration: 2,
    media_reference: 1,
    media_generate: 1,
    media_review: 1,
    media_rework: 2,
    media_unclassified: 1,
    planning: 1,
    configuration_detail: 1,
  })
  stopPlanning()
  stopPlanning()
  stopConfiguration()
  assert.deepEqual(counters.snapshot(), { research: 0, editorial: 0, planning: 0, configuration: 0 })
})

test('Agent status accepts mixed legacy and detailed execution fields without changing legacy totals', async () => {
  const counters = new ExecutionCounters()
  const harness = { inspect: async () => ({ tasks: [] }) }
  const status = await readAgentStatus(harness, {}, 'v-test', 'started', counters)
  assert.equal(status.executions.media, 0)
  assert.equal(status.executions.configuration, status.executions.planning + status.executions.configuration_detail)
  assert.equal(status.executions.media_reference, 0)
})
