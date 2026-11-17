import { expect, test } from '@playwright/test'
import { fulfillJson } from './support'

test('首页轮播展示最近比赛并可跳转', async ({ page }) => {
  await page.route('**/api/v1/problems?*', (route) => fulfillJson(route, { total: 0, problems: [] }))
  await page.route('**/api/v1/contests?*', (route) => fulfillJson(route, {
    total: 2,
    contests: [
      {
        contest_id: 42, title: '春季公开赛', mode: 'icpc', status: 'published',
        start_at: '2026-11-01T02:00:00.000Z', end_at: '2026-11-02T02:00:00.000Z',
        registration_policy: 'open', visibility: 'public', registration_open: true, participant_count: 12,
      },
      {
        contest_id: 43, title: '往届比赛', mode: 'ioi', status: 'finished',
        start_at: '2025-11-01T02:00:00.000Z', end_at: '2025-11-02T02:00:00.000Z',
        registration_policy: 'open', visibility: 'public', registration_open: false, participant_count: 30,
      },
    ],
  }))

  await page.goto('/')
  const carousel = page.locator('.hero-carousel')
  await expect(carousel).toBeVisible()
  await expect(carousel.locator('.carousel-slide')).toHaveCount(1)
  await expect(carousel.getByRole('link', { name: /春季公开赛/ })).toHaveAttribute('href', '/contests/42')
})

test('没有可展示比赛时轮播容器仍存在', async ({ page }) => {
  await page.route('**/api/v1/problems?*', (route) => fulfillJson(route, { total: 0, problems: [] }))
  await page.route('**/api/v1/contests?*', (route) => fulfillJson(route, { total: 0, contests: [] }))

  await page.goto('/')
  const carousel = page.locator('.hero-carousel')
  await expect(carousel).toBeVisible()
  await expect(carousel).toHaveClass(/is-empty/)
  await expect(carousel.locator('.carousel-slide')).toHaveCount(0)
  const box = await carousel.boundingBox()
  expect(box?.height ?? 0).toBeGreaterThan(0)
  expect(await carousel.evaluate((element) => getComputedStyle(element).backgroundColor)).not.toBe('rgba(0, 0, 0, 0)')
})

test('题库页在无后端时展示可恢复错误', async ({ page }) => {
  // 模拟后端不可达，避免依赖本地代理的超时时间
  await page.route('**/api/v1/**', (route) => route.abort('connectionrefused'))
  await page.goto('/problems')
  await expect(page.getByRole('heading', { level: 1, name: '题库' })).toBeVisible()
  const errorBlock = page.locator('.state-block.error')
  await expect(errorBlock).toBeVisible({ timeout: 15_000 })
  await expect(errorBlock.getByRole('button', { name: '重试' })).toBeVisible()
})

test('算法演示可逐步查找，并能展示不存在的目标', async ({ page }) => {
  await page.route('**/api/v1/**', (route) => route.abort('connectionrefused'))
  await page.goto('/')
  const demo = page.getByRole('region', { name: '二分查找演示' })
  for (let step = 0; step < 3; step++) await demo.getByRole('button', { name: '下一步' }).click()
  await expect(demo.getByRole('status')).toContainText('找到 23')
  await expect(demo.getByRole('button', { name: '下一步' })).toBeDisabled()
  await demo.getByLabel('查找目标').selectOption('24')
  for (let step = 0; step < 3; step++) await demo.getByRole('button', { name: '下一步' }).click()
  await expect(demo.getByRole('status')).toContainText('数组中没有 24')
})
