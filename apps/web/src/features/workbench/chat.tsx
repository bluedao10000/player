import { Button } from '@astryxdesign/core/Button'
import {
  AssistantRuntimeProvider,
  ComposerPrimitive,
  MessagePrimitive,
  ThreadPrimitive,
  useExternalStoreRuntime,
  type AppendMessage,
  type ThreadMessageLike,
} from '@assistant-ui/react'
import { useCallback, useEffect, useState } from 'react'

type Turn = {
  id: string
  world: string
  plan_id: string | null
  task_id?: string | null
  episode: number
  prompt: string
  reply: string | null
  state: 'running' | 'answered' | 'failed' | 'pending' | 'applied' | 'rejected'
  tools: string[]
  diff: Record<string, [unknown, unknown]> | null
}
type Chat = {
  choices: {
    works: { slug: string; name: string; edition: string }[]
    tasks: { id: string; world: string; chapter: string; status: string }[]
    plans: { id: string; world: string; first_episode: number; last_episode: number }[]
  }
  turns: Turn[]
}
type Request = <T>(path: string, body?: unknown) => Promise<T>
const labels: Record<string, string> = {
  world: '作品版本', first_episode: '起始集', last_episode: '截止集', token_limit: '总预算',
  production_policy: '制作策略', operator_paused: '运营暂停', episode: '单集执行', instruction: '计划说明',
}
const states: Record<Turn['state'], string> = {
  running: 'Pi 处理中', answered: '已查询', failed: '失败',
  pending: '等待确认', applied: '已执行', rejected: '已取消',
}

function TextMessage() {
  return <MessagePrimitive.Root className="wb-chat-message"><MessagePrimitive.Parts /></MessagePrimitive.Root>
}

export function WorkbenchChat({ request, initialWorld, initialPlan, initialTask, initialEpisode, initialPrompt }: {
  request: Request
  initialWorld?: string
  initialPlan?: string
  initialTask?: string
  initialEpisode?: number
  initialPrompt?: string
}) {
  const [chat, setChat] = useState<Chat>()
  const [world, setWorld] = useState(initialWorld)
  const [comparison, setComparison] = useState('')
  const [task, setTask] = useState(initialTask)
  const [plan, setPlan] = useState(initialPlan)
  const [episode, setEpisode] = useState<number>(initialEpisode ?? 0)
  const [suggestedPrompt, setSuggestedPrompt] = useState(initialPrompt ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const refresh = useCallback(async () => setChat(await request<Chat>('chat')), [request])
  useEffect(() => {
    void refresh().catch((err: Error) => setError(err.message))
  }, [refresh])
  const pendingTurn = chat?.turns.some(turn => turn.state === 'running') ?? false
  useEffect(() => {
    if (!pendingTurn) return
    const timer = setInterval(() => { void refresh().catch((err: Error) => setError(err.message)) }, 2500)
    return () => clearInterval(timer)
  }, [pendingTurn, refresh])
  const last = chat?.turns.at(-1)
  const works = chat?.choices.works ?? []
  const currentWorld = world ?? last?.world ?? works[0]?.slug ?? ''
  const plans = chat?.choices.plans.filter(p => p.world === currentWorld) ?? []
  const currentTask = task ?? (last?.world === currentWorld ? last.task_id : undefined) ?? ''
  const currentPlan = plan ?? (last?.world === currentWorld ? last.plan_id : undefined) ?? plans[0]?.id ?? ''

  const sendText = async (text: string) => {
    const trimmed = text.trim()
    if (!trimmed || busy || !currentWorld) return
    setBusy(true)
    setError('')
    try {
      const completion = request('chat', {
        request_id: crypto.randomUUID(),
        world: currentWorld,
        plan_id: currentPlan || null,
        task_id: currentTask || null,
        episode,
        text: trimmed,
        selected_works: comparison ? [{ work: comparison }] : [],
      })
      setComparison('')
      const timer = setInterval(() => { void refresh().catch(() => {}) }, 2500)
      try { await completion } finally { clearInterval(timer) }
      setSuggestedPrompt('')
    } catch (err) {
      setError(String(err))
    } finally {
      await refresh().catch((err: Error) => setError(err.message))
      setBusy(false)
    }
  }

  const send = async (message: AppendMessage) => {
    const text = message.content.flatMap(part => part.type === 'text' ? [part.text] : []).join('').trim()
    await sendText(text)
  }

  async function resolve(id: string, confirm: boolean) {
    setBusy(true)
    setError('')
    try {
      await request('chat/' + id + '/resolve', { confirm })
      await refresh()
    } catch (err) {
      setError(String(err))
    } finally {
      setBusy(false)
    }
  }

  const messages: ThreadMessageLike[] = (chat?.turns ?? []).flatMap(turn => {
    const rows: ThreadMessageLike[] = [{ id: turn.id + '-u', role: 'user', content: turn.prompt }]
    if (turn.reply) rows.push({ id: turn.id + '-a', role: 'assistant', content: turn.reply })
    return rows
  })
  const runtime = useExternalStoreRuntime({
    messages,
    convertMessage: (message) => message,
    onNew: send,
    isRunning: busy || pendingTurn,
    isDisabled: !chat || !currentWorld,
    unstable_persistsHistory: true,
  })

  return (
    <div className="wb-chat">
      <div className="wb-panel wb-chat-context">
        <label>作品<select value={currentWorld} onChange={e => { setWorld(e.target.value); setComparison(''); setPlan(undefined); setTask(''); setEpisode(0) }}>
          {works.map(w => <option value={w.slug} key={w.slug}>{w.name} · {w.edition}</option>)}
        </select></label>
        <label>本轮对照作品<select value={comparison} disabled={busy} onChange={e => setComparison(e.target.value)}>
          <option value="">不另选作品</option>
          {works.filter(w => w.slug !== currentWorld).map(w => <option value={w.slug} key={w.slug}>{w.name} · {w.edition}</option>)}
        </select></label>
        <label>计划<select value={currentPlan} onChange={e => setPlan(e.target.value)}>
          <option value="">未选计划</option>
          {plans.map(p => <option value={p.id} key={p.id}>第 {p.first_episode}–{p.last_episode} 集 · {p.id.slice(0, 8)}</option>)}
        </select></label>
        <label>审稿任务<select value={currentTask} onChange={e => setTask(e.target.value)}>
          <option value="">未选任务</option>
          {chat?.choices.tasks.filter(t => t.world === currentWorld).map(t => <option key={t.id} value={t.id}>{t.chapter} · {t.status} · {t.id.slice(0, 8)}</option>)}
        </select></label>
        <label>分集<input type="number" value={episode || ''} min={1} max={100000} placeholder="全部" onChange={e => setEpisode(e.target.valueAsNumber || 0)} /></label>
      </div>
      {error && <p role="alert" className="ed-error">{error}</p>}
      {suggestedPrompt && currentWorld && <section className="wb-panel wb-chat-prefill" aria-label="带入的知识缺口评估问题">
        <div><strong>已从知识资产页带入评估上下文</strong><p>{suggestedPrompt}</p></div>
        <div className="wb-chat-actions">
          <Button label="发送评估请求" size="sm" isDisabled={busy} onClick={() => void sendText(suggestedPrompt)} />
          <button type="button" disabled={busy} onClick={() => setSuggestedPrompt('')}>忽略</button>
        </div>
      </section>}
      <div className="wb-chat-layout">
        <AssistantRuntimeProvider runtime={runtime}>
          <ThreadPrimitive.Root className="wb-panel wb-chat-thread">
            <ThreadPrimitive.Viewport className="wb-chat-scroll">
              <ThreadPrimitive.Empty><p>可以问「评估这部作品前 10 话的知识覆盖，解释主要缺口并给出补全顺序与预算依据」</p></ThreadPrimitive.Empty>
              <ThreadPrimitive.Messages components={{ UserMessage: TextMessage, AssistantMessage: TextMessage }} />
            </ThreadPrimitive.Viewport>
            {busy && <p role="status">正在读取工作台数据或处理操作…</p>}
            <ComposerPrimitive.Root className="wb-chat-compose">
              <ComposerPrimitive.Input aria-label="给工作台发消息" placeholder="询问覆盖缺口、预算依据或描述计划修改…" maxLength={1800} />
              <ComposerPrimitive.Send asChild><button type="submit">发送</button></ComposerPrimitive.Send>
            </ComposerPrimitive.Root>
          </ThreadPrimitive.Root>
        </AssistantRuntimeProvider>
        <section className="wb-panel wb-chat-audit" aria-label="操作历史">
          <h2>工具与变更</h2>
          {[...(chat?.turns ?? [])].reverse().map(turn => (
            <article key={turn.id}>
              <strong>{states[turn.state]} · {turn.prompt}</strong>
              {!!turn.tools.length && <details><summary>工具记录</summary><ul>{turn.tools.map(tool => <li key={tool}>{tool}</li>)}</ul></details>}
              {turn.diff && <dl>{Object.entries(turn.diff).map(([key, [before, after]]) => (
                <div key={key}><dt>{labels[key] ?? key}</dt><dd>{before !== null && before !== undefined && <del>{JSON.stringify(before)}</del>}<ins>{JSON.stringify(after)}</ins></dd></div>
              ))}</dl>}
              {turn.state === 'pending' && <div className="wb-chat-actions">
                <Button label="确认变更" size="sm" isDisabled={busy} onClick={() => void resolve(turn.id, true)} />
                <button type="button" disabled={busy} onClick={() => void resolve(turn.id, false)}>取消</button>
              </div>}
              {turn.state === 'applied' && turn.plan_id && <p className="wb-chat-receipt">
                <a href={`/workbench/projects?plan=${encodeURIComponent(turn.plan_id)}`}>查看计划与执行回执 →</a>
                {' · '}
                <a href={`/workbench/knowledge/${encodeURIComponent(turn.world)}`}>返回作品知识详情 →</a>
              </p>}
            </article>
          ))}
        </section>
      </div>
    </div>
  )
}
