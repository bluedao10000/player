import { modelJSON, profile } from './model-api.ts'
import { createHash } from 'node:crypto'
import { constants } from 'node:fs'
import { access, readFile, rename, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import sharp from 'sharp'
import { portraits } from './bebop-portraits.ts'
import { bebopArt, casinoArt } from '@anime/shared/media'
import { MEDIA_TASK_KIND } from './executions.ts'
import { BACKGROUND_CONTEXT as ctx } from '@earendil-works/chord/context'
import {
  defineExtension,
  defineTask,
  type Harness,
} from '@earendil-works/pi-durable'

const dir = '/media'
const refHash =
  '3fd00b6fc23478697107451fe61359eaaf81571be9875fd6d85644d892192eff'
const hash = (buf: Buffer) => createHash('sha256').update(buf).digest('hex')
const scenes = [
  '清晨白帆港码头，阿澪正在邮路上，晨光照亮海面，半身像，棕色邮包可见。',
  '雨夜渡口，阿澪在暖色路灯下侧脸回望驶离的渡船，帽檐雨珠和侧脸清晰。',
  '渡船舷梯旁，阿澪低头辨认手中一封旧信上的邮戳，让眼睛、手和信件成为叙事中心。',
]
const cameras = [
  '平视中景，主体稍靠右，港口在左侧。',
  '从侧前方靠近，以岸边物件形成前景。',
  '稍远的半身环境像，让船舷或码头形成引导线。',
]
type Job = {
  batch?: string
  art?: {
    kind: 'scene' | 'portrait'
    brief: string
    series: string
    title: string
    episode: number
    referenceURL: string
    sourceSHA: string
    character?: string
    box?: number[]
  }
  position?: number
  referenceSHA?: string
  slot: number
  attempt?: number
  reviewVersion?: number
  round: number
  step: 'reference' | 'generate' | 'review' | 'recrop' | 'recheck'
  focus: number
  reason: string
}
type Result = {
  public_sha?: string
  source?: string
  firstOpinion?: string
  secondModel?: string
  image?: string
  reference?: string
  phone?: string
  sha?: string
  reference_sha?: string
  model?: string
  decision?: string
  reason?: string
  focus?: number
  checks?: Record<string, boolean>
}
const position = (j: Job) => j.position ?? j.slot
const isPortrait = (j: Job) =>
  j.batch === 'bebop-portrait' || j.art?.kind === 'portrait'
const portrait = (j: Job) => portraits[position(j) - 1]
const configuredRef = (j: Job) =>
  j.art?.character && /^[a-f0-9]{64}$/.test(j.art.sourceSHA)
    ? j.batch + '-reference-' + j.art.sourceSHA + '.png'
    : j.batch + '-reference.png'
const portraitRef = (j: Job) =>
  j.art ? configuredRef(j) : 'portrait-ref-' + portrait(j).id + '.jpg'
const sceneIndex = (j: Job) => Math.floor((position(j) - 1) / 3)
const isBebopScene = (j: Job) =>
  j.art?.kind === 'scene' || j.batch === 'bebop' || j.batch === 'bebop-e3'
const sceneFor = (j: Job) =>
  j.art ?? (j.batch === 'bebop-e3' ? casinoArt : bebopArt[sceneIndex(j)])
const sceneBrief = (j: Job) =>
  j.art
    ? j.art.brief
    : isPortrait(j)
      ? portrait(j).name + '。' + portrait(j).traits
      : isBebopScene(j)
        ? sceneFor(j).brief
        : scenes[sceneIndex(j)]
const key = (j: Job) => `${j.batch ?? 'mio'}-${position(j)}-r${j.round}`
const spikeReference = 'bebop-reference-v1.png'
const spikeTraits =
  '斯派克·斯皮格尔，1998 TV 动画版，高挑瘦削、蓬松凌乱的深墨绿色卷发、偏长而清瘦的脸、深蓝色便西装、浅黄色衬衫与松垮的深色细领带。'
const shaKey = (j: Job) =>
  j.step === 'reference'
    ? j.art
      ? (j.art.character ? configuredRef(j) : j.batch + '-reference')
      : isPortrait(j)
        ? portraitRef(j)
        : 'bebop-reference-v1'
    : key(j) + '-' + j.step
const cached = async (p: string) =>
  access(p).then(
    () => true,
    () => false,
  )
async function save(name: string, data: Buffer) {
  const p = join(dir, name)
  await writeFile(p + '.tmp', data)
  await rename(p + '.tmp', p)
}
async function ref(j: Job) {
  const data = await readFile(
    j.art
      ? join(dir, configuredRef(j))
      : isPortrait(j)
        ? join(dir, portraitRef(j))
        : isBebopScene(j)
          ? join(dir, spikeReference)
          : '/mio-reference.webp',
  )
  if (hash(data) !== (j.batch === 'mio' || !j.batch ? refHash : j.referenceSHA))
    throw Error('参考图版本变化，请核对图像指纹')
  return data
}
const imageProfile = (j: Job) =>
  profile('image', !j.art && (!j.batch || j.batch === 'mio'))
function mediaSubject(j: Job) {
  return (
    'media:' + j.slot + ':r' + j.round + ':' + j.step + ':a' + (j.attempt ?? 1)
  )
}
async function decodeImage(
  body: { data?: { b64_json?: string }[] },
  width: number,
  height: number,
  gateway = false,
) {
  if (body.data?.length !== 1 || !body.data[0].b64_json)
    throw Error('生成服务未返回单张图片')
  const bytes = Buffer.from(body.data[0].b64_json, 'base64')
  const meta = await sharp(bytes).metadata()
  if (gateway && meta.width && meta.height && ['png', 'jpeg', 'webp'].includes(meta.format ?? '') &&
      Math.min(meta.width, meta.height) >= 768 && Math.abs(meta.width / meta.height - width / height) < 0.055) {
    // Gemini chooses native pixel dimensions for an aspect ratio. Preserve them for review.
    return sharp(bytes).png().toBuffer()
  }
  if (meta.format !== 'png' || meta.width !== width || meta.height !== height)
    throw Error('原图格式或尺寸不符合任务要求')
  await sharp(bytes).stats()
  return bytes
}
async function configuredReference(j: Job): Promise<Result> {
  const a = j.art!,
    url = new URL(a.referenceURL)
  if (
    url.protocol !== 'https:' ||
    url.username || url.password ||
    !((url.href === 'https://www.codeiy.com/art/bebop/spike.webp' && a.series === '星际牛仔')
      || (url.host === 'www.codeiy.com' && /^\/art\/portraits\/[a-f0-9]{64}\.webp$/.test(url.pathname) && url.pathname.endsWith('/' + a.sourceSHA + '.webp'))
      || ['cmsapi-frontend.naruto-official.com', 'lain.bgm.tv'].includes(url.host)
      || (url.host === 'img2.animatetimes.com' && /^\/2026\/(07|09|10|11)\//.test(url.pathname))
      || (url.host === 'nama-anaru.com' && (/^\/teaser_assets\/img\/(episode|chara)\//.test(url.pathname) || /^\/episode\/SYS\/CONTENTS\/story_\d+_photo_\d+$/.test(url.pathname)))
      || (url.host === 'toaru-project.com' && /^\/item\/core_sys\/images\/contents\/\d+\/block\/\d+\/\d+\.jpg$/.test(url.pathname))
      || (url.host === 'korinojoheki-pr.com' && /^\/wp\/wp-content\/themes\/[0-9a-f]+\/static\/assets\/character\/0[1-4]\/main\.webp$/.test(url.pathname))
      || (url.host === 'romelia-senki.com' && /^\/news\/wp\/wp-content\/uploads\/2026\/09\/romelia_ep01_\d+\.jpg$/.test(url.pathname))
      || (url.host === 'www.aniplex.co.jp' && /^\/SYS\/CONTENTS\/story_(4709|4648)_photo_\d+$/.test(url.pathname))
      || (url.host === 'anime-yukionna.af-original.com' && /^\/wp-content\/uploads\/sites\/4\/c[12]_face\.png$/.test(url.pathname))
      || (url.host === 'tenken-anime.com' && url.pathname === '/assets/character/4c.webp')
      || (url.host === 'www.lastboss-anime.com' && (/^\/core_sys\/images\/contents\/00000034\/block\/00000171\/000001(5[6-9]|6[01])\.jpg$/.test(url.pathname)
        || url.pathname === '/core_sys/images/main/cont/chara/chara17-stand.png'))
      || (url.host === 'darksummoner.deregula.com' && /^\/wp-content\/uploads\/(amona|roni)_main\.png$/.test(url.pathname))
      || (url.host === 'fxkurumi-info.com' && /^\/story\/img\/ep02\/02_[1-6]\.jpg$/.test(url.pathname))
      || (url.host === 'lookback-anime.com' && url.pathname.startsWith('/assets/img/'))
      || (url.host === 'www.tbs.co.jp' && url.pathname.startsWith('/anime/takopi_project/series/')))
  )
    throw Error('参考图来源无效')
  const reference = configuredRef(j)
  if (!(await cached(join(dir, reference)))) {
    const res = await fetch(url, {
      redirect: 'error',
      signal: AbortSignal.timeout(30000),
    })
    if (!res.ok) throw Error('参考图 HTTP ' + res.status)
    const data = Buffer.from(await res.arrayBuffer())
    if (hash(data) !== a.sourceSHA) throw Error('参考图来源指纹变化')
    const im = sharp(data)
    if (a.box) {
      const { width, height } = await im.metadata()
      const [x, y, w, h] = a.box
      im.extract({
        left: Math.floor(x * width!),
        top: Math.floor(y * height!),
        width: Math.floor(w * width!),
        height: Math.floor(h * height!),
      })
    }
    await save(reference, await im.png().toBuffer())
  }
  return {
    source: url.href,
    reference,
    reference_sha: hash(await readFile(join(dir, reference))),
  }
}
async function preparePortrait(j: Job): Promise<Result> {
  const spec = portrait(j)
  const reference = portraitRef(j)
  const source = 'https://lain.bgm.tv/pic/crt/l/' + spec.path
  if (!(await cached(join(dir, reference)))) {
    const response = await fetch(source, {
      headers: { 'User-Agent': 'Tidepost-Knowledge/1.0' },
      signal: AbortSignal.timeout(30000),
    })
    if (!response.ok) throw Error('人物资料图 HTTP ' + response.status)
    const data = Buffer.from(await response.arrayBuffer())
    if (hash(data) !== spec.sha) throw Error('人物资料图已变更')
    await save(reference, data)
  }
  if (hash(await readFile(join(dir, reference))) !== spec.sha)
    throw Error('人物资料图指纹不匹配')
  return { reference, reference_sha: spec.sha, source }
}

async function prepareReference(j: Job): Promise<Result> {
  await access(dir, constants.W_OK)
  const path = join(dir, spikeReference)
  const p = imageProfile(j)
  if (!(await cached(path))) {
    const response = await modelJSON(
      {
        ...p,
        subject: mediaSubject(j),
        operation: 'reference',
        size: '1024x1536',
        quality: 'medium',
      },
      p.base + '/images/generations',
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: 'Bearer ' + p.key,
          'Idempotency-Key': 'bebop-reference-v1',
        },
        signal: AbortSignal.timeout(300000),
        body: JSON.stringify({
          model: imageProfile(j).model,
          prompt:
            spikeTraits +
            '为非官方故事导读创作角色视觉研究。3/4 正面从头到腰的单人设定，五官清晰、姿态放松，素净背景，复古手绘赛璐璐科幻动画质感。无文字、无标志、无拼贴。',
          size: '1024x1536',
          n: 1,
          quality: 'medium',
          output_format: 'png',
          ...(p.gateway ? { response_format: 'b64_json' } : {}),
        }),
      },
    )
    if (p.gateway && response.model !== p.model)
      throw Error('网关返回的出图模型与指定路由不一致')
    await save(spikeReference, await decodeImage(response, 1024, 1536, p.gateway))
  }
  return {
    reference: spikeReference,
    reference_sha: hash(await readFile(path)),
    model: imageProfile(j).model,
  }
}
async function generate(j: Job) {
  await access(dir, constants.W_OK)
  const name = key(j) + '.png'
  if (await cached(join(dir, name))) return name
  const prompt = isPortrait(j)
    ? [
        '为《' +
          (j.art?.series ?? '星际牛仔') +
          '》人物导读绘制一幅全新单人胸像。',
        sceneBrief(j),
        '以附图核对人物的脸型、发型、肤色和服饰，不照描原图。采用与九十年代手绘赛璐璐一致的干净线条、克制的阴影和有景深的背景。正方形，头肩在中央，面部占画面约一半，保留头发轮廓、耳饰与下颌，适应圆形头像。不加文字、水印或边框。',
        j.round > 1 ? '返工意见：' + j.reason : '',
      ].join('\n')
    : [
        sceneBrief(j),
        (isBebopScene(j)
          ? [
              '视线高度的中景，环境有层次。',
              '略低的侧前方近景，强调动作。',
              '斜侧方稍远的环境镜头，利用前景交代地点。',
            ]
          : cameras)[(position(j) - 1) % 3],
        isBebopScene(j)
          ? (j.art ? j.art.series : spikeTraits) +
            '以随附本批参考图保持人物外貌。重新创作横向分集插画，新镜头按场景重构环境和姿态，不复制参考图的站姿。人物和关键动作位于画面中部偏右，左下留出页面标题区，原图不加文字。'
          : '以随附原创立绘为唯一人物设定，保持阿澪的脸型、棕色短发、深蓝邮差帽与圆形徽章、制服和棕色邮包。保留原画笔触和人物年龄。新场景独立构图。竖幅，脸在上方三分之一，手与关键动作位于中上部；下方供角色页排字，图片本身不要文字、水印、分格。',
        j.round > 1 ? '上一版本的返工意见：' + j.reason : '',
      ].join('\n')
  const p = imageProfile(j)
  const form = new FormData()
  for (const [k, v] of Object.entries({
    model: imageProfile(j).model,
    prompt,
    size: isPortrait(j)
      ? '1024x1024'
      : isBebopScene(j)
        ? '1536x1024'
        : '1024x1536',
    quality: 'medium',
    n: '1',
    output_format: 'png',
  }))
    form.set(k, v)
  form.set(
    'image',
    new Blob([new Uint8Array(await ref(j))], {
      type: j.art
        ? 'image/png'
        : isPortrait(j)
          ? 'image/jpeg'
          : isBebopScene(j)
            ? 'image/png'
            : 'image/webp',
    }),
    isPortrait(j)
      ? portraitRef(j)
      : isBebopScene(j)
        ? spikeReference
        : 'mio-reference.webp',
  )
  if (p.gateway) form.set('response_format', 'b64_json')
  const response = await modelJSON(
    {
      ...p,
      subject: mediaSubject(j),
      operation: 'edit',
      size: String(form.get('size')),
      quality: 'medium',
    },
    p.base + '/images/edits',
    {
      method: 'POST',
      headers: {
        Authorization: 'Bearer ' + p.key,
        'Idempotency-Key': shaKey(j) + '-' + (j.referenceSHA || refHash),
      },
      body: form,
      signal: AbortSignal.timeout(300000),
    },
  )
  if (p.gateway && response.model !== p.model)
    throw Error('网关返回的出图模型与指定路由不一致')
  const bytes = await decodeImage(
    response,
    isBebopScene(j) ? 1536 : 1024,
    isPortrait(j) || isBebopScene(j) ? 1024 : 1536,
    p.gateway,
  )
  await save(name, bytes)
  return name
}

// The character hero measures 390 × 616 on a 390 × 844 phone. Its image
// occupies 79% of that height. Text positions follow the measured H5 hero.
const overlay = Buffer.from(
  `<svg width="390" height="616" xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="g" x2="0" y2="1" gradientUnits="objectBoundingBox"><stop offset=".35" stop-color="#0d0e16" stop-opacity="0"/><stop offset=".60" stop-color="#0d0e16" stop-opacity=".57"/><stop offset=".79" stop-color="#0d0e16" stop-opacity=".97"/><stop offset=".90" stop-color="#0d0e16"/></linearGradient></defs><rect width="390" height="616" fill="url(#g)"/><g text-anchor="middle" fill="#faf6fd" font-family="Noto Sans CJK SC"><text x="195" y="419" font-family="Noto Serif CJK SC" font-size="65" font-weight="bold">阿澪</text><text x="195" y="454" font-size="13">夜班邮差 · 白帆港</text><text x="195" y="498" font-size="19">「替每一封迷路的信，</text><text x="195" y="528" font-size="19">找到靠岸的灯。」</text><rect x="111" y="552" width="168" height="44" rx="22" fill="#c7b7ec"/><text x="195" y="580" fill="#181523" font-size="15">与阿澪进入剧情　→</text></g></svg>`,
)
async function crop(j: Job) {
  const name =
    key(j) +
    (j.step === 'recrop' || j.step === 'recheck' ? '-crop1.jpg' : '-crop0.jpg')
  const isBebop = isBebopScene(j)
  const width = isBebop ? 358 : 390
  const height = isBebop ? 225 : 616
  const imageHeight = isBebop ? height : 487
  const resized = await sharp(join(dir, key(j) + '.png'))
    .resize(width)
    .png()
    .toBuffer({ resolveWithObject: true })
  const cut = await sharp(resized.data)
    .extract({
      left: 0,
      top: Math.round(((resized.info.height - imageHeight) * j.focus) / 100),
      width,
      height: imageHeight,
    })
    .png()
    .toBuffer()
  const scene = sceneFor(j)
  const cover = isBebop
    ? Buffer.from(
        `<svg width="358" height="225" xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="g" x1="0" y1="1" x2=".25" y2="0"><stop stop-color="#10101d"/><stop offset=".32" stop-color="#151726" stop-opacity=".73"/><stop offset=".76" stop-color="#151726" stop-opacity="0"/></linearGradient></defs><rect width="358" height="225" fill="url(#g)"/><g font-family="Noto Sans CJK SC"><text x="24" y="146" fill="#d9c9f7" font-size="13">${(j.art?.series ?? '星际牛仔').replace(/[<>&]/g, '')} · 第 ${scene.episode} 集</text><text x="24" y="190" font-size="35" font-weight="bold" fill="white">${scene.title.replace(/[<>&]/g, '')}</text></g></svg>`,
      )
    : overlay
  const rendered = await sharp({
    create: { width, height, channels: 3, background: '#0d0e16' },
  })
    .composite([{ input: cut, left: 0, top: 0 }, { input: cover }])
    .jpeg({ quality: 90 })
    .toBuffer()
  await save(name, rendered)
  return name
}
async function portraitPreview(j: Job, image: string) {
  const webp = await sharp(join(dir, image))
    .resize(720, 720)
    .webp({ quality: 86 })
    .toBuffer()
  const digest = hash(webp)
  await save(digest + '.webp', webp)
  const avatar = await sharp(webp)
    .resize(52, 52)
    .ensureAlpha()
    .composite([
      {
        input: Buffer.from(
          '<svg width="52" height="52"><circle cx="26" cy="26" r="26" fill="white"/></svg>',
        ),
        blend: 'dest-in',
      },
    ])
    .png()
    .toBuffer()
  const phone = key(j) + '-crop0.jpg'
  await save(
    phone,
    await sharp({
      create: { width: 358, height: 430, channels: 3, background: '#10101a' },
    })
      .composite([
        {
          input: await sharp(webp).resize(358, 358).toBuffer(),
          left: 0,
          top: 0,
        },
        { input: avatar, left: 10, top: 368 },
      ])
      .jpeg({ quality: 92 })
      .toBuffer(),
  )
  return { phone, public_sha: digest }
}
async function review(j: Job): Promise<Result> {
  const phone = key(j) + (j.step === 'recheck' ? '-crop1.jpg' : '-crop0.jpg')
  const input = [
    await ref(j),
    await readFile(join(dir, key(j) + '.png')),
    await readFile(join(dir, phone)),
  ]
  const images = await Promise.all(
    input.map(async (b) => ({
      type: 'input_image',
      image_url:
        'data:image/jpeg;base64,' +
        (
          await sharp(b)
            .resize({ width: 768, withoutEnlargement: true })
            .jpeg({ quality: 90 })
            .toBuffer()
        ).toString('base64'),
    })),
  )
  const p = profile('vision')
  const primary = p.model
  const fields = ['identity', 'scene', 'anatomy', 'clean', 'crop']
  async function judge(model: string, operation: string) {
    const instruction = isPortrait(j)
      ? `审核《${j.art?.series ?? '星际牛仔'}》人物专属肖像：${sceneBrief(j)}。第一图是该人物的公开比对资料；第二图是新绘肖像；第三图是 358px 角色图与 52px 圆形列表头像。比较原人物及候选的五官、发型、肤色、配饰和服装，检查只绘制一人、小头像中五官仍可辨认。检查 identity（身份）、scene（单人胸像构图）、anatomy（面部结构）、clean（无水印与文字）、crop（大图与圆形头像裁切）。只报告足以退回的可定位缺陷。只返回 JSON {violations:[{field,detail}],reason,focus:50}，无问题时 violations 为空数组。用中文说明实际看到的特征。`
      : `你在审二次元场景试稿。目标：${sceneBrief(j)}。第一张仅供人物比对，不要求新图沿用其背景光线或动作。仅记录肉眼可定位且足以退回的缺陷。返回 JSON 对象 {violations:[{field,detail}],reason,focus}。五个 field：identity=第二图人物身份偏离，scene=第二图缺少目标剧情动作或环境，anatomy=第二图有明显结构错误，clean=第二图内有额外文字排版或水印（只查第二图），crop=第三图主要人物或关键动作被裁掉或遮住。正常情况 violations 为空数组。第三张的标题、台词和按钮是页面设计，不构成文字污染。只要不影响脸或叙事动作就通过手机裁切。不要仅因晨昏色调相近推断时间错误。detail 用中文说明画面中的具体位置与问题；reason 用中文概述观察；focus 是建议的垂直裁切位置（0 顶部，100 底部，当前 ${j.focus}）。`
    const content = [
      { type: 'input_text', text: instruction },
      ...images.flatMap((image, i) => [
        {
          type: 'input_text',
          text: ['第一图：参考立绘', '第二图：候选原图', '第三图：H5 效果'][i],
        },
        image,
      ]),
    ]
    const body = await modelJSON(
      { ...p, model, subject: mediaSubject(j), operation },
      p.base + '/responses',
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: 'Bearer ' + p.key,
        },
        signal: AbortSignal.timeout(90000),
        body: JSON.stringify({
          model,
          input: [{ role: 'user', content }],
          text: { format: { type: 'json_object' } },
        }),
      },
    )
    const verdictText = body.output
      ?.flatMap((o) => o.content ?? [])
      .map((v) => v.text ?? '')
      .join('')
    if (!verdictText) throw Error('视觉模型没有返回验图意见')
    const r = JSON.parse(verdictText)
    if (
      !Array.isArray(r.violations) ||
      typeof r.reason !== 'string' ||
      !r.reason.trim() ||
      !Number.isFinite(r.focus) ||
      r.focus < 0 ||
      r.focus > 100
    )
      throw Error('验图意见不完整')
    const checks: Record<string, boolean> = Object.fromEntries(
      fields.map((k) => [k, true]),
    )
    const issues: string[] = []
    for (const issue of r.violations) {
      if (
        !fields.includes(issue?.field) ||
        typeof issue.detail !== 'string' ||
        !issue.detail.trim()
      )
        throw Error('验图缺陷无定位')
      checks[issue.field] = false
      issues.push(issue.detail)
    }
    return {
      checks,
      reason: (issues.length ? issues.join('；') : r.reason).slice(0, 400),
      focus: Math.round(r.focus),
    }
  }
  let assessment = await judge(primary, 'review')
  let secondModel, firstOpinion
  if (Object.values(assessment.checks).some((v) => !v)) {
    firstOpinion = assessment.reason
    secondModel =
      process.env.VISION_AI_SECOND_MODEL ||
      process.env.MEDIA_SECOND_REVIEW_MODEL ||
      'gpt-4.1'
    assessment = await judge(secondModel, 'second-review')
  }
  const checks = assessment.checks
  const decision =
    !checks.identity || !checks.scene || !checks.anatomy || !checks.clean
      ? 'redraw'
      : !checks.crop
        ? isPortrait(j)
          ? 'redraw'
          : 'recrop'
        : 'pass'
  return {
    phone,
    model: primary,
    decision,
    ...assessment,
    ...(secondModel ? { secondModel, firstOpinion } : {}),
  }
}

const task = defineTask<Job, { phase: 'run' }, Result>({
  name: MEDIA_TASK_KIND,
  version: 1,
  initial: () => ({ phase: 'run' }),
  phases: {
    run: async (t, r, c) => {
      try {
        const j = t.input
        let result: Result
        if (j.step === 'reference')
          result = j.art
            ? await configuredReference(j)
            : isPortrait(j)
              ? await preparePortrait(j)
              : await prepareReference(j)
        else if (j.step === 'generate') {
          const image = await generate(j)
          result = {
            image,
            ...(isPortrait(j)
              ? await portraitPreview(j, image)
              : { phone: await crop(j) }),
            sha: hash(await readFile(join(dir, image))),
            reference_sha:
              isPortrait(j) || isBebopScene(j) ? j.referenceSHA : refHash,
            model: imageProfile(j).model,
          }
        } else if (j.step === 'recrop') {
          result = { phone: await crop(j), focus: j.focus }
        } else result = await review(j)
        await r.commit(
          () => ({
            status: 'terminal',
            outcome: { status: 'completed', result },
          }),
          c,
        )
      } catch (e) {
        await r.commit(
          () => ({
            status: 'terminal',
            outcome: {
              status: 'failed',
              error: {
                message: e instanceof Error ? e.message : '图片任务失败',
              },
            },
          }),
          c,
        )
      }
    },
  },
  abort: (_t, r, c) =>
    r.commit(() => ({ status: 'terminal', outcome: { status: 'aborted' } }), c),
})
export const mediaExtension = defineExtension({
  name: 'mio-media',
  tasks: [task],
})

export async function mediaStatus(harness: Harness, j: Job) {
  if (
    !Number.isInteger(j.slot) ||
    j.slot < 1 ||
    !Number.isInteger(position(j)) ||
    position(j) < 1 ||
    position(j) > (isPortrait(j) ? portraits.length : 9) ||
    (!(
      j.art &&
      /^cfg-[1-9]\d*$/.test(j.batch ?? '') &&
      ['scene', 'portrait'].includes(j.art.kind)
    ) &&
      !['mio', 'bebop', 'bebop-portrait', 'bebop-e3'].includes(
        j.batch ?? 'mio',
      )) ||
    (j.step === 'reference' && !isBebopScene(j) && !isPortrait(j)) ||
    ![1, 2].includes(j.round) ||
    !['reference', 'generate', 'review', 'recrop', 'recheck'].includes(
      j.step,
    ) ||
    !Number.isInteger(j.focus) ||
    j.focus < 0 ||
    j.focus > 100 ||
    typeof j.reason !== 'string' ||
    j.reason.length > 1000
  )
    throw Error('无效图片任务')
  const root = await harness.root(ctx)
  return root.commit(async (tx) => {
    let page = await tx.scanTasks({ kind: task.definition.name }, 100)
    const same = (t: (typeof page.items)[number]) => {
      const v = t.input as Job
      return (
        shaKey(v) === shaKey(j) &&
        (v.reviewVersion ?? 1) === (j.reviewVersion ?? 1) &&
        (v.attempt ?? 1) === (j.attempt ?? 1)
      )
    }
    let old = page.items.find(same)
    while (!old && page.next) {
      page = await tx.scanTasks({ kind: task.definition.name }, 100, page.next)
      old = page.items.find(same)
    }
    if (!old) {
      await tx.createTask(task, j, { ownership: { kind: 'conversation' } })
      return { state: 'running' }
    }
    if (old.state.status !== 'terminal') return { state: 'running' }
    const outcome = old.state.outcome
    return outcome.status === 'completed'
      ? { state: 'done', result: outcome.result }
      : {
          state: 'failed',
          error:
            outcome.status === 'failed' ? outcome.error.message : 'Pi 任务中断',
        }
  }, ctx)
}
