const assert = require('node:assert/strict')
const { chromium } = require('playwright')

async function check() {
  const port = Number(process.argv[2])
  const browser = await chromium.launch({
    channel: process.env.CHROME_CHANNEL,
    args: [
      '--no-proxy-server',
      `--host-resolver-rules=MAP www.codeiy.com 127.0.0.1:${port}, MAP img.codeiy.com 127.0.0.1:${port}`,
    ],
  })
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true, ignoreHTTPSErrors: true })
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    const routes = []
    page.on('request', request => {
      if (request.resourceType() === 'image' && /^https?:/.test(request.url())) routes.push(request.url())
    })
    const site = 'https://www.codeiy.com'
    const chapter = site + '/story/night-courier#passage-unstamped-letter'
    await page.goto(chapter)
    const anchor = page.locator('#passage-unstamped-letter')
    await anchor.waitFor()
    assert.equal(await anchor.textContent(), '测试章节正文')
    await page.waitForFunction(() => document.querySelector('.reading-cover img')?.naturalWidth > 0)
    await page.goto(site + '/work/tidepost/scene/unstamped-letter')
    await page.getByRole('link', { name: '回到章节 · 夜班信使' }).click()
    assert.equal(page.url(), chapter)
    await anchor.waitFor()
    await page.waitForFunction(() => {
      const top = document.querySelector('#passage-unstamped-letter')?.getBoundingClientRect().top
      return window.scrollY > 0 && top >= 0 && top < window.innerHeight
    })
    assert.ok(routes.length > 0 && routes.every(url => url.startsWith('https://img.codeiy.com/i/')), routes.join('\n'))
    assert.deepEqual(errors, [])
    console.log('PASS 390px chapter deep link, image loading and scene return anchor')
  } finally {
    await browser.close()
  }
}

check().catch(error => {
  console.error(error)
  process.exitCode = 1
})
