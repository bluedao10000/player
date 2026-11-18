import { useEffect, useRef, useState } from 'react'
import type {
  MediaCandidate,
  MediaStage,
  MediaStep,
} from '@anime/shared/knowledge'

const labels: Record<MediaStage, string> = {
  reference: '准备角色参考图',
  queued: '待生成',
  generate: '生成原图',
  review: '自动验图',
  recrop: '调整裁切',
  recheck: '复验裁切',
  passed: '验图通过',
  held: '需重新绘制',
  failed: '任务失败',
}
const checks = {
  identity: '人物一致',
  scene: '场景与动作',
  anatomy: '五官与手部',
  clean: '画面干净',
  crop: '手机构图',
}
const file = (name: string) => '/workbench-art/' + name
// Each embedded viewport uses the live character component at the acceptance
// device size. The saved crop is the no-JavaScript fallback and review input.
function Pair({
  image,
  phone,
  scene,
  focus = 40,
  kind,
}: {
  image: string
  phone: string
  scene: string
  kind?: 'portrait' | 'scene'
  focus?: number
}) {
  const portrait = kind === 'portrait' || image.startsWith('bebop-portrait-')
  const isScene = kind === 'scene' || (!portrait && image.startsWith('bebop-'))
  const frame = useRef<HTMLDivElement>(null)
  const [scale, setScale] = useState(0)
  useEffect(() => {
    const el = frame.current
    if (!el || portrait) return
    const observer = new ResizeObserver(([entry]) =>
      setScale(entry.contentRect.width / (isScene ? 358 : 390)),
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [isScene, portrait])
  return (
    <div className="wb-art-pair">
      <figure>
        <a href={file(image)} target="_blank" rel="noreferrer">
          <img src={file(image)} alt={scene + ' · 原图'} loading="lazy" />
        </a>
        <figcaption>候选原图</figcaption>
      </figure>
      <figure>
        <div
          ref={frame}
          className="wb-phone"
          style={
            portrait
              ? { aspectRatio: '358/430' }
              : isScene
                ? { aspectRatio: '358/225' }
                : undefined
          }
        >
          <img src={file(phone)} alt={scene + ' · 手机裁切'} loading="lazy" />
          {!kind && !portrait && scale > 0 && (
            <iframe
              title={scene + ' · H5 角色首屏'}
              loading="lazy"
              tabIndex={-1}
              src={'/workbench-hero/' + image + '?focus=' + focus}
              style={{
                transform:
                  'scale(' +
                  scale +
                  ')' +
                  (isScene ? ' translateX(-16px)' : ''),
              }}
            />
          )}
        </div>
        <figcaption>
          {portrait ? '角色大图与圆形头像' : 'H5 首屏'} ·{' '}
          <a href={file(phone)} target="_blank" rel="noreferrer">
            验图样张
          </a>
        </figcaption>
      </figure>
    </div>
  )
}
function Audit({ step }: { step: MediaStep }) {
  const r = step.result
  return (
    <li>
      <time>{new Date(step.started_at).toISOString().slice(11, 19)}</time>　v
      {step.round} · {labels[step.step]}
      {step.attempt > 1 && ' · 技术重试'}
      {step.finished_at === null && ' · 进行中'}
      {r?.decision && (
        <b>
          {' '}
          ·{' '}
          {{ pass: '通过', redraw: '退回重绘', recrop: '退回重裁' }[r.decision]}
        </b>
      )}
      {r?.checks && (
        <p>
          {(Object.keys(checks) as (keyof typeof checks)[])
            .map((k) => (r.checks![k] ? '✓ ' : '× ') + checks[k])
            .join('　')}
        </p>
      )}
      {r?.firstOpinion && <p>初检疑点 · {r.firstOpinion}</p>}
      {r?.secondModel && <small>独立复核 · {r.secondModel}</small>}
      {r?.reason && <p>{r.reason}</p>}
      {r?.source && <a href={r.source}>人物参考图</a>}
      {step.error && <p>{step.error}</p>}
      {r?.model && <small>{r.model}</small>}
    </li>
  )
}
function Candidate({ item }: { item: MediaCandidate }) {
  const images = item.steps.filter((s) => s.result?.image)
  const last = images.at(-1)
  const revised = item.steps.find(
    (s) => s.round === last?.round && s.step === 'recrop' && s.result?.phone,
  )
  return (
    <article className="wb-art-candidate">
      <header>
        <h3>
          {item.batch === 'bebop-portrait'
            ? item.scene
            : '候选 ' + item.variant}
        </h3>
        <span data-stage={item.stage}>
          {labels[item.stage]} · v{item.round}
        </span>
      </header>
      {last?.result?.image && last.result.phone && (
        <Pair
          image={last.result.image}
          phone={revised?.result?.phone ?? last.result.phone}
          focus={revised?.result?.focus ?? 40}
          kind={item.art?.kind}
          scene={item.scene}
        />
      )}
      {item.batch === 'bebop-portrait' &&
        item.stage === 'passed' &&
        item.reader_href && <a href={item.reader_href}>查看角色页</a>}
      {item.reason && <p>{item.reason}</p>}
      <details>
        <summary>任务记录（{item.steps.length}）</summary>
        <ol>
          {item.steps.map((s) => (
            <Audit key={s.round + s.step + s.attempt} step={s} />
          ))}
        </ol>
        {images
          .filter((s) => s !== last || revised)
          .map((s) => (
            <div key={s.round}>
              <small>v{s.round} · 初始构图</small>
              <Pair
                image={s.result!.image!}
                phone={s.result!.phone!}
                kind={item.art?.kind}
                scene={item.scene}
              />
            </div>
          ))}
      </details>
    </article>
  )
}
export function MediaGallery({
  items: all,
  batch,
}: {
  items: MediaCandidate[] | null
  batch: MediaCandidate['batch']
}) {
  const items = all?.filter((c) => c.batch === batch) ?? null
  const spec = items?.[0]?.art
  const third = batch === 'bebop-e3'
  const bebop = batch === 'bebop' || third
  const portrait = batch === 'bebop-portrait'
  const title = spec
    ? spec.series +
      ' · ' +
      (spec.kind === 'portrait' ? '人物肖像' : '第 ' + spec.episode + ' 集') +
      ' · ' +
      spec.title
    : portrait
      ? '星际牛仔 · 人物肖像'
      : third
        ? '星际牛仔 · 第 3 集赌场场景'
        : bebop
          ? '星际牛仔 · 1998 TV 场景试稿'
          : '阿澪 · 场景试稿'
  const reference = spec
    ? null
    : bebop
      ? items?.some((c) => c.reference_sha)
        ? file('bebop-reference-v1.png')
        : null
      : portrait
        ? null
        : '/art/mio-invitation.webp'
  const id = spec
    ? batch
    : third
      ? 'bebop-e3-media'
      : portrait
        ? 'bebop-portraits'
        : bebop
          ? 'bebop-media'
          : 'media'
  const [scene, select] = useState('')
  const names = [...new Set(items?.map((c) => c.scene))]
  const active = names.includes(scene) ? scene : names[0]
  return (
    <section className="wb-media" id={id} aria-labelledby={id + '-title'}>
      <div className="wb-art-heading">
        {reference && (
          <a href={reference}>
            <img
              src={reference}
              alt={bebop ? '斯派克 · AI 生成的非官方参考图' : '阿澪参考立绘'}
            />
          </a>
        )}
        <div>
          <h2 id={id + '-title'}>{title}</h2>
          <p>
            {spec
              ? spec.kind === 'portrait'
                ? '人物参考、肖像生成与自动验图记录'
                : '分集参考、场景原图与手机裁切'
              : portrait
                ? '人物参考、肖像生成与自动验图记录'
                : bebop
                  ? '斯派克 · 非官方生成参考图 · 手机场景页封面'
                  : '角色页参考立绘 · 手机预览 390 × 844，展示首屏主视觉区'}
          </p>
        </div>
      </div>
      {items === null && <p role="status">图片任务暂不可用。</p>}
      {items?.length === 0 && <p>暂无试稿任务。</p>}
      <div className="wb-art-tabs">
        {names.map((n) => (
          <button key={n} aria-pressed={active === n} onClick={() => select(n)}>
            {n}
          </button>
        ))}
      </div>
      <div className="wb-art-grid">
        {items
          ?.filter((c) => c.scene === active)
          .map((c) => (
            <Candidate key={c.slot} item={c} />
          ))}
      </div>
    </section>
  )
}
