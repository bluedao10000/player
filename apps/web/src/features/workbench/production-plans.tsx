import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { api } from './editor-api'

type PlanInfo = {
  id: string; world: string; first_episode: number; last_episode: number; revision: number
  episodes: { number: number; title: string; scene: string; research?: string; attempt?: number; elapsed: number; retries: number; blocker?: string; error?: string; accepted?: boolean; receipt?: number; href?: string }[]
  configurations?: { id: string; number: number; finished_at?: string; error?: string; registered: boolean }[]
  calls: { stage: string; actual: number | null; seconds: number | null }[]; stage: string; plan_brief?: string; gaps: number[]; paused_reason?: string; messages: string[]
  consumed: number; reserved: number; token_limit: number
}

export function ProductionPlans({ initialPlan }: { initialPlan?: string }) {
  const [desk, setDesk] = useState<{ works: { slug: string; name: string; edition: string }[]; plans: PlanInfo[] }>()
  const [issue, setIssue] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    let live = true
    async function refresh() {
      try { const data = await api<NonNullable<typeof desk>>('plans'); if (live) setDesk(data) }
      catch (error) { if (live) setIssue(String(error)) }
    }
    void refresh()
    const timer = setInterval(refresh, 10000)
    return () => { live = false; clearInterval(timer) }
  }, [])
  useEffect(() => {
    if (!desk || !initialPlan) return
    document.getElementById('plan-' + initialPlan)?.scrollIntoView({ block: 'start' })
  }, [desk, initialPlan])

  async function submit(event: FormEvent<HTMLFormElement>, plan?: PlanInfo) {
    event.preventDefault()
    if (busy) return
    const form = event.currentTarget
    const data = new FormData(form)
    const instruction = String(data.get('instruction') || '').trim()
    if (!instruction) return
    setBusy(true)
    try {
      await api(plan ? 'plans/' + plan.id + '/messages' : 'plans', plan ? { revision: plan.revision, instruction } : {
        id: crypto.randomUUID(), world: data.get('world'), first: Number(data.get('first')), last: Number(data.get('last')),
        budget: Number(data.get('budget')), instruction,
      })
      form.reset()
      setDesk(await api<NonNullable<typeof desk>>('plans'))
      setIssue('')
    } catch (error) { setIssue(String(error)) }
    finally { setBusy(false) }
  }

  return <section className="ed-decision"><h2>分集制作计划</h2>
    {issue && <p role="alert">{issue}</p>}
    {desk && <><form onSubmit={(event) => void submit(event)}>
      <label>作品版本<select name="world" required>{desk.works.map((work) => <option key={work.slug} value={work.slug}>{work.name} · {work.edition}</option>)}</select></label>
      <label>起始集<input name="first" type="number" min="1" required /></label>
      <label>截止集<input name="last" type="number" min="1" required /></label>
      <label>总 token 预算<input name="budget" type="number" min="1" required /></label>
      <label>制作要求<textarea name="instruction" required maxLength={1400} defaultValue="按缺口补集，五维验收通过后发布。" /></label>
      <button disabled={busy}>登记范围</button></form>
      {desk.plans.map((plan) => <article id={'plan-' + plan.id} key={plan.id}>
        <h3>{desk.works.find((work) => work.slug === plan.world)?.name} · 第 {plan.first_episode}–{plan.last_episode} 集</h3>
        <p>{({ inventory: '等待盘点', blocked: '需要处理', planning: '等待规划', drafting_plan: 'Pi 制定计划中', plan_validated: '范围校验通过', executing: '分集制作', local_ready: '本地验收通过', published: '正式站已发布' } as Record<string, string>)[plan.stage] || plan.stage} · 第 {plan.revision} 版</p>
        <a href={`/workbench/assistant?world=${encodeURIComponent(plan.world)}&plan=${plan.id}`}>在对话中打开计划</a>
        <p>缺集 {plan.gaps.join('、') || '—'} · 已用 {plan.consumed} · 在途 {plan.reserved} · 预算 {plan.token_limit}</p>
        {plan.episodes.map((episode) => <p key={episode.number}>{episode.title} · {episode.scene} · {episode.receipt ? '已发布' : episode.accepted ? '五维通过' : episode.research || '等待取证'} {episode.attempt && ' · 取证轮次 ' + episode.attempt} · 制作 {Math.round(episode.elapsed)} 秒 · 媒体失败记录 {episode.retries} 次 {episode.blocker || episode.error} {episode.receipt && ' · 发布回执 ' + episode.receipt} {episode.href && <a href={episode.href}>打开章节</a>}</p>)}
        {plan.configurations?.map((configuration) => <p key={configuration.id}>第 {configuration.number} 集配置 · {configuration.registered ? '已校验登记' : configuration.error || '正在定位来源和生成配置'}</p>)}
        {plan.calls.map((call, index) => <p key={index}>{call.stage} · {call.actual ?? '等待回执'} token · {call.seconds?.toFixed(1) ?? '—'} 秒</p>)}
        {plan.plan_brief && <p>{plan.plan_brief}</p>}
        {plan.paused_reason && <p>{plan.paused_reason}</p>}
        <ol>{plan.messages.map((message, index) => <li key={index}>{message}</li>)}</ol>
        {plan.stage !== 'published' && <form onSubmit={(event) => void submit(event, plan)}><label>修订意见<input required name="instruction" maxLength={1400} /></label><button disabled={busy}>提交修订</button></form>}
      </article>)}</>}
  </section>
}
