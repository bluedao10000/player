export function health(service: 'web' | 'api' | 'agent') {
  return { service, status: 'ok' } as const
}
