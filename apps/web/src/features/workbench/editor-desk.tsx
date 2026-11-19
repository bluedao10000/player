import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import type { ChapterDetail, SceneDetail } from '@anime/shared/knowledge'

type Scope = { World: string; Chapter: string; Name: string }
type Task = {
  id: string
  world: string
  chapter: string
  instruction: string
  status: string
  round: number
  attempt: number
}
type Source = {
  id: string
  uri: string
  text: string
  locator: string
  sha256: string
}
type Detail = Task & {
  base: { chapter: ChapterDetail; scenes: Record<string, SceneDetail> }
  issue: string
  reviewer: string
  feedback: string
  sources: Source[]
  draft: { key: string; text: string; source: string; quote: string }[]
  events: {
    id: number
    at: string
    round: number
    attempt: number
    stage: string
    note: string
  }[]
}
type List = { tasks: Task[]; scopes: Scope[] }
const status: Record<string, string> = {
  evidence: '取证中',
  drafting: '起草中',
  checking: '自检中',
  review: '待审核',
  failed: '需要处理',
  conflict: '定稿已变化',
  published: '已发布',
  created: '已登记',
  returned: '已退回',
}
class DeskError extends Error {
  constructor(
    readonly code: number,
    message: string,
  ) {
    super(message)
  }
}
async function api<T>(path: string, body?: unknown): Promise<T> {
  const r = await fetch('/editor-api/' + path, {
    cache: 'no-store',
    ...(body === undefined
      ? {}
      : {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        }),
  })
  if (!r.ok) {
    let msg = '请求失败，请重试'
    try {
      msg = (await r.json()).error || msg
    } catch {}
    throw new DeskError(
      r.status,
      r.status === 401 ? '请使用工作台管理凭证登录' : msg,
    )
  }
  return r.status === 204 ? (undefined as T) : r.json()
}
export function DeskNav({ data = false }: { data?: boolean }) {
  return (
    <nav className="ed-nav" aria-label="工作台">
      <a aria-current={!data ? 'page' : undefined} href="/workbench">
        编辑审稿
      </a>
      <a aria-current={data ? 'page' : undefined} href="/workbench?view=data">
        数据运行
      </a>
    </nav>
  )
}
export function EditorDesk() {
  const [list, setList] = useState<List | null>(null),
    [login, setLogin] = useState(false),
    [selected, select] = useState(''),
    [task, setTask] = useState<Detail | null>(null)
  const [scope, setScope] = useState('naruto-tv-2002'),
    [input, setInput] = useState(''),
    [note, setNote] = useState(''),
    [error, setError] = useState(''),
    [busy, setBusy] = useState(false),
    [tick, reload] = useState(0)
  const request = useRef<{ fingerprint: string; id: string } | null>(null),
    lock = useRef(false)
  useEffect(() => {
    let stopped = false,
      timer: ReturnType<typeof setTimeout>
    async function load() {
      try {
        const all = await api<List>('tasks')
        if (stopped) return
        setLogin(false)
        setList(all)
        const id = selected || all.tasks[0]?.id
        if (id) {
          const d = await api<Detail>('tasks/' + id)
          if (stopped) return
          setTask(d)
        } else setTask(null)
      } catch (e) {
        if (stopped) return
        if (e instanceof DeskError && e.code === 401) {
          setLogin(true)
          setList(null)
          setTask(null)
        } else setError('暂时无法读取编辑任务。')
      }
      if (!stopped) timer = setTimeout(load, 4000)
    }
    void load()
    return () => {
      stopped = true
      clearTimeout(timer)
    }
  }, [selected, tick])
  async function act(fn: () => Promise<void>) {
    if (lock.current) return
    lock.current = true
    setBusy(true)
    setError('')
    try {
      await fn()
    } catch (e) {
      setError(e instanceof Error ? e.message : '操作未完成')
    } finally {
      lock.current = false
      setBusy(false)
      reload((n) => n + 1)
    }
  }
  function submit(e: FormEvent) {
    e.preventDefault()
    void act(async () => {
      const target = list?.scopes.find((s) => s.World === scope)
      if (!target || !input.trim()) return
      const fingerprint = JSON.stringify([scope, input.trim()])
      if (request.current?.fingerprint !== fingerprint)
        request.current = { fingerprint, id: crypto.randomUUID() }
      const { id } = await api<{ id: string }>('tasks', {
        world: target.World,
        chapter: target.Chapter,
        instruction: input.trim(),
        request_id: request.current.id,
      })
      request.current = null
      setInput('')
      setTask(null)
      select(id)
      setNote('')
    })
  }
  async function decision(action: 'approve' | 'return') {
    if (!task) return
    const current = task
    await act(async () => {
      await api('tasks/' + current.id + '/' + action, {
        round: current.round,
        attempt: current.attempt,
        note,
      })
      setNote('')
    })
  }
  const review = task?.status === 'review'
  const preview = task
    ? '/editor-preview/' +
      task.id +
      '?round=' +
      task.round +
      '&attempt=' +
      task.attempt
    : ''
  const name = (w: string) => list?.scopes.find((s) => s.World === w)?.Name ?? w
  return (
    <main className="wb ed">
      <header>
        <a href="/" className="wb-brand">
          潮汐邮局
        </a>
        <span>/ 内容运营</span>
        {list && (
          <button
            className="ed-out"
            onClick={() =>
              act(async () => {
                await api('logout', {})
                setList(null)
                setTask(null)
                setLogin(true)
              })
            }
          >
            退出
          </button>
        )}
      </header>
      <div className="wb-heading">
        <h1>编辑工作台</h1>
        <DeskNav />
      </div>
      {error && (
        <p className="ed-error" role="alert">
          {error}
        </p>
      )}
      {!list ? (
        <section className="ed-login">
          <h2>登录编辑部</h2>
          <form
            method="post"
            onSubmit={(e) => {
              e.preventDefault()
              const form = e.currentTarget
              const token = new FormData(form).get('token')
              void act(async () => {
                await api('login', { token })
                form.reset()
              })
            }}
          >
            <label>
              工作台管理凭证
              <input
                type="password"
                autoComplete="current-password"
                disabled={!login || busy}
                name="token"
                minLength={32}
                required
              />
            </label>
            <button disabled={!login || busy}>进入工作台</button>
          </form>
          <small>
            {login
              ? '使用本地 .env 中的 KNOWLEDGE_ADMIN_TOKEN。'
              : '正在读取会话…'}
          </small>
        </section>
      ) : (
        <>
          <form className="ed-compose" onSubmit={submit}>
            <label>
              本次编辑范围
              <select value={scope} onChange={(e) => setScope(e.target.value)}>
                {list.scopes.map((s) => (
                  <option key={s.World} value={s.World}>
                    {s.Name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              修改意见
              <textarea
                required
                maxLength={1400}
                value={input}
                placeholder="哪一段读不懂，或希望怎样改？"
                onChange={(e) => setInput(e.target.value)}
              />
            </label>
            <button disabled={busy || !input.trim()}>提交编辑任务</button>
          </form>
          <div className="ed-grid">
            <aside>
              <h2>任务队列</h2>
              {!list.tasks.length && <p>暂无任务</p>}
              {list.tasks.map((t) => (
                <button
                  key={t.id}
                  className="ed-task"
                  aria-pressed={task?.id === t.id}
                  onClick={() => {
                    select(t.id)
                    setScope(t.world)
                    setTask(null)
                    setNote('')
                  }}
                >
                  <small>
                    {name(t.world)} · {status[t.status]}
                  </small>
                  {t.instruction}
                </button>
              ))}
            </aside>
            {task && (
              <div className="ed-detail">
                <small>
                  {name(task.world)} · 第 {task.round} 轮 ·{' '}
                  {status[task.status]}
                </small>
                <h2>{task.instruction}</h2>
                {task.feedback && <p>上次退回意见 · {task.feedback}</p>}
                {task.issue && <p className="ed-error">{task.issue}</p>}
                {task.draft?.length > 0 &&
                  ['review', 'published', 'conflict'].includes(task.status) && (
                    <>
                      <h3>本轮修订</h3>
                      {task.draft.map((d) => {
                        const chapter = task.base.chapter,
                          field = d.key.split('/')[1],
                          blocks = chapter.reader_passages ?? chapter.blocks,
                          blockIndex = blocks.findIndex((b) => b.key === field),
                          block = blocks[blockIndex],
                          scene = task.base.scenes[field],
                          source = task.sources.find((s) => s.id === d.source)
                        return (
                          <article key={d.key} className="ed-change">
                            <small>
                              {d.key.startsWith('scene/')
                                ? '场景 · ' + scene?.scene.title
                                : '章节 · 第 ' + (blockIndex + 1) + ' 段'}
                            </small>
                            <p>{d.text}</p>
                            <details>
                              <summary>对照原稿与出处</summary>
                              <p>
                                {d.key.startsWith('scene/')
                                  ? scene?.scene.text
                                  : block?.text}
                              </p>
                              <blockquote>{d.quote}</blockquote>
                              {source?.uri.startsWith('https://') ? (
                                <a
                                  href={source.uri}
                                  target="_blank"
                                  rel="noreferrer"
                                >
                                  {source.locator} ↗
                                </a>
                              ) : (
                                <small>
                                  {source?.uri} · {source?.locator}
                                </small>
                              )}
                            </details>
                          </article>
                        )
                      })}
                      <h3>H5 手机预览</h3>
                      <a
                        className="ed-link"
                        href={preview}
                        target="_blank"
                        rel="noreferrer"
                      >
                        单独打开预览 ↗
                      </a>
                      <iframe
                        title="H5 手机预览"
                        key={preview}
                        src={preview}
                        className="ed-phone"
                      />
                    </>
                  )}
                {review || task.status === 'failed' ? (
                  <section className="ed-decision">
                    <label>
                      返工意见
                      <textarea
                        value={note}
                        maxLength={1400}
                        onChange={(e) => setNote(e.target.value)}
                        placeholder="指出需要调整之处"
                      />
                    </label>
                    <button
                      disabled={busy || !note.trim()}
                      onClick={() => decision('return')}
                    >
                      退回并重新起草
                    </button>
                    {review && (
                      <button
                        className="ed-primary"
                        disabled={busy}
                        onClick={() => decision('approve')}
                      >
                        批准本轮并发布
                      </button>
                    )}
                  </section>
                ) : null}
                {task.status === 'published' && (
                  <p>
                    {task.reviewer} 已审核 ·{' '}
                    <a href={task.base.chapter.chapter.href}>查看公开章节 ↗</a>
                  </p>
                )}
                <h3>执行记录</h3>
                <ol>
                  {task.events.map((e) => (
                    <li key={e.id}>
                      <small>
                        {new Date(e.at).toLocaleTimeString('zh-CN')} · 第{' '}
                        {e.round} 轮 / {e.attempt}
                      </small>
                      <p>{e.note}</p>
                    </li>
                  ))}
                </ol>
                <details>
                  <summary>本次取证（{task.sources?.length ?? 0}）</summary>
                  {task.sources?.map((s) => (
                    <article key={s.id}>
                      <p>{s.locator}</p>
                      <small>{s.uri}</small>
                      <p>{s.text}</p>
                    </article>
                  ))}
                </details>
              </div>
            )}
          </div>
        </>
      )}
    </main>
  )
}
