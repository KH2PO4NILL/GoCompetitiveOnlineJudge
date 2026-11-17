import { expect, test } from '@playwright/test'
import { fulfillJson } from './support'

const appContest = {
  contest_id: 42,
  title: '春季公开赛',
  description: '欢迎参加 **春季公开赛**。',
  mode: 'icpc',
  status: 'running',
  start_at: '2026-10-01T02:00:00.000Z',
  end_at: '2026-10-02T02:00:00.000Z',
  registration_start_at: '2026-09-01T02:00:00.000Z',
  registration_end_at: '2026-10-01T01:00:00.000Z',
  registration_policy: 'open',
  visibility: 'public',
  scoreboard_visibility: 'public',
  registration_open: true,
  scoreboard_frozen: false,
  participant_count: 3,
}

test('比赛列表保留 int64 编号并进入概览', async ({ page }) => {
  const bigId = '9007199254740993'
  await page.route('**/api/v1/contests?*', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: `{"total":1,"contests":[{"contest_id":${bigId},"title":"64 位编号比赛","mode":"ioi","status":"published","registration_policy":"open","visibility":"public","registration_open":true,"participant_count":12}]}`,
  }))
  await page.route(`**/api/v1/contests/${bigId}`, (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: `{"contest":{"contest_id":${bigId},"title":"64 位编号比赛","description":"","mode":"ioi","status":"published","registration_policy":"open","visibility":"public","scoreboard_visibility":"public","registration_open":true,"scoreboard_frozen":false,"participant_count":12}}`,
  }))

  await page.goto('/contests')
  await expect(page.getByRole('heading', { level: 1, name: '比赛' })).toBeVisible()
  const link = page.getByRole('link', { name: '64 位编号比赛' })
  await expect(link).toHaveAttribute('href', `/contests/${bigId}`)
  await expect(page.getByText(bigId)).toBeVisible()

  await link.click()
  await expect(page.getByRole('heading', { level: 1, name: '64 位编号比赛' })).toBeVisible()
})

test('ICPC 排行榜渲染罚时与封榜提示', async ({ page }) => {
  await page.route('**/api/v1/contests/42', (route) => fulfillJson(route, { contest: appContest }))
  await page.route('**/api/v1/contests/42/scoreboard?*', (route) => fulfillJson(route, {
    scoreboard: {
      contest_id: 42,
      mode: 'icpc',
      generated_at: '2026-10-01T03:00:00.000Z',
      frozen: true,
      view: 'frozen',
      total: 1,
      rows: [{
        rank: 1,
        ordinal: 1,
        user_id: 7,
        display_name: 'alice',
        problem_results: [{
          problem_id: 101,
          label: 'A',
          icpc: { solved: true, wrong_attempts: 1, penalty_seconds: 3723, first_to_solve: false },
        }],
        icpc: { solved_count: 1, total_penalty_seconds: 3723 },
      }],
    },
  }))

  await page.goto('/contests/42/scoreboard')
  await expect(page.getByRole('heading', { level: 1, name: '排行榜' })).toBeVisible()
  await expect(page.getByText('榜单已封榜')).toBeVisible()
  await expect(page.getByRole('cell', { name: 'alice' })).toBeVisible()
  await expect(page.getByRole('cell', { name: '-1 ✓ 1:02:03' })).toBeVisible()
  await expect(page.getByRole('cell', { name: '1:02:03', exact: true })).toBeVisible()
})

test('未登录概览提示登录后报名', async ({ page }) => {
  await page.route('**/api/v1/contests/42', (route) => fulfillJson(route, { contest: appContest }))
  await page.goto('/contests/42')
  await expect(page.getByRole('heading', { level: 1, name: '春季公开赛' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '比赛公告' })).toBeVisible()
  await expect(page.getByText('欢迎参加')).toBeVisible()
  await expect(page.getByRole('link', { name: '登录后报名' })).toHaveAttribute('href', /\/login\?returnTo=/)
})
