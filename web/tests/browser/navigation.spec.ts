import { expect, test } from '@playwright/test'

test('public navigation and unauthenticated management guard', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1 })).toContainText('让思考')
  await page.goto('/admin/account')
  await expect(page).toHaveURL(/\/admin\/login\?returnTo=/)
  await expect(page.getByRole('heading', { name: '登录管理工作台' })).toBeVisible()
  await page.reload()
  await expect(page.getByLabel('用户名')).toBeVisible()
  await expect(page.getByLabel('密码', { exact: true })).toHaveAttribute('type', 'password')
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length }))).toEqual({ local: 0, session: 0 })
})

test('mobile layout and keyboard skip navigation', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: '跳到主要内容' })).toBeFocused()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})
