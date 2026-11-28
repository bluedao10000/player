// Keep the provider's structured diagnostics. Free-text errors can echo prompts,
// signed URLs or credentials, so store a normalized message for known classes.
function field(value: unknown) {
  return typeof value === 'string' &&
    /^[A-Za-z0-9_.[\]/-]{1,128}$/.test(value) &&
    !/^(sk-|Bearer)/i.test(value)
    ? value
    : undefined
}
function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}

export function mediaProviderFailure(
  response: Response,
  raw: unknown,
  url: string,
  model: string,
) {
  const error = record(raw)
  const code = field(error.code)
  const type = field(error.type)
  const param = field(error.param)
  const details = record(error.moderation_details)
  const stage = ['input', 'output', 'unknown'].find(
    (value) => value === details.moderation_stage,
  )
  const categories = Array.isArray(details.categories)
    ? details.categories
        .map(field)
        .filter((x): x is string => !!x)
        .slice(0, 16)
    : []
  const status = response.status
  let action = 'inspect'
  let message = '供应商错误，需核查'
  if (code === 'moderation_blocked' || code === 'content_policy_violation') {
    action = 'moderation'
    message = '内容审核拦截，需人工复核'
  } else if (
    code === 'insufficient_quota' ||
    code === 'billing_hard_limit_reached'
  ) {
    message = '供应商额度受限，需核查账户'
  } else if (
    status === 429 ||
    (status >= 500 && status < 600) ||
    code === 'server_is_overloaded'
  ) {
    action = 'retry'
    message = '供应商临时不可用'
  } else if (
    status === 400 &&
    (param ||
      type === 'invalid_request_error' ||
      type === 'image_generation_user_error')
  ) {
    action = 'correct_input'
    message = '需检查请求参数或输入文件'
  }
  // Extract a stable provider diagnostic phrase without copying its arguments.
  const providerMessage =
    typeof error.message === 'string'
      ? error.message.match(
          /request blocked by (?:the |our )?safety system|unsupported (?:image format|file type|parameter)|invalid (?:image format|image size|image|value|file|parameter)|missing required parameter|rate limit exceeded|server is overloaded/i,
        )?.[0]
      : undefined
  const paths = [
    '/images/edits',
    '/images/generations',
    '/chat/completions',
    '/responses',
  ]
  const endpoint = paths.find((path) => new URL(url).pathname.endsWith(path))
  return {
    status,
    action,
    summary: message,
    message: providerMessage ?? message,
    at: new Date().toISOString(),
    model,
    ...(endpoint && { endpoint }),
    ...(code && { code }),
    ...(type && { type }),
    ...(param && { param }),
    ...(field(response.headers.get('x-request-id')) && {
      request_id: response.headers.get('x-request-id')!,
    }),
    ...(stage && { moderation_stage: stage }),
    ...(categories.length && { categories }),
  }
}

export class MediaProviderError extends Error {
  readonly detail: ReturnType<typeof mediaProviderFailure>
  constructor(detail: ReturnType<typeof mediaProviderFailure>) {
    super(
      '模型服务 HTTP ' +
        detail.status +
        ' · ' +
        detail.summary +
        (detail.param ? '（' + detail.param + '）' : ''),
    )
    this.detail = detail
  }
}
