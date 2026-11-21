export const runStatusLabels: Record<string, string> = {
  pending: '等待执行',
  running: '执行中',
  success: '成功',
  failed: '失败',
}

export function formatClock(value: string | null | undefined) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toISOString().slice(11, 19)
}

export function formatDate(value: string | null | undefined) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '—' : date.toISOString().slice(0, 10)
}

export function formatDateTime(value: string | null | undefined) {
  if (!value) return '—'
  return `${formatDate(value)} ${formatClock(value)} UTC`
}

export function formatDuration(start: string, finish: string | null) {
  if (!finish) return '尚未结束'
  const seconds = Math.max(0, (Date.parse(finish) - Date.parse(start)) / 1000)
  if (seconds < 60) return `${seconds.toFixed(1)} 秒`
  return `${Math.floor(seconds / 60)} 分 ${Math.round(seconds % 60)} 秒`
}

export function isShanghaiToday(value: string, checked: string) {
  const day = new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  })
  return day.format(new Date(value)) === day.format(new Date(checked))
}
