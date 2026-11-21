import { Badge } from '@astryxdesign/core/Badge'
import { Button } from '@astryxdesign/core/Button'
import { Card } from '@astryxdesign/core/Card'
import { mediaAssetPattern } from '@anime/shared/media'
import { ImageOff, RotateCw, type LucideIcon } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { runStatusLabels } from './format'

export function Panel({
  children,
  className = '',
  id,
  labelledby,
}: {
  children: ReactNode
  className?: string
  id?: string
  labelledby?: string
}) {
  return (
    <Card
      id={id}
      aria-labelledby={labelledby}
      padding={0}
      className={`wb-panel ${className}`}
    >
      {children}
    </Card>
  )
}

export function SectionHeading({
  eyebrow,
  title,
  detail,
  actions,
  id,
}: {
  eyebrow?: string
  title: string
  detail?: string
  actions?: ReactNode
  id?: string
}) {
  return (
    <div className="wb-section-heading">
      <div>
        {eyebrow && <span className="wb-eyebrow">{eyebrow}</span>}
        <h2 id={id}>{title}</h2>
        {detail && <p>{detail}</p>}
      </div>
      {actions && <div className="wb-section-actions">{actions}</div>}
    </div>
  )
}

const badgeVariants: Record<string, 'neutral' | 'info' | 'success' | 'warning' | 'error' | 'purple'> = {
  pending: 'neutral',
  running: 'info',
  success: 'success',
  passed: 'success',
  failed: 'error',
  held: 'warning',
  queued: 'purple',
  reference: 'purple',
  generate: 'info',
  review: 'purple',
  recrop: 'warning',
  recheck: 'info',
}

export function StatusBadge({ status, label }: { status: string; label?: string }) {
  return (
    <Badge
      className="wb-status-badge"
      variant={badgeVariants[status] ?? 'neutral'}
      icon={<i className="wb-status-dot" data-status={status} />}
      label={label ?? runStatusLabels[status] ?? status}
    />
  )
}

export function EmptyState({
  icon: Icon,
  title,
  detail,
}: {
  icon: LucideIcon
  title: string
  detail?: string
}) {
  return (
    <div className="wb-empty" role="status">
      <Icon size={22} />
      <strong>{title}</strong>
      {detail && <p>{detail}</p>}
    </div>
  )
}

export function workbenchAssetUrl(name: string, retry = 0) {
  return `/workbench-art/${encodeURIComponent(name)}${retry ? `?retry=${retry}` : ''}`
}

export function ImageAsset({
  name,
  alt,
  className = '',
  eager = false,
}: {
  name: string
  alt: string
  className?: string
  eager?: boolean
}) {
  const [retry, setRetry] = useState(0)
  const [loaded, setLoaded] = useState(false)
  const [failed, setFailed] = useState(false)
  const valid = mediaAssetPattern.test(name)

  useEffect(() => {
    setRetry(0)
    setLoaded(false)
    setFailed(false)
  }, [name])

  const state = !valid || failed ? 'failed' : loaded ? 'loaded' : 'loading'
  return (
    <div className={`wb-image ${className}`} data-state={state}>
      {state === 'loading' && <span className="wb-image-loading" aria-label="图片加载中" />}
      {state === 'failed' ? (
        <div className="wb-image-error" role="status">
          <ImageOff size={22} />
          <span>{valid ? '图片未能加载' : '图片文件名不符合资产规则'}</span>
          {valid && (
            <Button
              className="wb-button"
              size="sm"
              variant="ghost"
              label="重试"
              icon={<RotateCw size={14} />}
              onClick={() => {
                setLoaded(false)
                setFailed(false)
                setRetry((value) => value + 1)
              }}
            />
          )}
        </div>
      ) : (
        <img
          key={retry}
          src={workbenchAssetUrl(name, retry)}
          alt={alt}
          loading={eager ? 'eager' : 'lazy'}
          onLoad={() => setLoaded(true)}
          onError={() => {
            setLoaded(true)
            setFailed(true)
          }}
        />
      )}
    </div>
  )
}
