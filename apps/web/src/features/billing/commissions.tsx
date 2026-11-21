import { useCallback, useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Layout } from '../post/design'

type Account = {
  id: string
  customer_ref: string
  currency: string
  available_micros: number
  alipay_available: boolean
  held_micros: number
}
type Rate = {
  id: string
  currency: string
  cap_micros: number
  unit: string
  unit_micros: number
  input_micros: number
  output_micros: number
  cached_micros: number | null
}
type Quote = { automatic: boolean; rate: Rate }
type Order = {
  seq: number
  subject: string
  id: string
  status: string
  created_at: string
  cap_micros: number
  charge_micros: number | null
}
type Entry = { id: number; kind: string; amount_micros: number; at: string }
type Topup = {
  client_key: string
  id: string
  amount_fen: number
  status: string
  created_at: string
}
type Wallet = {
  topups: Topup[]
  account: Account
  quote: Quote | null
  orders: Order[]
  entries: Entry[]
}
const stateNames: Record<string, string> = {
  reserved: '已排队',
  running: '绘制中',
  delivered: '等待结算',
  settled: '已完成',
  void: '已取消',
  refunded: '已退款',
}
const money = (n: number, currency: string) =>
  new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 6,
  }).format(n / 1_000_000)
const date = (s: string) =>
  new Date(s).toLocaleString('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
const messages: Record<number, string> = {
  400: '请检查输入后再试。',
  401: '请使用账户访问码登录。',
  402: '可用余额不足，请刷新余额后再试。',
  403: '当前操作未获授权，请重新打开此页。',
  409: '报价或订单状态已有变化，请刷新后确认。',
  503: '服务暂时无法连接，请稍后重试。',
}
class RequestError extends Error {
  constructor(readonly status: number) {
    super(messages[status] ?? '请求未完成，请重试。')
  }
}
async function api<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch('/reader-api/' + path, {
    method: body === undefined ? 'GET' : 'POST',
    cache: 'no-store',
    ...(body === undefined
      ? {}
      : {
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        }),
  })
  if (!response.ok) throw new RequestError(response.status)
  return response.status === 204 ? (undefined as T) : response.json()
}
function price(rate: Rate) {
  const format = (n: number) => money(n, rate.currency)
  if (rate.unit === 'image') return `每张 ${format(rate.unit_micros)}`
  if (rate.unit === 'total_tokens')
    return `每百万 token ${format(rate.unit_micros)}`
  return (
    `每百万 token · 输入 ${format(rate.input_micros)} / 输出 ${format(rate.output_micros)}` +
    (rate.cached_micros == null
      ? ''
      : ` / 缓存输入 ${format(rate.cached_micros)}`)
  )
}

export function Commissions(){return <CommissionPage native={false} />}
export function NativeCommissions(){return <CommissionPage native />}
function CommissionPage({native}:{native:boolean}) {
  const [wallet, setWallet] = useState<Wallet | null>()
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [beforeOrder, setBeforeOrder] = useState<number>()
  const [beforeEntry, setBeforeEntry] = useState<number>()
  const version = useRef(0)
  const refresh = useCallback(async () => {
    const seq = ++version.current
    try {
      const account = await api<Account>('account')
      const [quote, orders, entries, topups] = await Promise.all([
        api<Quote>('quote').catch((e) => {
          if (e instanceof RequestError && e.status === 404) return null
          throw e
        }),
        api<Order[]>('orders' + (beforeOrder ? '?before=' + beforeOrder : '')),
        api<Entry[]>('entries' + (beforeEntry ? '?before=' + beforeEntry : '')),
        !native && account.currency === 'CNY' ? api<Topup[]>('alipay/topups') : [],
      ])
      if (version.current === seq)
        setWallet({ account, quote, orders, entries, topups })
    } catch (e) {
      if (version.current !== seq) return
      if (e instanceof RequestError && e.status === 401) {
        setWallet(null)
        setBeforeOrder(undefined)
        setBeforeEntry(undefined)
      } else setNotice('账户资料暂时无法读取，请重试。')
    }
  }, [beforeOrder, beforeEntry, native])
  useEffect(() => {
    void refresh()
    return () => {
      version.current++
    }
  }, [refresh])
  const active =
    wallet?.topups.some((t) => t.status === 'pending') ||
    wallet?.orders.some((o) =>
      ['reserved', 'running', 'delivered'].includes(o.status),
    )
  useEffect(() => {
    if (!active) return
    const timer = setInterval(() => {
      if (!document.hidden) void refresh()
    }, 5000)
    return () => clearInterval(timer)
  }, [active, refresh])
  const actionLock = useRef(false)
  async function act(fn: () => Promise<void>) {
    if (actionLock.current) return
    actionLock.current = true
    setBusy(true)
    setNotice('')
    try {
      await fn()
    } catch (e) {
      setNotice(
        e instanceof RequestError ? e.message : '连接中断，请查看订单后重试。',
      )
      if (e instanceof RequestError && e.status === 401) setWallet(null)
    } finally {
      await refresh()
      actionLock.current = false
      setBusy(false)
    }
  }
  async function login(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = e.currentTarget
    const token = String(new FormData(form).get('token') ?? '').trim()
    await act(async () => {
      await api('login', { token })
      form.reset()
    })
  }
  const content = (
      <div className="commissions">
        {!native && <a className="return" href="/shelf">
          ← 我的书架
        </a>}
        <header className="commission-title">
          <h1>我的约稿</h1>
          <p>把你想见到的角色与场景，画成一张私人插画。</p>
        </header>
        {notice && (
          <p role="status" className="commission-notice">
            {notice}
          </p>
        )}
        {wallet === undefined ? (
          <div>
            <p>正在读取账户…</p>
            {notice && <button onClick={refresh}>重试</button>}
          </div>
        ) : wallet === null ? (
          <form onSubmit={login} className="commission-login">
            <label>
              读者账户访问码
              <input
                required
                type="password"
                name="token"
                autoComplete="current-password"
                minLength={32}
                maxLength={512}
              />
            </label>
            <p>输入开通预付账户时收到的访问码，查看余额与约稿。</p>
            <button className="buy" disabled={busy}>
              进入我的约稿
            </button>
          </form>
        ) : (
          <>
            <section className="wallet-line">
              <div>
                <small>可用余额</small>
                <strong>
                  {money(
                    wallet.account.available_micros,
                    wallet.account.currency,
                  )}
                </strong>
                <small>
                  已预留{' '}
                  {money(wallet.account.held_micros, wallet.account.currency)}
                </small>
              </div>
              <button
                disabled={busy}
                onClick={() =>
                  act(async () => {
                    await api('logout', {})
                    version.current++
                    setWallet(null)
                    setBeforeOrder(undefined)
                    setBeforeEntry(undefined)
                  })
                }
              >
                退出账户
              </button>
            </section>
            {native && wallet.account.currency === 'CNY' && (
              <a className="buy" href={'folio-wallet://topup/' + wallet.account.id}>App Store 充值 · 到账 ¥20</a>
            )}
            {!native && wallet.account.currency === 'CNY' && (
              <Recharge wallet={wallet} busy={busy} act={act} />
            )}
            <CommissionForm
              key={wallet.account.id}
              wallet={wallet}
              busy={busy}
              act={act}
            />
            <section className="commission-orders">
              <header>
                <h2>我的订单</h2>
                <button onClick={refresh}>刷新</button>
              </header>
              {!wallet.orders.length && <p>你还没有约稿。</p>}
              {wallet.orders.map((o) => (
                <article key={o.id}>
                  <div>
                    <strong>{o.subject}</strong>
                    <small>{stateNames[o.status] ?? o.status}</small>
                    <small>
                      {date(o.created_at)} · 订单 {o.id.slice(0, 8)}
                    </small>
                  </div>
                  <div>
                    <span>
                      {o.status === 'refunded'
                        ? '已退回'
                        : o.charge_micros != null
                          ? '已扣'
                          : o.status === 'void'
                            ? '已释放'
                            : '预留'}{' '}
                      {money(
                        o.charge_micros ?? o.cap_micros,
                        wallet.account.currency,
                      )}
                    </span>
                    {o.status === 'settled' && (
                      <a
                        href={'/reader-api/orders/' + o.id + '/image'}
                        download
                      >
                        下载原图
                      </a>
                    )}
                    {o.status === 'reserved' && (
                      <button
                        disabled={busy}
                        onClick={() =>
                          act(async () => {
                            await api('orders/' + o.id + '/cancel', {})
                          })
                        }
                      >
                        取消约稿
                      </button>
                    )}
                  </div>
                </article>
              ))}
              <Pages
                older={
                  wallet.orders.length === 100
                    ? () => setBeforeOrder(wallet.orders.at(-1)!.seq)
                    : undefined
                }
                latest={
                  beforeOrder ? () => setBeforeOrder(undefined) : undefined
                }
              />
            </section>
            <details className="commission-entries">
              <summary>余额明细</summary>
              {!wallet.entries.length && <p>暂无流水。</p>}
              {wallet.entries.map((row) => (
                <p key={row.id}>
                  <span>
                    {date(row.at)} ·{' '}
                    {
                      (
                        {
                          credit: '入账',
                          store_credit: 'App Store 充值',
                          store_refund: 'App Store 充值退款',
                          charge: '约稿扣款',
                          refund: '退款',
                        } as Record<string, string>
                      )[row.kind]
                    }
                  </span>
                  <b>{money(row.amount_micros, wallet.account.currency)}</b>
                </p>
              ))}
              <Pages
                older={
                  wallet.entries.length === 100
                    ? () => setBeforeEntry(wallet.entries.at(-1)!.id)
                    : undefined
                }
                latest={
                  beforeEntry ? () => setBeforeEntry(undefined) : undefined
                }
              />
            </details>
          </>
        )}
      </div>
  )
  return native ? <main>{content}</main> : <Layout active="我的">{content}</Layout>
}

function Recharge({
  wallet,
  busy,
  act,
}: {
  wallet: Wallet
  busy: boolean
  act: (fn: () => Promise<void>) => Promise<void>
}) {
  const [amount, setAmount] = useState('')
  useEffect(() => {
    for (const t of wallet.topups) {
      const key = 'tidepost.topup.' + wallet.account.id + '.' + t.amount_fen
      if (
        t.status !== 'pending' &&
        sessionStorage.getItem(key) === t.client_key
      )
        sessionStorage.removeItem(key)
    }
  }, [wallet.account.id, wallet.topups])
  async function openCashier(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    await act(async () => {
      const [whole, frac = ''] = amount.split('.')
      const fen = Number(whole) * 100 + Number((frac + '00').slice(0, 2))
      if (!Number.isSafeInteger(fen) || fen <= 0 || fen > 1000000)
        throw new RequestError(400)
      const storageKey = 'tidepost.topup.' + wallet.account.id + '.' + fen
      let key = sessionStorage.getItem(storageKey)
      if (!key) {
        key = crypto.randomUUID()
        sessionStorage.setItem(storageKey, key)
      }
      let result: { checkout_url: string }
      try {
        result = await api('alipay/topups', {
          amount_fen: fen,
          client_key: key,
        })
      } catch (e) {
        if (e instanceof RequestError && e.status === 409)
          sessionStorage.removeItem(storageKey)
        throw e
      }
      const gateway = new URL(result.checkout_url)
      if (
        gateway.origin !== 'https://openapi.alipay.com' ||
        gateway.pathname !== '/gateway.do'
      )
        throw new RequestError(503)
      window.location.assign(gateway.href)
    })
  }
  return (
    <section className="commission-recharge">
      <h2>余额充值</h2>
      {wallet.account.alipay_available ? (
        <form onSubmit={openCashier}>
          <label>
            充值金额（元）
            <input
              inputMode="decimal"
              required
              pattern="[0-9]{1,5}([.][0-9]{1,2})?"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              placeholder="0.00"
            />
          </label>
          <button className="buy" disabled={busy}>
            前往支付宝付款
          </button>
        </form>
      ) : (
        <p>支付宝充值暂未开放。</p>
      )}
      {wallet.topups.length > 0 && (
        <details>
          <summary>近期充值记录</summary>
          {wallet.topups.map((t) => (
            <p key={t.id}>
              {date(t.created_at)} · {money(t.amount_fen * 10000, 'CNY')} ·{' '}
              {t.status === 'paid'
                ? '已到账'
                : t.status === 'pending'
                  ? '待支付确认'
                  : '已关闭'}
            </p>
          ))}
        </details>
      )}
    </section>
  )
}

function Pages({ older, latest }: { older?: () => void; latest?: () => void }) {
  return (
    <>
      {latest && <button onClick={latest}>查看最新记录</button>}{' '}
      {older && <button onClick={older}>查看更早记录</button>}
    </>
  )
}

type Draft = {
  size: string
  brief: {
    original_subject: string
    visual_bible: string
    scene: string
    composition: string
    art_direction: string
  }
}
type Intent = { rate: Rate; request: Draft; key: string }
function CommissionForm({
  wallet,
  busy,
  act,
}: {
  wallet: Wallet
  busy: boolean
  act: (fn: () => Promise<void>) => Promise<void>
}) {
  const [intent, setIntent] = useState<Intent | null>(null)
  const [sent, setSent] = useState('')
  const { quote, account } = wallet
  const form = useRef<HTMLFormElement>(null)
  const storageKey = 'tidepost.purchase.' + account.id
  function review(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!quote) return
    const fields = new FormData(e.currentTarget)
    const field = (name: string) => String(fields.get(name) ?? '').trim()
    const request = {
      size: field('size'),
      brief: {
        original_subject: field('subject'),
        visual_bible: field('appearance'),
        scene: field('scene'),
        composition: field('composition'),
        art_direction: field('style'),
      },
    }
    const next = { rate: quote.rate, request, key: crypto.randomUUID() }
    setIntent(next)
    setSent('')
  }
  async function pay() {
    if (!intent || !quote || quote.rate.id !== intent.rate.id) return
    await act(async () => {
      // Keep the idempotency key across a lost response and page reload, scoped to the buyer.
      const digest = await crypto.subtle.digest(
        'SHA-256',
        new TextEncoder().encode(
          JSON.stringify([intent.rate.id, intent.request]),
        ),
      )
      const fingerprint = Array.from(new Uint8Array(digest), (v) =>
        v.toString(16).padStart(2, '0'),
      ).join('')
      const retryKey = storageKey + '.' + fingerprint
      const key = sessionStorage.getItem(retryKey) || intent.key
      sessionStorage.setItem(retryKey, key)
      const order = await api<Order>('orders', {
        client_key: key,
        rate_id: intent.rate.id,
        request: intent.request,
      })
      sessionStorage.removeItem(retryKey)
      setIntent(null)
      setSent('订单 ' + order.id.slice(0, 8) + ' 已受理，可在下方查看进度。')
      form.current?.reset()
    })
  }
  const available =
    quote?.automatic && account.available_micros >= quote.rate.cap_micros
  return (
    <section className="commission-form">
      <h2>约一张插画</h2>
      {sent && <p role="status">{sent}</p>}
      {!quote ? (
        <p>试稿价格尚未发布。</p>
      ) : (
        <>
          <p>
            {price(quote.rate)} · 单张最高{' '}
            <strong>{money(quote.rate.cap_micros, account.currency)}</strong>
          </p>
          {!quote.automatic && <p>画室暂未接单，请稍后刷新。</p>}
          {account.available_micros < quote.rate.cap_micros && (
            <p>可用余额不足以预留本单上限。</p>
          )}
          <form ref={form} onSubmit={review} hidden={!!intent}>
            <fieldset disabled={!available || busy}>
              <label>
                角色
                <input
                  name="subject"
                  required
                  maxLength={500}
                  placeholder="你想画谁？"
                />
              </label>
              <label>
                外观特征
                <input
                  name="appearance"
                  required
                  maxLength={1000}
                  placeholder="发型、服装或随身物件"
                />
              </label>
              <label>
                场景
                <textarea
                  name="scene"
                  required
                  maxLength={1000}
                  placeholder="角色在哪里，正在做什么？"
                />
              </label>
              <label>
                画风
                <input
                  name="style"
                  required
                  maxLength={1000}
                  placeholder="例如，雨夜水彩"
                />
              </label>
              <label>
                构图
                <input
                  name="composition"
                  required
                  maxLength={1000}
                  defaultValue="人物居中，保留场景细节"
                />
              </label>
              <label>
                画幅
                <select name="size">
                  <option value="1024x1536">竖幅 · 1024 × 1536</option>
                  <option value="1024x1024">方形 · 1024 × 1024</option>
                  <option value="1536x1024">横幅 · 1536 × 1024</option>
                </select>
              </label>
              <button className="buy">核对约稿</button>
            </fieldset>
          </form>
          {intent && (
            <div className="commission-confirm">
              <h3>确认约稿</h3>
              <p>
                <b>{intent.request.brief.original_subject}</b> ·{' '}
                {intent.request.size}
              </p>
              <p>{Object.values(intent.request.brief).slice(1).join(' · ')}</p>
              <p>
                将预留 {money(intent.rate.cap_micros, account.currency)}
                ，交付后按实际用量扣款，多余预留自动释放。
              </p>
              {intent.rate.id !== quote.rate.id && (
                <p>报价已更新，请返回核对。</p>
              )}
              <button
                className="buy"
                disabled={
                  busy || !available || intent.rate.id !== quote.rate.id
                }
                onClick={pay}
              >
                确认预留 {money(intent.rate.cap_micros, account.currency)}
              </button>
              <button disabled={busy} onClick={() => setIntent(null)}>
                返回修改
              </button>
            </div>
          )}
        </>
      )}
    </section>
  )
}
