import { expect, test } from '@playwright/test'
import { contestFixture, fulfillJson, loginAdmin, mockAdminContestList, openAdminContest } from './support'

test('管理员登录后查看比赛列表', async ({ page }) => {
  await loginAdmin(page)
  await mockAdminContestList(page)

  await page.getByRole('navigation', { name: '管理端导航' }).getByRole('link', { name: '比赛管理' }).click()
  await expect(page.getByRole('heading', { level: 1, name: '比赛管理' })).toBeVisible()
  await expect(page.getByRole('link', { name: '春季公开赛' })).toHaveAttribute('href', '/admin/contests/42')
  await expect(page.getByRole('table').getByText('进行中')).toBeVisible()
})

test('报名审核携带 expected_revision 与审核备注', async ({ page }) => {
  await loginAdmin(page)
  await mockAdminContestList(page)
  await page.route('**/api/v1/admin/contests/42', (route) => {
    if (route.request().method() === 'GET') return fulfillJson(route, { contest: contestFixture, problems: [] })
    return route.fallback()
  })
  const registration = {
    registration_id: 9,
    contest_id: 42,
    user_id: 7,
    status: 'pending',
    application_note: '希望参加',
    reviewer_id: 0,
    review_note: '',
    revision: 3,
    created_at: '2026-09-02T00:00:00.000Z',
  }
  await page.route('**/api/v1/admin/contests/42/registrations?*', (route) => fulfillJson(route, { total: 1, registrations: [registration] }))
  let reviewBody: Record<string, unknown> | undefined
  await page.route('**/api/v1/admin/contests/42/registrations/9/review', async (route) => {
    reviewBody = route.request().postDataJSON() as Record<string, unknown>
    await fulfillJson(route, {
      registration: { ...registration, status: 'approved', review_note: '欢迎', reviewer_id: 1, revision: 4 },
      participant: { contest_id: 42, user_id: 7, status: 'normal' },
    })
  })

  await openAdminContest(page)
  await page.getByRole('link', { name: '报名审核' }).click()
  await expect(page.getByRole('heading', { level: 1, name: '报名审核' })).toBeVisible()
  await page.getByRole('button', { name: '通过', exact: true }).click()
  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toBeVisible()
  await dialog.getByLabel('审核备注（选填）').fill('欢迎参加')
  await dialog.getByRole('button', { name: '通过', exact: true }).click()

  await expect.poll(() => reviewBody).toBeTruthy()
  expect(reviewBody).toMatchObject({
    contest_id: 42,
    registration_id: 9,
    decision: 'approve',
    review_note: '欢迎参加',
    expected_revision: 3,
  })
  await expect(page.getByText('报名已通过，参赛者已创建。')).toBeVisible()
})

test('保存题目编排使用 revision 乐观锁', async ({ page }) => {
  await loginAdmin(page)
  await mockAdminContestList(page)
  await page.route('**/api/v1/admin/contests/42', (route) => {
    if (route.request().method() === 'GET') return fulfillJson(route, { contest: contestFixture, problems: [] })
    return route.fallback()
  })
  let putBody: { problems?: { problem_id?: unknown }[]; expected_revision?: unknown } | undefined
  await page.route('**/api/v1/admin/contests/42/problems', async (route) => {
    putBody = route.request().postDataJSON()
    await fulfillJson(route, {
      contest: contestFixture,
      problems: [{ problem_id: 101, label: 'A', display_order: 1, max_score: 100 }],
    })
  })

  await openAdminContest(page)
  await page.getByRole('button', { name: '添加题目' }).click()
  await page.getByLabel('第 1 行题目编号').fill('101')
  await page.getByRole('button', { name: '保存编排' }).click()

  await expect.poll(() => putBody).toBeTruthy()
  expect(putBody?.expected_revision).toBe(7)
  expect(putBody?.problems?.[0]?.problem_id).toBe(101)
  await expect(page.getByText('题目编排已保存。')).toBeVisible()
})

test('版本冲突保留编辑，显式合并新版本时保留其他人的字段', async ({ page }) => {
  await loginAdmin(page)
  await mockAdminContestList(page)
  let current = structuredClone(contestFixture)
  const requests: { expected_revision: number; contest?: { title?: string; description?: string } }[] = []
  await page.route('**/api/v1/admin/contests/42', async (route) => {
    if (route.request().method() === 'GET') return fulfillJson(route, { contest: current, problems: [] })
    const body = route.request().postDataJSON()
    requests.push(body)
    if (requests.length === 1) {
      current = { ...current, revision: 8, editable: { ...current.editable, description: '其他管理员更新的描述' } }
      return route.fulfill({ status: 409, contentType: 'application/json', body: '{"code":"aborted","message":"版本已变化"}' })
    }
    current = { ...current, revision: 9, editable: { ...current.editable, title: body.contest.title } }
    return fulfillJson(route, { contest: current })
  })
  await openAdminContest(page)
  await page.getByLabel(/^比赛标题/).fill('我尚未保存的标题')
  await page.getByRole('button', { name: '保存改动', exact: true }).click()
  await expect(page.getByText('你的编辑内容已保留', { exact: false })).toBeVisible()
  await expect(page.getByLabel(/^比赛标题/)).toHaveValue('我尚未保存的标题')
  expect(requests[0].expected_revision).toBe(7)
  await page.getByRole('button', { name: '保留编辑并采用最新版本' }).click()
  await expect(page.getByLabel(/^比赛描述/)).toHaveValue('其他管理员更新的描述')
  await expect(page.getByLabel(/^比赛标题/)).toHaveValue('我尚未保存的标题')
  await page.getByRole('button', { name: '保存改动', exact: true }).click()
  await expect(page.getByText('已保存。', { exact: true })).toBeVisible()
  expect(requests[1].expected_revision).toBe(8)
  expect(requests[1].contest?.title).toBe('我尚未保存的标题')
  expect(requests[1].contest?.description).toBeUndefined()
})
