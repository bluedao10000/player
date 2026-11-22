import { Button } from '@astryxdesign/core/Button'
import type { MediaCandidate, MediaStep } from '@anime/shared/knowledge'
import {
  Check,
  Download,
  ExternalLink,
  FileWarning,
  GitCompareArrows,
  History,
  Image as ImageIcon,
  Images,
  RefreshCcw,
  X,
  ZoomIn,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { formatClock, formatDateTime } from './shared/format'
import { EmptyState, ImageAsset, Panel, SectionHeading, StatusBadge, workbenchAssetUrl } from './shared/ui'
import { mediaStageNames } from './workflow/workflow-canvas'

const checkLabels = {
  identity: '人物一致',
  scene: '场景与动作',
  anatomy: '五官与手部',
  clean: '画面干净',
  crop: '手机构图',
}

type MediaPair = { step: MediaStep; image: string; phone: string }

function pairs(item: MediaCandidate): MediaPair[] {
  return item.steps.flatMap((step) => {
    if (!step.result?.image || !step.result.phone) return []
    const revised = item.steps.find((candidate) => candidate.round === step.round && candidate.step === 'recrop' && candidate.result?.phone)
    return [{ step, image: step.result.image, phone: revised?.result?.phone ?? step.result.phone }]
  })
}

function series(item: MediaCandidate) {
  if (item.art) return item.art.series
  if (item.batch.startsWith('bebop')) return '星际牛仔'
  return '阿澪'
}

function chapter(item: MediaCandidate) {
  if (item.art) return item.art.episode > 0 ? `第 ${item.art.episode} 集 · ${item.art.title}` : `人物肖像 · ${item.art.title}`
  const labels: Record<string, string> = {
    mio: '角色首屏场景',
    bebop: '1998 TV 场景试稿',
    'bebop-e3': '第 3 集 · 赌场场景',
    'bebop-portrait': '人物肖像',
  }
  return labels[item.batch] ?? item.batch
}

function latestAudit(item: MediaCandidate) {
  return [...item.steps].reverse().find((step) => step.result?.checks || step.result?.decision)
}

export function MediaGallery({ items: all, candidate }: { items: MediaCandidate[] | null; candidate?: number }) {
  const target = all?.find((item) => item.slot === candidate)
  const [work, setWork] = useState(target ? series(target) : '')
  const [batch, setBatch] = useState(target?.batch ?? '')
  const [scene, setScene] = useState(target?.scene ?? '')
  const [status, setStatus] = useState('all')
  const [selected, setSelected] = useState<number | null>(target?.slot ?? null)
  const works = [...new Set(all?.map(series) ?? [])]
  const activeWork = works.includes(work) ? work : works.at(-1)
  const workItems = all?.filter((item) => series(item) === activeWork) ?? []
  const batches = [...new Set(workItems.map((item) => item.batch))]
  const activeBatch = batches.includes(batch) ? batch : batches.at(-1)
  const batchItems = workItems.filter((item) => item.batch === activeBatch)
  const scenes = [...new Set(batchItems.map((item) => item.scene))]
  const activeScene = scenes.includes(scene) ? scene : scenes[0]
  const sceneItems = batchItems.filter((item) => item.scene === activeScene)
  const items = sceneItems.filter((item) => {
    if (status === 'all') return true
    if (status === 'running') return !['passed', 'held', 'failed'].includes(item.stage)
    return item.stage === status
  })
  const active = items.find((item) => item.slot === selected) ?? items[0]
  const chapterLabel = batchItems[0] ? chapter(batchItems[0]) : ''

  function reset(next: 'work' | 'batch') {
    setSelected(null)
    setScene('')
    if (next === 'work') setBatch('')
  }

  return (
    <Panel className="wb-media-console" id="media" labelledby="media-title">
      <SectionHeading
        id="media-title"
        eyebrow="MEDIA ASSET MANAGEMENT"
        title="图片资产管理"
        detail="按作品、分集与场景组织候选；仅加载当前视图和展开的版本"
        actions={all && <StatusBadge status="queued" label={`${all.length} 个候选`} />}
      />
      {all && candidate && !target && <p className="wb-muted">候选槽位 #{candidate} 不在当前记录中。</p>}
      {all === null ? (
        <EmptyState icon={FileWarning} title="图片任务暂不可用" detail="媒体候选接口没有返回数据；运行台其他区域不受影响。" />
      ) : all.length === 0 ? (
        <EmptyState icon={Images} title="暂无图片候选" detail="等待数据服务登记场景或人物图片任务。" />
      ) : (
        <>
          <div className="wb-media-filters">
            <label>作品
              <select value={activeWork} onChange={(event) => { setWork(event.target.value); reset('work') }}>
                {works.map((name) => <option key={name}>{name}</option>)}
              </select>
            </label>
            <label>分集 / 资产组
              <select value={activeBatch} onChange={(event) => { setBatch(event.target.value); reset('batch') }}>
                {batches.map((name) => {
                  const item = workItems.find((candidate) => candidate.batch === name)!
                  return <option key={name} value={name}>{chapter(item)}</option>
                })}
              </select>
            </label>
            <label>候选状态
              <select value={status} onChange={(event) => { setStatus(event.target.value); setSelected(null) }}>
                <option value="all">全部状态</option>
                <option value="running">进行中</option>
                <option value="passed">验图通过</option>
                <option value="held">等待返工</option>
                <option value="failed">任务失败</option>
              </select>
            </label>
            <div className="wb-media-context">
              <span className="wb-eyebrow">CURRENT SCOPE</span>
              <strong>{activeWork} · {chapterLabel}</strong>
              <small>{sceneItems.length} 个场景候选 · {items.length} 个符合筛选</small>
            </div>
          </div>
          <div className="wb-scene-tabs" aria-label="场景">
            {scenes.map((name) => (
              <button key={name} aria-pressed={activeScene === name} onClick={() => { setScene(name); setSelected(null) }}>
                <ImageIcon size={14} />{name}
              </button>
            ))}
          </div>
          {items.length ? (
            <div className="wb-asset-grid">
              {items.map((item) => {
                const pair = pairs(item).at(-1)
                return (
                  <article className="wb-asset-card" data-selected={active?.slot === item.slot} key={item.slot}>
                    {pair ? <ImageAsset name={pair.image} alt={`${item.scene} · 候选 ${item.variant} 原图`} className="wb-asset-thumbnail" /> : <div className="wb-asset-no-image"><ImageIcon size={24} />等待原图</div>}
                    <div className="wb-asset-card-head">
                      <div><span className="wb-eyebrow">CANDIDATE {String(item.variant).padStart(2, '0')}</span><h3>{item.scene}</h3></div>
                      <StatusBadge status={item.stage} label={mediaStageNames[item.stage]} />
                    </div>
                    <div className="wb-asset-card-meta"><span>版本 v{item.round}</span><span>{item.steps.length} 条任务记录</span><span>槽位 #{item.slot}</span></div>
                    {item.reason && <p>{item.reason}</p>}
                    <Button className="wb-button" size="sm" variant={active?.slot === item.slot ? 'primary' : 'secondary'} label={active?.slot === item.slot ? '正在查看' : '查看资产'} onClick={() => setSelected(item.slot)} />
                  </article>
                )
              })}
            </div>
          ) : (
            <EmptyState icon={Images} title="当前筛选没有候选" detail="切换场景或状态查看其他真实候选。" />
          )}
          {active && <CandidateDetail item={active} key={active.slot} />}
        </>
      )}
    </Panel>
  )
}

function CandidateDetail({ item }: { item: MediaCandidate }) {
  const versions = pairs(item)
  const current = versions.at(-1)
  const audit = latestAudit(item)
  const [compare, setCompare] = useState(false)
  const [zoom, setZoom] = useState<string | null>(null)
  const rework = item.steps.filter((step) => step.step === 'recrop' || step.step === 'recheck' || step.result?.decision === 'redraw' || step.result?.decision === 'recrop')
  const reference = !item.art && (item.batch === 'bebop' || item.batch === 'bebop-e3') && item.reference_sha ? workbenchAssetUrl('bebop-reference-v1.png') : null
  const livePreview = current && /^(mio|bebop|bebop-e3)-/.test(current.image) ? `/workbench-hero/${current.image}?focus=${current.step.result?.focus ?? 40}` : null

  return (
    <section className="wb-candidate-detail" aria-labelledby={`candidate-${item.slot}`}>
      <div className="wb-candidate-title">
        <div><span className="wb-eyebrow">ASSET INSPECTOR · SLOT #{item.slot}</span><h3 id={`candidate-${item.slot}`}>{item.scene} · 候选 {item.variant}</h3><p>{chapter(item)} · 当前版本 v{item.round}</p></div>
        <div className="wb-candidate-actions">
          <Button className="wb-button" size="sm" variant="secondary" icon={<GitCompareArrows size={15} />} label={compare ? '关闭版本比较' : '版本比较'} onClick={() => setCompare((value) => !value)} isDisabled={versions.length < 2} />
          {current && <Button className="wb-button" size="sm" variant="secondary" icon={<ZoomIn size={15} />} label="放大原图" onClick={() => setZoom(current.image)} />}
        </div>
      </div>
      {current ? (
        <div className="wb-art-pair">
          <figure>
            <ImageAsset name={current.image} alt={`${item.scene} · 原图`} className="wb-detail-image" eager />
            <figcaption><span>候选原图 · PNG</span><span><a href={workbenchAssetUrl(current.image)} target="_blank" rel="noreferrer">查看原图 <ExternalLink size={13} /></a><a href={workbenchAssetUrl(current.image)} download>下载 <Download size={13} /></a></span></figcaption>
          </figure>
          <figure>
            <ImageAsset name={current.phone} alt={`${item.scene} · 手机裁切`} className="wb-detail-image" eager />
            <figcaption><span>手机裁切 · JPG</span><span><a href={workbenchAssetUrl(current.phone)} target="_blank" rel="noreferrer">验图样张 <ExternalLink size={13} /></a><a href={workbenchAssetUrl(current.phone)} download>下载 <Download size={13} /></a></span></figcaption>
          </figure>
        </div>
      ) : (
        <EmptyState icon={ImageIcon} title="尚未形成原图与裁切对" detail="任务步骤已保留，生成完成后可在此比较。" />
      )}
      <div className="wb-asset-links">
        {reference && <a href={reference} target="_blank" rel="noreferrer">查看角色参考图 <ExternalLink size={13} /></a>}
        {livePreview && <a href={livePreview} target="_blank" rel="noreferrer">打开 H5 实时首屏 <ExternalLink size={13} /></a>}
        {item.reader_href && <a href={item.reader_href}>查看已发布角色页 <ExternalLink size={13} /></a>}
        {current?.step.result?.source && <a href={current.step.result.source}>人物参考来源 <ExternalLink size={13} /></a>}
      </div>
      <div className="wb-review-grid">
        <section className="wb-quality-review">
          <div className="wb-subheading"><div><span className="wb-eyebrow">AUTOMATED REVIEW</span><h4>五项自动验图</h4></div>{audit?.result?.decision && <StatusBadge status={audit.result.decision === 'pass' ? 'passed' : 'held'} label={{ pass: '通过', redraw: '退回重绘', recrop: '退回重裁' }[audit.result.decision]} />}</div>
          {audit?.result?.checks ? (
            <div className="wb-checks">
              {(Object.keys(checkLabels) as (keyof typeof checkLabels)[]).map((key) => {
                const passed = audit.result!.checks![key]
                return <div key={key} data-passed={passed}>{passed ? <Check size={16} /> : <X size={16} />}<span>{checkLabels[key]}</span><strong>{passed ? '通过' : '未通过'}</strong></div>
              })}
            </div>
          ) : <p className="wb-muted">尚未产生五项验图结果。</p>}
          {(audit?.result?.reason || item.reason) && <p className="wb-review-reason">{audit?.result?.reason ?? item.reason}</p>}
          {audit?.result?.model && <small>验图模型 · {audit.result.model}{audit.result.secondModel ? ` · 独立复核 ${audit.result.secondModel}` : ''}</small>}
        </section>
        <section className="wb-rework">
          <div className="wb-subheading"><div><span className="wb-eyebrow">REWORK</span><h4>返工与复验</h4></div><RefreshCcw size={18} /></div>
          <strong>{rework.length} 条返工相关记录</strong>
          <p>{rework.length ? '保留退回原因、重裁与复验步骤，历史版本不会被覆盖。' : '当前候选尚未发生返工。'}</p>
          {rework.at(-1)?.result?.reason && <blockquote>{rework.at(-1)!.result!.reason}</blockquote>}
        </section>
      </div>
      {compare && <VersionCompare versions={versions} scene={item.scene} />}
      <details className="wb-task-history">
        <summary><History size={15} />任务执行与返工记录（{item.steps.length}）</summary>
        <ol>
          {[...item.steps].reverse().map((step) => <Audit step={step} key={`${step.round}-${step.step}-${step.attempt}-${step.started_at}`} />)}
        </ol>
      </details>
      {zoom && <Lightbox name={zoom} alt={`${item.scene} · 放大原图`} onClose={() => setZoom(null)} />}
    </section>
  )
}

function Audit({ step }: { step: MediaStep }) {
  const decision = step.result?.decision
  return (
    <li>
      <time>{formatClock(step.started_at)} UTC</time>
      <div><strong>v{step.round} · {mediaStageNames[step.step]}</strong>{step.attempt > 1 && <span> · 技术重试 {step.attempt}</span>}{step.finished_at === null && <span> · 进行中</span>}</div>
      {decision && <StatusBadge status={decision === 'pass' ? 'passed' : 'held'} label={{ pass: '通过', redraw: '退回重绘', recrop: '退回重裁' }[decision]} />}
      {step.result?.firstOpinion && <p>初检疑点 · {step.result.firstOpinion}</p>}
      {step.result?.reason && <p>{step.result.reason}</p>}
      {step.error && <p className="wb-error-text">{step.error}</p>}
      {step.result?.model && <small>{step.result.model} · {formatDateTime(step.started_at)}</small>}
    </li>
  )
}

function VersionCompare({ versions, scene }: { versions: MediaPair[]; scene: string }) {
  const [left, setLeft] = useState(Math.max(0, versions.length - 2))
  const [right, setRight] = useState(versions.length - 1)
  const options = versions.map((version, index) => <option value={index} key={`${version.step.round}-${version.step.started_at}`}>v{version.step.round} · {formatDateTime(version.step.started_at)}</option>)
  return (
    <section className="wb-version-compare">
      <div className="wb-subheading"><div><span className="wb-eyebrow">VERSION COMPARE</span><h4>原图与手机裁切版本比较</h4></div><GitCompareArrows size={19} /></div>
      <div className="wb-version-selects"><label>版本 A<select value={left} onChange={(event) => setLeft(Number(event.target.value))}>{options}</select></label><label>版本 B<select value={right} onChange={(event) => setRight(Number(event.target.value))}>{options}</select></label></div>
      <div className="wb-version-grid">
        {[versions[left], versions[right]].map((version, index) => (
          <article key={index}>
            <strong>{index === 0 ? 'A' : 'B'} · v{version.step.round}</strong>
            <ImageAsset name={version.image} alt={`${scene} · 比较原图 ${index === 0 ? 'A' : 'B'}`} className="wb-version-image" />
            <ImageAsset name={version.phone} alt={`${scene} · 比较裁切 ${index === 0 ? 'A' : 'B'}`} className="wb-version-phone" />
          </article>
        ))}
      </div>
    </section>
  )
}

function Lightbox({ name, alt, onClose }: { name: string; alt: string; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    const dialog = ref.current
    dialog?.showModal()
    return () => { if (dialog?.open) dialog.close() }
  }, [])
  return (
    <dialog ref={ref} className="wb-lightbox" onCancel={onClose} aria-label={alt}>
      <div><Button className="wb-button" variant="secondary" size="sm" icon={<X size={16} />} label="关闭" onClick={onClose} /><a href={workbenchAssetUrl(name)} target="_blank" rel="noreferrer">打开原始文件 <ExternalLink size={14} /></a></div>
      <ImageAsset name={name} alt={alt} className="wb-lightbox-image" eager />
    </dialog>
  )
}

export function RecentMedia({ items }: { items: MediaCandidate[] | null }) {
  const recent = (items ?? []).flatMap((item) => {
    const pair = pairs(item).at(-1)
    return pair ? [{ item, pair }] : []
  }).sort((a, b) => b.pair.step.started_at.localeCompare(a.pair.step.started_at)).slice(0, 4)

  return (
    <Panel>
      <SectionHeading
        title="图片资产 · 最近候选"
        actions={<a href="/workbench?view=data&section=media">查看全部 →</a>}
      />
      {recent.length ? (
        <div className="wb-recent-media">
          {recent.map(({ item, pair }) => (
            <article key={item.slot}>
              <ImageAsset name={pair.image} alt={item.scene} />
              <strong>{item.scene} · 候选 {item.variant}</strong>
              <small>{series(item)} · {mediaStageNames[item.stage]}</small>
            </article>
          ))}
        </div>
      ) : <p className="wb-muted">{items ? '暂无候选图' : '图片记录暂不可用'}</p>}
    </Panel>
  )
}
