import type { StreamOptions } from '@earendil-works/pi-ai'

import { createProvider } from '@earendil-works/pi-ai/models'
import { openaiProvider } from '@earendil-works/pi-ai/providers/openai'
import { openAIResponsesApi } from '@earendil-works/pi-ai/api/openai-responses.lazy'
import { openAICompletionsApi } from '@earendil-works/pi-ai/api/openai-completions.lazy'
import { profile, streamFetch, type ModelCallReceipt } from './model-api.ts'

export function contentProvider(run: string, phase: string, scope = 'research', structured = false, onReceipt?: (receipt: ModelCallReceipt) => void) {
  const p = profile('content')
  const arkKimi = p.gateway && p.model === 'volcengine/kimi-k3.1-pro'
  const protocol = p.gateway ? 'chat-completions' : process.env.CONTENT_AI_PROTOCOL || 'responses'
  if (protocol !== 'responses' && protocol !== 'chat-completions')
    throw Error('CONTENT_AI_PROTOCOL 无效')
  const streams =
    protocol === 'responses' ? openAIResponsesApi() : openAICompletionsApi()
  const api =
    protocol === 'responses' ? 'openai-responses' : 'openai-completions'
  const template = openaiProvider()
    .getModels()
    .find((m) => m.id === p.model)
  const model = {
    ...(template ?? {
      input: ['text'] as 'text'[],
      reasoning: false,
      contextWindow: 32000,
      maxTokens: 4096,
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
    }),
    id: p.model,
    name: p.model,
    provider: 'openai',
    baseUrl: p.base,
    api,
    ...(protocol === 'chat-completions' && p.gateway
      ? { compat: { supportsUsageInStreaming: true } } : {}),
  } as const
  const fetch = streamFetch({
    ...p,
    subject: scope + ':' + run,
    operation: phase,
  }, onReceipt)
  // Ark Kimi defaults to thinking and can spend the entire output budget before
  // emitting the structured draft. These tasks already carry a sourced outline.
  const transport = <T extends StreamOptions>(options?: T) => ({
    ...options,
    fetch,
    ...(arkKimi ? {
      onPayload: async (payload: unknown, selected: Parameters<NonNullable<StreamOptions['onPayload']>>[1]) => ({
        ...((await options?.onPayload?.(payload, selected) ?? payload) as Record<string, unknown>),
        thinking: { type: 'disabled' },
        ...(structured ? { response_format: { type: 'json_object' } } : {}),
      }),
    } : {}),
  })
  return createProvider({
    id: 'openai',
    baseUrl: p.base,
    auth: {
      apiKey: {
        name: 'Content API',
        resolve: async () => ({ auth: { apiKey: p.key } }),
      },
    },
    models: [model],
    api: {
      stream: (m, c, o) => streams.stream(m, c, transport(o)),
      streamSimple: (m, c, o) => streams.streamSimple(m, c, transport(o)),
    },
  })
}
