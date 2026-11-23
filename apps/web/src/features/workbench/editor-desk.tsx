import { ArrowRight, Mail, Send, Sparkles } from 'lucide-react'
import { WorkbenchChat } from './chat'
import { WorkbenchSidebar } from './shared/shell'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { editedField, type EditorPages } from './editor-pages'
import { api } from './editor-api'

type Scope = { World: string; Chapter: string; Name: string }
type Task = {
  id: string
  world: string
  chapter: string
  kind: 'chapter' | 'profiles'
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
  base: EditorPages
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
export function EditorDesk({ accountView = false, chatView = false, embedded = false, context }: { accountView?: boolean; chatView?: boolean; embedded?: boolean; context?: { world?: string; plan?: string; task?: string; episode?: number; prompt?: string } }) {
  const [list, setList] = useState<List | null>(null),
    [login, setLogin] = useState(false),
    [selected, select] = useState(context?.task ?? ''),
    [task, setTask] = useState<Detail | null>(null)
  const [email,setEmail]=useState('')
  const [passwordMode, setPasswordMode] = useState(false)
  const [loginPassword, setLoginPassword] = useState('')
  const [sent,setSent]=useState(false)
  const [code,setCode]=useState('')
  const [scope, setScope] = useState('naruto-tv-2002'),
    [kind, setKind] = useState<Task['kind']>('chapter'),
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
        if (e instanceof Error && 'code' in e && e.code === 401) {
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
  useEffect(() => { select(context?.task ?? '') }, [context?.task])
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
      const fingerprint = JSON.stringify([scope, kind, input.trim()])
      if (request.current?.fingerprint !== fingerprint)
        request.current = { fingerprint, id: crypto.randomUUID() }
      const { id } = await api<{ id: string }>('tasks', {
        world: target.World,
        chapter: target.Chapter,
        kind,
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
      task.attempt +
      (task.kind === 'profiles' && task.draft[0]
        ? '&page=' + encodeURIComponent(task.draft[0].key)
        : '')
    : ''
  const name = (w: string) => list?.scopes.find((s) => s.World === w)?.Name ?? w
  const Root = embedded ? 'div' : 'main'
  return (
    <Root className={embedded ? 'ed ed-embedded' : list ? 'wb wb-layout ed' : 'wb ed ed-guest'}>
      {list && !embedded && <WorkbenchSidebar current={accountView ? 'me' : chatView ? 'assistant' : 'exceptions'} />}
      <div className={embedded ? undefined : list ? 'wb-shell' : undefined}>
      {list && !accountView && !chatView && !embedded && (
      <div className="wb-heading">
        <div><h1>编辑审稿</h1><p>查看编辑任务、对照修订稿并审核发布</p></div>
      </div>
      )}
      {error && list && (
        <p className="ed-error" role="alert">
          {error}
        </p>
      )}
      {!list ? (
        <section className="ed-login-layout" aria-labelledby="workbench-login-title">
          <div className="ed-login-art">
            <img src="/art/night-post.webp" alt="" />
            <a href="/">Folio 时光</a>
          </div>
          <div className="ed-login-body">
            <div className="ed-login-icon"><Mail size={24} aria-hidden /></div>
            <h1 id="workbench-login-title">登录工作台</h1>
            <p>{sent ? `验证码已发送至 ${email}，5 分钟内有效。` : passwordMode ? '使用已设置的工作台密码登录。' : '使用邮箱验证码登录。首次验证会自动创建账户。'}</p>
            {error && <p className="ed-error" role="alert">{error}</p>}
            <form onSubmit={e => {
              e.preventDefault()
              void act(async () => {
                if (passwordMode) {
                  await api('password-login', { email: email.trim(), password: loginPassword })
                  setLoginPassword('')
                  location.reload()
                } else if (!sent) {
                  const address = email.trim()
                  await api('code', { email: address })
                  setEmail(address)
                  setSent(true)
                } else {
                  await api('verify', { email, code })
                  setSent(false)
                  setCode('')
                  location.reload()
                }
              })
            }}>
              {sent ? (
                <label>验证码
                  <input key="code" value={code} onChange={e => setCode(e.target.value)} required pattern="[0-9]{6}" maxLength={6} inputMode="numeric" autoComplete="one-time-code" autoFocus />
                </label>
              ) : (
                <label>邮箱
                  <input key="email" type="email" autoComplete="email" placeholder="you@example.com" required value={email} onChange={e => setEmail(e.target.value)} />
                </label>
              )}
              {passwordMode && <label>密码
                <input type="password" autoComplete="current-password" value={loginPassword} onChange={e => setLoginPassword(e.target.value)} required />
              </label>}
              <button className="ed-primary" disabled={!login || busy}>
                {busy ? (sent || passwordMode ? '正在验证…' : '正在发送…') : passwordMode ? '登录' : sent ? '验证并登录' : '获取验证码'}
                <ArrowRight size={16} aria-hidden />
              </button>
              {sent && <button type="button" className="ed-link-button" disabled={busy} onClick={() => {
                setSent(false)
                setCode('')
                setError('')
              }}>更换邮箱或重新发码</button>}
              {!sent && (
                <button type="button" className="ed-link-button" disabled={busy} onClick={() => {
                  setPasswordMode(!passwordMode)
                  setLoginPassword('')
                  setError('')
                }}>{passwordMode ? '使用邮箱验证码' : '使用密码登录'}</button>
              )}
            </form>
          </div>
        </section>
      ) : accountView ? <WorkbenchAccount showHeading={!embedded} /> : chatView ? <WorkbenchChat request={api} initialWorld={context?.world} initialPlan={context?.plan} initialTask={context?.task} initialEpisode={context?.episode} initialPrompt={context?.prompt} /> : (
        <>
          <div className="ed-composer-dock">
          <form id="editor-composer" className="ed-compose" onSubmit={submit} aria-label="AI 编辑助手">
            <div className="ed-composer-title"><Sparkles size={16} aria-hidden />AI 编辑助手</div>
            <div className="ed-composer-options">
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
              <span id="editor-kind-label">编辑内容</span>
              <select
                aria-labelledby="editor-kind-label"
                value={kind}
                onChange={(e) => setKind(e.target.value as Task['kind'])}
              >
                <option value="chapter">章节与场景</option>
                <option value="profiles">人物介绍与世界观</option>
              </select>
            </label>
            </div>
            <div className="ed-composer-row">
            <label>
              <span className="ed-visually-hidden">修改意见</span>
              <textarea
                rows={2}
                required
                maxLength={1400}
                value={input}
                placeholder="哪一段读不懂，或希望怎样改？"
                onChange={(e) => setInput(e.target.value)}
              />
            </label>
            <button className="ed-primary" disabled={busy || !input.trim()} aria-label="提交编辑任务"><Send size={18} aria-hidden /></button>
            </div>
          </form>
          </div>
          <div className="ed-grid">
            <aside className="wb-panel">
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
                    setKind(t.kind)
                    setTask(null)
                    setNote('')
                  }}
                >
                  <small>
                    {name(t.world)} ·{' '}
                    {t.kind === 'profiles' ? '人物与世界观' : '章节与场景'} ·{' '}
                    {status[t.status]}
                  </small>
                  {t.instruction}
                </button>
              ))}
            </aside>
            {task && (
              <div className="ed-detail wb-panel">
                <small>
                  {name(task.world)} · 第 {task.round} 轮 ·{' '}
                  {status[task.status]}
                </small>
                <h2>{task.instruction}</h2>
                <a href={`/workbench/assistant?world=${encodeURIComponent(task.world)}&task=${task.id}`}>在对话中查看任务</a>
                {task.feedback && <p>上次退回意见 · {task.feedback}</p>}
                {task.issue && <p className="ed-error">{task.issue}</p>}
                {task.draft?.length > 0 &&
                  ['review', 'published', 'conflict'].includes(task.status) && (
                    <>
                      <h3>本轮修订</h3>
                      {task.draft.map((d) => {
                        const { label, before } = editedField(task.base, d.key),
                          source = task.sources.find((s) => s.id === d.source)
                        return (
                          <article key={d.key} className="ed-change">
                            <small>{label}</small>
                            <p>{d.text}</p>
                            <details>
                              <summary>对照原稿与出处</summary>
                              <p>{before}</p>
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
                    {'profile_pages' in task.base ? (
                      task.draft.map((d) => {
                        const field = editedField(task.base, d.key)
                        return (
                          <a key={d.key} href={field.href}>
                            {field.label} ↗　
                          </a>
                        )
                      })
                    ) : (
                      <a href={task.base.chapter.chapter.href}>
                        查看公开章节 ↗
                      </a>
                    )}
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
      </div>
    </Root>
  )
}
function WorkbenchAccount({ showHeading = true }: { showHeading?: boolean }) {
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    api<{ email: string }>('account').then(account => {
      if (active) setEmail(account.email)
    }).catch((err: Error) => {
      if (active) setError(err.message)
    })
    return () => { active = false }
  }, [])

  async function act(fn: () => Promise<void>) {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await fn()
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作未完成，请重试。')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="ed-account">
      {showHeading && <div className="wb-heading"><h1>我的</h1></div>}
      <section className="wb-panel">
        <h2>工作台账户</h2>
        <p>{email || '正在读取邮箱…'}</p>
      </section>
      {error && <p className="ed-error" role="alert">{error}</p>}
      <section className="wb-panel">
        <h2>设置或修改密码</h2>
        <p className="wb-muted">验证当前邮箱后可更新密码。保存后需要重新登录。</p>
        <form onSubmit={e => {
          e.preventDefault()
          const fields = new FormData(e.currentTarget)
          void act(async () => {
            const password = fields.get('password')
            if (password !== fields.get('confirm')) throw new Error('两次输入的密码不一致。')
            await api('password', { code: fields.get('code'), password })
            window.location.replace('/workbench/me')
          })
        }}>
          <label>验证码
            <span className="ed-account-code">
              <input name="code" required pattern="[0-9]{6}" maxLength={6} inputMode="numeric" autoComplete="one-time-code" />
              <button type="button" disabled={!email || busy} onClick={() => void act(async () => {
                await api('code', { email })
                setSent(true)
              })}>{sent ? '重新获取' : '获取验证码'}</button>
            </span>
          </label>
          {sent && <p className="wb-muted" role="status">验证码已发送，5 分钟内有效。</p>}
          <label>新密码<input name="password" type="password" minLength={12} maxLength={72} autoComplete="new-password" required /></label>
          <label>确认新密码<input name="confirm" type="password" minLength={12} maxLength={72} autoComplete="new-password" required /></label>
          <button className="ed-primary" disabled={busy || !email}>保存密码</button>
        </form>
      </section>
      <button type="button" className="ed-logout" disabled={busy} onClick={() => void act(async () => {
        await api('logout', {})
        window.location.replace('/workbench/me')
      })}>退出登录</button>
    </div>
  )
}
