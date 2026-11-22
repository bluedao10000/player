import type { Context } from '@earendil-works/chord/context'
import type { Harness } from '@earendil-works/pi-durable'

export const MEDIA_TASK_KIND = 'tidepost.media-candidate'

export type ExecutionKind = 'research' | 'editorial' | 'configuration'

export class ExecutionCounters {
  private readonly counts: Record<ExecutionKind, number> = {
    research: 0,
    editorial: 0,
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
  const media = inspection.tasks.filter(
    ({ record }) =>
      record.kind === MEDIA_TASK_KIND && record.state.status !== 'terminal',
  ).length
  return {
    version,
    startedAt,
    executions: { media, ...counters.snapshot() },
  }
}
