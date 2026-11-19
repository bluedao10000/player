import { createProvider } from '@earendil-works/pi-ai/models'
import { openaiProvider } from '@earendil-works/pi-ai/providers/openai'
import { openAIResponsesApi } from '@earendil-works/pi-ai/api/openai-responses.lazy'
import { openAICompletionsApi } from '@earendil-works/pi-ai/api/openai-completions.lazy'
import { profile, streamFetch } from './model-api.ts'

export function contentProvider(run: string, phase: string, scope = 'research') {
  const p = profile('content')
  const protocol = process.env.CONTENT_AI_PROTOCOL || 'responses'
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
  } as const
  const fetch = streamFetch({
    ...p,
    subject: scope + ':' + run,
    operation: phase,
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
      stream: (m, c, o) => streams.stream(m, c, { ...o, fetch }),
      streamSimple: (m, c, o) => streams.streamSimple(m, c, { ...o, fetch }),
    },
  })
}
