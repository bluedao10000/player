import { useState, useEffect } from 'react'
import { request } from './account'

export function WechatButton({
  readerId,
  returnTo,
}: {
  readerId?: string
  returnTo: string
}) {
  const [enabled, setEnabled] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    void request('methods')
      .then((v) => {
        if (active) setEnabled(v.wechat === true)
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [])
  if (!enabled) return null
  return (
    <div className="wechat-action">
      <button
        type="button"
        disabled={busy}
        onClick={async () => {
          setError('')
          setBusy(true)
          try {
            const res = await request(
              readerId ? 'wechat/link' : 'wechat/start',
              { returnTo },
              readerId,
            )
            location.assign(res.url)
          } catch (e) {
            setError(e instanceof Error ? e.message : '微信连接失败')
            setBusy(false)
          }
        }}
      >
        {busy ? '正在打开微信…' : readerId ? '绑定微信' : '微信扫码登录'}
      </button>
      {!readerId && <p>已有邮箱账户？先用邮箱登录，再到「我的」绑定微信。</p>}
      {error && <p role="alert">{error}</p>}
    </div>
  )
}

export function LinkEmail({ readerId }: { readerId: string }) {
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [sent, setSent] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  return (
    <details className="reader-email-link">
      <summary>绑定邮箱</summary>
      <form
        onSubmit={async (e) => {
          e.preventDefault()
          setBusy(true)
          setError('')
          try {
            if (sent) {
              await request('email/link', { email, code }, readerId)
              location.reload()
            } else {
              await request('code', { email })
              setSent(true)
            }
          } catch (e) {
            setError(e instanceof Error ? e.message : '绑定失败')
          } finally {
            setBusy(false)
          }
        }}
      >
        <label>
          电子邮箱
          <input
            required
            type="email"
            autoComplete="email"
            value={email}
            disabled={sent}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        {sent && (
          <label>
            邮箱验证码
            <input
              required
              value={code}
              autoComplete="one-time-code"
              inputMode="numeric"
              pattern="[0-9]{6}"
              maxLength={6}
              onChange={(e) => setCode(e.target.value)}
            />
          </label>
        )}
        <button disabled={busy}>{sent ? '确认绑定' : '获取验证码'}</button>
        {sent && (
          <button
            type="button"
            disabled={busy}
            onClick={() => {
              setSent(false)
              setCode('')
            }}
          >
            更换邮箱或重新发码
          </button>
        )}
        {error && <p role="alert">{error}</p>}
      </form>
    </details>
  )
}
