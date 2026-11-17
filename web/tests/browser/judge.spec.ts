import { expect, test, type Page } from '@playwright/test'
import { fulfillJson } from './support'

const user = { user_id: 7, username: 'alice', email: 'alice@example.com', role: 'user', status: 'normal', school_id: 0 }
const tokens = { access_token: 'access', refresh_token: 'refresh', expires_in: 3600 }
const problem = {
  problem_id: 42, revision: 1,
  statement: { title: 'A + B', description: '求和：$a+b$。\n\n| 输入 | 输出 |\n|---|---|\n| 1 2 | 3 |', input_description: '两个整数', output_description: '和', hint: '', samples: [{ input: '1 2', output: '3' }] },
  metadata: { difficulty: 'easy', tags: ['基础'], source: '' },
  judge_settings: { time_limit_ms: 1000, memory_limit_mb: 128, allowed_languages: ['cpp20-v3', 'go-v3'], checker_type: 'standard' },
}
const summary = { submission_id: 99, problem_id: 42, user_id: 7, language: 'cpp20-v3', status: 'queued', verdict: 'unspecified', result_visibility: 'full', submitted_at: '2026-11-01T00:00:00Z' }
async function session(page: Page, role = 'user') {
  await page.route('**/api/v1/**', (route) => route.fulfill({ status: 401, contentType: 'application/json', body: '{"code":"unauthenticated","message":"no session"}' }))
  await page.route('**/auth/token/refresh', (route) => fulfillJson(route, { tokens }))
  await page.route('**/users/me', (route) => fulfillJson(route, { user: { ...user, role } }))
  await page.route('**/api/v1/problems/42', (route) => fulfillJson(route, { problem }))
  await page.route('**/api/v1/problems?*', (route) => fulfillJson(route, { total: 1, problems: [{ problem_id: 42, title: 'A + B', difficulty: 'easy', tags: ['基础'] }] }))
}

test('提交使用精确数字编号，未知结果的相同请求复用幂等键，并停止终态轮询', async ({ page }) => {
  await session(page)
  const writes: string[] = []
  let reads = 0
  await page.route('**/api/v1/submissions', async (route) => {
    writes.push(route.request().postData()!)
    if (writes.length === 1) return route.fulfill({ status: 503, contentType: 'application/json', body: '{"code":"unavailable","message":"暂时不可用"}' })
    if (writes.length === 2) return fulfillJson(route, { submission: { summary: { ...summary, submission_id: 0 } } })
    return fulfillJson(route, { submission: { summary } })
  })
  await page.route('**/api/v1/submissions/99', (route) => {
    reads++
    return fulfillJson(route, { submission: { summary: { ...summary, status: reads > 1 ? 'finished' : 'judging', verdict: reads > 1 ? 'accepted' : 'unspecified' }, case_results: [], source_code: 'int main(){}' } })
  })
  await page.goto('/problems/42/submit')
  const editor = page.getByRole('textbox', { name: '源代码编辑器' })
  await editor.fill('int main(){}')
  await editor.press('Tab')
  await expect(editor).not.toBeFocused()
  await page.getByRole('button', { name: '提交评测', exact: true }).click()
  await expect(page.getByText('上一次提交结果未知', { exact: false })).toBeVisible()
  await page.getByRole('button', { name: '结果未知 · 核对后可重试' }).click()
  await expect(page.getByText('提交编号响应无效，请先重新查询服务端结果', { exact: false })).toBeVisible()
  await page.getByRole('button', { name: '结果未知 · 核对后可重试' }).click()
  await expect(page).toHaveURL(/\/submissions\/99$/)
  expect(writes).toHaveLength(3)
  expect(writes[0]).toBe(writes[1])
  expect(writes[1]).toBe(writes[2])
  expect(JSON.parse(writes[0]).problem_id).toBe(42)
  await expect(page.getByText('Accepted', { exact: true }).first()).toBeVisible({ timeout: 8000 })
  const finalReads = reads
  await page.waitForTimeout(3000)
  expect(reads).toBe(finalReads)
})

test('不复用跨语言的提交幂等键，切换题目与退出后不泄漏源码草稿', async ({ page }) => {
  await session(page)
  const requests: { language: string; client_request_id: string }[] = []
  await page.route('**/api/v1/submissions', (route) => {
    requests.push(route.request().postDataJSON())
    return route.fulfill({ status: 503, contentType: 'application/json', body: '{"code":"unavailable","message":"暂时不可用"}' })
  })
  await page.goto('/problems/42/submit')
  await page.getByRole('textbox', { name: '源代码编辑器' }).fill('private draft')
  await page.getByRole('button', { name: '提交评测', exact: true }).click()
  await page.getByLabel('语言', { exact: true }).selectOption('go-v3')
  await page.getByRole('button', { name: '结果未知 · 核对后可重试' }).click()
  await expect.poll(() => requests.length).toBe(2)
  expect(requests[1].client_request_id).not.toBe(requests[0].client_request_id)
  await page.getByRole('button', { name: '退出', exact: true }).click()
  await page.getByRole('link', { name: '登录 / 注册' }).click()
  await page.route('**/api/v1/auth/login/username', (route) => fulfillJson(route, { tokens, user: { ...user, user_id: 8, username: 'bob' } }))
  await page.getByLabel('用户名 *').fill('bob')
  await page.getByLabel('密码 *').fill('secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await page.getByRole('link', { name: '提交代码', exact: true }).click()
  await page.getByLabel('选择题目').selectOption('42')
  await expect(page.getByRole('textbox', { name: '源代码编辑器' })).toHaveValue('')
})

for (const width of [320, 390, 640, 768, 1024, 1440]) {
  test(`有数据的题面、提交与评测在 ${width}px 不溢出`, async ({ page }) => {
    await session(page)
    await page.setViewportSize({ width, height: 900 })
    await page.route('**/api/v1/submissions?*', (route) => fulfillJson(route, { total: 1, submissions: [{ ...summary, status: 'finished', verdict: 'accepted' }] }))
    for (const path of ['/problems', '/problems/42', '/problems/42/submit', '/submissions']) {
      await page.goto(path)
      await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible()
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), { message: `${path} @${width}` }).toBe(true)
    }
  })
}
