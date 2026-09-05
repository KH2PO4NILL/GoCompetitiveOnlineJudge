import { expect, test } from '@playwright/test'

test('public navigation and unauthenticated management guard', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('一站式在线评测服务')
  if (process.env.GCOJ_PUBLIC_SCREENSHOT) await page.screenshot({ path: process.env.GCOJ_PUBLIC_SCREENSHOT, fullPage: true })
  await page.goto('/admin/account')
  await expect(page).toHaveURL(/\/admin\/login\?returnTo=/)
  await expect(page.getByRole('heading', { name: '登录管理工作台' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('textbox', { name: '用户名', exact: true })).toBeVisible()
  await expect(page.getByLabel('密码', { exact: true })).toHaveAttribute('type', 'password')
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length }))).toEqual({ local: 0, session: 0 })
})

test('mobile layout and keyboard skip navigation', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: '跳到主要内容' })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('main')).toBeFocused()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
})

test('service directory exposes only available services and guide navigation works', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('a.service-card')).toHaveCount(2)
  await expect(page.locator('article.service-card')).toHaveCount(4)
  await expect(page.locator('article.service-card').getByRole('link')).toHaveCount(0)
  await expect(page.getByText('本平台为在线评测项目，非政府机构网站。')).toBeVisible()
  await page.getByRole('link', { name: /个人信息服务/ }).click()
  await expect(page).toHaveURL(/\/login\?returnTo=%2Faccount/)
  await page.getByRole('navigation', { name: '主要导航' }).getByRole('link', { name: '办事指南' }).click()
  await expect(page).toHaveURL(/\/#guide$/)
  await expect(page.getByRole('heading', { name: '办事指南' })).toBeInViewport()
})

for (const width of [320, 768, 1440]) {
  test(`service portal and authentication layouts fit ${width}px viewport`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    for (const path of ['/', '/login', '/admin/login']) {
      await page.goto(path)
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
    }
  })
}

test('authentication modes retain their fields in the service layout', async ({ page }) => {
  await page.goto('/login')
  const method = page.getByRole('combobox', { name: '认证方式', exact: true })
  await method.selectOption('register')
  await expect(page.getByRole('button', { name: '创建账号' })).toBeVisible()
  await expect(page.getByLabel('邮箱验证码', { exact: true })).toBeVisible()
  await method.selectOption('email-code')
  await expect(page.getByLabel('密码', { exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: '发送验证码' })).toBeVisible()
  await method.selectOption('reset')
  await expect(page.getByLabel('新密码', { exact: true })).toBeVisible()
  await page.getByRole('link', { name: '管理员入口' }).click()
  await expect(page.getByRole('combobox', { name: '认证方式', exact: true }).locator('option[value="register"]')).toHaveCount(0)
})

test('opening a service from a scrolled mobile homepage starts at the top', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/')
  await page.getByRole('link', { name: /个人信息服务/ }).click()
  await expect(page).toHaveURL(/\/login\?returnTo=%2Faccount/)
  await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0)
})

test('switching authentication scope clears entered values and registration state', async ({ page }) => {
  await page.goto('/login')
  await page.getByRole('combobox', { name: '认证方式', exact: true }).selectOption('register')
  await page.getByLabel('用户名', { exact: true }).fill('scope-review-placeholder')
  await page.getByLabel('密码', { exact: true }).fill('not-a-real-password')
  await page.getByLabel('邮箱', { exact: true }).fill('scope-review@example.invalid')
  await page.getByRole('link', { name: '管理员入口' }).click()
  await expect(page.getByRole('heading', { name: '登录管理工作台' })).toBeVisible()
  await expect(page.getByRole('combobox', { name: '认证方式', exact: true })).toHaveValue('username')
  await expect(page.getByRole('button', { name: '创建账号' })).toHaveCount(0)
  await expect(page.getByLabel('用户名', { exact: true })).toHaveValue('')
  await expect(page.getByLabel('密码', { exact: true })).toHaveValue('')
  await page.getByRole('link', { name: '返回用户登录' }).click()
  await expect(page.getByRole('combobox', { name: '认证方式', exact: true })).toHaveValue('username')
  await page.getByRole('combobox', { name: '认证方式', exact: true }).selectOption('email-password')
  await expect(page.getByLabel('邮箱', { exact: true })).toHaveValue('')
})
