import type { Context } from '@earendil-works/chord/context'
import type { Harness } from '@earendil-works/pi-durable'

export const MEDIA_TASK_KIND = 'tidepost.media-candidate'

export type ExecutionKind = 'research' | 'editorial' | 'planning' | 'configuration'

export class ExecutionCounters {
  private readonly counts: Record<ExecutionKind, number> = {
    research: 0,
    editorial: 0,
    planning: 0,
    configuration: 0,
  }

  start(kind: ExecutionKind) {
    this.counts[kind]++
    let active = true
    return () => {
      if (!active) return
      active = false
      this.counts[kind]--
    }
  }

  snapshot() {
    return { ...this.counts }
  }
}

export async function readAgentStatus(
  harness: Harness,
  context: Context,
  version: string,
  startedAt: string,
  counters: ExecutionCounters,
) {
  const inspection = await harness.inspect(context)
  const mediaTasks = inspection.tasks.filter(
    ({ record }) =>
      record.kind === MEDIA_TASK_KIND && record.state.status !== 'terminal',
  )
  const media = {
    reference: 0,
    generate: 0,
    review: 0,
    rework: 0,
    unclassified: 0,
  }
  for (const { record } of mediaTasks) {
    const input = (record as { input?: { step?: unknown } }).input
    switch (input?.step) {
      case 'reference':
        media.reference++
        break
      case 'generate':
        media.generate++
        break
      case 'review':
        media.review++
        break
      case 'recrop':
      case 'recheck':
        media.rework++
        break
      default:
        media.unclassified++
    }
  }
  const active = counters.snapshot()
  return {
    version,
    startedAt,
    executions: {
      // Legacy aggregate keys remain intact for Phase 0 and Phase 1 readers.
      media: mediaTasks.length,
      research: active.research,
      editorial: active.editorial,
      configuration: active.planning + active.configuration,
      // Version 2 detail keys are additive; configuration_detail is the exact configuration count.
      media_reference: media.reference,
      media_generate: media.generate,
      media_review: media.review,
      media_rework: media.rework,
      media_unclassified: media.unclassified,
      planning: active.planning,
      configuration_detail: active.configuration,
    },
  }
}
