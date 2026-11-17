import { expect, type Page, type Route } from '@playwright/test'

export const adminUser = {
  user_id: 1,
  username: 'root',
  email: 'root@example.com',
  role: 'admin',
  status: 'normal',
  school_id: 0,
}

export const contestFixture = {
  contest_id: 42,
  creator_id: 1,
  editable: {
    title: '春季公开赛',
    description: '欢迎参加 **春季公开赛**。',
    mode: 'icpc',
    start_at: '2026-10-01T02:00:00.000Z',
    end_at: '2026-10-02T02:00:00.000Z',
    registration_start_at: '2026-09-01T02:00:00.000Z',
    registration_end_at: '2026-10-01T01:00:00.000Z',
    registration_policy: 'open',
    visibility: 'public',
    scoreboard_visibility: 'public',
    invite_code: '',
  },
  status: 'running',
  revision: 7,
  created_at: '2026-08-01T00:00:00.000Z',
  updated_at: '2026-10-01T02:00:00.000Z',
  scoreboard_frozen: false,
}

export const contestSummaryFixture = {
  contest_id: 42,
  title: '春季公开赛',
  mode: 'icpc',
  status: 'running',
  start_at: '2026-10-01T02:00:00.000Z',
  end_at: '2026-10-02T02:00:00.000Z',
  registration_policy: 'open',
  visibility: 'public',
  registration_open: true,
  participant_count: 12,
}

export async function fulfillJson(route: Route, body: unknown) {
  await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
}

export async function mockAdminContestList(page: Page) {
  await page.route('**/api/v1/admin/contests?*', (route) => fulfillJson(route, {
    total: 1,
    contests: [contestSummaryFixture],
  }))
}

/** 登录后经列表进入比赛工作台（此测试不设置 Cookie，因此使用 SPA 导航） */
export async function openAdminContest(page: Page) {
  await page.getByRole('navigation', { name: '管理端导航' }).getByRole('link', { name: '比赛管理' }).click()
  await page.getByRole('link', { name: contestSummaryFixture.title }).click()
  await expect(page.getByRole('heading', { level: 1, name: '比赛工作台' })).toBeVisible()
}

export async function loginAdmin(page: Page) {
  await page.route('**/api/v1/admin/auth/login/username', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      tokens: { access_token: 'admin-access', refresh_token: 'admin-refresh', expires_in: 3600 },
      user: adminUser,
    }),
  }))
  await page.goto('/admin/login')
  await page.getByLabel(/^用户名/).fill('root')
  await page.getByLabel(/^密码/).fill('secret')
  await page.getByRole('button', { name: '登录', exact: true }).click()
  await expect(page.getByRole('navigation', { name: '管理端导航' })).toBeVisible()
  await expect.poll(() => page.evaluate(() => localStorage.length + sessionStorage.length)).toBe(0)
}
