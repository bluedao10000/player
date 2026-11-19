// Optional authoring dependencies are installed outside the application workspace.
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright')
const docs = path.resolve(__dirname, '..')
const views = [
  ['product', '产品架构', 'diagrams/product.mmd'],
  ['business', '知识库构建流程', 'diagrams/business.mmd'],
  ['technology', '技术架构', 'architecture.mmd'],
  ['deployment', '网络与部署', 'diagrams/deployment.mmd'],
]

async function build() {
  const browser = await chromium.launch(process.env.CHROME_CHANNEL ? {channel:process.env.CHROME_CHANNEL} : {})
  try {
    const page = await browser.newPage({viewport:{width:1280,height:960},deviceScaleFactor:1.5})
    await page.setContent('<meta charset="utf-8"><style>body{margin:0;background:#fafbf7}</style>')
    const library = require.resolve('mermaid/package.json')
    await page.addScriptTag({path:path.join(path.dirname(library),'dist/mermaid.min.js')})
    await page.evaluate(() => mermaid.initialize({
      startOnLoad:false,theme:'base',deterministicIds:true,deterministicIDSeed:'player',
      flowchart:{htmlLabels:false,useMaxWidth:false,curve:'linear',nodeSpacing:28,rankSpacing:37},
      themeVariables:{fontFamily:'Arial, PingFang SC, Microsoft YaHei, sans-serif',fontSize:'16px',
        primaryColor:'#eaf3ed',primaryTextColor:'#183d39',primaryBorderColor:'#488071',lineColor:'#56756f',
        clusterBkg:'#f6f7f3',clusterBorder:'#a9bbb2',edgeLabelBackground:'#fafbf7'},
    }))
    const backlogOnly = process.argv[2] === 'backlog'
    const selected = backlogOnly ? [['backlog', '待开发功能', 'diagrams/backlog.mmd']] : views
    const atlas=[]
    for (const [key,title,source] of selected) {
      const code=fs.readFileSync(path.join(docs,source),'utf8')
      const nodes=Object.fromEntries([...code.matchAll(/^%% (\w+) \| (.+?) \| (.+)$/gm)].map(([,id,detail,file])=>{
        if(!fs.existsSync(path.resolve(docs,'..',file))) throw new Error('Missing source: '+file)
        return [id,{detail,file}]
      }))
      const svg=await page.evaluate(async({key,code,nodes})=>{
        const {svg}=await mermaid.render('atlas-'+key,code)
        const template=document.createElement('template')
        template.innerHTML=svg
        for(const node of template.content.querySelectorAll('.node')){
          const id=/flowchart-(.+)-\d+$/.exec(node.id)?.[1]
          if(!id || !nodes[id]) throw Error('Missing node annotation: '+node.id)
          node.setAttribute('data-node',id)
        }
        if(template.content.querySelectorAll('[data-node]').length!==Object.keys(nodes).length) throw Error('Unused annotation')
        return new XMLSerializer().serializeToString(template.content.querySelector('svg'))
      },{key,code,nodes})
      fs.writeFileSync(path.join(__dirname,key+'.svg'),svg)
      if(backlogOnly||key==='product'||key==='business'){
        await page.evaluate(svg=>{document.body.innerHTML=svg},svg)
        await page.locator('svg').screenshot({path:path.join(__dirname,key+'.png')})
      }
      atlas.push({key,title,source,nodes,svg})
    }
    if (backlogOnly) return
    const template=fs.readFileSync(path.join(__dirname,'atlas.template.html'),'utf8')
    const payload=JSON.stringify(atlas).replaceAll('<','\\u003c')
    fs.writeFileSync(path.join(docs,'architecture.html'),template.replace('/* ATLAS_DATA */ []',()=>payload))
  } finally {await browser.close()}
}
build().catch(error=>{console.error(error);process.exitCode=1})
