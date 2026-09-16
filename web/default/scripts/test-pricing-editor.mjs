import assert from 'node:assert/strict'
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

// Run against a served build with a pricing-options-before.json fixture in
// PRICING_TEST_OUTPUT. All administrative API reads/writes are intercepted.
// PLAYWRIGHT_MODULE may point to an existing external Playwright installation.
const root = process.env.PRICING_TEST_OUTPUT || dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const phase = process.argv[2] || 'baseline'
const origin = process.env.PRICING_BROWSER_URL || 'http://127.0.0.1:18300'
const originalOptions = JSON.parse(readFileSync(resolve(root, 'pricing-options-before.json'), 'utf8'))
const user = { id: 1, username: 'pricing-ui-test', display_name: 'Pricing UI Test', role: 100, status: 1, group: 'default', quota: 1000000, setting: '{}' }
const publicStatusPath = resolve(root, 'public-status.json')
const publicStatus = existsSync(publicStatusPath) ? JSON.parse(readFileSync(publicStatusPath, 'utf8')) : await fetch(origin + '/api/status').then(r => r.json())
writeFileSync(publicStatusPath, JSON.stringify(publicStatus))
const browser = await chromium.launch({ headless: true, channel: process.env.PRICING_BROWSER_CHANNEL })
const results = []
try {
  for (const viewport of [{ width: 1680, height: 1000 }, { width: 390, height: 844 }]) {
    let options = structuredClone(originalOptions)
    const writes = [], pageErrors = [], consoleErrors = [], failedRequests = []
    const context = await browser.newContext({ viewport })
    await context.addInitScript(user => {
      localStorage.setItem('user', JSON.stringify(user))
      localStorage.setItem('i18nextLng', 'en')
      localStorage.setItem('theme', 'light')
    }, user)
    await context.route('**/api/**', async route => {
      const request = route.request(), path = new URL(request.url()).pathname
      if (path === '/api/status') return route.fulfill({json:publicStatus})
      let data = {}
      if (path === '/api/notice') data = ''
      if (path === '/api/setup') data = { status: true, root_init: true, database_type: 'postgres' }
      if (path === '/api/user/self') data = user
      if (path === '/api/option/' || path === '/api/option') {
        if (request.method() === 'GET') data = options
        else {
          const payload = request.postDataJSON()
          writes.push(payload)
          const row = options.find(row => row.key === payload.key)
          if (row) row.value = payload.value
          else options.push(payload)
        }
      }
      await route.fulfill({ json: { success: true, message: '', data } })
    })
    const page = await context.newPage()
    page.on('pageerror', error => { pageErrors.push(error.message); console.log('PAGE_ERROR', error.message) })
    page.on('console', message => { if (message.type() === 'error') { consoleErrors.push(message.text()); console.log('CONSOLE_ERROR', message.text()) } })
    page.on('requestfailed', request => failedRequests.push({url: request.url(), error: request.failure()?.errorText}))
    const started = Date.now()
    const response = await page.goto(origin + '/system-settings/billing/model-pricing', {waitUntil: 'domcontentloaded', timeout: 60000})
    await page.screenshot({path: resolve(root, `${phase}-${viewport.width}-immediate.png`)})
    await page.getByRole('button', {name: /添加模型|Add model/}).first().waitFor({ timeout: 15000 }).catch(async error => {
      writeFileSync(resolve(root, `${phase}-${viewport.width}-initial-text.txt`), await page.locator('body').innerText())
      throw error
    })
    writeFileSync(resolve(root, `${phase}-${viewport.width}-initial-text.txt`), await page.locator('body').innerText())
    const inputs = await page.locator('input').evaluateAll(nodes => nodes.map(n => ({placeholder:n.placeholder,value:n.value})))
    assert(inputs.length > 0)
    const search = page.locator('input').first()
    await search.fill('deepseek-flash')
    const row = page.locator('tr').filter({has: page.getByText('deepseek-flash', {exact:true})})
    await row.getByRole('button').first().click()
    await page.waitForTimeout(700)
    const expr = page.locator('textarea').last()
    const value = await expr.inputValue()
    const expected = JSON.parse(originalOptions.find(row => row.key === 'billing_setting.billing_expr').value)['deepseek-flash']
    await page.screenshot({path: resolve(root, `${phase}-${viewport.width}-final.png`)})
    const result = {phase,viewport,httpStatus:response.status(),renderMs:Date.now()-started,expected,actual:value,preserved:value===expected,writes:writes.length,pageErrors,consoleErrors,failedRequests,checks:[]}
    results.push(result)
    console.log(JSON.stringify(result))
    if (phase === 'baseline') assert.equal(value, 'p * 0 + c * 0')
    else assert.equal(value, expected)
    assert.equal(writes.length, 0)
    if (phase !== 'baseline') {
      const save = () => page.getByRole('button', {name:/^保存模型价格$|^Save model prices$/}).last().click()
      await save()
      await page.waitForTimeout(1200)
      for (const option of originalOptions) {
        const saved = options.find(row => row.key === option.key)
        assert.deepEqual(JSON.parse(saved.value), JSON.parse(option.value), `untouched save: ${option.key}`)
      }
      result.checks.push('untouched_save_preserves_all_prices')
      result.mockedRequests = writes
      const mode = page.getByRole('combobox').filter({hasText:/表达式编辑器|Expression editor/})
      await mode.click()
      assert.equal(await page.getByRole('option', {name:/^可视化编辑器$|^Visual editor$/}).isEnabled(), false)
      await page.keyboard.press('Escape')
      result.checks.push('unsupported_visual_mode_disabled')
      if (viewport.width < 768) await page.keyboard.press('Escape')
      await search.fill('deepseek')
      const expressions = JSON.parse(originalOptions.find(row => row.key === 'billing_setting.billing_expr').value)
      for (const model of Object.keys(expressions).filter(name => name.includes('deepseek'))) {
        const modelRow = page.locator('tr').filter({has:page.getByText(model,{exact:true})})
        await modelRow.getByRole('button').first().click()
        await page.waitForTimeout(100)
        assert.equal(await page.locator('textarea').last().inputValue(), expressions[model], `model switch: ${model}`)
        if (viewport.width < 768) await page.keyboard.press('Escape')
      }
      result.checks.push('all_eight_deepseek_models_preserved_on_switch')
      await page.locator('tr').filter({has:page.getByText('deepseek-flash',{exact:true})}).getByRole('button').first().click()
      const edited = expected.replace('p * 2 +', 'p * 2.25 +')
      await page.locator('textarea').last().fill(edited)
      await save()
      await page.waitForTimeout(700)
      const stored = JSON.parse(options.find(row => row.key === 'billing_setting.billing_expr').value)
      assert.equal(stored['deepseek-flash'], edited)
      for (const [model, expression] of Object.entries(expressions)) {
        if (model !== 'deepseek-flash') assert.equal(stored[model], expression, `unrelated model: ${model}`)
      }
      result.checks.push('raw_edit_save_updates_only_selected_expression')
      result.mockedWritesAfterEditing = writes.length
      await page.getByRole('button', {name:'Flat',exact:true}).click()
      const visualMode = page.getByRole('combobox').filter({hasText:/可视化编辑器|Visual editor/})
      await visualMode.click()
      await page.getByRole('option', {name:/^表达式编辑器$|^Expression editor$/}).click()
      const presetExpr = await page.locator('textarea').last().inputValue()
      assert(presetExpr.includes('tier(') && presetExpr !== edited)
      await page.getByRole('combobox').filter({hasText:/表达式编辑器|Expression editor/}).click()
      await page.getByRole('option', {name:/^可视化编辑器$|^Visual editor$/}).click()
      await page.getByRole('combobox').filter({hasText:/可视化编辑器|Visual editor/}).click()
      await page.getByRole('option', {name:/^表达式编辑器$|^Expression editor$/}).click()
      assert.equal(await page.locator('textarea').last().inputValue(),presetExpr)
      await save()
      await page.waitForTimeout(700)
      assert.equal(JSON.parse(options.find(row=>row.key==='billing_setting.billing_expr').value)['deepseek-flash'],presetExpr)
      result.checks.push('preset_and_visual_raw_roundtrip_save')
      console.log(JSON.stringify({phase,viewport,checks:result.checks,success:true}))
    }
    assert.deepEqual(pageErrors, [])
    assert.deepEqual(consoleErrors, [])
    await context.close()
  }
} finally {
  writeFileSync(resolve(root, `${phase}-browser.json`), JSON.stringify(results,null,2)+'\n')
  await browser.close()
}
