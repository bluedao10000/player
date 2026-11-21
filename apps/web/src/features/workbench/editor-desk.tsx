import { siteName } from '../../site'
import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { editedField, type EditorPages } from './editor-pages'

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
      r.status === 401 ? (path === 'login' ? '管理凭证无效，请重新复制本地工作台凭证。' : '登录已过期，请重新登录。') : msg,
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
  const [superadmin,setSuperadmin]=useState(false)
  const [notice,setNotice]=useState('')
  const [member,setMember]=useState('')
  const [email,setEmail]=useState('')
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
        const account=await api<{email:string;editor:boolean;superadmin:boolean}>('account')
        if(stopped)return
        setSuperadmin(account.superadmin)
        if(!account.editor){setMember(account.email);setLogin(false);setList(null);return}
        setMember('')
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
          setMember('')
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
  return (
    <main className="wb ed">
      <header>
        <a href="/" className="wb-brand">
          {siteName}
        </a>
        <span>/ 内容运营</span>
        {(list || member) && (
          <button
            className="ed-out"
            onClick={() =>
              act(async () => {
                await api('logout', {})
                setMember('');setSent(false);setCode('')
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
      {list && superadmin && <section className="ed-decision"><h2>超级管理员 · 编辑权限</h2>
      <form onSubmit={e=>{e.preventDefault();const data=new FormData(e.currentTarget);setNotice('');void act(async()=>{
        await api('editor-access',{email:data.get('email'),action:data.get('action')});setNotice('邮箱权限已更新。')
      })}}><label>成员邮箱<input type="email" name="email" required/></label>
      <label>权限操作<select name="action"><option value="grant">开通编辑权限</option><option value="revoke">撤销编辑权限</option></select></label>
      <button disabled={busy}>保存权限</button></form><p role="status">{notice}</p></section>}
      {list && <ProductionPlans />}
      {member ? <section className="ed-login"><h2>我的工作台账户</h2><p>{member}</p><p>已登录，当前账户为普通成员。请将上方邮箱交给管理员开通编辑权限，开通后刷新本页即可。</p></section> : !list ? (
        <section className="ed-login">
          <h2>登录工作台</h2>
          <p>首次验证邮箱后自动注册。</p>
          <form onSubmit={e=>{e.preventDefault();void act(async()=>{
            if(!sent){await api('code',{email});setSent(true)}
            else{await api('verify',{email,code});setCode('');setSent(false);if(location.search.includes('view=data'))location.reload()}
          })}}>
            <label>邮箱<input type="email" autoComplete="email" required readOnly={sent} value={email} onChange={e=>setEmail(e.target.value)}/></label>
            {sent&&<><p>验证码已发送，5 分钟内有效。</p><label>六位验证码<input value={code} onChange={e=>setCode(e.target.value)} required pattern="[0-9]{6}" maxLength={6} inputMode="numeric" autoComplete="one-time-code"/></label></>}
            <button disabled={!login||busy}>{sent?'验证并登录':'获取验证码'}</button>
            {sent&&<button type="button" disabled={busy} onClick={()=>{setSent(false);setCode('')}}>更换邮箱或重新发码</button>}
          </form>
          <details><summary>超级管理员凭证登录</summary>
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
          </small></details>
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
    </main>
  )
}

type PlanInfo = {
  id: string; world: string; first_episode: number; last_episode: number; revision: number
  episodes:{number:number;title:string;scene:string;research?:string;attempt?:number;elapsed:number;retries:number;blocker?:string;error?:string;accepted?:boolean;receipt?:number;href?:string}[]
  calls:{stage:string;actual:number|null;seconds:number|null}[]; stage: string; plan_brief?: string; gaps: number[]; paused_reason?: string; messages: string[]
  consumed: number; reserved: number; token_limit: number
}
function ProductionPlans() {
  const [desk, setDesk] = useState<{works:{slug:string;name:string;edition:string}[];plans:PlanInfo[]}>()
  const [issue,setIssue] = useState('')
  const [busy,setBusy] = useState(false)
  useEffect(()=>{
    let live=true
    async function refresh() {
      try { const data=await api<NonNullable<typeof desk>>('plans'); if(live)setDesk(data) }
      catch(e){if(live)setIssue(String(e))}
    }
    void refresh()
    const timer=setInterval(refresh,10000)
    return ()=>{live=false;clearInterval(timer)}
  },[])
  async function submit(e:FormEvent<HTMLFormElement>,plan?:PlanInfo){
    e.preventDefault()
    if(busy)return
    const form=e.currentTarget, data=new FormData(form), instruction=String(data.get('instruction')||'').trim()
    if(!instruction)return
    setBusy(true)
    try {
      await api(plan?'plans/'+plan.id+'/messages':'plans',plan?{revision:plan.revision,instruction}:{
        id:crypto.randomUUID(),world:data.get('world'),first:Number(data.get('first')),last:Number(data.get('last')),budget:Number(data.get('budget')),instruction
      })
      form.reset()
      setDesk(await api<NonNullable<typeof desk>>('plans'));setIssue('')
    }catch(err){setIssue(String(err))}finally{setBusy(false)}
  }
  return <section className="ed-decision"><h2>分集制作计划</h2>
    {issue&&<p role="alert">{issue}</p>}
    {desk&&<><form onSubmit={e=>void submit(e)}>
      <label>作品版本<select name="world" required>{desk.works.map(w=><option key={w.slug} value={w.slug}>{w.name} · {w.edition}</option>)}</select></label>
      <label>起始集<input name="first" type="number" min="1" required/></label>
      <label>截止集<input name="last" type="number" min="1" required/></label>
      <label>总 token 预算<input name="budget" type="number" min="1" required/></label>
      <label>制作要求<textarea name="instruction" required maxLength={1400} defaultValue="按缺口补集，五维验收通过后发布。"/></label>
      <button disabled={busy}>登记范围</button></form>
      {desk.plans.map(p=><article key={p.id}><h3>{desk.works.find(w=>w.slug===p.world)?.name} · 第 {p.first_episode}–{p.last_episode} 集</h3>
      <p>{({inventory:'等待盘点',blocked:'需要处理',planning:'等待规划',drafting_plan:'Pi 制定计划中',plan_validated:'范围校验通过',executing:'分集制作',published:'正式站已发布'} as Record<string,string>)[p.stage]||p.stage} · 第 {p.revision} 版</p>
      <p>缺集 {p.gaps.join('、')||'—'} · 已用 {p.consumed} · 在途 {p.reserved} · 预算 {p.token_limit}</p>
      {p.episodes.map(ep=><p key={ep.number}>{ep.title} · {ep.scene} · {ep.receipt?'已发布':ep.accepted?'五维通过':ep.research||'等待取证'} {ep.attempt&&' · 取证轮次 '+ep.attempt}  · 制作 {Math.round(ep.elapsed)} 秒 · 媒体失败记录 {ep.retries} 次 {ep.blocker||ep.error} {ep.receipt&&' · 发布回执 '+ep.receipt}</p>)}
      {p.calls.map((call,i)=><p key={i}>{call.stage} · {call.actual??"等待回执"} token · {call.seconds?.toFixed(1)??"—"} 秒</p>)}
      {p.plan_brief&&<p>{p.plan_brief}</p>}
      {p.paused_reason&&<p>{p.paused_reason}</p>}
      <ol>{p.messages.map((m,i)=><li key={i}>{m}</li>)}</ol>
      {p.stage!=='published'&&<form onSubmit={e=>void submit(e,p)}><label>修订意见<input required name="instruction" maxLength={1400}/></label><button disabled={busy}>提交修订</button></form>}
      </article>)}</>}
  </section>
}
