export async function fetchSourceText(url: string) {
  const host = new URL(url).hostname
  for (let attempt = 1; ; attempt++) {
    try {
      const response = await fetch(url, {
        redirect: 'error',
        signal: AbortSignal.timeout(25000),
        headers: { 'User-Agent': 'FolioKnowledge/1.0 (+https://www.codeiy.com/)' },
      })
      if (!response.ok || !response.body) throw Error('来源 HTTP ' + response.status)
      const chunks = []
      let size = 0
      for await (const bytes of response.body) {
        size += bytes.length
        if (size > 2_000_000) throw Error('来源文档超出大小限制')
        chunks.push(bytes)
      }
      return Buffer.concat(chunks).toString('utf8')
    } catch (error) {
      const failure = error instanceof Error ? error : Error(String(error))
      const cause = failure.cause as { code?: string; message?: string } | undefined
      const network = failure.name === 'TimeoutError' || failure.message === 'terminated' ||
        (failure.message === 'fetch failed' && cause?.message !== 'unexpected redirect')
      if (!network || attempt >= 3)
        throw Error(host + ' · ' + failure.message + (cause?.code ? ' (' + cause.code + ')' : ''), { cause: failure })
      await new Promise((resolve) => setTimeout(resolve, 300 * attempt))
    }
  }
}
