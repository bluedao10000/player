import { useEffect, useState } from 'react'
import { request } from './account'

type PlanId = 'free' | 'plus' | 'pro'
type Plan = { id: PlanId; fen: number; credits: number }
type Order = {
  id: string
  plan: PlanId
  amount_fen: number
  status: 'paid' | 'closed' | 'pending' | 'refunded'
}
type Summary = {
  enabled: boolean
  plan: PlanId
  periodEnd: string
  credits: number
  usedMicro: number
  plans: Plan[]
  orders: Order[]
}
const names = { free: 'Free', plus: 'Plus', pro: 'Pro' }
const states = {
  paid: '已付款',
  closed: '已关闭',
  pending: '待付款',
  refunded: '已退款',
}
const amount = (fen: number) =>
  '¥' + (fen / 100).toLocaleString('zh-CN', { maximumFractionDigits: 2 })

export function Membership({ readerId }: { readerId: string }) {
  const [data, setData] = useState<Summary>()
  const [issue, setIssue] = useState('')
  const [selected, setSelected] = useState<Plan>()
  const [busy, setBusy] = useState(false)
  const [retry, setRetry] = useState(0)
  useEffect(() => {
    let active = true
    let timer: ReturnType<typeof setTimeout> | undefined
    let checks = 0
    async function load() {
      try {
        const next: Summary = await request('membership')
        if (!active) return
        setData(next)
        setIssue('')
        if (next.orders.some((o) => o.status === 'pending') && checks++ < 12)
          timer = setTimeout(load, 5000)
      } catch {
        if (active) setIssue('会员状态暂时无法读取。')
      }
    }
    void load()
    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [readerId, retry])
  async function pay() {
    if (!selected || busy) return
    setBusy(true)
    setIssue('')
    try {
      const result = await request(
        'membership/checkout',
        { plan: selected.id, expectedFen: selected.fen },
        readerId,
      )
      const url = new URL(result.checkout_url)
      if (
        url.origin !== 'https://openapi.alipay.com' ||
        url.pathname !== '/gateway.do'
      )
        throw new Error('未能打开支付宝收银台。')
      location.assign(url.href)
    } catch (e) {
      setIssue(e instanceof Error ? e.message : '暂时无法创建订单。')
      setBusy(false)
    }
  }
  return (
    <section className="reader-membership" aria-label="会员权益">
      {data && (
        <>
          <strong>{data.plan === 'free' ? 'Free' : names[data.plan] + ' 会员'}</strong>
          <p>
            本期可用{' '}
            {Math.floor(
              Math.max(0, data.credits - data.usedMicro / 1000000),
            ).toLocaleString('zh-CN')}{' '}
            / {data.credits} 积分 · 至{' '}
            {new Date(data.periodEnd).toLocaleDateString('zh-CN', {
              timeZone: 'Asia/Shanghai',
            })}
          </p>
          <details>
            <summary>
              {data.plan === 'free' ? '查看与购买套餐' : '查看会员权益'}
            </summary>
            <ul className="reader-plans">
              {data.plans.map((p) => (
                <li key={p.id}>
                  <span>
                    <b>{names[p.id]}</b> · {p.credits} 积分
                    <br />
                    <small>
                      {p.id === 'free'
                        ? '免费 · 每自然月'
                        : amount(p.fen) + ' / 30 天'}
                    </small>
                  </span>
                  {p.id !== 'free' && data.plan === 'free' && data.enabled && (
                    <button
                      className="guest-remove"
                      type="button"
                      onClick={() => {
                        setSelected(p)
                        setIssue('')
                      }}
                    >
                      选择 {names[p.id]}
                    </button>
                  )}
                </li>
              ))}
            </ul>
            {data.plan === 'free' && !data.enabled && (
              <p>支付宝收款暂未开放。</p>
            )}
            {selected && data.plan === 'free' && (
              <div>
                <p>
                  {names[selected.id]} · {selected.credits} 积分，有效期 30
                  天。单次付款，到期不自动续费。
                </p>
                <button
                  className="button"
                  disabled={busy}
                  type="button"
                  onClick={() => void pay()}
                >
                  确认并去支付宝支付 {amount(selected.fen)}
                </button>
                <button
                  className="guest-remove"
                  disabled={busy}
                  onClick={() => setSelected(undefined)}
                >
                  取消
                </button>
              </div>
            )}
            <small>读者编号 {readerId}</small>
          </details>
          {data.orders.length > 0 && (
            <details open={new URLSearchParams(location.search).has('payment')}>
              <summary>会员订单 · {states[data.orders[0].status]}</summary>
              <ul>
                {data.orders.map((o) => (
                  <li key={o.id}>
                    {names[o.plan]} · {amount(o.amount_fen)} ·{' '}
                    {states[o.status]}
                  </li>
                ))}
              </ul>
              <button
                className="guest-remove"
                onClick={() => setRetry((r) => r + 1)}
              >
                刷新到账状态
              </button>
            </details>
          )}
        </>
      )}
      {!data && !issue && <p role="status">正在读取会员权益…</p>}
      {issue && (
        <p role="alert">
          {issue}{' '}
          <button
            className="guest-remove"
            onClick={() => setRetry((r) => r + 1)}
          >
            刷新
          </button>
        </p>
      )}
    </section>
  )
}
