/*
Copyright (C) 2023-2026 QuantumNous
SPDX-License-Identifier: AGPL-3.0-or-later
*/
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { readFileSync, writeFileSync, existsSync } from 'node:fs'
import { resolve, extname } from 'node:path'
import { createRequire } from 'node:module'
const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const out = process.env.AUTO_QA_OUTPUT
assert(out)
const repo = process.env.AUTO_QA_SOURCE || '/work'
const status = JSON.parse(readFileSync(out+'/status.json','utf8'))
const user={id:1,username:'auto-fixture',display_name:'Auto Fixture',role:100,status:1,group:'default',quota:1000000,used_quota:0,request_count:0,setting:'{}'}
const groupData={
  'codex特惠分组':{desc:'低价文字分组',ratio:0.08,auto_eligible:true},
  'codex-稳定版':{desc:'稳定文字分组',ratio:0.35,auto_eligible:true},
  'claude特惠分组':{desc:'Claude 文字分组',ratio:0.3,auto_eligible:true},
  '生图分组-image2':{desc:'图片专用',ratio:1,auto_eligible:false},
  auto:{desc:'按个人顺序选择文字分组，按实际分组计费',ratio:'自动'}
}
const base={id:11,user_id:1,name:'Auto fixture',key:'sk-fixture********',status:1,created_time:1,accessed_time:1,expired_time:-1,remain_quota:1000,used_quota:0,unlimited_quota:true,group:'codex特惠分组',cross_group_retry:false,auto_groups:[],model_limits:'',model_limits_enabled:false,allow_ips:''}
const servers=[]
for(const [theme,port] of [['default',4382],['classic',4383]]) {
 const root=repo+'/web/'+theme+'/dist'
 const server=createServer((req,res)=>{
  const requested=resolve(root,'.'+decodeURIComponent(new URL(req.url,'http://localhost').pathname))
  if(!requested.startsWith(root+'/')&&requested!==root){res.writeHead(403);return res.end()}
  const path=existsSync(requested)&&extname(requested)?requested:root+'/index.html'
  const mime={'.html':'text/html','.js':'application/javascript','.css':'text/css','.json':'application/json','.svg':'image/svg+xml','.woff2':'font/woff2','.png':'image/png'}
  try{res.writeHead(200,{'Content-Type':mime[extname(path)]||'application/octet-stream'});res.end(readFileSync(path))}catch{res.writeHead(404);res.end()}
 })
 await new Promise(resolve=>server.listen(port,'127.0.0.1',resolve));servers.push(server)
}
const browser = await chromium.launch({headless:true})
const records=[]
try {
 for (const viewport of (process.env.AUTO_QA_BASELINE ? [{width:1440,height:1000}] : [{width:1440,height:1000},{width:390,height:844}])) {
  for (const theme of (process.env.AUTO_QA_BASELINE ? ['light'] : ['light','dark'])) {
   const record={viewport,theme,errors:[],console:[],failed:[],checks:[],metrics:0,writes:[]}
   records.push(record)
   let tokens=[structuredClone(base)]
   const context=await browser.newContext({viewport,colorScheme:theme})
   await context.addCookies([{name:'vite-ui-theme',value:theme,url:'http://127.0.0.1:4382'}])
   await context.addInitScript(({user,theme})=>{localStorage.setItem('user',JSON.stringify(user));localStorage.setItem('i18nextLng','zh');localStorage.setItem('theme',theme)}, {user,theme})
   await context.route('**/api/**',async route=>{
    const req=route.request(),path=new URL(req.url()).pathname
    if(path==='/api/status')return route.fulfill({json:status})
    let data={}
    if(path==='/api/user/self')data=user
    if(path==='/api/setup')data={status:true,root_init:true}
    if(path==='/api/notice')data=''
    if(path==='/api/user/self/groups'||path==='/api/user/groups')data=groupData
    if(path==='/api/user/models')data=['gpt-4o']
    if(path==='/api/token/metrics'){
     record.metrics++
     data={items:tokens.map(t=>({id:t.id,active:record.metrics,rpm:record.metrics,today_quota:record.metrics,today_tokens:record.metrics})),consumption_status:'available',consumption_updated_at:record.metrics,as_of:record.metrics,timezone:'UTC',activity_scope:'fixture'}
    }
    if(path==='/api/token/'||path==='/api/token/search'){
     if(req.method()==='GET')data={items:tokens,total:tokens.length,page:1,page_size:20}
     else {const payload=req.postDataJSON();record.writes.push(payload);data={}}
    }
    if(/^\/api\/token\/\d+$/.test(path))data=tokens[0]
    if(path==='/api/token/batch/group'){
     const payload=req.postDataJSON();record.writes.push(payload)
     tokens=tokens.map(t=>payload.ids.includes(t.id)?{...t,...payload}:t);data=payload.ids.length
    }
    await route.fulfill({json:{success:true,message:'',data}})
   })
   const page=await context.newPage()
   page.on('pageerror',e=>record.errors.push(e.message))
   page.on('console',m=>{if(m.type()==='error')record.console.push(m.text())})
   page.on('requestfailed',r=>record.failed.push(r.url()))
   const prefix=out+'/'+viewport.width+'-'+theme
   try {
    const start=Date.now(),response=await page.goto('http://127.0.0.1:4382/keys',{waitUntil:'domcontentloaded'})
    record.http=response.status();await page.screenshot({path:prefix+'-immediate.png'})
    const trigger=page.getByRole('combobox').filter({hasText:'codex特惠分组'}).first()
    assert.equal(await page.evaluate(()=>document.documentElement.classList.contains('dark')),theme==='dark')
    await trigger.click()
    const search=page.locator('[cmdk-input]')
    await search.fill('codex')
    const metricBefore=record.metrics
    await page.waitForTimeout(6200)
    assert(record.metrics>metricBefore,'live metrics must continue refreshing')
    assert(await search.isVisible(),'group menu must remain open across live refresh')
    assert.equal(await search.inputValue(),'codex','search must survive live refresh')
    record.checks.push('live_refresh_preserves_menu_and_search')
    if(process.env.AUTO_QA_BASELINE)continue
    await search.fill('')
    assert((await page.getByRole('option').first().innerText()).includes('Auto · 自动分组'),'Auto must be first')
    record.renderMs=Date.now()-start
    await page.screenshot({path:prefix+'-menu.png'})
    await page.getByRole('option').filter({hasText:'Auto · 自动分组'}).click()
    const editor=page.getByTestId('auto-group-editor')
    await editor.getByRole('button',{name:/codex特惠分组/}).click()
    await editor.getByRole('button',{name:/codex-稳定版/}).click()
    await editor.getByRole('button',{name:'上移: codex-稳定版',exact:true}).click()
    const draft=['codex-稳定版','codex特惠分组']
    const beforeDialog=record.metrics
    await page.waitForTimeout(6200)
    assert(record.metrics>beforeDialog)
    assert(await editor.isVisible(),'Auto editor must remain open across refresh')
    assert.deepEqual(await editor.locator('[data-auto-group]').evaluateAll(nodes=>nodes.map(n=>n.dataset.autoGroup)),draft)
    await page.screenshot({path:prefix+'-editor.png'})
    await page.getByRole('dialog').last().getByRole('button',{name:'保存更改'}).click()
    await page.waitForTimeout(300)
    assert.deepEqual(record.writes.at(-1).auto_groups,draft)
    const auto=page.getByRole('combobox').filter({hasText:'Auto · 自动分组'}).first()
    assert(await auto.isVisible())
    await page.screenshot({path:prefix+'-selected.png'})
    await auto.click()
    assert((await page.getByRole('option').first().innerText()).includes('Auto · 自动分组'))
    await page.keyboard.press('Escape')
    await page.locator('[cmdk-input]').waitFor({state:'hidden'})
    await auto.click()
    await page.getByRole('option').filter({hasText:'Auto · 自动分组'}).click()
    assert.deepEqual(await page.getByTestId('auto-group-editor').locator('[data-auto-group]').evaluateAll(nodes=>nodes.map(n=>n.dataset.autoGroup)),draft)
    await page.getByRole('dialog').last().getByRole('button',{name:'取消',exact:true}).click()
    record.checks.push('auto_first','editor_survives_live_refresh','draft_order_saved_and_reopened','escape_closes_menu')
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
    assert.deepEqual(record.errors,[]);assert.deepEqual(record.console,[]);assert.deepEqual(record.failed,[])
   }catch(error){
    await page.screenshot({path:prefix+'-failure.png'})
    record.failure=error.message
    throw error
   }finally{await context.close()}
  }
 }
}finally{
 writeFileSync(out+'/refresh-report.json',JSON.stringify(records,null,2))
 await browser.close()
 await Promise.all(servers.map(s=>new Promise(resolve=>s.close(resolve))))
}
console.log(JSON.stringify(records))
