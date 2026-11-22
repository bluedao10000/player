export const runStatusLabels: Record<string, string> = {
  pending: '等待执行',
  running: '执行中',
  success: '成功',
  failed: '失败',
}

// 工作台统一按北京时间显示，见 docs/界面文案规范.md。
const shanghaiDate = new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})
const shanghaiClock = new Intl.DateTimeFormat('en-GB', {
  timeZone: 'Asia/Shanghai',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hourCycle: 'h23',
})

function parse(value: string | null | undefined) {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

export function formatClock(value: string | null | undefined) {
  const date = parse(value)
  return date ? shanghaiClock.format(date) : '—'
}

export function formatDate(value: string | null | undefined) {
  const date = parse(value)
  return date ? shanghaiDate.format(date) : '—'
}

export function formatDateTime(value: string | null | undefined) {
  const date = parse(value)
  return date ? `${shanghaiDate.format(date)} ${shanghaiClock.format(date).slice(0, 5)}` : '—'
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
