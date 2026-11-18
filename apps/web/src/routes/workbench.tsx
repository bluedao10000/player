import { createFileRoute, useRouter } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { readFeed } from '../features/workbench/feed'
import { ScopeReport } from '../features/workbench/scope-report'
import { MediaGallery } from '../features/workbench/media-gallery'
import { ResearchHistory } from '../features/workbench/research-history'
import { WorldInventory } from '../features/workbench/world-inventory'
import css from '../features/workbench/board.css?url'

export const Route = createFileRoute('/workbench')({
  loader: () => readFeed(),
  head: () => ({
    meta: [
      { title: '数据工作台 · 潮汐邮局' },
      { name: 'robots', content: 'noindex' },
    ],
    links: [{ rel: 'stylesheet', href: css }],
  }),
  component: Board,
})
const flows = {
  sync: [
    [
      '数据服务调度',
      'knowledge-sync',
      '每分钟检查到期来源。同步和全库质检均由数据服务自动调度。',
    ],
    [
      '到期检查',
      '来源 / 质检计划',
      '独立检查来源的同步间隔和质检的到期时间。来源版本未变化时记录跳过。',
    ],
    ['作品来源', 'AOD / GitHub', '流式解析作品条目并验证 schema。'],
    ['播出来源', 'Bangumi / npm', '解析官方包内的数据及播出映射。'],
    [
      '质量扫描',
      '13 项规则',
      '每六小时扫描活动作品；失败后十五分钟再试。报告保存逐规则命中数，数据服务自动完成。',
    ],
    [
      '结果与审计',
      'PostgreSQL',
      '同步任务事务提交作品；质检任务另行提交报告。失败时保留上次可用的作品版本。',
    ],
  ],
  media: [
    ['场景选题', '数据服务', '每个作品批次选三个场景，每个场景三个候选，任务与修订记录保存在 PostgreSQL。'],
    ['任务执行', 'Pi Durable', '按候选、版本和操作建立持久任务，重复轮询复用同一次执行。'],
    ['参考图生成', '图片模型', '基于现有角色立绘执行图像编辑，保存原图和参考指纹。'],
    ['手机裁切', '角色首屏', '按手机角色页的尺寸、渐变和文字位置生成预览。'],
    ['自动验图', '视觉模型', '对照参考、原图和手机预览检查人物、场景、手部与构图。'],
    ['返工与复验', '数据服务', '裁切问题先重裁，原图问题重绘一次；仍未通过则保留试稿及原因。'],
  ],
} as const
const states = {
  pending: '排队',
  running: '执行中',
  success: '成功',
  failed: '失败',
}
const time = (v: string) => new Date(v).toISOString().slice(11, 19)
const date = (v: string) => new Date(v).toISOString().slice(0, 10)
function Board() {
  const feed = Route.useLoaderData(),
    router = useRouter()
  const [flow, setFlow] = useState<'sync' | 'media'>('sync'),
    [node, setNode] = useState(0),
    [auto, setAuto] = useState(true),
    [filter, setFilter] = useState('all'),
    [selected, setSelected] = useState<string | null>(null)
  useEffect(() => {
    if (!auto) return
    const id = setInterval(() => {
      if (document.visibilityState === 'visible') void router.invalidate()
    }, 15000)
    return () => clearInterval(id)
  }, [router, auto])
  const nodes = flows[flow],
    list = feed.runs.filter((r) => filter === 'all' || r.status === filter)
  const active = list.find((r) => r.id === selected) ?? list[0]
  const branchTasks = ['anime-offline-database', 'bangumi-data', '质量扫描']
  const metrics = branchTasks.map((task) =>
    feed.runs.find((r) => r.task === task),
  )
  function tile(n: number) {
    const branch = flow === 'sync' && n >= 2 && n <= 4
    const recent = branch ? metrics[n - 2] : undefined
    return (
      <button
        className={'wb-node tone-' + n}
        aria-pressed={n === node}
        onClick={() => setNode(n)}
      >
        <span>{nodes[n][1]}</span>
        <strong>{nodes[n][0]}</strong>
        {branch && (
          <em data-state={recent?.status}>
            {recent
              ? (recent.skip === 'version_unchanged'
                  ? '版本未变'
                  : states[recent.status]) +
                ' · ' +
                time(recent.start)
              : '暂无记录'}
          </em>
        )}
      </button>
    )
  }

  return (
    <main className="wb">
      <header>
        <a className="wb-brand" href="/">
          潮汐邮局
        </a>
        <span className="wb-slash">/</span>
        <span>数据服务</span>
        <button
          className="wb-poll"
          aria-pressed={auto}
          onClick={() => setAuto(!auto)}
        >
          <i className={auto ? 'lit' : ''} />
          {auto ? '自动刷新 · 15s' : '已暂停刷新'}
        </button>
      </header>
      <div className="wb-heading">
        <div>
          <small>DATA OPERATIONS</small>
          <h1>数据工作台</h1>
        </div>
        <code>UTC　{time(feed.checked)}</code>
      </div>
      {[...new Set(feed.media?.filter(c=>c.art).map(c=>c.batch))].map(batch=><MediaGallery key={batch} items={feed.media} batch={batch} />)}
      <MediaGallery items={feed.media} batch="bebop-e3" />
      <MediaGallery items={feed.media} batch="bebop-portrait" />
      <MediaGallery items={feed.media} batch="bebop" />
      <MediaGallery items={feed.media} batch="mio" />
      <ResearchHistory runs={feed.research} />
      <ScopeReport scopes={feed.scopes} />
      <WorldInventory worlds={feed.worlds} scopes={feed.scopes} />
      <div className="wb-grid">
        <aside>
          <h2>运行摘要</h2>
          <p className="wb-scope">最近记录窗口</p>
          <dl>
            {[
              [
                '同步批次',
                feed.syncOnline
                  ? feed.runs.filter((r) => r.task !== '质量扫描').length
                  : '—',
              ],
              [
                '质检批次',
                feed.qualityOnline
                  ? feed.runs.filter((r) => r.task === '质量扫描').length
                  : '—',
              ],
              [
                '执行中',
                feed.runs.filter((r) => r.status === 'running').length,
              ],
              ['失败', feed.runs.filter((r) => r.status === 'failed').length],
            ].map(([k, v]) => (
              <div key={k}>
                <dt>{k}</dt>
                <dd>{v}</dd>
              </div>
            ))}
          </dl>
          <div className="wb-detail">
            <small>节点详情</small>
            <h3>{nodes[node][0]}</h3>
            <p>{nodes[node][2]}</p>
          </div>
        </aside>
        <section className="wb-map">
          <div className="wb-bar">
            <h2>运行拓扑</h2>
            <div>
              {(['sync', 'media'] as const).map((k) => (
                <button
                  key={k}
                  aria-pressed={flow === k}
                  onClick={() => {
                    setFlow(k)
                    setNode(0)
                  }}
                >
                  {k === 'sync' ? '知识治理' : '图片优化'}
                </button>
              ))}
            </div>
          </div>
          <p className="wb-caption">
            {flow === 'sync'
              ? '已接入同步与质检记录 · 节点可查看职责'
              : '当前候选和实际验图结果见上方场景试稿'}
          </p>
          <div className="wb-flow">
            <div>{tile(0)}</div>
            <b>↓</b>
            <div>{tile(1)}</div>
            <b>↓</b>
            <div className="wb-branches">
              {[2, 3, 4].map((n) => (
                <div key={n}>{tile(n)}</div>
              ))}
            </div>
            <b>↓</b>
            <div>{tile(5)}</div>
          </div>
        </section>
      </div>
      <section className="wb-log">
        <div className="wb-bar">
          <h2>运行日志</h2>
          <select
            aria-label="筛选运行状态"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          >
            <option value="all">全部状态</option>
            {Object.entries(states).map(([k, v]) => (
              <option key={k} value={k}>
                {v}
              </option>
            ))}
          </select>
        </div>
        {(!feed.syncOnline || !feed.qualityOnline) && (
          <p role="status">
            {!feed.syncOnline ? '同步记录暂不可用。 ' : ''}
            {!feed.qualityOnline ? '质检记录接口暂不可用。' : ''}
          </p>
        )}
        <div className="wb-events">
          {list.length ? (
            list.map((r) => (
              <button
                key={r.id}
                aria-pressed={active?.id === r.id}
                onClick={() => setSelected(r.id)}
              >
                <time>
                  {date(r.start).slice(5)} {time(r.start)}
                </time>
                <strong>{r.task}</strong>
                <span data-state={r.status}>
                  {r.skip === 'version_unchanged'
                    ? '版本未变'
                    : states[r.status]}
                </span>
              </button>
            ))
          ) : (
            <p>当前筛选下没有运行记录。</p>
          )}
        </div>
        {active && (
          <footer>
            <code>{active.id}</code>
            {active.http != null && <span>HTTP {active.http}</span>}
            {active.failed != null && active.failed > 0 && (
              <span>失败条目 {active.failed}</span>
            )}
            {active.score != null && <span>平均质量分 {active.score}</span>}
            <span>读取 {active.read.toLocaleString('en-US')}</span>
            <span>
              {active.issues === null
                ? '新增或更新 ' + active.updated
                : '规则命中 ' + active.issues}
            </span>
            <span>
              {active.finish
                ? '耗时 ' +
                  Math.max(
                    0,
                    (Date.parse(active.finish) - Date.parse(active.start)) /
                      1000,
                  ).toFixed(1) +
                  's'
                : '尚未结束'}
            </span>
          </footer>
        )}
      </section>
    </main>
  )
}
