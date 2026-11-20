import { useEffect, useState } from 'react'
import { WechatButton } from './wechat'
import { Layout } from '../post/design'
import { guestPositions } from '../post/progress'
import { identityChanged, refresh, request, useAccount } from './account'
import { readBookmarks } from './bookmarks'
import './guest.css'

export function returnPath(raw: unknown) {
  if (
    typeof raw !== 'string' ||
    !raw.startsWith('/') ||
    /[\\\x00-\x1f]/.test(raw)
  )
    return '/me'
  try {
    const url = new URL(raw, 'https://local.invalid')
    if (
      url.origin === 'https://local.invalid' &&
      /^\/(?:$|(?:me|shelf|works|stories|characters|commissions)$|(?:story|work|character|scene|world)\/[a-z0-9/-]+$)/.test(
        url.pathname,
      )
    )
      return url.pathname + url.search + url.hash
  } catch {}
  return '/me'
}

type Challenge = { email: string; expires: number; resend: number }
const challengeKey = 'folio.pending-email'
function remember(c: Challenge | null) {
  try {
    if (c) sessionStorage.setItem(challengeKey, JSON.stringify(c))
    else sessionStorage.removeItem(challengeKey)
  } catch {}
}

export function ReaderLogin({
  returnTo,
  chapters,
  wechat,
}: {
  returnTo: string
  wechat?: string
  chapters: { href: string; work: string; title: string; image: string }[]
}) {
  const reader = useAccount()
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [challenge, setChallenge] = useState<Challenge | null>(null)
  const [time, setTime] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const chapter = chapters.find((c) => c.href === returnTo.split(/[?#]/)[0])
  const pending = reader && reader !== 'error' && reader.pending
  useEffect(() => {
    try {
      const c: Challenge = JSON.parse(
        sessionStorage.getItem(challengeKey) || 'null',
      )
      if (c?.expires > Date.now() && typeof c.email === 'string') {
        setChallenge(c)
        setEmail(c.email)
      }
    } catch {}
    setTime(Date.now())
  }, [])
  useEffect(() => {
    if (!challenge) return
    const interval = setInterval(() => setTime(Date.now()), 1000)
    return () => clearInterval(interval)
  }, [challenge])
  useEffect(() => {
    if (!wechat && reader && reader !== 'error' && !reader.pending)
      location.replace(returnTo)
  }, [reader, returnTo, wechat])
  async function run(fn: () => Promise<void>) {
    setBusy(true)
    setError('')
    try {
      await fn()
    } catch (err) {
      setError(err instanceof Error ? err.message : '操作失败，请重试。')
    } finally {
      setBusy(false)
    }
  }
  async function send() {
    await request('code', { email })
    const now = Date.now()
    const c = {
      email: email.trim(),
      expires: now + 300000,
      resend: now + 60000,
    }
    setChallenge(c)
    remember(c)
    setCode('')
    setTime(now)
  }
  const localBookmarks = pending ? readBookmarks() : []
  const positions = pending ? guestPositions(chapters) : []
  async function decide(choice: 'merge' | 'skip') {
    if (!reader || reader === 'error') return
    const data =
      choice === 'merge'
        ? {
            choice,
            bookmarks: localBookmarks.map(({ order, ...b }) => ({
              ...b,
              recordedAt: order,
            })),
            positions,
          }
        : { choice }
    await request('merge', data, reader.id)
    await identityChanged()
  }
  const wait = challenge
    ? Math.max(0, Math.ceil((challenge.resend - time) / 1000))
    : 0
  const expired = !!challenge && time >= challenge.expires
  return (
    <Layout active="我的">
      <section className="reader-login">
        <figure>
          <img src={chapter?.image || '/art/night-post.webp'} alt="" />
          <figcaption>{chapter?.work || '潮汐邮局'}</figcaption>
        </figure>
        <div className="reader-form">
          <a className="return" href={returnTo}>
            ← 返回{chapter ? '章节' : '阅读'}
          </a>
          <h1>{pending ? '带上本机的阅读记录' : '读者登录'}</h1>
          {wechat && (
            <p role="alert">
              {{
                conflict: '该微信已绑定其他读者账户。',
                cancelled: '已取消微信授权。',
                expired: '授权已过期，请重新登录。',
              }[wechat] || '微信授权未完成，请重新登录。'}
            </p>
          )}
          {reader === null && !challenge && (
            <WechatButton returnTo={returnTo} />
          )}
          {reader === undefined && <p role="status">正在读取会话…</p>}
          {reader === 'error' && (
            <button onClick={() => void refresh()}>重新连接</button>
          )}
          {reader === null && (
            <form
              onSubmit={(e) => {
                e.preventDefault()
                void run(async () => {
                  if (!challenge) {
                    await send()
                    return
                  }
                  await request('verify', { email: challenge.email, code })
                  remember(null)
                  setCode('')
                  if (wechat)
                    location.replace(
                      '/login?returnTo=' + encodeURIComponent(returnTo),
                    )
                  else await identityChanged()
                })
              }}
            >
              {challenge ? (
                <>
                  <p>已向 {challenge.email} 发送六位验证码，5 分钟内有效。</p>
                  <label>
                    邮箱验证码
                    <input
                      autoFocus
                      autoComplete="one-time-code"
                      inputMode="numeric"
                      pattern="[0-9]{6}"
                      maxLength={6}
                      required
                      value={code}
                      onChange={(e) =>
                        setCode(e.target.value.replace(/\D/g, ''))
                      }
                    />
                  </label>
                  {expired && <p role="status">验证码已过期，请重新发送。</p>}
                  <button
                    className="reader-primary"
                    disabled={busy || expired || code.length !== 6}
                  >
                    确认登录
                  </button>
                  <div className="reader-actions">
                    <button
                      type="button"
                      disabled={busy || wait > 0}
                      onClick={() => void run(send)}
                    >
                      {wait ? wait + ' 秒后可重发' : '重新发送'}
                    </button>
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => {
                        remember(null)
                        setChallenge(null)
                        setCode('')
                        setError('')
                      }}
                    >
                      更换邮箱
                    </button>
                  </div>
                </>
              ) : (
                <>
                  <p>让书架和阅读位置跟你去下一台设备。</p>
                  <label>
                    电子邮箱
                    <input
                      type="email"
                      autoComplete="email"
                      required
                      maxLength={254}
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="name@example.com"
                    />
                  </label>
                  <button className="reader-primary" disabled={busy}>
                    {busy ? '正在发送…' : '获取验证码'}
                  </button>
                </>
              )}
            </form>
          )}
          {pending && (
            <>
              <p>
                本机有 {localBookmarks.length} 篇收藏、{positions.length}{' '}
                篇阅读进度。合并后可用 {reader.email || '微信'}{' '}
                在其他设备接着读。
              </p>
              <button
                className="reader-primary"
                disabled={busy}
                onClick={() => void run(() => decide('merge'))}
              >
                合并到我的账户
              </button>
              <button
                disabled={busy}
                onClick={() => void run(() => decide('skip'))}
              >
                跳过，仅保留在本机
              </button>
            </>
          )}
          {error && <p role="alert">{error}</p>}
        </div>
      </section>
    </Layout>
  )
}
