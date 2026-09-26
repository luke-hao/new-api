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
  'codex特惠分组':{desc:'低价文字分组',ratio:0.08,auto_eligible:true,auto_types:['text']},
  'codex-稳定版':{desc:'稳定文字分组',ratio:0.35,auto_eligible:true,auto_types:['text']},
  'claude特惠分组':{desc:'Claude 文字分组',ratio:0.3,auto_eligible:true,auto_types:['text']},
  '生图分组-image2':{desc:'图片专用',ratio:1,auto_eligible:true,auto_types:['image']},
  auto:{desc:'按个人顺序选择文字或生图分组，按实际分组计费',ratio:'自动'}
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
const browser=await chromium.launch({headless:true})
const records=[]
let currentPage
try {
 for(const viewport of [{width:1440,height:1000},{width:390,height:844}]) {
  const record={theme:'default',viewport,errors:[],console:[],failed:[],writes:[],checks:[]};records.push(record)
  let tokens=[structuredClone(base)]
  const context=await browser.newContext({viewport})
  await context.addInitScript(user=>{localStorage.setItem('user',JSON.stringify(user));localStorage.setItem('i18nextLng','zh');localStorage.setItem('theme','light')},user)
  await context.route('**/api/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname
   if(path==='/api/status')return route.fulfill({json:status})
   let data={}
   if(path==='/api/user/self')data=user
   if(path==='/api/setup')data={status:true,root_init:true,database_type:'postgres'}
   if(path==='/api/notice')data=''
   if(path==='/api/user/self/groups'||path==='/api/user/groups')data=groupData
   if(path==='/api/user/models')data=['gpt-4o','claude-sonnet-4']
   if(path==='/api/token/metrics')data={items:tokens.map(t=>({id:t.id,active:0,rpm:0,today_quota:0,today_tokens:0})),consumption_status:'available',consumption_updated_at:0,as_of:0,timezone:'UTC',activity_scope:'fixture'}
   if(path==='/api/token/'||path==='/api/token/search'){
    if(req.method()==='GET')data={items:tokens,total:tokens.length,page:1,page_size:10}
    else {
     const payload=req.postDataJSON();record.writes.push({path,payload})
     if(req.method()==='POST')tokens.push({...base,...payload,id:12})
     else tokens=tokens.map(t=>t.id===payload.id?{...t,...payload}:t)
     data=tokens.find(t=>t.id===payload.id)
    }
   }
   if(/^\/api\/token\/\d+$/.test(path))data=tokens.find(t=>t.id===Number(path.split('/').at(-1)))
   if(path==='/api/token/batch/group'){
    const payload=req.postDataJSON();record.writes.push({path,payload})
    tokens=tokens.map(t=>payload.ids.includes(t.id)?{...t,...payload}:t);data=payload.ids.length
   }
   await route.fulfill({json:{success:true,message:'',data}})
  })
  const page=await context.newPage();currentPage=page
  page.on('pageerror',e=>record.errors.push(e.message))
  page.on('console',m=>{if(m.type()==='error')record.console.push(m.text())})
  page.on('requestfailed',r=>record.failed.push({url:r.url(),error:r.failure()?.errorText}))
  const start=Date.now()
  const response=await page.goto('http://127.0.0.1:4382/keys',{waitUntil:'domcontentloaded'})
  record.http=response.status();await page.screenshot({path:out+'/'+viewport.width+'-immediate.png'})
  await page.getByRole('button',{name:/创建.*密钥|Create API Key/i}).first().click()
  await page.getByRole('textbox',{name:/^名称$|^Name$/}).fill('My auto key')
  const drawer=page.getByRole('dialog').last()
  await drawer.getByRole('combobox').first().click()
  await page.getByRole('option').filter({hasText:'Auto · 自动分组'}).click()
  const editor=page.getByTestId('auto-group-editor')
  await editor.waitFor();record.renderMs=Date.now()-start
  await editor.getByRole('button',{name:/codex特惠分组/}).click()
  await editor.getByRole('button',{name:/codex-稳定版/}).click()
  await editor.getByRole('button',{name:'上移: codex-稳定版',exact:true}).click()
  await editor.getByRole('button',{name:/生图分组-image2/}).click()
  await editor.getByRole('button',{name:'上移: 生图分组-image2',exact:true}).click()
  await editor.getByRole('button',{name:'上移: 生图分组-image2',exact:true}).click()
  assert.deepEqual(await editor.locator('[data-auto-group]').evaluateAll(nodes=>nodes.map(n=>n.dataset.autoGroup)),['生图分组-image2','codex-稳定版','codex特惠分组'])
  assert((await editor.locator('[data-auto-group="生图分组-image2"]').innerText()).includes('图片'))
  await editor.getByRole('textbox',{name:'搜索...'}).fill('claude')
  await editor.getByRole('button',{name:/claude特惠分组/}).click()
  await editor.getByRole('button',{name:'移除: claude特惠分组',exact:true}).click()
  record.checks.push('mixed_text_image_add_search_reorder_remove')
  await editor.scrollIntoViewIfNeeded()
  await page.screenshot({path:out+'/'+viewport.width+'-editor-final.png'})
  await drawer.getByRole('button',{name:/保存更改|Save changes/}).click()
  await page.waitForTimeout(300)
  const saved=record.writes.find(w=>w.payload.name==='My auto key')?.payload
  assert(saved);assert.equal(saved.group,'auto');assert.equal(saved.cross_group_retry,true);assert.deepEqual(saved.auto_groups,['生图分组-image2','codex-稳定版','codex特惠分组'])
  record.checks.push('create_persists_order_and_default_retry')
  // Reopen personal order via the inline group editor on desktop and mobile cards.
  const combos=page.getByRole('combobox').filter({hasText:'Auto · 自动分组'})
  await combos.first().click()
  await page.getByRole('option').filter({hasText:'Auto · 自动分组'}).click()
  await page.getByTestId('auto-group-editor').waitFor()
  assert.deepEqual(await page.getByTestId('auto-group-editor').locator('[data-auto-group]').evaluateAll(nodes=>nodes.map(n=>n.dataset.autoGroup)),saved.auto_groups)
  const dialog=page.getByRole('dialog').last()
  await dialog.getByRole('switch').click()
  await dialog.getByRole('button',{name:/保存更改|Save changes/}).click()
  await page.waitForTimeout(200)
  assert.equal(record.writes.at(-1).payload.cross_group_retry,false)
  record.checks.push('inline_auto_dialog_restores_order_and_retry_toggle')
  await page.getByRole('checkbox',{name:/全选|选择行/}).first().click()
  await page.getByRole('button',{name:'修改所选 API 密钥的分组',exact:true}).click()
  const batchDialog=page.getByRole('dialog').last()
  await batchDialog.getByRole('combobox').click()
  await page.getByRole('option').filter({hasText:'Auto · 自动分组'}).click()
  const batchEditor=page.getByTestId('auto-group-editor')
  await batchEditor.getByRole('button',{name:/codex特惠分组/}).click()
  await batchEditor.getByRole('button',{name:/claude特惠分组/}).click()
  await batchEditor.getByRole('button',{name:'上移: claude特惠分组',exact:true}).click()
  await batchEditor.getByRole('button',{name:/生图分组-image2/}).click()
  await page.screenshot({path:out+'/'+viewport.width+'-batch-final.png'})
  await batchDialog.getByRole('button',{name:'修改分组',exact:true}).click()
  await page.waitForTimeout(200)
  assert.equal(record.writes.at(-1).path,'/api/token/batch/group')
  assert.deepEqual(record.writes.at(-1).payload.auto_groups,['claude特惠分组','codex特惠分组','生图分组-image2'])
  record.checks.push('batch_order_and_save')
  record.overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth)
  assert.equal(record.overflow,false)
  assert.deepEqual(record.errors,[]);assert.deepEqual(record.console,[])
  await context.close()
 }
 // Classic shares the wire contract; exercise its ordered editor with both widths.
 for(const viewport of [{width:1440,height:1000},{width:390,height:844}]) {
  const record={theme:'classic',viewport,errors:[],console:[],failed:[],writes:[],checks:[]};records.push(record)
  const context=await browser.newContext({viewport})
  await context.addInitScript(user=>{localStorage.setItem('user',JSON.stringify(user));localStorage.setItem('i18nextLng','zh');localStorage.setItem('theme','light')},user)
  await context.route('**/api/**',async route=>{
   const req=route.request(),path=new URL(req.url()).pathname
   if(path==='/api/status')return route.fulfill({json:status})
   let data={}
   if(path==='/api/user/self')data=user
   if(path==='/api/setup')data={status:true,root_init:true}
   if(path==='/api/user/self/groups')data=groupData
   if(path==='/api/user/models')data=['gpt-4o']
   if(path==='/api/token/'||path==='/api/token/search'){
    if(req.method()==='GET')data={items:[base],total:1,page:1,page_size:10}
    else record.writes.push(req.postDataJSON())
   }
   if(path==='/api/notice')data=''
   await route.fulfill({json:{success:true,data}})
  })
  const page=await context.newPage();currentPage=page
  page.on('pageerror',e=>record.errors.push(e.message))
  page.on('console',m=>{if(m.type()==='error')record.console.push(m.text())})
  page.on('requestfailed',r=>record.failed.push(r.url()))
  const start=Date.now(),response=await page.goto('http://127.0.0.1:4383/console/token',{waitUntil:'domcontentloaded'})
  record.http=response.status();await page.screenshot({path:out+'/classic-'+viewport.width+'-immediate.png'})
  if(viewport.width < 768) await page.getByText('显示操作项',{exact:true}).click()
  await page.getByRole('button',{name:/添加令牌|创建令牌|Add Token/}).first().click()
  await page.getByRole('textbox',{name:/名称/}).fill('Classic auto')
  await page.locator('.semi-select').filter({hasText:/令牌分组，默认为用户的分组/}).click()
  await page.getByText('Auto · 自动分组',{exact:true}).last().click()
  const editor=page.getByTestId('classic-auto-group-editor')
  await editor.waitFor();record.renderMs=Date.now()-start
  await editor.getByRole('button',{name:/codex特惠分组/}).click()
  await editor.getByRole('button',{name:/codex-稳定版/}).click()
  await editor.getByRole('button',{name:'上移: codex-稳定版'}).click()
  await editor.getByRole('button',{name:/生图分组-image2/}).click()
  await editor.getByRole('button',{name:'上移: 生图分组-image2'}).click()
  await editor.getByRole('button',{name:'上移: 生图分组-image2'}).click()
  await editor.scrollIntoViewIfNeeded()
  await page.screenshot({path:out+'/classic-'+viewport.width+'-final.png'})
  await page.getByText('提交',{exact:true}).last().click()
  await page.waitForTimeout(300)
  assert.deepEqual(record.writes.at(-1).auto_groups,['生图分组-image2','codex-稳定版','codex特惠分组'])
  assert.equal(record.writes.at(-1).cross_group_retry,true)
  record.checks.push('classic_ordered_create')
  assert.deepEqual(record.errors,[]);assert.deepEqual(record.console,[])
  await context.close()
 }
} catch(error) {
 if(currentPage) {await currentPage.screenshot({path:out+'/failure.png'}).catch(()=>{});writeFileSync(out+'/failure-text.txt',await currentPage.locator('body').innerText().catch(()=>''))}
 throw error
} finally {
 writeFileSync(out+'/report.json',JSON.stringify(records,null,2))
 await browser.close()
 await Promise.all(servers.map(s=>new Promise(resolve=>s.close(resolve))))
}
console.log(JSON.stringify(records))
