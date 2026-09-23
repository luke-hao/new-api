import assert from 'node:assert/strict'
import { readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const root = process.env.IMAGE_PRICING_TEST_OUTPUT
assert(root, 'IMAGE_PRICING_TEST_OUTPUT is required')
const origin = process.env.IMAGE_PRICING_TEST_URL || 'http://127.0.0.1:18321'
const original = JSON.parse(readFileSync(`${root}/options.json`, 'utf8'))
const models = JSON.parse(readFileSync(`${root}/groups.json`, 'utf8'))
const status = JSON.parse(readFileSync(`${root}/public-status.json`, 'utf8'))
const user = {id:1,username:'image-pricing-test',display_name:'Image Pricing Test',role:100,status:1,group:'default',quota:1000000,setting:'{}'}
const browser = await chromium.launch({headless:true})
const records=[]
let currentPage
try {
 for (const viewport of [{width:1680,height:1000},{width:390,height:844}]) {
  let options=structuredClone(original)
  const record={viewport,writes:[],pageErrors:[],consoleErrors:[],failedRequests:[],checks:[]};records.push(record)
  const context=await browser.newContext({viewport})
  await context.addInitScript(user=>{localStorage.setItem('user',JSON.stringify(user));localStorage.setItem('i18nextLng','zh');localStorage.setItem('theme','light')},user)
  await context.route('**/api/**',async route=>{
   const request=route.request(),path=new URL(request.url()).pathname
   if(path==='/api/status')return route.fulfill({json:status})
   let data={}
   if(path==='/api/notice')data=''
   if(path==='/api/setup')data={status:true,root_init:true,database_type:'postgres'}
   if(path==='/api/user/self')data=user
   if(path==='/api/group/image-models')data=models
   if(path==='/api/group/user')data=[]
   if(path==='/api/option/'||path==='/api/option') {
    if(request.method()==='GET')data=options
    else {const payload=request.postDataJSON();record.writes.push({path,payload});const row=options.find(item=>item.key===payload.key);if(row)row.value=payload.value;else options.push(payload)}
   }
   if(path==='/api/group/settings') {
    const payload=request.postDataJSON();record.writes.push({path,payload})
    for(const [key,value] of Object.entries(payload)){const row=options.find(item=>item.key===key);if(row)row.value=String(value);else options.push({key,value:String(value)})}
   }
   await route.fulfill({json:{success:true,message:'',data}})
  })
  const page=await context.newPage();currentPage=page
  page.on('pageerror',e=>record.pageErrors.push(e.message))
  page.on('console',m=>{if(m.type()==='error')record.consoleErrors.push(m.text())})
  page.on('requestfailed',r=>record.failedRequests.push({url:r.url(),error:r.failure()?.errorText}))
  const start=Date.now()
  const response=await page.goto(origin+'/system-settings/billing/group-pricing',{waitUntil:'domcontentloaded'})
  record.httpStatus=response.status()
  await page.screenshot({path:`${root}/${viewport.width}-immediate.png`})
  const card=page.getByTestId('image-token-prices')
  await card.waitFor({timeout:20000});record.renderMs=Date.now()-start
  await card.scrollIntoViewIfNeeded()
  await page.locator('#image-token-input').fill('5.1234')
  assert.equal(await page.locator('#image-token-image_input').inputValue(),'8')
  assert.equal(await page.locator('#image-token-image_output').inputValue(),'30')
  await page.getByRole('button',{name:/保存分组设置|保存分组比率|Save group settings/}).click()
  await page.waitForTimeout(600)
  const saved=record.writes.find(item=>item.path==='/api/group/settings')?.payload
  assert(saved,'group settings saved')
  assert.equal(JSON.parse(saved.ImageTokenGroupPrices)['OpenAI官key']['gpt-image-2'].input,5.1234)
  assert.deepEqual(JSON.parse(saved.ImageSizeGroupPrices),JSON.parse(original.find(item=>item.key==='ImageSizeGroupPrices').value))
  record.checks.push('group_token_price_saved_without_changing_fixed_prices')
  await card.getByRole('combobox',{name:/生图 Token 计费分组|Image token billing group/}).click()
  await page.getByRole('option',{name:'生图分组-nanobanana',exact:true}).click()
  await card.getByRole('combobox',{name:/生图 Token 模型|Image token model/}).click()
  for(const model of models['生图分组-nanobanana'])await page.getByRole('option',{name:model,exact:true}).waitFor()
  await page.getByRole('option',{name:'nano-banana-pro',exact:true}).click()
  await page.getByRole('option',{name:'nano-banana-pro',exact:true}).waitFor({state:'hidden'})
  record.checks.push('all_seven_nanobanana_models_available')
  const fixed=page.getByTestId('image-fixed-prices')
  await fixed.getByRole('button',{name:/^添加$|^Add$/}).click()
  const row=fixed.locator('[data-price-row]').last()
  await row.getByRole('combobox').nth(1).click()
  const groupOptions=await page.getByRole('option').allTextContents()
  assert(groupOptions.every(group=>group.trim().startsWith('生图分组-')), JSON.stringify(groupOptions))
  await page.getByRole('option',{name:'生图分组-nanobanana',exact:true}).click()
  await row.getByRole('combobox').nth(2).click()
  const nano=page.getByRole('option',{name:'nano-banana-pro',exact:true})
  await nano.click()
  await row.getByRole('textbox',{name:/1K/}).fill('0.08')
  await row.getByRole('textbox',{name:/2K/}).fill('0.15')
  await row.getByRole('textbox',{name:/4K/}).fill('0.3')
  await page.getByRole('button',{name:/保存分组设置|保存分组比率|Save group settings/}).click()
  await page.waitForTimeout(600)
  const second=record.writes.filter(item=>item.path==='/api/group/settings').at(-1).payload
  const fixedMap=JSON.parse(second.ImageSizeGroupPrices)
  assert(Object.values(fixedMap).some(groups=>groups['生图分组-nanobanana']?.['nano-banana-pro']?.['4K']===0.3))
  record.checks.push('nanobanana_size_prices_saved_with_group_model_scope')
  await card.scrollIntoViewIfNeeded()
  await page.screenshot({path:`${root}/${viewport.width}-group-final.png`})
  await page.goto(origin+'/system-settings/billing/model-pricing',{waitUntil:'domcontentloaded'})
  await page.getByRole('button',{name:/添加模型|Add model/}).first().waitFor()
  await page.locator('input').first().fill('gpt-image-2')
  const modelRow=page.locator('tr').filter({has:page.getByText('gpt-image-2',{exact:true})})
  await modelRow.getByText(/分组定价|Group pricing/).waitFor()
  assert.equal(await modelRow.getByText(/矛盾|Conflict/).count(),0)
  await modelRow.getByRole('button').first().click()
  await page.getByText(/这里设置全局兜底价格|This is the global fallback price/).last().waitFor()
  await page.screenshot({path:`${root}/${viewport.width}-model-final.png`})
  await page.getByRole('button',{name:/^保存模型价格$|^Save model prices$/}).last().click()
  await page.waitForTimeout(600)
  assert.deepEqual(JSON.parse(options.find(item=>item.key==='ImageTokenGroupPrices').value),JSON.parse(second.ImageTokenGroupPrices))
  record.checks.push('no_false_conflict_and_global_save_preserves_group_rates')
  assert.equal(record.httpStatus,200)
  assert.deepEqual(record.pageErrors,[]);assert.deepEqual(record.consoleErrors,[]);assert.deepEqual(record.failedRequests,[])
  await context.close()
 }
} catch(error) {
 if(currentPage && !currentPage.isClosed()){await currentPage.screenshot({path:`${root}/failure.png`});writeFileSync(`${root}/failure.txt`,await currentPage.locator("body").innerText())}
 throw error
} finally {
 writeFileSync(`${root}/browser-record.json`,JSON.stringify(records,null,2))
 await browser.close()
}
console.log(JSON.stringify(records.map(({writes,...record})=>({...record,writes:writes.length})),null,2))
