import type { BackupStatus } from './feed'

const clock = new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
})
function time(value: string | null) {
  return value ? clock.format(new Date(value)) : '—'
}
function size(bytes: number | null) {
  return bytes === null ? '—' : (bytes / 1048576).toFixed(2) + ' MiB'
}
function elapsed(start: string, end: string | null) {
  if (!end) return '进行中'
  const secs = Math.max(0, Math.round((Date.parse(end) - Date.parse(start)) / 1000))
  return secs >= 60 ? Math.floor(secs / 60) + ' 分 ' + secs % 60 + ' 秒' : secs + ' 秒'
}
export function BackupReport({ data, checked }: { data: BackupStatus | null; checked: string }) {
  const verified = data?.latest_verified
  const stale = data?.observed_at && Date.parse(checked) - Date.parse(data.observed_at) > 10 * 60_000
  return <section className="wb-inventory wb-backups" aria-labelledby="backup-heading">
    <div className="wb-backup-title">
      <h2 id="backup-heading">广州制作库备份</h2>
      <span>每天 03:15 · 北京时间</span>
    </div>
    {!data ? <p>备份记录接口暂不可用。</p> : <>
      {verified ? <dl>
        <div><dt>最近一次成功</dt><dd>{time(verified.finished_at)}</dd></div>
        <div><dt>备份大小</dt><dd>{size(verified.archive_bytes)}</dd></div>
        <div><dt>耗时</dt><dd>{elapsed(verified.started_at, verified.finished_at)}</dd></div>
        <div><dt>恢复校验</dt><dd>通过 · {verified.verified_tables} 张表</dd></div>
      </dl> : <p>尚无通过恢复校验的备份记录。</p>}
      <p className="wb-observed" data-delayed={Boolean(stale)}>{data.observed_at ? '最近采集于 ' + time(data.observed_at) : '等待读取 K3s 任务'}
        {stale && ' · 采集已延迟，请检查备份节点和 Mesh 连接。'}</p>
      {data.runs.map(run => <details key={run.job_uid} data-status={run.status}>
        <summary><time dateTime={run.started_at}>{time(run.started_at)}</time>　<strong>{run.status === 'failed' ? '失败' : run.status === 'running' ? '进行中' : run.verified_tables ? '校验通过' : '已结束 · 缺少校验回执'}</strong>
          {run.failure_reason && ' · ' + run.failure_reason}
          {Boolean(run.attempts?.length) && ' · ' + run.attempts!.length + ' 次尝试失败'}</summary>
        <p>{run.trigger_kind === 'manual' ? '手动执行 · ' : run.trigger_kind === 'scheduled' ? '定时执行 · ' : ''}{run.job_name} · 耗时 {elapsed(run.started_at, run.finished_at)}</p>
        {run.archive_bytes !== null && <p>归档 {size(run.archive_bytes)}（{run.archive_bytes.toLocaleString('zh-CN')} 字节）。SHA-256 通过，恢复后 {run.verified_tables} 张表的行数逐表一致。</p>}
        {run.attempts?.map((attempt, index) => <p key={index}>尝试失败 · {time(attempt.at)} · {attempt.reason}</p>)}
        {run.status === 'failed' && <p>本次未生成通过全部校验的备份。</p>}
      </details>)}
    </>}
  </section>
}
