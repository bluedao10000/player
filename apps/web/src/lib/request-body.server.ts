export async function readBody(request: Request, limit = 16_384) {
  const reader = request.body?.getReader()
  if (!reader) return ''
  const chunks: Uint8Array[] = []
  let size = 0
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    size += value.length
    if (size > limit) {
      await reader.cancel()
      throw new Error('body too large')
    }
    chunks.push(value)
  }
  return Buffer.concat(chunks).toString('utf8')
}
